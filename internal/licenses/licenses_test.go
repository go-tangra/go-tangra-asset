package licenses

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var subj = authz.Subjects{TenantID: tenant, UserID: "u", ActorKind: authz.ActorUser}

func ctx() context.Context { return context.Background() }

func TestCRUD(t *testing.T) {
	mem := memstore.New()
	s := New(mem, nil)
	_ = mem.CreateSupplier(ctx(), store.Supplier{ID: "s1", TenantID: tenant, Name: "MS"})
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, 0)
	l, err := s.Create(ctx(), subj, Input{Name: "Office", SupplierID: "s1", ValidFrom: &from, ValidTo: &to, PurchaseCost: 99})
	if err != nil || l.Status != store.LicActive {
		t.Fatalf("%+v %v", l, err)
	}
	bad := from.AddDate(-1, 0, 0)
	for _, in := range []Input{{}, {Name: "x", Status: "odd"}, {Name: "x", PurchaseCost: -1}, {Name: "x", ValidFrom: &from, ValidTo: &bad}, {Name: "x", SupplierID: "nope"}} {
		if _, err := s.Create(ctx(), subj, in); err == nil {
			t.Fatalf("validation: %+v", in)
		}
	}
	if _, err := s.Create(ctx(), authz.Subjects{}, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	up, err := s.Update(ctx(), subj, l.ID, Input{Name: "Office 365", Status: store.LicSuspended})
	if err != nil || up.Status != store.LicSuspended || up.Name != "Office 365" {
		t.Fatalf("%+v %v", up, err)
	}
	if _, err := s.Update(ctx(), subj, "missing", Input{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx(), subj, l.ID, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Update(ctx(), subj, l.ID, Input{Name: "x", SupplierID: "nope"}); err == nil {
		t.Fatal("bad supplier")
	}
	if _, err := s.Update(ctx(), authz.Subjects{}, l.ID, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	if g, err := s.Get(ctx(), subj, l.ID); err != nil || g.Name != "Office 365" {
		t.Fatal(g, err)
	}
	if _, err := s.Get(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx(), authz.Subjects{}, l.ID); err == nil {
		t.Fatal("forbidden")
	}
	if list, err := s.List(ctx(), subj, store.ListOpts{}); err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	if _, err := s.List(ctx(), authz.Subjects{}, store.ListOpts{}); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), authz.Subjects{}, l.ID); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), subj, l.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), subj, l.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	mem.FailNext("CreateLicense")
	if _, err := s.Create(ctx(), subj, Input{Name: "x"}); err == nil {
		t.Fatal("want err")
	}
	mem.FailNext("ListLicenses")
	if _, err := s.List(ctx(), subj, store.ListOpts{}); err == nil {
		t.Fatal("want err")
	}
	x, _ := s.Create(ctx(), subj, Input{Name: "x"})
	mem.FailNext("UpdateLicense")
	if _, err := s.Update(ctx(), subj, x.ID, Input{Name: "x"}); err == nil {
		t.Fatal("want err")
	}
}
