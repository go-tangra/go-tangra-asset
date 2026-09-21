package stats

import (
	"context"
	"testing"
	"time"

	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/config"
	"github.com/go-freya/freya/services/asset/internal/memstore"
	"github.com/go-freya/freya/services/asset/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var subj = authz.Subjects{TenantID: tenant, UserID: "u", ActorKind: authz.ActorUser}

func ctx() context.Context { return context.Background() }

func TestDashboard(t *testing.T) {
	mem := memstore.New()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	s := New(mem, config.SoonWindows{})
	s.SetClock(func() time.Time { return now })
	pd := now.AddDate(-1, 0, 0)
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "1", Name: "a", Status: store.AssetAssigned, PurchaseCost: 1000, PurchaseDate: &pd, UsefulLifeYears: 5, DepreciationRate: 0.4, WarrantyMonths: 36})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a2", TenantID: tenant, AssetTag: "2", Name: "b", Status: store.AssetDeployable, PurchaseCost: 500})
	soonPD := now.AddDate(0, -12, 5)
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a3", TenantID: tenant, AssetTag: "3", Name: "c", Status: store.AssetBroken, PurchaseDate: &soonPD, WarrantyMonths: 12})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "ax", TenantID: "22222222-2222-7222-8222-222222222222", AssetTag: "x", Name: "x", PurchaseCost: 99999})
	soon := now.Add(24 * time.Hour)
	_ = mem.CreateLicense(ctx(), store.License{ID: "l1", TenantID: tenant, Name: "l", Status: store.LicActive, ValidTo: &soon})
	_ = mem.CreateLicense(ctx(), store.License{ID: "l2", TenantID: tenant, Name: "l2", Status: store.LicExpired, ValidTo: &soon})
	_ = mem.CreateInsurance(ctx(), store.InsurancePolicy{ID: "p1", TenantID: tenant, Name: "p", PolicyNumber: "1", Status: store.InsActive, ValidTo: &soon})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c1", TenantID: tenant, Name: "c", Amount: 1, MinAmount: 1})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c2", TenantID: tenant, Name: "d", Amount: 9, MinAmount: 1})
	_ = mem.CreateCategory(ctx(), store.Category{ID: "k", TenantID: tenant, Name: "k"})
	_ = mem.CreateSupplier(ctx(), store.Supplier{ID: "s", TenantID: tenant, Name: "s"})
	_ = mem.CreateLocation(ctx(), store.Location{ID: "loc", TenantID: tenant, Name: "loc"})

	d, err := s.Get(ctx(), subj)
	if err != nil {
		t.Fatal(err)
	}
	if d.TotalAssets != 3 || d.AssetsByStatus[store.AssetAssigned] != 1 || d.AssignedAssets != 1 {
		t.Fatalf("%+v", d)
	}
	if d.TotalCost != 1500 {
		t.Fatalf("cost %v", d.TotalCost)
	}
	// a1: 1000*(0.6)^1 = 600; a2 not depreciable → 500; a3 no cost → 0.
	if d.TotalDepreciatedValue < 1099 || d.TotalDepreciatedValue > 1101 {
		t.Fatalf("book %v", d.TotalDepreciatedValue)
	}
	if d.WarrantyExpiringSoon != 1 || d.LicensesExpiringSoon != 1 || d.InsuranceExpiringSoon != 1 || d.ExpiringSoon != 3 {
		t.Fatalf("expiring: %+v", d)
	}
	if d.LowStock != 1 || d.TotalConsumables != 2 || d.TotalCategories != 1 || d.TotalSuppliers != 1 || d.TotalLocations != 1 || d.TotalLicenses != 2 || d.TotalInsurance != 1 {
		t.Fatalf("counts: %+v", d)
	}
	if _, err := s.Get(ctx(), authz.Subjects{}); err == nil {
		t.Fatal("forbidden")
	}
	for _, m := range []string{"TenantStats", "AllAssets", "AllLicenses", "AllInsurance", "AllConsumables"} {
		mem.FailNext(m)
		if _, err := s.Get(ctx(), subj); err == nil {
			t.Fatalf("%s: want error", m)
		}
	}
	// Empty tenant → empty map, zero values.
	e, err := New(memstore.New(), config.SoonWindows{Warranty: time.Hour, License: time.Hour, Insurance: time.Hour}).Get(ctx(), subj)
	if err != nil || e.AssetsByStatus == nil || e.TotalAssets != 0 {
		t.Fatal(e, err)
	}
}
