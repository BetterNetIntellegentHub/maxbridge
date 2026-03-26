package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"maxbridge/internal/app"
	"maxbridge/internal/delivery"
	maxapi "maxbridge/internal/max"
	"maxbridge/internal/storage"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "run" {
		fmt.Println("usage: worker run")
		os.Exit(1)
	}

	cfg, err := app.LoadConfig()
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		os.Exit(1)
	}
	log := app.NewLogger(cfg.LogLevel)

	store, err := storage.NewStore(context.Background(), cfg.DBDSN)
	if err != nil {
		log.Error("db init failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	metrics := app.NewMetrics(prometheus.DefaultRegisterer)
	mx := maxapi.NewClient(cfg.MaxAPIBaseURL, cfg.MaxToken)
	worker := delivery.NewWorker(
		store,
		mx,
		log,
		metrics,
		cfg.WorkerConcurrency,
		cfg.WorkerLease,
		cfg.WorkerMaxRetry,
		cfg.WorkerRateLimitRPS,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	metricsSrv := &http.Server{Addr: cfg.MetricsAddr, Handler: promhttp.Handler()}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("worker metrics server failed", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	if err := worker.Run(ctx); err != nil {
		log.Error("worker run failed", "error", err)
		os.Exit(1)
	}
}
