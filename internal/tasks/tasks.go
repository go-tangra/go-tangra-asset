// Package tasks holds the scheduled task types the asset module executes for
// the platform scheduler (feature 030):
//
//   - asset:inventory-sync (tenant-scoped) imports the tenant's inventory
//     hosts as assets and updates the sync-owned fields of matched ones,
//     applying the tenant's persisted inventory-sync filter.
//
// The SDK executor server verifies that the caller is the scheduler.
package tasks

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-tangra/go-tangra-scheduler/sdk/v4/pkg/schedulerclient"
	sdk "github.com/go-tangra/go-tangra-scheduler/sdk/v4/pkg/taskexec"

	"github.com/go-tangra/go-tangra-asset/v4/internal/invsync"
)

// TypeInventorySync is the task type of the inventory sync.
const TypeInventorySync = "asset:inventory-sync"

const inventorySyncSchema = `{"type":"object","properties":{` +
	`"create":{"type":"boolean","description":"Create assets for new inventory hosts (default true)."},` +
	`"update":{"type":"boolean","description":"Update the synced fields of existing assets (default true)."}` +
	`},"additionalProperties":false}`

// Descriptors are the task types the asset module registers.
func Descriptors() []schedulerclient.Descriptor {
	return []schedulerclient.Descriptor{{
		Type: TypeInventorySync, DisplayName: "Sync assets from inventory",
		Description: "Imports the tenant's inventory hosts as assets and updates the synced fields of matched assets, " +
			"skipping hosts excluded by the inventory-sync filter (VMs, containers, hostnames, OS, stale or retired hosts).",
		PayloadSchema: inventorySyncSchema, DefaultCron: "0 3 * * *", DefaultMaxRetry: 2,
	}}
}

// Syncer runs the sync for a tenant (invsync.Service).
type Syncer interface {
	RunScheduled(ctx context.Context, tenantID string, opts invsync.Options) (invsync.Result, error)
}

// Runner executes the task types.
type Runner struct {
	Sync Syncer
}

// Handlers maps the task types to their handlers for sdk.NewServer.
func (r *Runner) Handlers() map[string]sdk.Handler {
	return map[string]sdk.Handler{TypeInventorySync: r.InventorySync}
}

// InventorySync runs asset:inventory-sync for the task's tenant.
func (r *Runner) InventorySync(ctx context.Context, req sdk.Request) sdk.Result {
	if req.TenantID == "" {
		return sdk.Permanent(TypeInventorySync + " is tenant-scoped")
	}
	var p struct {
		Create *bool `json:"create"`
		Update *bool `json:"update"`
	}
	if err := sdk.DecodeStrict(req.Payload, &p); err != nil {
		return sdk.Permanent("invalid payload: " + err.Error())
	}
	opts := invsync.Options{Create: p.Create == nil || *p.Create, Update: p.Update == nil || *p.Update}
	if r.Sync == nil {
		return sdk.Retry("inventory sync is not available")
	}
	res, err := r.Sync.RunScheduled(ctx, req.TenantID, opts)
	if errors.Is(err, invsync.ErrUnavailable) {
		return sdk.Retry("inventory unavailable")
	}
	if err != nil {
		return sdk.Retry("inventory sync failed")
	}
	msg := fmt.Sprintf("%d created, %d updated, %d excluded by the filter, %d skipped", res.Created, res.Updated, res.Excluded, res.Skipped)
	if len(res.Errors) > 0 {
		return sdk.Result{Success: false, Message: fmt.Sprintf("%s, %d failed", msg, len(res.Errors))}
	}
	return sdk.OK(msg)
}
