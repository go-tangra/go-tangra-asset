package categories

import (
	"context"
	"errors"
	"testing"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var subj = authz.Subjects{TenantID: tenant, UserID: "u", Roles: []string{"admin"}, ActorKind: authz.ActorUser}

func ctx() context.Context { return context.Background() }

func TestTreeAndGuards(t *testing.T) {
	mem := memstore.New()
	s := New(mem, nil)
	root, err := s.Create(ctx(), subj, Input{Name: "Hardware"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.Create(ctx(), subj, Input{Name: "Laptops", ParentID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	// Same name under the same parent → conflict; under another parent → ok.
	if _, err := s.Create(ctx(), subj, Input{Name: "Laptops", ParentID: root.ID}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup: %v", err)
	}
	if _, err := s.Create(ctx(), subj, Input{Name: "Laptops"}); err != nil {
		t.Fatalf("same name, different parent: %v", err)
	}
	if _, err := s.Create(ctx(), subj, Input{Name: "x", ParentID: "nope"}); err == nil {
		t.Fatal("bad parent")
	}
	if _, err := s.Create(ctx(), subj, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Create(ctx(), authz.Subjects{}, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T1", Name: "n", CategoryID: child.ID})

	tree, err := s.Tree(ctx(), subj)
	if err != nil || len(tree) != 2 {
		t.Fatalf("%v %v", tree, err)
	}
	var hw *Node
	for _, n := range tree {
		if n.ID == root.ID {
			hw = n
		}
	}
	if hw == nil || len(hw.Children) != 1 || hw.Children[0].ID != child.ID {
		t.Fatalf("tree: %+v", tree)
	}
	if hw.ChildCount != 1 || hw.Children[0].AssetCount != 1 {
		t.Fatalf("counts: %+v / %+v", hw.Category, hw.Children[0].Category)
	}
	// Delete guards.
	if err := s.Delete(ctx(), subj, root.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("has children: %v", err)
	}
	if err := s.Delete(ctx(), subj, child.ID); !errors.Is(err, ErrInUse) {
		t.Fatalf("has assets: %v", err)
	}
	if err := s.Delete(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), authz.Subjects{}, root.ID); err == nil {
		t.Fatal("forbidden")
	}
	_ = mem.DeleteAsset(ctx(), tenant, "a1")
	if err := s.Delete(ctx(), subj, child.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), subj, root.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCycleAndList(t *testing.T) {
	mem := memstore.New()
	s := New(mem, nil)
	a, _ := s.Create(ctx(), subj, Input{Name: "A"})
	b, _ := s.Create(ctx(), subj, Input{Name: "B", ParentID: a.ID})
	c, _ := s.Create(ctx(), subj, Input{Name: "C", ParentID: b.ID})
	if _, err := s.Update(ctx(), subj, a.ID, Input{Name: "A", ParentID: c.ID}); err == nil {
		t.Fatal("cycle accepted")
	}
	if _, err := s.Update(ctx(), subj, a.ID, Input{Name: "A", ParentID: a.ID}); err == nil {
		t.Fatal("self parent accepted")
	}
	if _, err := s.Update(ctx(), subj, a.ID, Input{Name: "A", ParentID: "nope"}); err == nil {
		t.Fatal("bad parent accepted")
	}
	if _, err := s.Update(ctx(), subj, "missing", Input{Name: "A"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx(), subj, a.ID, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Update(ctx(), authz.Subjects{}, a.ID, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	up, err := s.Update(ctx(), subj, c.ID, Input{Name: "C2", ParentID: a.ID, Icon: "mdi-x"})
	if err != nil || up.ParentID != a.ID || up.Name != "C2" || up.Icon != "mdi-x" {
		t.Fatalf("%+v %v", up, err)
	}
	// Renaming into a sibling's name under the same parent conflicts.
	if _, err := s.Update(ctx(), subj, c.ID, Input{Name: "B", ParentID: a.ID}); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	list, err := s.List(ctx(), subj)
	if err != nil || len(list) != 3 || list[0].Name != "A" {
		t.Fatalf("%v %v", list, err)
	}
	other := authz.Subjects{TenantID: "22222222-2222-7222-8222-222222222222", ActorKind: authz.ActorUser}
	if l, _ := s.List(ctx(), other); len(l) != 0 {
		t.Fatal("tenant isolation")
	}
	if _, err := s.Get(ctx(), other, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("tenant isolation get")
	}
	if _, err := s.List(ctx(), authz.Subjects{}); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.Get(ctx(), authz.Subjects{}, a.ID); err == nil {
		t.Fatal("forbidden")
	}
	// Store failures surface.
	for _, m := range []string{"ListCategories", "CreateCategory", "UpdateCategory", "DeleteCategory", "CountChildCategories", "CountAssetsByCategory"} {
		mem.FailNext(m)
		var err error
		switch m {
		case "ListCategories":
			_, err = s.Tree(ctx(), subj)
		case "CreateCategory":
			_, err = s.Create(ctx(), subj, Input{Name: "Z"})
		case "UpdateCategory":
			_, err = s.Update(ctx(), subj, a.ID, Input{Name: "A"})
		default:
			err = s.Delete(ctx(), subj, c.ID)
		}
		if err == nil {
			t.Fatalf("%s: want error", m)
		}
	}
	mem.FailNext("ListCategories")
	if _, err := s.Update(ctx(), subj, c.ID, Input{Name: "C", ParentID: a.ID}); err == nil {
		t.Fatal("list failure in update")
	}
}

func TestBuildTreeOrphansAndCycles(t *testing.T) {
	rows := []store.Category{
		{ID: "1", ParentID: "2"},
		{ID: "2", ParentID: "1"},
		{ID: "3", ParentID: "missing"},
		{ID: "4", ParentID: "4"},
	}
	roots := BuildTree(rows)
	if len(roots) < 3 {
		t.Fatalf("cycles/orphans must not vanish: %d roots", len(roots))
	}
}
