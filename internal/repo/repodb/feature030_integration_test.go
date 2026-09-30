//go:build integration

package repodb_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// TestFeature030Repo: inventory-sync settings are stored per tenant (RLS,
// text[] round trip) and documents move to paperless (unmigrated listing
// across tenants, move, lookup by paperless id).
func TestFeature030Repo(t *testing.T) {
	db := openRepo(t)
	ctx := context.Background()

	if s, found, err := db.GetInvSyncSettings(ctx, tenantA); err != nil || found || s.TenantID != tenantA {
		t.Fatalf("default %+v %v %v", s, found, err)
	}
	in := store.InvSyncSettings{TenantID: tenantA, ExcludeVMs: true, SkipRetired: true, HostnameExclude: []string{"*.lab.*"}, OSInclude: []string{"ubuntu*", "windows*"},
		UpdatedBy: "u1", UpdatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if err := db.PutInvSyncSettings(ctx, in); err != nil {
		t.Fatal(err)
	}
	in.ExcludeContainers = true
	if err := db.PutInvSyncSettings(ctx, in); err != nil { // upsert
		t.Fatal(err)
	}
	got, found, err := db.GetInvSyncSettings(ctx, tenantA)
	if err != nil || !found || !got.ExcludeVMs || !got.ExcludeContainers || len(got.OSInclude) != 2 || got.HostnameExclude[0] != "*.lab.*" || len(got.HostnameInclude) != 0 {
		t.Fatalf("round trip %+v %v", got, err)
	}
	if _, found, _ := db.GetInvSyncSettings(ctx, tenantB); found {
		t.Fatal("settings visible to another tenant")
	}

	// Documents.
	a := store.Asset{ID: store.NewID(), TenantID: tenantA, AssetTag: "F030-1", Name: "doc host"}
	b := store.Asset{ID: store.NewID(), TenantID: tenantB, AssetTag: "F030-2", Name: "doc host"}
	if err := db.CreateAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateAsset(ctx, b); err != nil {
		t.Fatal(err)
	}
	d1 := store.Document{ID: store.NewID(), TenantID: tenantA, EntityType: store.EntityAsset, EntityID: a.ID, FileName: "a.pdf", StorageKey: "tenants/" + tenantA + "/documents/x1"}
	d2 := store.Document{ID: store.NewID(), TenantID: tenantB, EntityType: store.EntityAsset, EntityID: b.ID, FileName: "b.pdf", StorageKey: "tenants/" + tenantB + "/documents/x2"}
	for _, d := range []store.Document{d1, d2} {
		if err := db.InsertDocument(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	un, err := db.ListUnmigratedDocuments(ctx, 0)
	if err != nil || len(un) < 2 {
		t.Fatalf("unmigrated %d %v", len(un), err)
	}
	if err := db.MoveDocumentToPaperless(ctx, tenantA, d1.ID, "pd-1"); err != nil {
		t.Fatal(err)
	}
	if err := db.MoveDocumentToPaperless(ctx, tenantB, d1.ID, "pd-x"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("cross-tenant move %v", err)
	}
	moved, err := db.GetDocument(ctx, tenantA, d1.ID)
	if err != nil || moved.PaperlessDocumentID != "pd-1" || moved.StorageKey != store.PaperlessKeyPrefix+"pd-1" {
		t.Fatalf("moved %+v %v", moved, err)
	}
	if un, _ := db.ListUnmigratedDocuments(ctx, 10); len(un) != 1 || un[0].ID != d2.ID {
		t.Fatalf("after move %+v", un)
	}
	byP, err := db.ListDocumentsByPaperlessIDs(ctx, tenantA, []string{"pd-1", "pd-missing"})
	if err != nil || len(byP) != 1 || byP[0].ID != d1.ID {
		t.Fatalf("by paperless id %+v %v", byP, err)
	}
	if byP, _ := db.ListDocumentsByPaperlessIDs(ctx, tenantB, []string{"pd-1"}); len(byP) != 0 {
		t.Fatal("paperless lookup crossed tenants")
	}
	if byP, err := db.ListDocumentsByPaperlessIDs(ctx, tenantA, nil); err != nil || byP != nil {
		t.Fatal("empty lookup")
	}
	// A paperless-backed row is inserted with its id (restore path).
	d3 := store.Document{ID: store.NewID(), TenantID: tenantA, EntityType: store.EntityAsset, EntityID: a.ID, FileName: "c.pdf", StorageKey: store.PaperlessKeyPrefix + "pd-3", PaperlessDocumentID: "pd-3"}
	if err := db.InsertDocument(ctx, d3); err != nil {
		t.Fatal(err)
	}
	if g, _ := db.GetDocument(ctx, tenantA, d3.ID); g.PaperlessDocumentID != "pd-3" {
		t.Fatalf("insert with paperless id %+v", g)
	}
}
