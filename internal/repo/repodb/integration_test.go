//go:build integration

// Package repodb integration test: exercises the TimescaleDB-backed asset store
// against a real database (testcontainers). It covers migration (idempotent),
// every unique constraint (asset tag, category name+parent, supplier/location
// name, policy number, policy-asset pair, document storage key), the category/
// location trees with computed counts and delete guards, assignment history,
// polymorphic documents, notify-state dedup, tenant statistics and per-tenant
// row-level security. Run with:
//
//	go test -tags integration ./internal/repo/repodb/
//
// It skips cleanly when Docker/testcontainers is unavailable.
package repodb_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo/repodb"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

const (
	tenantA = "0190f7c2-6a3e-7c1a-9b2e-2f6f9d1b4c55"
	tenantB = "0190f7c2-6a3e-7c1a-9b2e-2f6f9d1b4c66"
)

func startDB(t *testing.T) (adminDSN, appDSN string) {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "timescale/timescaledb:latest-pg16", ExposedPorts: []string{"5432/tcp"},
			Env:        map[string]string{"POSTGRES_PASSWORD": "test", "POSTGRES_DB": "asset"},
			WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2 * time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432/tcp")
	adminDSN = "postgres://postgres:test@" + host + ":" + port.Port() + "/asset?sslmode=disable"
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Exec(ctx, "CREATE ROLE asset_app LOGIN PASSWORD 'app' NOBYPASSRLS")
	_ = conn.Close(ctx)
	appDSN = "postgres://asset_app:app@" + host + ":" + port.Port() + "/asset?sslmode=disable"
	return
}

func openRepo(t *testing.T) repo.Store {
	t.Helper()
	adminDSN, appDSN := startDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, adminDSN); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := store.Migrate(ctx, adminDSN); err != nil { // idempotent
		t.Fatalf("migrate idempotent: %v", err)
	}
	st, err := store.Open(ctx, appDSN, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(st.Close)
	return repodb.New(st)
}

func TestAssetRepo(t *testing.T) {
	db := openRepo(t)
	ctx := context.Background()

	// --- org records + unique constraints ---
	sup := store.Supplier{ID: store.NewID(), TenantID: tenantA, Name: "Acme", Email: "sealed"}
	if err := db.CreateSupplier(ctx, sup); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSupplier(ctx, store.Supplier{ID: store.NewID(), TenantID: tenantA, Name: "Acme"}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("supplier name unique: %v", err)
	}
	if err := db.CreateSupplier(ctx, store.Supplier{ID: store.NewID(), TenantID: tenantB, Name: "Acme"}); err != nil {
		t.Fatalf("unique is per tenant: %v", err)
	}
	hq := store.Location{ID: store.NewID(), TenantID: tenantA, Name: "HQ", Path: "HQ"}
	fl := store.Location{ID: store.NewID(), TenantID: tenantA, Name: "Floor 1", ParentID: hq.ID, Path: "HQ / Floor 1"}
	if err := db.CreateLocation(ctx, hq); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateLocation(ctx, fl); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateLocation(ctx, store.Location{ID: store.NewID(), TenantID: tenantA, Name: "HQ"}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("location name unique: %v", err)
	}
	root := store.Category{ID: store.NewID(), TenantID: tenantA, Name: "Hardware"}
	child := store.Category{ID: store.NewID(), TenantID: tenantA, Name: "Laptops", ParentID: root.ID}
	if err := db.CreateCategory(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateCategory(ctx, child); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateCategory(ctx, store.Category{ID: store.NewID(), TenantID: tenantA, Name: "Laptops", ParentID: root.ID}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("category name+parent unique: %v", err)
	}
	if err := db.CreateCategory(ctx, store.Category{ID: store.NewID(), TenantID: tenantA, Name: "Laptops"}); err != nil {
		t.Fatalf("same name under a different parent (root): %v", err)
	}
	if err := db.CreateCategory(ctx, store.Category{ID: store.NewID(), TenantID: tenantA, Name: "Laptops"}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("root-level name unique (nil parent coalesced): %v", err)
	}

	// --- assets + tag unique + FK-set-null lookups ---
	pd := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	a := store.Asset{ID: store.NewID(), TenantID: tenantA, AssetTag: "AST-000001", Name: "Laptop", Serial: "SN1", CategoryID: child.ID, SupplierID: sup.ID, LocationID: fl.ID,
		PurchaseDate: &pd, PurchaseCost: 1000, Tags: map[string]string{"env": "prod"}}
	if err := db.CreateAsset(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateAsset(ctx, store.Asset{ID: store.NewID(), TenantID: tenantA, AssetTag: "AST-000001", Name: "Dup"}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("asset tag unique: %v", err)
	}
	got, err := db.GetAsset(ctx, tenantA, a.ID)
	if err != nil || got.CategoryID != child.ID || got.Tags["env"] != "prod" || got.PurchaseDate == nil || got.Status != store.AssetDeployable {
		t.Fatalf("get asset: %+v %v", got, err)
	}
	if g, err := db.FindAssetByTag(ctx, tenantA, "AST-000001"); err != nil || g.ID != a.ID {
		t.Fatal(g, err)
	}
	if g, err := db.FindAssetBySerial(ctx, tenantA, "SN1"); err != nil || g.ID != a.ID {
		t.Fatal(g, err)
	}
	if _, err := db.FindAssetBySerial(ctx, tenantA, ""); !errors.Is(err, repo.ErrNotFound) {
		t.Fatal("empty serial must not match")
	}
	// Computed counts on trees.
	if c, _ := db.GetCategory(ctx, tenantA, root.ID); c.ChildCount != 1 || c.AssetCount != 0 {
		t.Fatalf("root counts: %+v", c)
	}
	if c, _ := db.GetCategory(ctx, tenantA, child.ID); c.AssetCount != 1 {
		t.Fatalf("child counts: %+v", c)
	}
	if l, _ := db.GetLocation(ctx, tenantA, hq.ID); l.ChildCount != 1 {
		t.Fatalf("hq counts: %+v", l)
	}
	if n, _ := db.CountAssetsBySupplier(ctx, tenantA, sup.ID); n != 1 {
		t.Fatal("count by supplier")
	}
	// Delete guards.
	if err := db.DeleteCategory(ctx, tenantA, root.ID); !errors.Is(err, repo.ErrNotEmpty) {
		t.Fatalf("category with children: %v", err)
	}
	if err := db.DeleteLocation(ctx, tenantA, fl.ID); !errors.Is(err, repo.ErrNotEmpty) {
		t.Fatalf("location with assets: %v", err)
	}
	// Supplier delete SET NULLs the asset reference.
	if err := db.DeleteSupplier(ctx, tenantA, sup.ID); err != nil {
		t.Fatal(err)
	}
	if g, _ := db.GetAsset(ctx, tenantA, a.ID); g.SupplierID != "" {
		t.Fatal("supplier FK should be set null")
	}
	// List filters + cursor.
	b := store.Asset{ID: store.NewID(), TenantID: tenantA, AssetTag: "AST-000002", Name: "Monitor", Status: store.AssetBroken}
	if err := db.CreateAsset(ctx, b); err != nil {
		t.Fatal(err)
	}
	if rows, _ := db.ListAssets(ctx, tenantA, store.AssetFilter{Query: "lap"}); len(rows) != 1 || rows[0].ID != a.ID {
		t.Fatalf("query filter: %+v", rows)
	}
	if rows, _ := db.ListAssets(ctx, tenantA, store.AssetFilter{Status: store.AssetBroken}); len(rows) != 1 {
		t.Fatal("status filter")
	}
	page1, _ := db.ListAssets(ctx, tenantA, store.AssetFilter{Limit: 1})
	page2, _ := db.ListAssets(ctx, tenantA, store.AssetFilter{Limit: 1, CursorID: page1[0].ID})
	if len(page1) != 1 || len(page2) != 1 || page1[0].ID == page2[0].ID {
		t.Fatal("cursor paging")
	}
	// Update. A stale reference to the deleted supplier is refused as not found.
	got.Name, got.Status, got.UserID = "Laptop 2", store.AssetAssigned, "user-1"
	if err := db.UpdateAsset(ctx, got); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("stale fk: %v", err)
	}
	got.SupplierID = ""
	if err := db.UpdateAsset(ctx, got); err != nil {
		t.Fatal(err)
	}
	if g, _ := db.GetAsset(ctx, tenantA, a.ID); g.Name != "Laptop 2" || g.UserID != "user-1" {
		t.Fatal("update")
	}
	if err := db.UpdateAsset(ctx, store.Asset{ID: store.NewID(), TenantID: tenantA}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatal("update missing")
	}

	// --- assignments ---
	if err := db.InsertAssignment(ctx, store.Assignment{TenantID: tenantA, AssetID: a.ID, UserID: "user-1", Action: store.ActionAssigned}); err != nil {
		t.Fatal(err)
	}
	if err := db.CloseActiveAssignment(ctx, tenantA, a.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertAssignment(ctx, store.Assignment{TenantID: tenantA, AssetID: a.ID, UserID: "user-1", Action: store.ActionUnassigned}); err != nil {
		t.Fatal(err)
	}
	hist, err := db.ListAssignments(ctx, tenantA, a.ID, 10)
	if err != nil || len(hist) != 2 || hist[0].Action != store.ActionUnassigned || hist[1].ReturnedAt == nil {
		t.Fatalf("history: %+v %v", hist, err)
	}

	// --- documents (polymorphic, storage key unique) ---
	doc := store.Document{ID: store.NewID(), TenantID: tenantA, EntityType: store.EntityAsset, EntityID: a.ID, FileName: "inv.pdf", StorageKey: "tenants/" + tenantA + "/documents/x"}
	if err := db.InsertDocument(ctx, doc); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertDocument(ctx, store.Document{ID: store.NewID(), TenantID: tenantA, EntityType: store.EntityAsset, EntityID: a.ID, StorageKey: doc.StorageKey}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("storage key unique: %v", err)
	}
	if rows, _ := db.ListDocuments(ctx, tenantA, store.EntityAsset, a.ID); len(rows) != 1 {
		t.Fatal("list docs")
	}
	if d, err := db.GetDocument(ctx, tenantA, doc.ID); err != nil || d.FileName != "inv.pdf" {
		t.Fatal(d, err)
	}

	// --- consumables, licenses, insurance + policy assets ---
	con := store.Consumable{ID: store.NewID(), TenantID: tenantA, Name: "Toner", Amount: 1, MinAmount: 2, CategoryID: root.ID}
	if err := db.CreateConsumable(ctx, con); err != nil {
		t.Fatal(err)
	}
	con.Amount = 5
	if err := db.UpdateConsumable(ctx, con); err != nil {
		t.Fatal(err)
	}
	if rows, _ := db.ListConsumables(ctx, tenantA, store.ListOpts{Query: "ton"}); len(rows) != 1 || rows[0].Amount != 5 {
		t.Fatal("consumables")
	}
	to := time.Now().Add(24 * time.Hour)
	lic := store.License{ID: store.NewID(), TenantID: tenantA, Name: "Office", ValidTo: &to, Metadata: map[string]string{"seats": "10"}}
	if err := db.CreateLicense(ctx, lic); err != nil {
		t.Fatal(err)
	}
	if l, _ := db.GetLicense(ctx, tenantA, lic.ID); l.ValidTo == nil || l.Metadata["seats"] != "10" || l.Status != store.LicActive {
		t.Fatalf("license: %+v", l)
	}
	lic.Status = store.LicExpired
	if err := db.UpdateLicense(ctx, lic); err != nil {
		t.Fatal(err)
	}
	pol := store.InsurancePolicy{ID: store.NewID(), TenantID: tenantA, Name: "Fleet", PolicyNumber: "P-1"}
	if err := db.CreateInsurance(ctx, pol); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateInsurance(ctx, store.InsurancePolicy{ID: store.NewID(), TenantID: tenantA, Name: "Other", PolicyNumber: "P-1"}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("policy number unique: %v", err)
	}
	if err := db.CreateInsurance(ctx, store.InsurancePolicy{ID: store.NewID(), TenantID: tenantA, Name: "Fleet", PolicyNumber: "P-2"}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("policy name unique: %v", err)
	}
	if err := db.AddPolicyAsset(ctx, store.PolicyAsset{TenantID: tenantA, PolicyID: pol.ID, AssetID: a.ID, CoveredValue: 900}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddPolicyAsset(ctx, store.PolicyAsset{TenantID: tenantA, PolicyID: pol.ID, AssetID: a.ID}); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("policy-asset pair unique: %v", err)
	}
	if err := db.AddPolicyAsset(ctx, store.PolicyAsset{TenantID: tenantA, PolicyID: pol.ID, AssetID: store.NewID()}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("missing asset: %v", err)
	}
	if p, _ := db.GetInsurance(ctx, tenantA, pol.ID); p.AssetCount != 1 {
		t.Fatalf("asset_count: %+v", p)
	}
	if rows, _ := db.ListPolicyAssets(ctx, tenantA, pol.ID); len(rows) != 1 || rows[0].CoveredValue != 900 {
		t.Fatal("policy assets")
	}
	if rows, _ := db.ListInsurance(ctx, tenantA, store.ListOpts{Query: "P-1", Limit: 5}); len(rows) != 1 {
		t.Fatal("insurance list query")
	}
	if err := db.RemovePolicyAsset(ctx, tenantA, pol.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RemovePolicyAsset(ctx, tenantA, pol.ID, a.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatal(err)
	}

	// --- notify state dedup ---
	if _, err := db.GetNotifyState(ctx, tenantA, "license:"+lic.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatal(err)
	}
	if err := db.UpsertNotifyState(ctx, store.NotifyState{TenantID: tenantA, ConditionKey: "license:" + lic.ID, LastState: "soon"}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNotifyState(ctx, store.NotifyState{TenantID: tenantA, ConditionKey: "license:" + lic.ID, LastState: "expired"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := db.GetNotifyState(ctx, tenantA, "license:"+lic.ID); n.LastState != "expired" {
		t.Fatal("upsert")
	}

	// --- stats, tenants, audit ---
	st, err := db.TenantStats(ctx, tenantA)
	if err != nil || st.TotalAssets != 2 || st.AssetsByStatus[store.AssetAssigned] != 1 || st.TotalCost != 1000 || st.TotalLicenses != 1 || st.TotalInsurance != 1 || st.TotalConsumables != 1 {
		t.Fatalf("stats: %+v %v", st, err)
	}
	ids, err := db.TenantIDs(ctx)
	if err != nil || len(ids) != 1 || ids[0] != tenantA {
		t.Fatalf("tenants: %v %v", ids, err)
	}
	if err := db.AppendAudit(ctx, store.AuditRow{TenantID: tenantA, ActorKind: "user", ActorID: "u", Action: "asset_created", SubjectKind: "asset", SubjectID: a.ID, Outcome: "ok", Detail: map[string]any{"k": "v"}}); err != nil {
		t.Fatal(err)
	}

	// --- RLS: tenant B sees nothing of tenant A and cannot touch its rows ---
	if _, err := db.GetAsset(ctx, tenantB, a.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("rls get: %v", err)
	}
	if rows, _ := db.ListAssets(ctx, tenantB, store.AssetFilter{}); len(rows) != 0 {
		t.Fatal("rls list")
	}
	if err := db.DeleteAsset(ctx, tenantB, a.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("rls delete: %v", err)
	}
	if rows, _ := db.ListDocuments(ctx, tenantB, store.EntityAsset, a.ID); len(rows) != 0 {
		t.Fatal("rls documents")
	}
	if rows, _ := db.ListCategories(ctx, tenantB); len(rows) != 0 {
		t.Fatal("rls categories")
	}
	stB, _ := db.TenantStats(ctx, tenantB)
	if stB.TotalAssets != 0 {
		t.Fatal("rls stats")
	}

	// --- cascades: deleting the asset removes its assignments/documents/links ---
	if err := db.DeleteAsset(ctx, tenantA, a.ID); err != nil {
		t.Fatal(err)
	}
	if rows, _ := db.ListAssignments(ctx, tenantA, a.ID, 0); len(rows) != 0 {
		t.Fatal("assignments cascade")
	}
	if rows, _ := db.ListDocuments(ctx, tenantA, store.EntityAsset, a.ID); len(rows) != 0 {
		t.Fatal("documents removed")
	}
	if err := db.DeleteLicense(ctx, tenantA, lic.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteConsumable(ctx, tenantA, con.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteInsurance(ctx, tenantA, pol.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteAsset(ctx, tenantA, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteLocation(ctx, tenantA, fl.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteLocation(ctx, tenantA, hq.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteCategory(ctx, tenantA, child.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteCategory(ctx, tenantA, root.ID); err != nil {
		t.Fatal(err)
	}
}
