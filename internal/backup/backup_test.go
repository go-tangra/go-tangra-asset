package backup

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

const (
	tenant = "11111111-1111-7111-8111-111111111111"
	other  = "22222222-2222-7222-8222-222222222222"
)

var (
	admin    = authz.Subjects{TenantID: tenant, UserID: "u", Roles: []string{"member"}, ActorKind: authz.ActorUser}
	platform = authz.Subjects{TenantID: tenant, UserID: "p", Roles: []string{"platform-admin"}, ActorKind: authz.ActorUser}
	osubj    = authz.Subjects{TenantID: other, UserID: "o", ActorKind: authz.ActorUser}
)

func ctx() context.Context { return context.Background() }

func seed(t *testing.T, mem *memstore.Mem) {
	t.Helper()
	_ = mem.CreateSupplier(ctx(), store.Supplier{ID: "s1", TenantID: tenant, Name: "Acme", Email: "SEALED-EMAIL", ContactPerson: "SEALED-CONTACT"})
	_ = mem.CreateLocation(ctx(), store.Location{ID: "l1", TenantID: tenant, Name: "HQ", Phone: "SEALED-PHONE"})
	_ = mem.CreateLocation(ctx(), store.Location{ID: "l2", TenantID: tenant, Name: "Floor", ParentID: "l1", Path: "HQ / Floor"})
	_ = mem.CreateCategory(ctx(), store.Category{ID: "c1", TenantID: tenant, Name: "HW"})
	_ = mem.CreateCategory(ctx(), store.Category{ID: "c2", TenantID: tenant, Name: "Laptops", ParentID: "c1"})
	pd := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T1", Name: "Laptop", CategoryID: "c2", SupplierID: "s1", LocationID: "l2", PurchaseCost: 1000, PurchaseDate: &pd, Status: store.AssetAssigned, UserID: "u9"})
	_ = mem.InsertAssignment(ctx(), store.Assignment{ID: "g1", TenantID: tenant, AssetID: "a1", UserID: "u9", Action: store.ActionAssigned})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "k1", TenantID: tenant, Name: "Toner", Amount: 3, MinAmount: 1})
	_ = mem.CreateLicense(ctx(), store.License{ID: "x1", TenantID: tenant, Name: "Office", Status: store.LicActive})
	_ = mem.CreateInsurance(ctx(), store.InsurancePolicy{ID: "p1", TenantID: tenant, Name: "Fleet", PolicyNumber: "P-1", Status: store.InsActive})
	_ = mem.AddPolicyAsset(ctx(), store.PolicyAsset{ID: "pa1", TenantID: tenant, PolicyID: "p1", AssetID: "a1", CoveredValue: 900})
	_ = mem.InsertDocument(ctx(), store.Document{ID: "d1", TenantID: tenant, EntityType: store.EntityAsset, EntityID: "a1", FileName: "inv.pdf", StorageKey: "tenants/" + tenant + "/documents/d1"})
	_ = mem.InsertDocument(ctx(), store.Document{ID: "d2", TenantID: tenant, EntityType: store.EntityConsumable, EntityID: "k1", FileName: "sds.txt", StorageKey: "tenants/" + tenant + "/documents/d2"})
	_ = mem.InsertDocument(ctx(), store.Document{ID: "d3", TenantID: tenant, EntityType: store.EntityLicense, EntityID: "x1", FileName: "key.txt", StorageKey: "tenants/" + tenant + "/documents/d3"})
	// Another tenant's data must never leak.
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "ax", TenantID: other, AssetTag: "TX", Name: "Other"})
}

func TestExportRoundTrip(t *testing.T) {
	mem := memstore.New()
	seed(t, mem)
	s := New(mem, nil)
	b, err := s.Export(ctx(), admin)
	if err != nil {
		t.Fatal(err)
	}
	if b.SchemaVersion != SchemaVersion || b.TenantID != tenant {
		t.Fatalf("%+v", b)
	}
	if len(b.Assets) != 1 || len(b.Suppliers) != 1 || len(b.Locations) != 2 || len(b.Categories) != 2 || len(b.Assignments) != 1 ||
		len(b.Consumables) != 1 || len(b.Licenses) != 1 || len(b.Insurance) != 1 || len(b.PolicyAssets) != 1 || len(b.Documents) != 3 {
		t.Fatalf("counts: %+v", b)
	}
	raw, _ := json.Marshal(b)
	for _, secret := range []string{"SEALED", "TX", "Other"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("export leaks %q", secret)
		}
	}
	// Restore into a clean tenant (cross-tenant → platform admin).
	if _, err := s.Import(ctx(), admin, b, Options{TenantID: other}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("cross-tenant by non-admin: %v", err)
	}
	if _, err := s.Import(ctx(), admin, b, Options{Full: true}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("full by non-admin: %v", err)
	}
	target := "33333333-3333-7333-8333-333333333333"
	res, err := s.Import(ctx(), platform, b, Options{TenantID: target})
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported["assets"] != 1 || res.Imported["suppliers"] != 1 || res.Imported["locations"] != 2 || res.Imported["categories"] != 2 ||
		res.Imported["assignments"] != 1 || res.Imported["consumables"] != 1 || res.Imported["licenses"] != 1 || res.Imported["insurance_policies"] != 1 || res.Imported["policy_assets"] != 1 {
		t.Fatalf("%+v", res)
	}
	if res.Skipped["documents"] != 3 {
		t.Fatalf("documents from another tenant's key space must be skipped: %+v", res)
	}
	// Cross-tenant restores get fresh ids (primary keys are global) with the
	// foreign keys rewritten consistently.
	a, err := mem.FindAssetByTag(ctx(), target, "T1")
	if err != nil || a.ID == "a1" || a.CategoryID == "c2" || a.CategoryID == "" || a.TenantID != target {
		t.Fatalf("remapped: %+v %v", a, err)
	}
	if c, err := mem.GetCategory(ctx(), target, a.CategoryID); err != nil || c.Name != "Laptops" || c.ParentID == "" || c.ParentID == "c1" {
		t.Fatalf("fk rewrite: %+v %v", c, err)
	}
	if asg, _ := mem.ListAssignments(ctx(), target, a.ID, 0); len(asg) != 1 {
		t.Fatal("history restored")
	}
	pols, _ := mem.AllInsurance(ctx(), target)
	if len(pols) != 1 || pols[0].AssetCount != 1 || pols[0].ID == "p1" {
		t.Fatalf("policy asset restored: %+v", pols)
	}
	if src, _ := mem.GetAsset(ctx(), tenant, "a1"); src.TenantID != tenant {
		t.Fatal("origin tenant clobbered")
	}
	// Same-tenant re-import: skip vs overwrite.
	res, err = s.Import(ctx(), admin, b, Options{})
	if err != nil || res.Skipped["assets"] != 1 || res.Imported["assets"] != 0 || res.Skipped["assignments"] != 1 || res.Skipped["policy_assets"] != 1 || res.Skipped["documents"] != 3 {
		t.Fatalf("skip: %+v %v", res, err)
	}
	b.Assets[0].Name = "Renamed"
	res, err = s.Import(ctx(), admin, b, Options{Mode: ModeOverwrite})
	if err != nil || res.Imported["assets"] != 1 || res.Skipped["assignments"] != 1 {
		t.Fatalf("overwrite: %+v %v", res, err)
	}
	if a, _ := mem.GetAsset(ctx(), tenant, "a1"); a.Name != "Renamed" {
		t.Fatal("overwrite did not apply")
	}
	// Full restore wipes first (platform admin).
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "stray", TenantID: tenant, AssetTag: "STRAY", Name: "stray"})
	res, err = s.Import(ctx(), platform, b, Options{Full: true})
	if err != nil || res.Deleted == 0 || res.Imported["assets"] != 1 {
		t.Fatalf("full: %+v %v", res, err)
	}
	if _, err := mem.GetAsset(ctx(), tenant, "stray"); err == nil {
		t.Fatal("full restore must wipe")
	}
	if all, _ := mem.AllAssets(ctx(), other); len(all) != 1 {
		t.Fatal("other tenant touched")
	}
	// Bad schema / oversized / malformed rows.
	bad := b
	bad.SchemaVersion = 99
	if _, err := s.Import(ctx(), admin, bad, Options{}); !errors.Is(err, ErrBadSchema) {
		t.Fatal(err)
	}
	huge := Backup{SchemaVersion: SchemaVersion, Assets: make([]store.Asset, MaxRows+1)}
	if err := Validate(huge); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	for _, v := range []Backup{
		{SchemaVersion: SchemaVersion, Assets: []store.Asset{{ID: "x"}}},
		{SchemaVersion: SchemaVersion, Suppliers: []store.Supplier{{ID: "x"}}},
		{SchemaVersion: SchemaVersion, Locations: []store.Location{{ID: "x"}}},
		{SchemaVersion: SchemaVersion, Categories: []store.Category{{ID: "x"}}},
		{SchemaVersion: SchemaVersion, Insurance: []store.InsurancePolicy{{ID: "x"}}},
	} {
		if err := Validate(v); !errors.Is(err, ErrBadSchema) {
			t.Fatalf("%+v: %v", v, err)
		}
	}
	if _, err := s.Import(ctx(), authz.Subjects{}, b, Options{}); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.Export(ctx(), authz.Subjects{}); err == nil {
		t.Fatal("forbidden")
	}
	if ob, _ := s.Export(ctx(), osubj); len(ob.Assets) != 1 || ob.Assets[0].ID != "ax" {
		t.Fatal("tenant isolation")
	}
}

func TestOrderingAndFailures(t *testing.T) {
	cats := orderCategories([]store.Category{{ID: "c", ParentID: "b"}, {ID: "b", ParentID: "a"}, {ID: "a"}, {ID: "loop", ParentID: "loop"}, {ID: "orphan", ParentID: "zzz"}})
	if cats[0].ID != "a" || cats[1].ID != "b" || cats[2].ID != "c" || len(cats) != 5 {
		t.Fatalf("%+v", cats)
	}
	locs := orderLocations([]store.Location{{ID: "c", ParentID: "b"}, {ID: "b", ParentID: "a"}, {ID: "a"}, {ID: "x", ParentID: "y"}, {ID: "y", ParentID: "x"}})
	if locs[0].ID != "a" || len(locs) != 5 {
		t.Fatalf("%+v", locs)
	}
	mem := memstore.New()
	seed(t, mem)
	s := New(mem, nil)
	b, _ := s.Export(ctx(), admin)
	for _, m := range []string{"ListSuppliers", "ListLocations", "ListCategories", "AllAssets", "ListAssignments", "ListDocuments", "AllConsumables", "AllLicenses", "AllInsurance", "ListPolicyAssets"} {
		mem.FailNext(m)
		if _, err := s.Export(ctx(), admin); err == nil {
			t.Fatalf("export %s: want error", m)
		}
	}
	target := "44444444-4444-7444-8444-444444444444"
	for _, m := range []string{"CreateSupplier", "CreateLocation", "CreateCategory", "CreateAsset", "InsertAssignment", "CreateConsumable", "CreateLicense", "CreateInsurance", "AddPolicyAsset",
		"GetSupplier", "GetLocation", "GetCategory", "GetAsset", "GetConsumable", "GetLicense", "GetInsurance"} {
		fresh := memstore.New()
		fs := New(fresh, nil)
		fresh.FailNext(m)
		if _, err := fs.Import(ctx(), platform, b, Options{TenantID: target}); err == nil {
			t.Fatalf("import %s: want error", m)
		}
	}
	// Overwrite update failures.
	for _, m := range []string{"UpdateSupplier", "UpdateLocation", "UpdateCategory", "UpdateAsset", "UpdateConsumable", "UpdateLicense", "UpdateInsurance"} {
		mem.FailNext(m)
		if _, err := s.Import(ctx(), admin, b, Options{Mode: ModeOverwrite}); err == nil {
			t.Fatalf("overwrite %s: want error", m)
		}
	}
	// Document insert failure (same tenant, new id).
	b2 := b
	b2.Documents = []store.Document{{ID: "dnew", EntityType: store.EntityAsset, EntityID: "a1", StorageKey: "tenants/" + tenant + "/documents/dnew"}}
	mem.FailNext("InsertDocument")
	if _, err := s.Import(ctx(), admin, b2, Options{}); err == nil {
		t.Fatal("document insert failure")
	}
	if res, err := s.Import(ctx(), admin, b2, Options{}); err != nil || res.Imported["documents"] != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	// Duplicate storage key → skipped.
	b2.Documents = []store.Document{{ID: "dnew2", EntityType: store.EntityAsset, EntityID: "a1", StorageKey: "tenants/" + tenant + "/documents/dnew"}}
	if res, _ := s.Import(ctx(), admin, b2, Options{}); res.Skipped["documents"] != 1 {
		t.Fatalf("%+v", res)
	}
	// Wipe failures.
	for _, m := range []string{"AllInsurance", "DeleteInsurance", "AllLicenses", "DeleteLicense", "AllConsumables", "DeleteConsumable", "AllAssets", "DeleteAsset", "ListCategories", "DeleteCategory", "ListLocations", "DeleteLocation", "ListSuppliers", "DeleteSupplier"} {
		fresh := memstore.New()
		seed(t, fresh)
		fs := New(fresh, nil)
		fresh.FailNext(m)
		if _, err := fs.Import(ctx(), platform, b, Options{Full: true}); err == nil {
			t.Fatalf("wipe %s: want error", m)
		}
	}
	// A duplicate policy-asset pair is skipped, not fatal.
	b3 := Backup{SchemaVersion: SchemaVersion, PolicyAssets: []store.PolicyAsset{{ID: "z", PolicyID: "p1", AssetID: "a1"}}}
	if res, err := s.Import(ctx(), admin, b3, Options{}); err != nil || res.Skipped["policy_assets"] != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	mem.FailNext("AddPolicyAsset")
	if _, err := s.Import(ctx(), admin, b3, Options{}); err == nil {
		t.Fatal("policy asset failure")
	}
}
