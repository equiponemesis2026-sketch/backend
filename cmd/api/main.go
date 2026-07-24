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

	contactHttp "github.com/nemesis-project/api-nemesis/internal/contact/delivery/http"
	contactMongo "github.com/nemesis-project/api-nemesis/internal/contact/repository/mongo"
	contactUsecase "github.com/nemesis-project/api-nemesis/internal/contact/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/config"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/database"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	tokenHttp "github.com/nemesis-project/api-nemesis/internal/token/delivery/http"
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
	contactUseCase := contactUsecase.NewContactUseCase(contactRepoImpl)
	contactHandler := contactHttp.NewContactHandler(contactUseCase)

	// --- Módulo 3: Tokens de Vinculación (WearOS/NMS) ---
	tokenRepoImpl := tokenRepo.NewDeviceRepository(db)
	tokenUc := tokenUseCase.NewTokenUseCase(tokenRepoImpl)
	tokenHandler := tokenHttp.NewTokenHandler(tokenUc)

	// Middleware de autenticación JWT
	authMiddleware := middleware.JWTAuth(cfg.JWTSecret)

	r := chi.NewRouter()

	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)

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

	// Rutas del módulo de autenticación (públicas)
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)
	})

	// Rutas del módulo de contactos (protegidas con JWT)
	r.Route("/api/v1/contacts", func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/", contactHandler.GetAll)
		r.Post("/", contactHandler.Create)
		r.Put("/{id}", contactHandler.Update)
		r.Delete("/{id}", contactHandler.Delete)
	})

	// Rutas del módulo de vinculación de dispositivos
	r.Route("/api/v1/devices", func(r chi.Router) {
		r.Post("/tokens/generate", tokenHandler.GenerateCode)
		r.With(authMiddleware).Post("/pair", tokenHandler.PairDevice)
	})

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