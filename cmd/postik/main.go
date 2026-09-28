package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zetesis-labs/postik/internal/app"
	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/database"
	"github.com/zetesis-labs/postik/internal/database/migrations"
	"github.com/zetesis-labs/postik/internal/webui"
)

const usage = "usage: postik serve | postik migrate"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(ctx, logger)
	case "migrate":
		err = migrate(ctx, logger)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		logger.Error(os.Args[1]+" failed", "error", err)
		os.Exit(1)
	}
}

func migrate(ctx context.Context, logger *slog.Logger) error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	db, err := database.Open(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrations.Run(ctx, db.DB); err != nil {
		return err
	}
	logger.Info("database migrations are up to date")
	return nil
}

func serve(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	server := &http.Server{
		Addr: cfg.ListenAddr,
		Handler: app.New(app.Deps{
			Config: cfg,
			DB:     db,
			Now:    time.Now,
			WebUI:  webui.Dist(),
			Logger: logger,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	errs := make(chan error, 1)
	go func() {
		logger.Info("postik is listening", "addr", cfg.ListenAddr, "public_url", cfg.PublicURL.String())
		errs <- server.ListenAndServe()
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}
