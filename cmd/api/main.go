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
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nemesis-project/api-nemesis/internal/infrastructure/config"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/database"
	userHttp "github.com/nemesis-project/api-nemesis/internal/user/delivery/http"
	mongoRepo "github.com/nemesis-project/api-nemesis/internal/user/repository/mongo"
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

	// --- Módulo 1: Usuarios y Auth (Inyección de Dependencias) ---
	userRepo := mongoRepo.NewUserRepository(db)
	userUseCase := usecase.NewUserUseCase(userRepo, "super-secret-key", 24*time.Hour)
	userHandler := userHttp.NewUserHandler(userUseCase)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

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

	// Rutas del módulo de autenticación
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)
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
