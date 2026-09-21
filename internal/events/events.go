// Package events publishes asset (ITAM) realtime events to the shared platform
// event bus so the gateway SSE hub relays them to the browser. The module only
// publishes (it consumes no external events); payloads carry no supplier/location
// contact fields, credentials or other PII.
package events

import (
	"context"
	"time"

	"github.com/go-freya/freya/services/asset/internal/stream"
)

// Event types published to platform:events:<tenant>.
const (
	AssetAssigned      = "asset.assigned"
	AssetUnassigned    = "asset.unassigned"
	WarrantyExpiring   = "asset.warranty.expiring"
	LicenseExpiring    = "license.expiring"
	InsuranceExpiring  = "insurance.expiring"
	ConsumableLowStock = "consumable.low_stock"
)

// Publisher emits a realtime event to all of a tenant's subscribers.
type Publisher interface {
	Publish(ctx context.Context, tenantID, eventType string, payload any)
}

// HubPublisher publishes through the stream hub (nil hub is a no-op).
type HubPublisher struct{ Hub *stream.Hub }

// Publish broadcasts eventType to every subscriber of tenantID. A nil hub (or
// nil-hub publisher) is a safe no-op so callers need not branch on it.
func (p HubPublisher) Publish(ctx context.Context, tenantID, eventType string, payload any) {
	if p.Hub == nil {
		return
	}
	_, _ = p.Hub.PublishID(ctx, tenantID, nil, true, eventType, payload, true)
}

// AssetAssignedPayload is the content-safe payload for a check-out event.
func AssetAssignedPayload(assetID, assetTag, userID string) map[string]any {
	return map[string]any{
		"asset_id":  assetID,
		"asset_tag": assetTag,
		"user_id":   userID,
	}
}

// AssetUnassignedPayload is the content-safe payload for a check-in event.
func AssetUnassignedPayload(assetID, assetTag, userID string) map[string]any {
	return map[string]any{
		"asset_id":  assetID,
		"asset_tag": assetTag,
		"user_id":   userID,
	}
}

// WarrantyExpiringPayload reports an asset whose warranty is within the window.
func WarrantyExpiringPayload(assetID, assetTag string, expiresAt time.Time) map[string]any {
	return map[string]any{
		"asset_id":   assetID,
		"asset_tag":  assetTag,
		"expires_at": expiresAt,
	}
}

// LicenseExpiringPayload reports a license whose validity ends within the window.
func LicenseExpiringPayload(id, name string, validTo time.Time) map[string]any {
	return map[string]any{
		"id":       id,
		"name":     name,
		"valid_to": validTo,
	}
}

// InsuranceExpiringPayload reports a policy whose validity ends within the window.
func InsuranceExpiringPayload(id, policyNumber string, validTo time.Time) map[string]any {
	return map[string]any{
		"id":            id,
		"policy_number": policyNumber,
		"valid_to":      validTo,
	}
}

// LowStockPayload reports a consumable at or below its reorder threshold.
func LowStockPayload(id, name string, amount, minAmount int) map[string]any {
	return map[string]any{
		"id":         id,
		"name":       name,
		"amount":     amount,
		"min_amount": minAmount,
	}
}
