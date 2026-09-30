package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
)

type fakeMigrator struct {
	mu    sync.Mutex
	calls int
	plan  []documents.MigrateResult
	err   error
	done  chan struct{}
}

func (f *fakeMigrator) Migrate(context.Context, int) (documents.MigrateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return documents.MigrateResult{}, f.err
	}
	if len(f.plan) == 0 {
		if f.done != nil {
			close(f.done)
			f.done = nil
		}
		return documents.MigrateResult{}, nil
	}
	r := f.plan[0]
	f.plan = f.plan[1:]
	return r, nil
}

func TestMigrateDocuments(t *testing.T) {
	defer func(a, b time.Duration) { migrateFirst, migrateEvery = a, b }(migrateFirst, migrateEvery)
	migrateFirst, migrateEvery = time.Millisecond, time.Hour
	log := slog.New(slog.DiscardHandler)
	// Drains batches while they make progress, stops on an empty batch.
	f := &fakeMigrator{plan: []documents.MigrateResult{{Migrated: 100}, {Migrated: 3, Failed: 1}}, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	go migrateDocuments(ctx, f, log)
	select {
	case <-f.done:
	case <-time.After(5 * time.Second):
		t.Fatal("migration did not drain")
	}
	cancel()
	f.mu.Lock()
	if f.calls != 3 {
		t.Errorf("calls %d", f.calls)
	}
	f.mu.Unlock()
	// Errors and failure-only batches stop the pass; cancellation ends the loop.
	for _, g := range []*fakeMigrator{{err: errors.New("down")}, {plan: []documents.MigrateResult{{Failed: 2}}}} {
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan struct{})
		go func() { migrateDocuments(ctx, g, log); close(stopped) }()
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not stop")
		}
		g.mu.Lock()
		if g.calls != 1 {
			t.Errorf("calls %d", g.calls)
		}
		g.mu.Unlock()
	}
}
