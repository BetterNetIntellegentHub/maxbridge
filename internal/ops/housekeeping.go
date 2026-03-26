package ops

import (
	"context"
	"log/slog"
	"time"

	"maxbridge/internal/storage"
)

type RetentionConfig struct {
	JobsDays     int
	DedupeDays   int
	PayloadHours int
}

func RunHousekeeping(ctx context.Context, log *slog.Logger, store *storage.Store, cfg RetentionConfig) {
	cleanupTicker := time.NewTicker(6 * time.Hour)
	defer cleanupTicker.Stop()
	partitionTicker := time.NewTicker(24 * time.Hour)
	defer partitionTicker.Stop()

	runCleanup := func() {
		if err := store.CleanupRetention(ctx, cfg.JobsDays, cfg.DedupeDays, cfg.PayloadHours); err != nil {
			log.Error("retention cleanup failed", "error", err)
			return
		}
		log.Info("retention cleanup completed")
	}
	runPartitions := func() {
		if err := store.EnsureAttemptPartitions(ctx, 2); err != nil {
			log.Error("ensure partitions failed", "error", err)
			return
		}
	}

	runCleanup()
	runPartitions()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanupTicker.C:
			runCleanup()
		case <-partitionTicker.C:
			runPartitions()
		}
	}
}
