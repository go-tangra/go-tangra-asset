package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/config"
	"github.com/go-tangra/go-tangra-asset/v4/internal/events"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

type fakePub struct {
	mu   sync.Mutex
	evts []string
}

func (p *fakePub) Publish(_ context.Context, _ string, typ string, _ any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evts = append(p.evts, typ)
}

func (p *fakePub) count(typ string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, e := range p.evts {
		if e == typ {
			n++
		}
	}
	return n
}

func ctx() context.Context { return context.Background() }

func TestSweepEvaluatesAndDedups(t *testing.T) {
	mem := memstore.New()
	pub := &fakePub{}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	s := New(mem, pub, nil, config.SoonWindows{}, nil)
	s.SetClock(func() time.Time { return now })

	// Warranty: expiring in 10 days (soon), expired, far away, archived.
	soonPD := now.AddDate(0, -12, 10)
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a-soon", TenantID: tenant, AssetTag: "S", Name: "s", PurchaseDate: &soonPD, WarrantyMonths: 12})
	pastPD := now.AddDate(-2, 0, 0)
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a-past", TenantID: tenant, AssetTag: "P", Name: "p", PurchaseDate: &pastPD, WarrantyMonths: 12})
	farPD := now
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a-far", TenantID: tenant, AssetTag: "F", Name: "f", PurchaseDate: &farPD, WarrantyMonths: 36})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a-arch", TenantID: tenant, AssetTag: "A", Name: "a", PurchaseDate: &pastPD, WarrantyMonths: 1, Status: store.AssetArchived})
	// Licenses: one lapsed (auto-expire), one soon, one suspended (ignored).
	lapsed := now.Add(-time.Hour)
	soon := now.Add(48 * time.Hour)
	_ = mem.CreateLicense(ctx(), store.License{ID: "l-lapsed", TenantID: tenant, Name: "L1", Status: store.LicActive, ValidTo: &lapsed})
	_ = mem.CreateLicense(ctx(), store.License{ID: "l-soon", TenantID: tenant, Name: "L2", Status: store.LicActive, ValidTo: &soon})
	_ = mem.CreateLicense(ctx(), store.License{ID: "l-susp", TenantID: tenant, Name: "L3", Status: store.LicSuspended, ValidTo: &lapsed})
	// Insurance: one lapsed, one cancelled (ignored).
	_ = mem.CreateInsurance(ctx(), store.InsurancePolicy{ID: "p-lapsed", TenantID: tenant, Name: "P1", PolicyNumber: "1", Status: store.InsActive, ValidTo: &lapsed})
	_ = mem.CreateInsurance(ctx(), store.InsurancePolicy{ID: "p-canc", TenantID: tenant, Name: "P2", PolicyNumber: "2", Status: store.InsCancelled, ValidTo: &lapsed})
	// Consumables: one low, one fine, one without threshold.
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c-low", TenantID: tenant, Name: "toner", Amount: 1, MinAmount: 2})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c-ok", TenantID: tenant, Name: "cable", Amount: 10, MinAmount: 2})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c-none", TenantID: tenant, Name: "misc", Amount: 0, MinAmount: 0})

	reps := s.SweepAll(ctx())
	if len(reps) != 1 {
		t.Fatalf("reports: %+v", reps)
	}
	r := reps[0]
	if r.WarrantyExpiring != 2 || r.LicenseEvents != 2 || r.LicensesExpired != 1 || r.InsuranceEvents != 1 || r.InsuranceExpired != 1 || r.LowStock != 1 || r.Errors != 0 {
		t.Fatalf("report: %+v", r)
	}
	if l, _ := mem.GetLicense(ctx(), tenant, "l-lapsed"); l.Status != store.LicExpired {
		t.Fatal("license not auto-expired")
	}
	if p, _ := mem.GetInsurance(ctx(), tenant, "p-lapsed"); p.Status != store.InsExpired {
		t.Fatal("policy not auto-expired")
	}
	if pub.count(events.WarrantyExpiring) != 2 || pub.count(events.LicenseExpiring) != 2 || pub.count(events.InsuranceExpiring) != 1 || pub.count(events.ConsumableLowStock) != 1 {
		t.Fatalf("events: %v", pub.evts)
	}
	// A second pass re-alerts nothing.
	r2 := s.Sweep(ctx(), tenant)
	if r2.WarrantyExpiring+r2.LicenseEvents+r2.InsuranceEvents+r2.LowStock != 0 || r2.LicensesExpired != 0 {
		t.Fatalf("dedup: %+v", r2)
	}
	// Stock recovers then dips again → alerts again; a changed low amount also re-alerts.
	c, _ := mem.GetConsumable(ctx(), tenant, "c-low")
	c.Amount = 5
	_ = mem.UpdateConsumable(ctx(), c)
	if r := s.Sweep(ctx(), tenant); r.LowStock != 0 {
		t.Fatal("recovered must not alert")
	}
	c.Amount = 0
	_ = mem.UpdateConsumable(ctx(), c)
	if r := s.Sweep(ctx(), tenant); r.LowStock != 1 {
		t.Fatal("recurring condition must alert again")
	}
	// The soon warranty moves to expired → a new state, new event.
	s.SetClock(func() time.Time { return now.AddDate(0, 0, 20) })
	if r := s.Sweep(ctx(), tenant); r.WarrantyExpiring != 1 {
		t.Fatalf("state change must alert: %+v", r)
	}
}

func seedErr(mem *memstore.Mem) {
	lapsed := time.Now().Add(-time.Hour)
	_ = mem.CreateLicense(ctx(), store.License{ID: "l", TenantID: tenant, Name: "L", Status: store.LicActive, ValidTo: &lapsed})
	_ = mem.CreateInsurance(ctx(), store.InsurancePolicy{ID: "p", TenantID: tenant, Name: "P", PolicyNumber: "1", Status: store.InsActive, ValidTo: &lapsed})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c", TenantID: tenant, Name: "c", Amount: 0, MinAmount: 1})
	pd := time.Now().AddDate(-5, 0, 0)
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a", TenantID: tenant, AssetTag: "A", Name: "a", PurchaseDate: &pd, WarrantyMonths: 1})
}

func TestSweepErrors(t *testing.T) {
	for _, m := range []string{"AllAssets", "AllLicenses", "AllInsurance", "AllConsumables", "UpsertNotifyState", "UpdateLicense", "UpdateInsurance"} {
		mem := memstore.New()
		seedErr(mem)
		s := New(mem, nil, nil, config.SoonWindows{}, nil)
		mem.FailNext(m)
		if r := s.Sweep(ctx(), tenant); r.Errors == 0 {
			t.Fatalf("%s: error not reported", m)
		}
	}
	mem := memstore.New()
	seedErr(mem)
	s := New(mem, nil, nil, config.SoonWindows{}, nil)
	mem.FailNext("TenantIDs")
	if reps := s.SweepAll(ctx()); reps != nil {
		t.Fatal("enumeration failure must yield nil")
	}
	// Run stops on ctx cancellation after the first sweep.
	c, cancel := context.WithCancel(ctx())
	done := make(chan struct{})
	go func() { s.Run(c, 0); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
	// Run with a real tick.
	c2, cancel2 := context.WithTimeout(ctx(), 50*time.Millisecond)
	defer cancel2()
	s.Run(c2, 10*time.Millisecond)
	// Cancelled context mid SweepAll returns early.
	c3, cancel3 := context.WithCancel(ctx())
	cancel3()
	if reps := s.SweepAll(c3); len(reps) != 0 {
		t.Fatal("cancelled sweep should stop")
	}
}
