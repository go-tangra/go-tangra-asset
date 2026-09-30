package invsync

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra-asset/v4/internal/invclient"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

func filterFixture(t *testing.T) (*Service, *memstore.Mem, *invclient.Fake) {
	t.Helper()
	mem := memstore.New()
	inv := invclient.NewFake()
	inv.Set(tenant, []invclient.Host{
		{ID: "h1", Hostname: "srv-db-01", OSName: "Ubuntu", Status: "active"},
		{ID: "h2", Hostname: "vm-web-01", OSName: "Ubuntu", Status: "active"},
		{ID: "h3", Hostname: "ct-cache", OSName: "Alpine Linux", Status: "active"},
		{ID: "h4", Hostname: "old-box", OSName: "Windows 10", Status: "stale"},
		{ID: "h5", Hostname: "gone", OSName: "Windows 11", Status: "retired"},
		{ID: "h6", Hostname: "lab-1.lab.example", OSName: "Debian", Status: "active"},
		{ID: "h7", Hostname: "desk-7", OSName: "Windows Server 2022", Status: "active"},
	})
	inv.SetRole(tenant, "h1", "physical")
	inv.SetRole(tenant, "h2", "vm")
	inv.SetRole(tenant, "h3", "container")
	return New(mem, inv, nil), mem, inv
}

func reasons(p Preview) map[string]string {
	out := map[string]string{}
	for _, c := range p.Changes {
		out[c.Hostname] = c.Action + ":" + c.Reason
	}
	return out
}

func TestFilterExcludesAndExecuteSkips(t *testing.T) {
	s, mem, _ := filterFixture(t)
	saved, err := s.SaveSettings(ctx(), subj, store.InvSyncSettings{ExcludeVMs: true, ExcludeContainers: true, SkipStale: true, SkipRetired: true,
		HostnameExclude: []string{" *.LAB.* ", "*.lab.*", ""}, OSExclude: []string{"windows server*"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.HostnameExclude) != 1 || saved.HostnameExclude[0] != "*.lab.*" || saved.UpdatedBy != "u" || saved.HostnameInclude == nil {
		t.Fatalf("normalized %+v", saved)
	}
	p, err := s.Preview(ctx(), subj)
	if err != nil {
		t.Fatal(err)
	}
	r := reasons(p)
	want := map[string]string{"srv-db-01": "create:", "vm-web-01": "excluded:virtual_machine", "ct-cache": "excluded:container", "old-box": "excluded:stale",
		"gone": "excluded:retired", "lab-1.lab.example": "excluded:hostname_excluded", "desk-7": "excluded:os_excluded"}
	for h, w := range want {
		if r[h] != w {
			t.Errorf("%s: %s, want %s", h, r[h], w)
		}
	}
	if p.Excluded != 6 || p.Create != 1 {
		t.Fatalf("summary %+v", p)
	}
	res, err := s.Execute(ctx(), subj, nil)
	if err != nil || res.Created != 1 || res.Excluded != 6 {
		t.Fatalf("execute %+v %v", res, err)
	}
	if all, _ := mem.AllAssets(ctx(), tenant); len(all) != 1 || all[0].Name != "srv-db-01" {
		t.Fatalf("assets %+v", all)
	}
}

func TestFilterIncludeLists(t *testing.T) {
	s, _, _ := filterFixture(t)
	if _, err := s.SaveSettings(ctx(), subj, store.InvSyncSettings{HostnameInclude: []string{"srv-*", "vm-*", "desk-*"}, OSInclude: []string{"ubuntu", "windows*"}}); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview(ctx(), subj)
	if err != nil {
		t.Fatal(err)
	}
	r := reasons(p)
	if r["srv-db-01"] != "create:" || r["vm-web-01"] != "create:" || r["desk-7"] != "create:" || r["ct-cache"] != "excluded:hostname_not_included" {
		t.Fatalf("include %v", r)
	}
	if _, err := s.SaveSettings(ctx(), subj, store.InvSyncSettings{OSInclude: []string{"debian"}}); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Preview(ctx(), subj)
	if r := reasons(p); r["lab-1.lab.example"] != "create:" || r["srv-db-01"] != "excluded:os_not_included" {
		t.Fatalf("os include %v", r)
	}
}

// Roles are only needed (and only fetched) when the VM/container toggles are on.
func TestFilterRolesUnavailable(t *testing.T) {
	s, _, inv := filterFixture(t)
	inv.RolesDown = true
	if _, err := s.Preview(ctx(), subj); err != nil {
		t.Fatalf("roles fetched although unused: %v", err)
	}
	if _, err := s.SaveSettings(ctx(), subj, store.InvSyncSettings{ExcludeVMs: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Preview(ctx(), subj); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing roles: %v", err)
	}
	if _, _, err := New(memstore.New(), errClient{}, nil).plan(ctx(), tenant); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("hosts down: %v", err)
	}
}

func TestRunScheduledOptions(t *testing.T) {
	s, mem, inv := filterFixture(t)
	res, err := s.RunScheduled(ctx(), tenant, Options{Create: false, Update: true})
	if err != nil || res.Created != 0 || res.Skipped != 7 {
		t.Fatalf("update-only %+v %v", res, err)
	}
	if res, err = s.RunScheduled(ctx(), tenant, Options{Create: true, Update: true}); err != nil || res.Created != 7 {
		t.Fatalf("full %+v %v", res, err)
	}
	inv.Set(tenant, []invclient.Host{{ID: "h1", Hostname: "srv-db-01-renamed", OSName: "Ubuntu", Status: "active"}})
	if res, err = s.RunScheduled(ctx(), tenant, Options{Create: true, Update: false}); err != nil || res.Updated != 0 || res.Skipped != 1 {
		t.Fatalf("create-only %+v %v", res, err)
	}
	if res, err = s.RunScheduled(ctx(), tenant, Options{Create: true, Update: true}); err != nil || res.Updated != 1 {
		t.Fatalf("update %+v %v", res, err)
	}
	if all, _ := mem.AllAssets(ctx(), tenant); len(all) != 7 {
		t.Fatalf("scheduled sync deleted or duplicated assets: %d", len(all))
	}
}

func TestSettingsValidationAndDefaults(t *testing.T) {
	s, mem, _ := filterFixture(t)
	st, err := s.Settings(ctx(), subj)
	if err != nil || st.ExcludeVMs || st.HostnameInclude == nil {
		t.Fatalf("defaults %+v %v", st, err)
	}
	many := make([]string, MaxPatterns+1)
	for i := range many {
		many[i] = strings.Repeat("a", i+1)
	}
	for name, in := range map[string]store.InvSyncSettings{
		"bad glob": {HostnameExclude: []string{"[abc"}},
		"too long": {OSInclude: []string{strings.Repeat("x", MaxPatternLen+1)}},
		"too many": {OSExclude: many},
	} {
		var ve SettingsValidationError
		if _, err := s.SaveSettings(ctx(), subj, in); !errors.As(err, &ve) || ve.Error() == "" {
			t.Errorf("%s: %v", name, err)
		}
	}
	other := subj
	other.TenantID = ""
	if _, err := s.Settings(ctx(), other); err == nil {
		t.Error("no tenant accepted")
	}
	if _, err := s.SaveSettings(ctx(), other, store.InvSyncSettings{}); err == nil {
		t.Error("no tenant accepted on save")
	}
	mem.FailNext("PutInvSyncSettings")
	if _, err := s.SaveSettings(ctx(), subj, store.InvSyncSettings{}); err == nil {
		t.Error("store failure swallowed")
	}
	mem.FailNext("GetInvSyncSettings")
	if _, err := s.Preview(ctx(), subj); err == nil {
		t.Error("settings failure swallowed")
	}
}
