package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Asia/Almaty must load in minimal images that ship no system zoneinfo.

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
	"github.com/ic-group/ic-tender/apps/api/internal/config"
	"github.com/ic-group/ic-tender/apps/api/internal/db"
	"github.com/ic-group/ic-tender/apps/api/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if !cfg.IsProduction() && cfg.SessionSecret == "" {
		logger.Warn("SESSION_SECRET is empty: session tokens are hashed without a key (development only)")
	}
	authService, err := auth.NewService(auth.NewPGStore(pool), auth.Config{
		Secret:          []byte(cfg.SessionSecret),
		IdleTimeout:     auth.DefaultIdleTimeout,
		AbsoluteTimeout: auth.DefaultAbsoluteTimeout,
		Logger:          logger,
	})
	if err != nil {
		logger.Error("auth service init failed", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: cfg.APIAddr,
		Handler: httpapi.New(httpapi.Deps{
			DB: pool, Queries: pool, Auth: authService, WebOrigin: cfg.WebOrigin, Logger: logger,
			Production: cfg.IsProduction(), TrustForwardedFor: cfg.TrustForwardedFor,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API server starting", "addr", cfg.APIAddr)
		serverErrors <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("API server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-signalCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("API server stopped")
	}
}
