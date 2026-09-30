package app

import (
	"context"

	"google.golang.org/grpc"

	schedulerv1 "github.com/go-tangra/go-tangra-scheduler/sdk/v4/api/proto/scheduler/v1"
	"github.com/go-tangra/go-tangra-scheduler/sdk/v4/pkg/schedulerclient"
	sdktask "github.com/go-tangra/go-tangra-scheduler/sdk/v4/pkg/taskexec"
	"github.com/go-tangra/go-tangra/v4/authn"

	"github.com/go-tangra/go-tangra-asset/v4/internal/invsync"
	"github.com/go-tangra/go-tangra-asset/v4/internal/tasks"
)

// wireScheduler serves scheduler.v1.TaskExecutor (always: the mesh policy
// admits only svc/scheduler and the SDK server re-checks the peer) and, when
// task_scheduler.enabled, keeps the asset task types registered.
func (a *App) wireScheduler(sync *invsync.Service) {
	cfg := a.Cfg
	runner := &tasks.Runner{Sync: sync}
	scheduler := cfg.TaskScheduler.Service
	if scheduler == "" {
		scheduler = sdktask.DefaultScheduler
	}
	schedulerv1.RegisterTaskExecutorServer(a.Freya.GRPC(), sdktask.NewServer(runner.Handlers(), sdktask.Options{
		Caller: schedulerCaller(cfg.Config.TrustDomain), Scheduler: scheduler, Log: a.Log,
	}))
	if !cfg.TaskScheduler.Enabled {
		return
	}
	reg := &schedulerclient.Registrar{
		Dial:  func(ctx context.Context) (grpc.ClientConnInterface, error) { return a.Freya.Client(ctx, scheduler) },
		Types: tasks.Descriptors(),
		Log:   a.Log,
	}
	a.workers = append(a.workers, reg.Run)
}

// schedulerCaller returns the verified peer's service name, only for a peer
// of the module's own trust domain.
func schedulerCaller(trustDomain string) func(ctx context.Context) (string, bool) {
	return func(ctx context.Context) (string, bool) {
		p, ok := authn.FromContext(ctx)
		if !ok || p.ID.TrustDomain() != trustDomain {
			return "", false
		}
		return p.ID.ServiceName(), true
	}
}
