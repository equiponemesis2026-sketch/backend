package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	alertHttp "github.com/nemesis-project/api-nemesis/internal/alert/delivery/http"
	wsDelivery "github.com/nemesis-project/api-nemesis/internal/alert/delivery/ws"
	alertMongo "github.com/nemesis-project/api-nemesis/internal/alert/repository/mongo"
	wsHub "github.com/nemesis-project/api-nemesis/internal/alert/services/ws"
	alertUsecase "github.com/nemesis-project/api-nemesis/internal/alert/usecase"
	contactHttp "github.com/nemesis-project/api-nemesis/internal/contact/delivery/http"
	contactMongo "github.com/nemesis-project/api-nemesis/internal/contact/repository/mongo"
	contactUsecase "github.com/nemesis-project/api-nemesis/internal/contact/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/config"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/database"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	notifService "github.com/nemesis-project/api-nemesis/internal/notifications/service"
	subscriptionHttp "github.com/nemesis-project/api-nemesis/internal/subscription/delivery/http"
	subscriptionMongo "github.com/nemesis-project/api-nemesis/internal/subscription/repository/mongo"
	subscriptionUsecase "github.com/nemesis-project/api-nemesis/internal/subscription/usecase"
	tokenHttp "github.com/nemesis-project/api-nemesis/internal/token/delivery/http"
	deviceDomain "github.com/nemesis-project/api-nemesis/internal/token/domain"
	tokenRepo "github.com/nemesis-project/api-nemesis/internal/token/repository"
	tokenUseCase "github.com/nemesis-project/api-nemesis/internal/token/usecase"
	userHttp "github.com/nemesis-project/api-nemesis/internal/user/delivery/http"
	userMongo "github.com/nemesis-project/api-nemesis/internal/user/repository/mongo"
	"github.com/nemesis-project/api-nemesis/internal/user/usecase"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		slog.Error("failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to MongoDB", "db", cfg.DBName)

	db := client.Database(cfg.DBName)

	// --- Módulo 1: Usuarios y Auth ---
	userRepoImpl := userMongo.NewUserRepository(db)
	userUseCase := usecase.NewUserUseCase(userRepoImpl, cfg.JWTSecret, cfg.JWTExpiry)
	userHandler := userHttp.NewUserHandler(userUseCase)

	// --- Módulo 2: Contactos (Red de Apoyo) ---
	contactRepoImpl := contactMongo.NewContactRepository(db)
	contactUseCase := contactUsecase.NewContactUseCase(contactRepoImpl, userRepoImpl)
	contactHandler := contactHttp.NewContactHandler(contactUseCase)

	// Backfill: al registrarse un usuario, se vinculan contactos pendientes
	if linker, ok := userUseCase.(interface{ SetContactLinker(usecase.ContactLinker) }); ok {
		linker.SetContactLinker(contactRepoImpl)
	}

	// --- Módulo 3: Suscripciones y Facturación (Stripe) ---
	subscriptionRepoImpl := subscriptionMongo.NewSubscriptionRepository(db)
	billingUC := subscriptionUsecase.NewBillingUseCase(
		subscriptionRepoImpl,
		cfg.StripeKey,
		cfg.StripeWebhookSecret,
		cfg.StripePricePro,
		cfg.StripePriceFamiliar,
	)
	billingHandler := subscriptionHttp.NewBillingHandler(billingUC)

	// --- Módulo 4: Tokens de Vinculación (WearOS/NMS) ---
	tokenRepoImpl := tokenRepo.NewDeviceRepository(db)
	tokenUc := tokenUseCase.NewTokenUseCase(tokenRepoImpl)
	tokenHandler := tokenHttp.NewTokenHandler(tokenUc)

	// --- Módulo 5: Motor de Alertas SOS + Notificaciones FCM + Telemetría WS ---
	notifier, err := notifService.NewService(ctx, cfg.FirebaseServiceAccount, tokenRepoImpl)
	if err != nil {
		slog.Error("failed to initialize FCM service", "error", err)
		os.Exit(1)
	}

	// Notificar al usuario vinculado cuando alguien lo agrega como contacto
	// de confianza, para que acepte (o rechace) la solicitud de observación.
	if supportNotifier, ok := contactUseCase.(interface {
		SetSupportNotifier(notifDomain.PushNotifier, deviceDomain.DeviceRepository)
	}); ok {
		supportNotifier.SetSupportNotifier(notifier, tokenRepoImpl)
	}

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	telemetryHub := wsHub.NewHub(hubCtx)

	alertRepoImpl := alertMongo.NewAlertRepository(db)
	alertUC := alertUsecase.NewAlertUseCase(
		alertRepoImpl,
		contactRepoImpl,
		tokenRepoImpl,
		notifier,
		userUseCase, // PinVerifier
		telemetryHub,
	)
	alertHandler := alertHttp.NewAlertHandler(alertUC)
	wsHandler := wsDelivery.NewHandler(wsDelivery.Deps{
		Hub:         telemetryHub,
		AlertRepo:   alertRepoImpl,
		ContactRepo: contactRepoImpl,
		AuthSecret:  []byte(cfg.JWTSecret),
	})

	// Middleware de autenticación JWT
	authMiddleware := middleware.JWTAuth(cfg.JWTSecret)

	r := chi.NewRouter()

	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, pingCancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer pingCancel()

		dbStatus := "up"
		if err := client.Ping(pingCtx, nil); err != nil {
			dbStatus = "down"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"version": "0.1.0",
			"mongodb": dbStatus,
		})
	})

	// Rutas del módulo de autenticación (públicas, con rate limit)
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(middleware.RateLimit(10, time.Minute))
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)
	})

	// Rutas del módulo de contactos (protegidas con JWT)
	r.Route("/api/v1/contacts", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", contactHandler.GetAll)
		r.Post("/", contactHandler.Create)
		r.Get("/pending", contactHandler.GetPending)
		r.Post("/{id}/accept", contactHandler.AcceptLink)
		r.Post("/{id}/reject", contactHandler.RejectLink)
		r.Put("/{id}", contactHandler.Update)
		r.Delete("/{id}", contactHandler.Delete)
	})

	// Rutas del módulo de suscripciones y facturación
	r.Route("/api/v1/billing", func(r chi.Router) {
		r.Post("/webhook", billingHandler.HandleWebhook)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Post("/checkout", billingHandler.CreateCheckout)
			r.Get("/subscription", billingHandler.GetSubscription)
		})
	})

	// Rutas del módulo de vinculación de dispositivos
	r.Route("/api/v1/devices", func(r chi.Router) {
		r.With(authMiddleware).Post("/tokens/generate", tokenHandler.GenerateCode)
		r.With(authMiddleware).Post("/pair", tokenHandler.PairDevice)
		r.With(authMiddleware).Post("/fcm-token", tokenHandler.RegisterFCMToken)
	})

	// Rutas del motor de alertas SOS (protegidas con JWT)
	r.Route("/api/v1/alerts", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Post("/sos", alertHandler.CreateSOS)
		r.Post("/coercion", alertHandler.CreateCoercion)
		r.Put("/{id}/resolved", alertHandler.Resolve)
		r.Get("/observing", alertHandler.GetObserving)
		r.Get("/{id}", alertHandler.GetByID)
	})

	// Rutas de seguridad del perfil de usuario (PINs real / coerción)
	r.Route("/api/v1/user", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Put("/security/pins", userHandler.SetSecurityPins)
	})

	// Streaming de telemetría en vivo (autenticación manual por token)
	r.Get("/ws/v1/alerts/stream", wsHandler.Stream)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	if err := client.Disconnect(shutdownCtx); err != nil {
		slog.Error("MongoDB disconnect error", "error", err)
	}

	slog.Info("server stopped")
}
