package invsync

import (
	"context"
	"errors"
	"testing"

	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/invclient"
	"github.com/go-freya/freya/services/asset/internal/memstore"
	"github.com/go-freya/freya/services/asset/internal/store"
)

const tenant = "11111111-1111-7111-8111-111111111111"

var subj = authz.Subjects{TenantID: tenant, UserID: "u", ActorKind: authz.ActorUser}

func ctx() context.Context { return context.Background() }

func TestPreviewAndExecute(t *testing.T) {
	mem := memstore.New()
	inv := invclient.NewFake()
	inv.Set(tenant, []invclient.Host{
		{ID: "h1", Hostname: "pc-1", SystemSerial: "SN1", Manufacturer: "Dell", Model: "Latitude", OSName: "Windows", OSVersion: "11", OSArch: "amd64"},
		{ID: "h2", Hostname: "pc-2", SystemSerial: "SN2", Model: "XPS"},
		{ID: "h3", Hostname: "pc-3", Model: "Mac"},
		{ID: "h4", Hostname: "pc-4"},
	})
	// SN1 exists but is stale (name differs); pc-3 matches by hostname and is unchanged; pc-4 matches by host id.
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T1", Name: "old-name", Serial: "sn1", ModelName: "Latitude"})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a3", TenantID: tenant, AssetTag: "T3", Name: "PC-3", ModelName: "Mac", Tags: map[string]string{TagInventoryHostID: "h3"}})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a4", TenantID: tenant, AssetTag: "T4", Name: "pc-4", Tags: map[string]string{TagInventoryHostID: "h4"}})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "ax", TenantID: "22222222-2222-7222-8222-222222222222", AssetTag: "TX", Name: "pc-2", Serial: "SN2"})
	s := New(mem, inv, nil)

	p, err := s.Preview(ctx(), subj)
	if err != nil {
		t.Fatal(err)
	}
	if p.Hosts != 4 || p.Create != 1 || p.Update != 2 || p.Unchanged != 1 {
		t.Fatalf("summary: %+v", p)
	}
	byHost := map[string]Change{}
	for _, c := range p.Changes {
		byHost[c.Hostname] = c
	}
	if byHost["pc-1"].Action != ActionUpdate || byHost["pc-1"].AssetID != "a1" || byHost["pc-1"].Changes["name"].New != "pc-1" {
		t.Fatalf("pc-1: %+v", byHost["pc-1"])
	}
	if byHost["pc-2"].Action != ActionCreate {
		t.Fatalf("pc-2 must not match the other tenant's asset: %+v", byHost["pc-2"])
	}
	if byHost["pc-3"].Action != ActionUpdate { // name case differs → update
		t.Fatalf("pc-3: %+v", byHost["pc-3"])
	}
	if byHost["pc-4"].Action != ActionUnchanged {
		t.Fatalf("pc-4: %+v", byHost["pc-4"])
	}
	// Preview writes nothing.
	if all, _ := mem.AllAssets(ctx(), tenant); len(all) != 3 {
		t.Fatal("preview wrote")
	}

	// Execute a selection.
	res, err := s.Execute(ctx(), subj, []string{"pc-1", "pc-2"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || res.Updated != 1 || res.Selected != 2 || len(res.Errors) != 0 {
		t.Fatalf("%+v", res)
	}
	a1, _ := mem.GetAsset(ctx(), tenant, "a1")
	if a1.Name != "pc-1" || a1.Serial != "SN1" || a1.Tags[TagOSName] != "Windows" || a1.Tags[TagInventoryHostID] != "h1" || a1.UpdatedBy != "u" {
		t.Fatalf("updated asset: %+v", a1)
	}
	created, err := mem.FindAssetBySerial(ctx(), tenant, "SN2")
	if err != nil || created.Name != "pc-2" || created.ModelName != "XPS" || created.AssetTag == "" || created.Status != store.AssetDeployable {
		t.Fatalf("created asset: %+v %v", created, err)
	}
	// Second execute (all hosts): everything is now unchanged except pc-3.
	res, _ = s.Execute(ctx(), subj, nil)
	if res.Created != 0 || res.Updated != 1 || res.Skipped != 3 {
		t.Fatalf("second run: %+v", res)
	}
	res, _ = s.Execute(ctx(), subj, nil)
	if res.Updated != 0 || res.Skipped != 4 {
		t.Fatalf("converged: %+v", res)
	}
}

func TestUnavailableAndErrors(t *testing.T) {
	mem := memstore.New()
	inv := invclient.NewFake()
	inv.Set(tenant, []invclient.Host{{ID: "h1", Hostname: "pc-1"}})
	s := New(mem, inv, nil)
	inv.Down = true
	if _, err := s.Preview(ctx(), subj); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("preview: %v", err)
	}
	if _, err := s.Execute(ctx(), subj, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("execute: %v", err)
	}
	if all, _ := mem.AllAssets(ctx(), tenant); len(all) != 0 {
		t.Fatal("wrote while unavailable")
	}
	inv.Down = false
	if _, err := s.Preview(ctx(), authz.Subjects{}); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := s.Execute(ctx(), authz.Subjects{}, nil); err == nil {
		t.Fatal("forbidden")
	}
	mem.FailNext("AllAssets")
	if _, err := s.Preview(ctx(), subj); err == nil {
		t.Fatal("store failure")
	}
	mem.FailNext("CreateAsset")
	res, err := s.Execute(ctx(), subj, nil)
	if err != nil || len(res.Errors) != 1 || res.Created != 0 {
		t.Fatalf("create failure is reported per host: %+v %v", res, err)
	}
	res, _ = s.Execute(ctx(), subj, nil)
	if res.Created != 1 {
		t.Fatal(res)
	}
	inv.Set(tenant, []invclient.Host{{ID: "h1", Hostname: "pc-1", Model: "new"}})
	mem.FailNext("UpdateAsset")
	res, _ = s.Execute(ctx(), subj, nil)
	if len(res.Errors) != 1 || res.Updated != 0 {
		t.Fatalf("update failure reported: %+v", res)
	}
	mem.FailNext("GetAsset")
	res, _ = s.Execute(ctx(), subj, nil)
	if len(res.Errors) != 1 {
		t.Fatalf("get failure reported: %+v", res)
	}
	// nil client → unavailable
	if _, err := New(mem, nil, nil).Preview(ctx(), subj); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	// non-sentinel client error is wrapped as unavailable
	if _, err := New(mem, errClient{}, nil).Preview(ctx(), subj); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

type errClient struct{}

func (errClient) ListHosts(context.Context, string) ([]invclient.Host, error) {
	return nil, errors.New("rpc: connection refused")
}

func TestDiffEdgeCases(t *testing.T) {
	// Two hosts resolving to one asset: the second creates.
	hosts := []invclient.Host{{ID: "h1", Hostname: "a", SystemSerial: "S"}, {ID: "h2", Hostname: "b", SystemSerial: "s"}}
	assets := []store.Asset{{ID: "x", Serial: "S", Name: "a", Tags: map[string]string{TagInventoryHostID: "h1"}}}
	ch := Diff(hosts, assets)
	if len(ch) != 2 || ch[0].Action != ActionUnchanged || ch[1].Action != ActionCreate {
		t.Fatalf("%+v", ch)
	}
	// A host with no hostname/serial gets its id as name.
	d := desired(invclient.Host{ID: "only-id"})
	if d.Name != "only-id" {
		t.Fatal(d)
	}
	if d := desired(invclient.Host{SystemSerial: " SN "}); d.Name != "SN" {
		t.Fatal(d)
	}
	// Duplicate serials/names/host ids among assets keep the first.
	ix := buildIndex([]store.Asset{{ID: "1", Serial: "S", Name: "n", Tags: map[string]string{TagInventoryHostID: "h"}}, {ID: "2", Serial: "s", Name: "N", Tags: map[string]string{TagInventoryHostID: "h"}}})
	if a, _ := ix.match(invclient.Host{SystemSerial: "S"}); a.ID != "1" {
		t.Fatal(a)
	}
	if a, _ := ix.match(invclient.Host{ID: "h"}); a.ID != "1" {
		t.Fatal(a)
	}
	if a, _ := ix.match(invclient.Host{Hostname: "n"}); a.ID != "1" {
		t.Fatal(a)
	}
	if _, ok := ix.match(invclient.Host{}); ok {
		t.Fatal("empty host must not match")
	}
	if len(Diff(nil, nil)) != 0 {
		t.Fatal("empty")
	}
}
