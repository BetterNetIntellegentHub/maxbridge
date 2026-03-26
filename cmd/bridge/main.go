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

	"github.com/prometheus/client_golang/prometheus"

	"maxbridge/internal/app"
	"maxbridge/internal/httpapi"
	"maxbridge/internal/invites"
	maxapi "maxbridge/internal/max"
	"maxbridge/internal/ops"
	"maxbridge/internal/storage"
	"maxbridge/internal/telegram"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		os.Exit(1)
	}
	log := app.NewLogger(cfg.LogLevel)

	ctx := context.Background()
	store, err := storage.NewStore(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("db init failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	switch os.Args[1] {
	case "serve":
		runServe(cfg, log, store)
	case "migrate":
		runMigrate(ctx, store)
	case "health":
		runHealth(ctx, store)
	case "--help", "-h", "help":
		printUsage()
	default:
		printUsage()
		os.Exit(1)
	}
}

func runServe(cfg app.Config, log *slog.Logger, store *storage.Store) {
	reg := prometheus.DefaultRegisterer
	metrics := app.NewMetrics(reg)

	inviteSvc := invites.NewService(store, cfg.InviteHashPepper)
	tg := telegram.NewClient(cfg.TelegramToken)
	mx := maxapi.NewClient(cfg.MaxAPIBaseURL, cfg.MaxToken)
	srv := httpapi.NewServer(cfg, log, metrics, store, inviteSvc, tg, mx)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go ops.RunHousekeeping(ctx, log, store, ops.RetentionConfig{
		JobsDays:     cfg.RetentionJobsDays,
		DedupeDays:   cfg.RetentionDedupeDays,
		PayloadHours: cfg.RetentionPayloadHours,
	})

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

func runMigrate(ctx context.Context, store *storage.Store) {
	if len(os.Args) < 3 {
		fmt.Println("usage: bridge migrate <up|down>")
		os.Exit(1)
	}
	dir := "migrations"
	switch os.Args[2] {
	case "up":
		if err := store.RunMigrations(ctx, dir, storage.Up); err != nil {
			fmt.Printf("migrate up failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrate up done")
	case "down":
		if err := store.RunMigrations(ctx, dir, storage.Down); err != nil {
			fmt.Printf("migrate down failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrate down done")
	default:
		fmt.Println("usage: bridge migrate <up|down>")
		os.Exit(1)
	}
}

func runHealth(ctx context.Context, store *storage.Store) {
	if err := store.Ping(ctx); err != nil {
		fmt.Printf("db: fail (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("db: ok")
}

func printUsage() {
	fmt.Println(`bridge commands:
  bridge serve
  bridge migrate up
  bridge migrate down
  bridge health`)
}
