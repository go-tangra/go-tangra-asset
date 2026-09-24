package assets

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/blob"
	"github.com/go-tangra/go-tangra-asset/v4/internal/events"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
	"github.com/go-tangra/go-tangra-asset/v4/internal/userdir"
)

const (
	tenant = "11111111-1111-7111-8111-111111111111"
	actor  = "22222222-2222-7222-8222-222222222222"
	user1  = "33333333-3333-7333-8333-333333333333"
)

var subj = authz.Subjects{TenantID: tenant, UserID: actor, Roles: []string{"admin"}, ActorKind: authz.ActorUser}

// fakePub records published events.
type fakePub struct {
	mu     sync.Mutex
	events []string
}

func (p *fakePub) Publish(_ context.Context, _ string, typ string, _ any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, typ)
}

// fakeAudit records audit events.
type fakeAudit struct {
	mu   sync.Mutex
	rows []audit.Event
}

func (a *fakeAudit) Record(_ context.Context, e audit.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, e)
	return nil
}

type fixture struct {
	svc  *Service
	mem  *memstore.Mem
	pub  *fakePub
	aud  *fakeAudit
	dir  *userdir.Fake
	blob *blob.Fake
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	mem := memstore.New()
	pub := &fakePub{}
	aud := &fakeAudit{}
	dir := userdir.NewFake()
	dir.Add(tenant, userdir.User{ID: user1, DisplayName: "Ann Example"})
	svc := New(mem, dir, pub, aud)
	b := blob.NewFake()
	svc.SetBlobStore(b)
	return &fixture{svc: svc, mem: mem, pub: pub, aud: aud, dir: dir, blob: b}
}

func ctx() context.Context { return context.Background() }

func TestCreate_AutoTagAndDuplicate(t *testing.T) {
	f := newFixture(t)
	v, err := f.svc.Create(ctx(), subj, Input{Name: "Laptop"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.AssetTag, "AST-") || len(v.AssetTag) != 10 {
		t.Fatalf("auto tag: %q", v.AssetTag)
	}
	if v.Status != store.AssetDeployable || v.DepreciationRate != 0.40 || v.CreatedBy != actor {
		t.Fatalf("defaults: %+v", v)
	}
	if _, err := f.svc.Create(ctx(), subj, Input{Name: "Other", AssetTag: v.AssetTag}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate tag: %v", err)
	}
	if _, err := f.svc.Create(ctx(), subj, Input{Name: "X", AssetTag: "BAD TAG"}); err == nil {
		t.Fatal("tag with whitespace accepted")
	}
	if len(f.aud.rows) != 1 || f.aud.rows[0].EventType != audit.AssetCreated {
		t.Fatalf("audit: %+v", f.aud.rows)
	}
}

func TestCreate_Validation(t *testing.T) {
	f := newFixture(t)
	cases := []Input{
		{},
		{Name: "x", Status: "weird"},
		{Name: "x", Status: store.AssetAssigned},
		{Name: "x", PurchaseCost: -1},
		{Name: "x", DepreciationRate: 2},
		{Name: "x", CategoryID: "nope"},
		{Name: "x", SupplierID: "nope"},
		{Name: "x", LocationID: "nope"},
		{Name: strings.Repeat("n", 201)},
	}
	for i, in := range cases {
		var ve ValidationError
		if _, err := f.svc.Create(ctx(), subj, in); !errors.As(err, &ve) {
			t.Fatalf("case %d: want validation error, got %v", i, err)
		}
	}
	if _, err := f.svc.Create(ctx(), authz.Subjects{}, Input{Name: "x"}); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("no tenant: %v", err)
	}
	// Referenced records must exist.
	_ = f.mem.CreateCategory(ctx(), store.Category{ID: "c1", TenantID: tenant, Name: "Laptops"})
	_ = f.mem.CreateSupplier(ctx(), store.Supplier{ID: "s1", TenantID: tenant, Name: "Acme"})
	_ = f.mem.CreateLocation(ctx(), store.Location{ID: "l1", TenantID: tenant, Name: "HQ"})
	v, err := f.svc.Create(ctx(), subj, Input{Name: "x", CategoryID: "c1", SupplierID: "s1", LocationID: "l1", Tags: map[string]string{"a": "b"}})
	if err != nil || v.CategoryID != "c1" || v.LocationID != "l1" {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestCreate_StoreFailure(t *testing.T) {
	f := newFixture(t)
	f.mem.FailNext("CreateAsset")
	if _, err := f.svc.Create(ctx(), subj, Input{Name: "x"}); err == nil {
		t.Fatal("want error")
	}
}

func TestGetListUpdateDelete(t *testing.T) {
	f := newFixture(t)
	pd := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	f.svc.SetClock(func() time.Time { return pd.AddDate(1, 0, 0) })
	v, _ := f.svc.Create(ctx(), subj, Input{Name: "Laptop", PurchaseCost: 1000, PurchaseDate: &pd, UsefulLifeYears: 5, SalvageValue: 100})
	got, err := f.svc.Get(ctx(), subj, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BookValue <= 100 || got.BookValue >= 1000 {
		t.Fatalf("book value computed via deprec: %v", got.BookValue)
	}
	if _, err := f.svc.Get(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	other := authz.Subjects{TenantID: "44444444-4444-7444-8444-444444444444", UserID: actor, ActorKind: authz.ActorUser}
	if _, err := f.svc.Get(ctx(), other, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant isolation: %v", err)
	}
	list, err := f.svc.List(ctx(), subj, store.AssetFilter{Query: "lap"})
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if l, _ := f.svc.List(ctx(), other, store.AssetFilter{}); len(l) != 0 {
		t.Fatal("cross-tenant list")
	}
	f.mem.FailNext("ListAssets")
	if _, err := f.svc.List(ctx(), subj, store.AssetFilter{}); err == nil {
		t.Fatal("want err")
	}
	if _, err := f.svc.List(ctx(), authz.Subjects{}, store.AssetFilter{}); err == nil {
		t.Fatal("want forbidden")
	}
	if _, err := f.svc.Get(ctx(), authz.Subjects{}, v.ID); err == nil {
		t.Fatal("want forbidden")
	}

	// Update.
	up, err := f.svc.Update(ctx(), subj, v.ID, Input{Name: "Laptop 2", Status: store.AssetBroken})
	if err != nil || up.Name != "Laptop 2" || up.Status != store.AssetBroken || up.AssetTag != v.AssetTag {
		t.Fatalf("%+v %v", up, err)
	}
	if up.UpdatedBy != actor {
		t.Fatal("updated_by")
	}
	if _, err := f.svc.Update(ctx(), subj, v.ID, Input{Name: "x", Status: store.AssetAssigned}); err == nil {
		t.Fatal("assigned via update")
	}
	if _, err := f.svc.Update(ctx(), subj, v.ID, Input{}); err == nil {
		t.Fatal("invalid update")
	}
	if _, err := f.svc.Update(ctx(), subj, v.ID, Input{Name: "x", CategoryID: "nope"}); err == nil {
		t.Fatal("bad ref")
	}
	if _, err := f.svc.Update(ctx(), subj, "missing", Input{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Update(ctx(), authz.Subjects{}, v.ID, Input{Name: "x"}); err == nil {
		t.Fatal("forbidden")
	}
	// A blank tag on update keeps the current one; a conflicting one is refused.
	w, _ := f.svc.Create(ctx(), subj, Input{Name: "W", AssetTag: "W-1"})
	if w2, err := f.svc.Update(ctx(), subj, w.ID, Input{Name: "W"}); err != nil || w2.AssetTag != "W-1" {
		t.Fatal(w2, err)
	}
	if _, err := f.svc.Update(ctx(), subj, w.ID, Input{Name: "W", AssetTag: v.AssetTag}); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	f.mem.FailNext("UpdateAsset")
	if _, err := f.svc.Update(ctx(), subj, w.ID, Input{Name: "W"}); err == nil {
		t.Fatal("want err")
	}

	// Delete removes objects too.
	_, _ = f.blob.Put(ctx(), "tenants/"+tenant+"/assets/"+v.ID+"/photo", strings.NewReader("img"), 3, "image/png")
	a, _ := f.mem.GetAsset(ctx(), tenant, v.ID)
	a.PhotoKey = "tenants/" + tenant + "/assets/" + v.ID + "/photo"
	_ = f.mem.UpdateAsset(ctx(), a)
	_ = f.mem.InsertDocument(ctx(), store.Document{ID: "d1", TenantID: tenant, EntityType: store.EntityAsset, EntityID: v.ID, StorageKey: "k1"})
	_, _ = f.blob.Put(ctx(), "k1", strings.NewReader("doc"), 3, "text/plain")
	if err := f.svc.Delete(ctx(), subj, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.blob.Get(ctx(), a.PhotoKey); err == nil {
		t.Fatal("photo object should be gone")
	}
	if _, err := f.blob.Get(ctx(), "k1"); err == nil {
		t.Fatal("document object should be gone")
	}
	if err := f.svc.Delete(ctx(), subj, v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx(), authz.Subjects{}, v.ID); err == nil {
		t.Fatal("forbidden")
	}
	f.mem.FailNext("DeleteAsset")
	if err := f.svc.Delete(ctx(), subj, w.ID); err == nil {
		t.Fatal("want err")
	}
}

func TestAssignUnassignLifecycle(t *testing.T) {
	f := newFixture(t)
	_ = f.mem.CreateLocation(ctx(), store.Location{ID: "l1", TenantID: tenant, Name: "HQ"})
	v, _ := f.svc.Create(ctx(), subj, Input{Name: "Laptop", LocationID: "l1"})

	// Assign.
	as, err := f.svc.Assign(ctx(), subj, v.ID, user1, "for onboarding")
	if err != nil {
		t.Fatal(err)
	}
	if as.Status != store.AssetAssigned || as.UserID != user1 || as.LocationID != "" || as.AssigneeName != "Ann Example" {
		t.Fatalf("%+v", as)
	}
	// Re-assign refused.
	if _, err := f.svc.Assign(ctx(), subj, v.ID, user1, ""); !errors.Is(err, ErrState) {
		t.Fatalf("re-assign: %v", err)
	}
	// Update keeps the lifecycle fields.
	up, _ := f.svc.Update(ctx(), subj, v.ID, Input{Name: "Laptop!", Status: store.AssetBroken, LocationID: "l1"})
	if up.Status != store.AssetAssigned || up.UserID != user1 || up.LocationID != "" {
		t.Fatalf("update must not break assignment: %+v", up)
	}
	// Unassign to a location.
	un, err := f.svc.Unassign(ctx(), subj, v.ID, "l1", "returned")
	if err != nil {
		t.Fatal(err)
	}
	if un.Status != store.AssetDeployable || un.UserID != "" || un.LocationID != "l1" || un.AssigneeName != "" {
		t.Fatalf("%+v", un)
	}
	if _, err := f.svc.Unassign(ctx(), subj, v.ID, "", ""); !errors.Is(err, ErrState) {
		t.Fatalf("double unassign: %v", err)
	}
	// History newest-first: unassigned then assigned; the assigned row is closed.
	hist, err := f.svc.Assignments(ctx(), subj, v.ID, 0)
	if err != nil || len(hist) != 2 {
		t.Fatalf("%v %v", hist, err)
	}
	if hist[0].Action != store.ActionUnassigned || hist[1].Action != store.ActionAssigned {
		t.Fatalf("order: %+v", hist)
	}
	if hist[1].ReturnedAt == nil || hist[1].AssignedBy != actor || hist[1].UserName != "Ann Example" || hist[1].Notes != "for onboarding" {
		t.Fatalf("assigned row: %+v", hist[1])
	}
	if hist[0].UserID != user1 || hist[0].AssignedBy != actor {
		t.Fatalf("unassigned row: %+v", hist[0])
	}
	if got := strings.Join(f.pub.events, ","); got != events.AssetAssigned+","+events.AssetUnassigned {
		t.Fatalf("events: %s", got)
	}
	// Audit trail carries both ok rows plus the two refusals.
	var kinds []string
	for _, e := range f.aud.rows {
		kinds = append(kinds, string(e.EventType)+":"+e.Outcome)
	}
	joined := strings.Join(kinds, ",")
	for _, want := range []string{"asset_assigned:ok", "asset_assigned:refused", "asset_unassigned:ok", "asset_unassigned:refused"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("audit missing %s in %s", want, joined)
		}
	}
	if _, err := f.svc.Assignments(ctx(), subj, "missing", 0); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Assignments(ctx(), authz.Subjects{}, v.ID, 0); err == nil {
		t.Fatal("forbidden")
	}
	f.mem.FailNext("ListAssignments")
	if _, err := f.svc.Assignments(ctx(), subj, v.ID, 1000); err == nil {
		t.Fatal("want err")
	}
}

func TestAssign_Refusals(t *testing.T) {
	f := newFixture(t)
	v, _ := f.svc.Create(ctx(), subj, Input{Name: "Laptop"})
	if _, err := f.svc.Assign(ctx(), subj, v.ID, "", ""); err == nil {
		t.Fatal("empty user")
	}
	if _, err := f.svc.Assign(ctx(), subj, "missing", user1, ""); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Assign(ctx(), authz.Subjects{}, v.ID, user1, ""); err == nil {
		t.Fatal("forbidden")
	}
	var ve ValidationError
	if _, err := f.svc.Assign(ctx(), subj, v.ID, "unknown-user", ""); !errors.As(err, &ve) {
		t.Fatalf("unknown user: %v", err)
	}
	f.dir.Err = errors.New("auth down")
	if _, err := f.svc.Assign(ctx(), subj, v.ID, user1, ""); err == nil || errors.As(err, &ve) {
		t.Fatalf("directory outage must surface: %v", err)
	}
	f.dir.Err = nil
	f.mem.FailNext("UpdateAsset")
	if _, err := f.svc.Assign(ctx(), subj, v.ID, user1, ""); err == nil {
		t.Fatal("want err")
	}
	f.mem.FailNext("InsertAssignment")
	if _, err := f.svc.Assign(ctx(), subj, v.ID, user1, ""); err == nil {
		t.Fatal("want err")
	}
	if a, _ := f.mem.GetAsset(ctx(), tenant, v.ID); a.Status != store.AssetAssigned {
		t.Fatal("asset was updated before the history insert failed")
	}
	// Unassign refusals.
	if _, err := f.svc.Unassign(ctx(), subj, "missing", "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Unassign(ctx(), authz.Subjects{}, v.ID, "", ""); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := f.svc.Unassign(ctx(), subj, v.ID, "no-such-location", ""); !errors.As(err, &ve) {
		t.Fatalf("bad location: %v", err)
	}
	f.mem.FailNext("UpdateAsset")
	if _, err := f.svc.Unassign(ctx(), subj, v.ID, "", ""); err == nil {
		t.Fatal("want err")
	}
	f.mem.FailNext("CloseActiveAssignment")
	if _, err := f.svc.Unassign(ctx(), subj, v.ID, "", ""); err == nil {
		t.Fatal("want err")
	}
	// restore assigned so the InsertAssignment failure path is reachable
	a, _ := f.mem.GetAsset(ctx(), tenant, v.ID)
	a.Status, a.UserID = store.AssetAssigned, user1
	_ = f.mem.UpdateAsset(ctx(), a)
	f.mem.FailNext("InsertAssignment")
	if _, err := f.svc.Unassign(ctx(), subj, v.ID, "", ""); err == nil {
		t.Fatal("want err")
	}
}

func TestNilCollaborators(t *testing.T) {
	mem := memstore.New()
	svc := New(mem, nil, nil, nil)
	v, err := svc.Create(ctx(), subj, Input{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	as, err := svc.Assign(ctx(), subj, v.ID, user1, "")
	if err != nil || as.AssigneeName != "" {
		t.Fatalf("%+v %v", as, err)
	}
	if _, err := svc.Unassign(ctx(), subj, v.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if svc.BookValue(store.Asset{PurchaseCost: 5}) != 5 {
		t.Fatal("book value")
	}
}

func TestAutoTagCollisionRetry(t *testing.T) {
	f := newFixture(t)
	// Fill the store with a colliding tag by monkeying the generator range is
	// not possible; instead assert the tag charset/length and uniqueness over many.
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tag := AutoTag()
		if !ValidTag(tag) || len(tag) != 10 {
			t.Fatalf("bad tag %q", tag)
		}
		seen[tag] = true
	}
	if len(seen) < 190 {
		t.Fatalf("tags not unique enough: %d", len(seen))
	}
	if ValidTag(strings.Repeat("x", 65)) || ValidTag("a\tb") {
		t.Fatal("ValidTag")
	}
	_ = f
}
