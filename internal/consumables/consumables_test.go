package consumables

import (
	"context"
	"errors"
	"testing"

	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/memstore"
	"github.com/go-freya/freya/services/asset/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var subj = authz.Subjects{TenantID: tenant, UserID: "u", ActorKind: authz.ActorUser}

func ctx() context.Context { return context.Background() }

func TestCRUDAndLowStock(t *testing.T) {
	mem := memstore.New()
	s := New(mem, nil)
	_ = mem.CreateCategory(ctx(), store.Category{ID: "c1", TenantID: tenant, Name: "Toner"})
	c, err := s.Create(ctx(), subj, Input{Name: "Black toner", Amount: 10, MinAmount: 3, CategoryID: "c1"})
	if err != nil || c.LowStock {
		t.Fatalf("%+v %v", c, err)
	}
	for _, in := range []Input{{}, {Name: "x", Amount: -1}, {Name: "x", CategoryID: "nope"}, {Name: "x", SupplierID: "nope"}, {Name: "x", LocationID: "nope"}} {
		if _, err := s.Create(ctx(), subj, in); err == nil {
			t.Fatalf("validation: %+v", in)
		}
	}
	if _, err := s.Create(ctx(), authz.Subjects{}, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	up, err := s.Update(ctx(), subj, c.ID, Input{Name: "Black toner", Amount: 3, MinAmount: 3})
	if err != nil || !up.LowStock {
		t.Fatalf("low stock at amount<=min: %+v %v", up, err)
	}
	if g, _ := s.Get(ctx(), subj, c.ID); !g.LowStock || g.Amount != 3 {
		t.Fatalf("%+v", g)
	}
	if _, err := s.Update(ctx(), subj, "missing", Input{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx(), subj, c.ID, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Update(ctx(), subj, c.ID, Input{Name: "x", CategoryID: "nope"}); err == nil {
		t.Fatal("bad ref")
	}
	if _, err := s.Update(ctx(), authz.Subjects{}, c.ID, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	list, err := s.List(ctx(), subj, store.ListOpts{Query: "toner"})
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if _, err := s.Get(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx(), authz.Subjects{}, c.ID); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.List(ctx(), authz.Subjects{}, store.ListOpts{}); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), authz.Subjects{}, c.ID); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), subj, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), subj, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for _, m := range []string{"CreateConsumable", "ListConsumables", "UpdateConsumable"} {
		mem.FailNext(m)
		var err error
		switch m {
		case "CreateConsumable":
			_, err = s.Create(ctx(), subj, Input{Name: "x"})
		case "ListConsumables":
			_, err = s.List(ctx(), subj, store.ListOpts{})
		default:
			x, _ := s.Create(ctx(), subj, Input{Name: "y"})
			_, err = s.Update(ctx(), subj, x.ID, Input{Name: "y"})
		}
		if err == nil {
			t.Fatalf("%s: want error", m)
		}
	}
	if IsLowStock(store.Consumable{Amount: 0, MinAmount: 0}) {
		t.Fatal("no threshold → never low")
	}
}
