package insurance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/memstore"
	"github.com/go-freya/freya/services/asset/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var subj = authz.Subjects{TenantID: tenant, UserID: "u", ActorKind: authz.ActorUser}

func ctx() context.Context { return context.Background() }

func TestCRUDAndPolicyAssets(t *testing.T) {
	mem := memstore.New()
	s := New(mem, nil)
	p, err := s.Create(ctx(), subj, Input{Name: "Fleet", PolicyNumber: "P-1", CoverageType: store.CovAllRisk, PremiumAmount: 100})
	if err != nil || p.Status != store.InsActive || p.AssetCount != 0 {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Create(ctx(), subj, Input{Name: "Other", PolicyNumber: "P-1"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup policy number: %v", err)
	}
	if _, err := s.Create(ctx(), subj, Input{Name: "Fleet", PolicyNumber: "P-2"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup name: %v", err)
	}
	from := time.Now()
	bad := from.Add(-time.Hour)
	for _, in := range []Input{{}, {Name: "x"}, {Name: "x", PolicyNumber: "n", CoverageType: "odd"}, {Name: "x", PolicyNumber: "n", Status: "odd"},
		{Name: "x", PolicyNumber: "n", Deductible: -1}, {Name: "x", PolicyNumber: "n", ValidFrom: &from, ValidTo: &bad}} {
		if _, err := s.Create(ctx(), subj, in); err == nil {
			t.Fatalf("validation: %+v", in)
		}
	}
	if _, err := s.Create(ctx(), authz.Subjects{}, Input{Name: "x", PolicyNumber: "n"}); err == nil {
		t.Fatal("forbidden")
	}

	// Policy assets.
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T1", Name: "Laptop", ModelName: "X1"})
	pa, err := s.AddAssetToPolicy(ctx(), subj, p.ID, "a1", 1200, "primary")
	if err != nil || pa.AssetTag != "T1" || pa.AssetName != "Laptop" || pa.CoveredValue != 1200 {
		t.Fatalf("%+v %v", pa, err)
	}
	if _, err := s.AddAssetToPolicy(ctx(), subj, p.ID, "a1", 1, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup link: %v", err)
	}
	if _, err := s.AddAssetToPolicy(ctx(), subj, p.ID, "", 1, ""); err == nil {
		t.Fatal("empty asset")
	}
	if _, err := s.AddAssetToPolicy(ctx(), subj, p.ID, "a1", -1, ""); err == nil {
		t.Fatal("negative value")
	}
	if _, err := s.AddAssetToPolicy(ctx(), subj, "missing", "a1", 1, ""); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.AddAssetToPolicy(ctx(), subj, p.ID, "missing", 1, ""); err == nil {
		t.Fatal("missing asset")
	}
	if _, err := s.AddAssetToPolicy(ctx(), authz.Subjects{}, p.ID, "a1", 1, ""); err == nil {
		t.Fatal("forbidden")
	}
	if g, _ := s.Get(ctx(), subj, p.ID); g.AssetCount != 1 {
		t.Fatalf("asset_count: %+v", g)
	}
	list, err := s.ListPolicyAssets(ctx(), subj, p.ID)
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	if _, err := s.ListPolicyAssets(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.ListPolicyAssets(ctx(), authz.Subjects{}, p.ID); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.RemoveAssetFromPolicy(ctx(), subj, p.ID, "a1"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveAssetFromPolicy(ctx(), subj, p.ID, "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.RemoveAssetFromPolicy(ctx(), authz.Subjects{}, p.ID, "a1"); err == nil {
		t.Fatal("forbidden")
	}
	if g, _ := s.Get(ctx(), subj, p.ID); g.AssetCount != 0 {
		t.Fatalf("asset_count after remove: %+v", g)
	}

	// Update/list/delete.
	up, err := s.Update(ctx(), subj, p.ID, Input{Name: "Fleet 2", PolicyNumber: "P-1", Status: store.InsCancelled})
	if err != nil || up.Status != store.InsCancelled || up.Name != "Fleet 2" {
		t.Fatalf("%+v %v", up, err)
	}
	if _, err := s.Update(ctx(), subj, "missing", Input{Name: "x", PolicyNumber: "n"}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx(), subj, p.ID, Input{}); err == nil {
		t.Fatal("validation")
	}
	if _, err := s.Update(ctx(), authz.Subjects{}, p.ID, Input{Name: "x", PolicyNumber: "n"}); err == nil {
		t.Fatal("forbidden")
	}
	if l, err := s.List(ctx(), subj, store.ListOpts{}); err != nil || len(l) != 1 {
		t.Fatal(l, err)
	}
	if _, err := s.List(ctx(), authz.Subjects{}, store.ListOpts{}); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.Get(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx(), authz.Subjects{}, p.ID); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), authz.Subjects{}, p.ID); err == nil {
		t.Fatal("forbidden")
	}
	if err := s.Delete(ctx(), subj, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx(), subj, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// Store failures.
	q, _ := s.Create(ctx(), subj, Input{Name: "Q", PolicyNumber: "Q-1"})
	for _, m := range []string{"CreateInsurance", "ListInsurance", "UpdateInsurance", "ListPolicyAssets", "AddPolicyAsset", "GetAsset"} {
		mem.FailNext(m)
		var err error
		switch m {
		case "CreateInsurance":
			_, err = s.Create(ctx(), subj, Input{Name: "Z", PolicyNumber: "Z-1"})
		case "ListInsurance":
			_, err = s.List(ctx(), subj, store.ListOpts{})
		case "UpdateInsurance":
			_, err = s.Update(ctx(), subj, q.ID, Input{Name: "Q", PolicyNumber: "Q-1"})
		case "ListPolicyAssets":
			_, err = s.ListPolicyAssets(ctx(), subj, q.ID)
		default:
			_, err = s.AddAssetToPolicy(ctx(), subj, q.ID, "a1", 1, "")
		}
		if err == nil {
			t.Fatalf("%s: want error", m)
		}
	}
}
