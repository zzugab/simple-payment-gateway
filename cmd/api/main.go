package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zzugab/simple-payment-gateway/internal/config"
	"github.com/zzugab/simple-payment-gateway/internal/platform/database"
	"github.com/zzugab/simple-payment-gateway/internal/server"
)

func main() {
	var level slog.Level
	if value := os.Getenv("LOG_LEVEL"); value != "" {
		if err := level.UnmarshalText([]byte(value)); err != nil {
			slog.Error("invalid LOG_LEVEL; use DEBUG, INFO, WARN or ERROR")
			os.Exit(1)
		}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", "payment-gateway")
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to PostgreSQL", "error_type", "database_connection_failed")
		os.Exit(1)
	}
	defer pool.Close()

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.NewRouter(pool),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server starting", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			_ = httpServer.Close()
		}
		logger.Info("HTTP server stopped")
	}
}
