package locations

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
	member = authz.Subjects{TenantID: tenant, UserID: "m", ActorKind: authz.ActorUser}
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

func TestTreePathAndPII(t *testing.T) {
	s, mem := newSvc(t)
	hq, err := s.Create(ctx(), admin, Input{Name: "HQ", Contact: "Bob", Phone: "1", Email: "bob@x.example"})
	if err != nil {
		t.Fatal(err)
	}
	fl, err := s.Create(ctx(), admin, Input{Name: "Floor 1", ParentID: hq.ID})
	if err != nil {
		t.Fatal(err)
	}
	if fl.Path != "HQ / Floor 1" {
		t.Fatalf("path: %q", fl.Path)
	}
	if _, err := s.Create(ctx(), admin, Input{Name: "Floor 1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("unique name per tenant: %v", err)
	}
	if _, err := s.Create(ctx(), admin, Input{Name: "x", ParentID: "nope"}); err == nil {
		t.Fatal("bad parent")
	}
	for _, in := range []Input{{}, {Name: "x", Status: "odd"}, {Name: "x", Phone: string(make([]byte, 600))}} {
		if _, err := s.Create(ctx(), admin, in); err == nil {
			t.Fatal("validation")
		}
	}
	if _, err := s.Create(ctx(), authz.Subjects{}, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	raw, _ := mem.GetLocation(ctx(), tenant, hq.ID)
	if raw.Email == "bob@x.example" || raw.Contact == "Bob" || raw.Phone == "1" {
		t.Fatalf("PII stored in clear: %+v", raw)
	}
	if g, _ := s.Get(ctx(), admin, hq.ID); g.Email != "bob@x.example" || g.ChildCount != 1 {
		t.Fatalf("admin get: %+v", g)
	}
	if g, _ := s.Get(ctx(), member, hq.ID); g.Email != "" || g.Contact != "" || g.Phone != "" {
		t.Fatalf("member get must redact: %+v", g)
	}
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T", Name: "n", LocationID: fl.ID})
	tree, err := s.Tree(ctx(), member)
	if err != nil || len(tree) != 1 || len(tree[0].Children) != 1 || tree[0].Children[0].AssetCount != 1 || tree[0].Email != "" {
		t.Fatalf("tree: %+v %v", tree, err)
	}
	// Guards.
	if err := s.Delete(ctx(), admin, hq.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("children: %v", err)
	}
	if err := s.Delete(ctx(), admin, fl.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("assets: %v", err)
	}
	_ = mem.DeleteAsset(ctx(), tenant, "a1")
	// Update: rename parent → child path is recomputed on the child's next update.
	up, err := s.Update(ctx(), admin, hq.ID, Input{Name: "Head Office", Email: "new@x.example", Status: store.LocPlanned})
	if err != nil || up.Path != "Head Office" || up.Email != "new@x.example" || up.Status != store.LocPlanned {
		t.Fatalf("%+v %v", up, err)
	}
	up2, _ := s.Update(ctx(), admin, fl.ID, Input{Name: "Floor 1", ParentID: hq.ID})
	if up2.Path != "Head Office / Floor 1" {
		t.Fatalf("path: %q", up2.Path)
	}
	if _, err := s.Update(ctx(), admin, hq.ID, Input{Name: "Head Office", ParentID: fl.ID}); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err := s.Update(ctx(), admin, hq.ID, Input{Name: "Head Office", ParentID: hq.ID}); err == nil {
		t.Fatal("self parent")
	}
	if _, err := s.Update(ctx(), admin, hq.ID, Input{Name: "Head Office", ParentID: "nope"}); err == nil {
		t.Fatal("bad parent")
	}
	if _, err := s.Update(ctx(), admin, "missing", Input{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx(), admin, hq.ID, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Update(ctx(), authz.Subjects{}, hq.ID, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), admin, fl.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), admin, hq.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), admin, hq.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), authz.Subjects{}, hq.ID); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.Get(ctx(), authz.Subjects{}, hq.ID); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.List(ctx(), authz.Subjects{}); err == nil {
		t.Fatal("forbidden")
	}
	// tampered ciphertext redacts
	l, _ := s.Create(ctx(), admin, Input{Name: "T", Email: "e@x.example"})
	raw, _ = mem.GetLocation(ctx(), tenant, l.ID)
	raw.Email = "###"
	_ = mem.UpdateLocation(ctx(), raw)
	if g, _ := s.Get(ctx(), admin, l.ID); g.Email != "" {
		t.Fatal("tampered must redact")
	}
}

func TestStoreFailures(t *testing.T) {
	s, mem := newSvc(t)
	l, _ := s.Create(ctx(), admin, Input{Name: "A"})
	for _, m := range []string{"CreateLocation", "ListLocations", "UpdateLocation", "CountChildLocations", "CountAssetsByLocation", "DeleteLocation"} {
		mem.FailNext(m)
		var err error
		switch m {
		case "CreateLocation":
			_, err = s.Create(ctx(), admin, Input{Name: "B"})
		case "ListLocations":
			_, err = s.Tree(ctx(), admin)
		case "UpdateLocation":
			_, err = s.Update(ctx(), admin, l.ID, Input{Name: "A"})
		default:
			err = s.Delete(ctx(), admin, l.ID)
		}
		if err == nil {
			t.Fatalf("%s: want error", m)
		}
	}
	b, _ := s.Create(ctx(), admin, Input{Name: "B"})
	mem.FailNext("ListLocations")
	if _, err := s.Update(ctx(), admin, b.ID, Input{Name: "B", ParentID: l.ID}); err == nil {
		t.Fatal("list failure in update")
	}
	if BuildTree([]store.Location{{ID: "1", ParentID: "2"}, {ID: "2", ParentID: "1"}, {ID: "3", ParentID: "3"}}) == nil {
		t.Fatal("cycles become roots")
	}
}
