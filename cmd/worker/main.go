package main

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Folsik/Property-Management-Inspections/internal/api"
	"github.com/Folsik/Property-Management-Inspections/internal/config"
	"github.com/Folsik/Property-Management-Inspections/internal/jobs"
	"github.com/Folsik/Property-Management-Inspections/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if strings.EqualFold(cfg.Env, "production") {
		u, err := url.Parse(cfg.DatabaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
			strings.Contains(cfg.DatabaseURL, "inspections_dev_password") || u.Query().Get("sslmode") != "verify-full" {
			logger.Error("production worker requires PostgreSQL TLS verify-full and nondevelopment credentials")
			os.Exit(1)
		}
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
	// The private worker does not need the API JWT signing key.
	media := api.New(db.DB, nil, logger, cfg.Env)
	pipeline := &jobs.Pipeline{DB: db.DB, Media: media, Logger: logger}
	runner := &jobs.Runner{DB: db.DB, Logger: logger, Handlers: pipeline.Handlers()}
	logger.Info("worker starting")
	if err = runner.Run(ctx); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
