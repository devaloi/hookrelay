// Package main provides the entry point for the hookrelay webhook relay service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/devaloi/hookrelay/internal/config"
	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/handler"
	"github.com/devaloi/hookrelay/internal/middleware"
	"github.com/devaloi/hookrelay/internal/queue"
	"github.com/devaloi/hookrelay/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("starting hookrelay", "port", cfg.Port, "database", cfg.DatabaseURL)

	store, err := queue.NewSQLiteStore(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatcher := worker.NewDispatcher(store, cfg, logger)
	dispatcher.Start(ctx)

	mux := http.NewServeMux()
	ingestHandler := handler.NewIngestHandler(store, logger)
	adminHandler := handler.NewAdminHandler(store, logger)
	healthHandler := handler.NewHealthHandler(store, logger)
	statsHandler := handler.NewStatsHandler(store, logger)

	mux.Handle("/webhooks/", ingestHandler)
	mux.Handle("/admin/", adminHandler)
	mux.Handle("/health", healthHandler)
	mux.Handle("/stats", statsHandler)

	h := middleware.Chain(
		mux,
		middleware.Recovery(logger),
		middleware.Logging(logger),
		middleware.RequestID,
	)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h,
		ReadTimeout:  domain.ReadTimeout,
		WriteTimeout: domain.WriteTimeout,
		IdleTimeout:  domain.IdleTimeout,
	}

	go func() {
		logger.Info("HTTP server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), domain.ShutdownTimeout)
	defer shutdownCancel()

	dispatcher.Stop()
	cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	logger.Info("shutdown complete")
}
