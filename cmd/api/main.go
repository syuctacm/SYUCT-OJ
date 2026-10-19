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

	"github.com/joho/godotenv"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/judge"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	slog.Info("starting syuct-oj api", "port", cfg.Port)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, db, err := database.Connect(ctx, cfg.MongoURI, cfg.MongoDatabase, 5*time.Second)
	if err != nil {
		slog.Error("unable to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	judgeStore := judge.NewFileStore(cfg.JudgeFileRoot)
	if err := judgeStore.Ensure(); err != nil {
		slog.Error("unable to initialize judge file store", "error", err)
		os.Exit(1)
	}
	hub := judge.NewHub(judge.DefaultLanguages())
	sessions := auth.NewSessionManager(24 * time.Hour)
	judgeHandler := judge.NewHandler(cfg.JudgeUsername, cfg.JudgePassword, sessions, judgeStore, hub)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewRouter(cfg, db, judgeHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		slog.Info("shutdown requested", "signal", sig.String())
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("api stopped")
}
