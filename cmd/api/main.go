package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/syuctacm/syuct-oj/internal/api"
	"github.com/syuctacm/syuct-oj/internal/api/handler"
	"github.com/syuctacm/syuct-oj/internal/auth"
	"github.com/syuctacm/syuct-oj/internal/config"
	"github.com/syuctacm/syuct-oj/internal/contest"
	"github.com/syuctacm/syuct-oj/internal/database"
	"github.com/syuctacm/syuct-oj/internal/discussion"
	"github.com/syuctacm/syuct-oj/internal/judge"
	"github.com/syuctacm/syuct-oj/internal/problem"
	"github.com/syuctacm/syuct-oj/internal/submission"
	"github.com/syuctacm/syuct-oj/internal/training"
	"github.com/syuctacm/syuct-oj/internal/user"
	"github.com/syuctacm/syuct-oj/internal/webauthn"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	auth.SetCookieSecure(cfg.CookieSecure)

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
	languages := judge.DefaultLanguages()
	var languageSetting struct {
		Value any `bson:"value"`
	}
	if err := db.Collection("system").FindOne(ctx, bson.M{"_id": "hydrooj.langs"}).Decode(&languageSetting); err == nil {
		if raw, ok := languageSetting.Value.(string); ok && strings.TrimSpace(raw) != "" {
			if parsed, parseErr := judge.ParseLanguages(raw); parseErr == nil {
				languages = parsed
			} else {
				slog.Warn("unable to load hydrooj.langs; using built-in languages", "error", parseErr)
			}
		}
	}
	hub := judge.NewHub(languages)
	sessions, err := auth.NewMongoSessionManager(ctx, db, cfg.SessionTTL)
	if err != nil {
		slog.Error("unable to initialize session store", "error", err)
		os.Exit(1)
	}
	savedTTL := cfg.SessionTTL
	unsavedTTL := cfg.SessionTTL
	if strings.TrimSpace(os.Getenv("SESSION_TTL")) == "" {
		savedTTL = 30 * 24 * time.Hour
		unsavedTTL = 3 * time.Hour
	}
	savedTTL = readSystemDuration(ctx, db, "session.saved_expire_seconds", savedTTL)
	unsavedTTL = readSystemDuration(ctx, db, "session.unsaved_expire_seconds", unsavedTTL)
	sessions.SetTTLs(savedTTL, unsavedTTL)
	judgeHandler := judge.NewHandler(cfg.JudgeUsername, cfg.JudgePassword, sessions, judgeStore, hub)
	userRepo := user.NewRepository(db)
	go user.RunDeletionWorker(context.Background(), db, sessions, time.Hour)
	authHandler := auth.NewHandler(userRepo, sessions)
	webauthnService, err := webauthn.New(ctx, db, userRepo, webauthn.Config{
		RPID:          cfg.WebAuthnRPID,
		RPDisplayName: cfg.WebAuthnName,
		RPOrigins:     cfg.WebAuthnOrigins,
	})
	if err != nil {
		slog.Error("unable to initialize WebAuthn", "error", err)
		os.Exit(1)
	}
	problemRepo := problem.NewRepository(db)
	judgeHandler.EnableGenerateUpload(db, problemRepo)
	contestRepo := contest.NewRepository(db)
	trainingRepo := training.NewRepository(db)
	discussionRepo := discussion.NewRepository(db)
	submissionRepo := submission.NewRepository(db, problemRepo)
	submissionHandler := handler.NewSubmissionHandler(submissionRepo, sessions, contestRepo, userRepo)
	adminHandler := handler.NewAdminHandler(db, userRepo, sessions)
	adminHandler.SetSessionTTLUpdater(func(saved, unsaved time.Duration) {
		sessions.SetTTLs(saved, unsaved)
	})
	adminHandler.SetLanguageUpdater(func(raw string) error {
		parsed, err := judge.ParseLanguages(raw)
		if err != nil {
			return err
		}
		hub.SetLanguages(parsed)
		return nil
	})
	submissionRepo.SetAcceptedHook(func(ctx context.Context, contestID bson.ObjectID, uid, pid int64) {
		_ = contestRepo.AddBalloon(ctx, contestID, uid, pid)
	})
	pump := judge.NewTaskPump(db, hub)
	pump.SetLeaseDuration(cfg.JudgeTaskLease)
	pump.SetResultHandler(func(payload map[string]any) error {
		return submissionRepo.ApplyJudgeResult(context.Background(), payload)
	})
	hub.SetResultHandler(func(payload map[string]any) error {
		return pump.HandleResult(context.Background(), payload)
	})
	go pump.Run(context.Background())

	router := api.NewRouter(cfg, db, judgeHandler, authHandler, problemRepo, submissionHandler, contestRepo, trainingRepo, discussionRepo, userRepo, adminHandler, sessions)
	handler.NewWebAuthnHandler(webauthnService, userRepo, sessions).RegisterRoutes(router.Group("/"))
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
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

func readSystemDuration(ctx context.Context, db *mongo.Database, key string, fallback time.Duration) time.Duration {
	var doc struct {
		Value any `bson:"value"`
	}
	if err := db.Collection("system").FindOne(ctx, bson.M{"_id": key}).Decode(&doc); err != nil {
		return fallback
	}
	var seconds int64
	switch value := doc.Value.(type) {
	case int:
		seconds = int64(value)
	case int32:
		seconds = int64(value)
	case int64:
		seconds = value
	case float64:
		seconds = int64(value)
	default:
		return fallback
	}
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
