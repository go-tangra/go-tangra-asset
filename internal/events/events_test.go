package events

import (
	"context"
	"testing"
	"time"
)

func TestHubPublisherNilNoOp(t *testing.T) {
	// A nil-hub publisher is a safe no-op and must satisfy Publisher.
	var p Publisher = HubPublisher{}
	p.Publish(context.Background(), "t1", AssetAssigned, map[string]any{"k": "v"})
	// Explicit nil Hub too.
	HubPublisher{Hub: nil}.Publish(context.Background(), "t1", AssetUnassigned, nil)
}

func TestEventTypeConsts(t *testing.T) {
	want := map[string]string{
		AssetAssigned:      "asset.assigned",
		AssetUnassigned:    "asset.unassigned",
		WarrantyExpiring:   "asset.warranty.expiring",
		LicenseExpiring:    "license.expiring",
		InsuranceExpiring:  "insurance.expiring",
		ConsumableLowStock: "consumable.low_stock",
	}
	for got, exp := range want {
		if got != exp {
			t.Errorf("event type %q != %q", got, exp)
		}
	}
}

func TestAssignPayloads(t *testing.T) {
	a := AssetAssignedPayload("a1", "TAG-1", "u1")
	if a["asset_id"] != "a1" || a["asset_tag"] != "TAG-1" || a["user_id"] != "u1" {
		t.Errorf("AssetAssignedPayload=%+v", a)
	}
	u := AssetUnassignedPayload("a1", "TAG-1", "u1")
	if u["asset_id"] != "a1" || u["asset_tag"] != "TAG-1" || u["user_id"] != "u1" {
		t.Errorf("AssetUnassignedPayload=%+v", u)
	}
	// No contact/PII keys leak into assignment payloads.
	for _, m := range []map[string]any{a, u} {
		for _, k := range []string{"email", "phone", "contact", "secret"} {
			if _, ok := m[k]; ok {
				t.Errorf("payload leaked key %q", k)
			}
		}
	}
}

func TestExpiryPayloads(t *testing.T) {
	when := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	w := WarrantyExpiringPayload("a1", "TAG-1", when)
	if w["asset_id"] != "a1" || w["asset_tag"] != "TAG-1" || w["expires_at"] != when {
		t.Errorf("WarrantyExpiringPayload=%+v", w)
	}
	l := LicenseExpiringPayload("l1", "Adobe", when)
	if l["id"] != "l1" || l["name"] != "Adobe" || l["valid_to"] != when {
		t.Errorf("LicenseExpiringPayload=%+v", l)
	}
	i := InsuranceExpiringPayload("p1", "POL-9", when)
	if i["id"] != "p1" || i["policy_number"] != "POL-9" || i["valid_to"] != when {
		t.Errorf("InsuranceExpiringPayload=%+v", i)
	}
}

func TestLowStockPayload(t *testing.T) {
	m := LowStockPayload("c1", "Toner", 2, 5)
	if m["id"] != "c1" || m["name"] != "Toner" || m["amount"] != 2 || m["min_amount"] != 5 {
		t.Errorf("LowStockPayload=%+v", m)
	}
}
