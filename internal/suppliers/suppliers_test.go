package suppliers

import (
	"context"
	"errors"
	"testing"

	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/memstore"
	"github.com/go-freya/freya/services/asset/internal/sealed"
	"github.com/go-freya/freya/services/asset/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var (
	admin  = authz.Subjects{TenantID: tenant, UserID: "u", Roles: []string{"admin"}, ActorKind: authz.ActorUser}
	member = authz.Subjects{TenantID: tenant, UserID: "m", Roles: []string{"member"}, ActorKind: authz.ActorUser}
)

func ctx() context.Context { return context.Background() }

func newSvc(t *testing.T) (*Service, *memstore.Mem) {
	t.Helper()
	env, err := sealed.NewEnvelope(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	mem := memstore.New()
	return New(mem, env, nil), mem
}

func TestCRUDAndPIISealing(t *testing.T) {
	s, mem := newSvc(t)
	sp, err := s.Create(ctx(), admin, Input{Name: "Acme", ContactPerson: "Ann", Telephone: "+1 555", Email: "ann@acme.example"})
	if err != nil {
		t.Fatal(err)
	}
	if sp.Email != "ann@acme.example" || sp.ContactPerson != "Ann" || sp.Status != store.SupActive {
		t.Fatalf("admin sees clear contact: %+v", sp)
	}
	raw, _ := mem.GetSupplier(ctx(), tenant, sp.ID)
	if raw.Email == "ann@acme.example" || raw.Telephone == "+1 555" || raw.ContactPerson == "Ann" {
		t.Fatalf("contact PII stored in clear: %+v", raw)
	}
	m, err := s.Get(ctx(), member, sp.ID)
	if err != nil || m.Email != "" || m.Telephone != "" || m.ContactPerson != "" {
		t.Fatalf("member must see redacted contact: %+v %v", m, err)
	}
	if m.Name != "Acme" {
		t.Fatal("non-PII fields visible")
	}
	if _, err := s.Create(ctx(), admin, Input{Name: "Acme"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup name: %v", err)
	}
	for _, in := range []Input{{}, {Name: "x", Status: "odd"}, {Name: "x", Email: string(make([]byte, 600))}} {
		if _, err := s.Create(ctx(), admin, in); err == nil {
			t.Fatalf("validation: %+v", in)
		}
	}
	if _, err := s.Create(ctx(), authz.Subjects{}, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	list, err := s.List(ctx(), member, store.ListOpts{})
	if err != nil || len(list) != 1 || list[0].Email != "" {
		t.Fatalf("%v %v", list, err)
	}
	al, _ := s.List(ctx(), admin, store.ListOpts{Query: "ac"})
	if len(al) != 1 || al[0].Email != "ann@acme.example" {
		t.Fatalf("admin list: %+v", al)
	}
	up, err := s.Update(ctx(), admin, sp.ID, Input{Name: "Acme Inc", Email: "new@acme.example", Status: store.SupInactive})
	if err != nil || up.Email != "new@acme.example" || up.Status != store.SupInactive || up.Name != "Acme Inc" {
		t.Fatalf("%+v %v", up, err)
	}
	if _, err := s.Update(ctx(), admin, "missing", Input{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx(), admin, sp.ID, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Update(ctx(), authz.Subjects{}, sp.ID, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	// A tampered ciphertext is redacted, never surfaced.
	raw, _ = mem.GetSupplier(ctx(), tenant, sp.ID)
	raw.Email = "bm90IGEgY2lwaGVydGV4dA=="
	_ = mem.UpdateSupplier(ctx(), raw)
	if g, _ := s.Get(ctx(), admin, sp.ID); g.Email != "" {
		t.Fatalf("tampered must redact: %q", g.Email)
	}
	// Delete guard.
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T", Name: "n", SupplierID: sp.ID})
	if err := s.Delete(ctx(), admin, sp.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("in use: %v", err)
	}
	_ = mem.DeleteAsset(ctx(), tenant, "a1")
	if err := s.Delete(ctx(), admin, sp.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), admin, sp.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), authz.Subjects{}, sp.ID); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.Get(ctx(), authz.Subjects{}, sp.ID); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.List(ctx(), authz.Subjects{}, store.ListOpts{}); err == nil {
		t.Fatal("forbidden")
	}
}

func TestStoreFailures(t *testing.T) {
	s, mem := newSvc(t)
	sp, _ := s.Create(ctx(), admin, Input{Name: "A"})
	mem.FailNext("CreateSupplier")
	if _, err := s.Create(ctx(), admin, Input{Name: "B"}); err == nil {
		t.Fatal("want err")
	}
	mem.FailNext("ListSuppliers")
	if _, err := s.List(ctx(), admin, store.ListOpts{}); err == nil {
		t.Fatal("want err")
	}
	mem.FailNext("UpdateSupplier")
	if _, err := s.Update(ctx(), admin, sp.ID, Input{Name: "A"}); err == nil {
		t.Fatal("want err")
	}
	mem.FailNext("CountAssetsBySupplier")
	if err := s.Delete(ctx(), admin, sp.ID); err == nil {
		t.Fatal("want err")
	}
	mem.FailNext("DeleteSupplier")
	if err := s.Delete(ctx(), admin, sp.ID); err == nil {
		t.Fatal("want err")
	}
}
