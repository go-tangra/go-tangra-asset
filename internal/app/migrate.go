package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
)

// migrator is the part of the documents service the migration worker uses.
type migrator interface {
	Migrate(ctx context.Context, limit int) (documents.MigrateResult, error)
}

// Migration pacing (variables for tests).
var (
	migrateFirst = time.Minute
	migrateEvery = time.Hour
)

// migrateDocuments moves the documents still held in the object store into
// paperless: a minute after start, then hourly, draining in batches of 100
// while a batch makes progress (feature 030).
func migrateDocuments(ctx context.Context, m migrator, log *slog.Logger) {
	wait := migrateFirst
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = migrateEvery
		for {
			res, err := m.Migrate(ctx, 100)
			if err != nil {
				log.WarnContext(ctx, "document migration to paperless failed", "err", err)
				break
			}
			if res.Migrated > 0 || res.Failed > 0 {
				log.InfoContext(ctx, "documents migrated to paperless", "migrated", res.Migrated, "failed", res.Failed)
			}
			if res.Migrated == 0 || ctx.Err() != nil {
				break // nothing left, or only failures (retried next pass)
			}
		}
	}
}
