package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Folsik/Property-Management-Inspections/internal/api"
	"github.com/Folsik/Property-Management-Inspections/internal/auth"
	"github.com/Folsik/Property-Management-Inspections/internal/config"
	"github.com/Folsik/Property-Management-Inspections/internal/jobs"
	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if strings.EqualFold(cfg.Env, "production") && (cfg.JWTPrivateKey == "" || strings.Contains(cfg.DatabaseURL, "inspections_dev_password")) {
		logger.Error("worker production configuration invalid")
		os.Exit(1)
	}
	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	// HTTP authentication manager is not used by the private worker pipeline.
	var unused *auth.Manager
	media := api.New(db.DB, unused, logger, cfg.Env)
	pipeline := &jobs.Pipeline{DB: db.DB, Media: media, Logger: logger}
	runner := &jobs.Runner{DB: db.DB, Logger: logger, Handlers: pipeline.Handlers()}
	logger.Info("worker starting")
	if err = runner.Run(ctx); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
