package engine

import (
	"context"
	"time"

	"github.com/kainhuck/signalix/internal/config"
	"github.com/kainhuck/signalix/internal/ports"
	"github.com/kainhuck/signalix/pkg/logger"
)

func (e *Engine) retentionLoop() {
	settings := e.persistenceSettings
	if settings.CleanupInterval <= 0 {
		settings.CleanupInterval = time.Hour
	}

	e.runRetentionOnceLogged(settings)

	ticker := time.NewTicker(settings.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.runRetentionOnceLogged(settings)
		}
	}
}

func (e *Engine) runRetentionOnceLogged(settings config.PersistenceSettings) {
	logsDeleted, snapsDeleted, err := runRetentionOnce(e.ctx, e.store, settings, time.Now())
	if err != nil {
		logger.WarnContext(e.ctx, "persistence retention cleanup failed", logger.Any("error", err))
		return
	}
	if logsDeleted == 0 && snapsDeleted == 0 {
		return
	}
	logger.InfoContext(e.ctx, "persistence retention cleanup completed",
		logger.Int64("logs_deleted", logsDeleted),
		logger.Int64("snapshots_deleted", snapsDeleted),
	)
}

func runRetentionOnce(ctx context.Context, store ports.PersistenceStore, settings config.PersistenceSettings, now time.Time) (logsDeleted, snapsDeleted int64, err error) {
	if store == nil {
		return 0, 0, nil
	}
	now = now.UTC()

	if settings.StrategyLogRetentionDays > 0 {
		cutoff := retentionCutoff(now, settings.StrategyLogRetentionDays)
		logsDeleted, err = store.PurgeStrategyLogsBefore(ctx, cutoff)
		if err != nil {
			return logsDeleted, snapsDeleted, err
		}
	}

	if settings.AccountSnapshotRetentionDays > 0 {
		cutoff := retentionCutoff(now, settings.AccountSnapshotRetentionDays)
		snapsDeleted, err = store.PurgeAccountSnapshotsBefore(ctx, cutoff)
		if err != nil {
			return logsDeleted, snapsDeleted, err
		}
	}

	return logsDeleted, snapsDeleted, nil
}

func retentionCutoff(now time.Time, retentionDays int) time.Time {
	return now.UTC().AddDate(0, 0, -retentionDays)
}
