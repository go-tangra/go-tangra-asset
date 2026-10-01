package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// List contract (go-tangra specs/032-server-side-tables): page / page_size /
// sort / order in, {items,total,page,page_size,sort,order} out; legacy
// cursor / limit keeps {items} plus total; invalid values are
// validation_failed naming the parameter only.

func (f *apiFixture) seedList(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	var ids []string
	for i := range 7 {
		id := fmt.Sprintf("0190f7c2-0000-7000-8000-%012d", i)
		ids = append(ids, id)
		pd := base.AddDate(0, i, 0)
		a := store.Asset{ID: id, TenantID: apiTenant, AssetTag: fmt.Sprintf("T-%02d", 6-i), Name: []string{"delta", "Alpha", "charlie", "Bravo"}[i%4],
			Status: store.AssetDeployable, PurchaseDate: &pd, WarrantyMonths: 12 * (i % 3), CreatedAt: base.Add(time.Duration(i) * time.Hour)}
		if i%2 == 0 {
			a.Status = store.AssetBroken
		}
		if err := f.mem.CreateAsset(ctx, a); err != nil {
			t.Fatal(err)
		}
		for _, err := range []error{
			f.mem.CreateSupplier(ctx, store.Supplier{TenantID: apiTenant, Name: fmt.Sprintf("Supplier %d", i)}),
			f.mem.CreateConsumable(ctx, store.Consumable{TenantID: apiTenant, Name: fmt.Sprintf("Toner %d", i), Amount: 10 - i}),
			f.mem.CreateLicense(ctx, store.License{TenantID: apiTenant, Name: fmt.Sprintf("License %d", i)}),
			f.mem.CreateInsurance(ctx, store.InsurancePolicy{ID: fmt.Sprintf("0190f7c2-0001-7000-8000-%012d", i), TenantID: apiTenant,
				Name: fmt.Sprintf("Policy %d", i), PolicyNumber: fmt.Sprintf("P%d", i)}),
		} {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return ids
}

func tags(m map[string]any, key string) []string {
	var out []string
	for _, it := range items(m) {
		out = append(out, fmt.Sprint(it.(map[string]any)[key]))
	}
	return out
}

func TestListContractValidation(t *testing.T) {
	f := newAPI(t, false)
	f.seedList(t)
	for _, path := range []string{"/assets", "/suppliers", "/consumables", "/licenses", "/insurance-policies"} {
		for q, param := range map[string]string{
			"sort=bogus": "sort", "order=up": "order", "page_size=201": "page_size", "page_size=0": "page_size", "page_size=abc": "page_size",
			"page=0": "page", "page=abc": "page", "page=-1": "page", "cursor=x&page=1": "cursor", "limit=5&sort=name": "cursor",
		} {
			e := want(t, f.req(t, "GET", p+path+"?"+q, "admin", ""), 422)
			d, _ := e["detail"].(map[string]any)
			if e["reason"] != "validation_failed" || d["param"] != param || len(d) != 1 {
				t.Fatalf("%s?%s: %v (want param %s)", path, q, e, param)
			}
			if strings.Contains(fmt.Sprint(e), "bogus") || strings.Contains(fmt.Sprint(e), "abc") {
				t.Fatalf("%s?%s echoes the value: %v", path, q, e)
			}
		}
	}
	// The sub-lists validate the same way.
	e := want(t, f.req(t, "GET", p+"/insurance-policies/x/assets?sort=covered_value", "admin", ""), 422)
	if e["detail"].(map[string]any)["param"] != "sort" {
		t.Fatal(e)
	}
	e = want(t, f.req(t, "GET", p+"/assets/x/assignments?order=sideways", "admin", ""), 422)
	if e["detail"].(map[string]any)["param"] != "order" {
		t.Fatal(e)
	}
	e = want(t, f.req(t, "GET", p+"/documents/search?q=a&page_size=101", "admin", ""), 422)
	if e["detail"].(map[string]any)["param"] != "page_size" {
		t.Fatal(e)
	}
}

func TestListContractAssets(t *testing.T) {
	f := newAPI(t, false)
	ids := f.seedList(t)
	// Defaults: asset tag ascending, 25 per page.
	m := want(t, f.req(t, "GET", p+"/assets", "admin", ""), 200)
	if m["total"] != 7.0 || m["page"] != 1.0 || m["page_size"] != 25.0 || m["sort"] != "asset_tag" || m["order"] != "asc" {
		t.Fatalf("defaults: %v", m)
	}
	if got := strings.Join(tags(m, "asset_tag"), ","); got != "T-00,T-01,T-02,T-03,T-04,T-05,T-06" {
		t.Fatal(got)
	}
	// Name descending, case-insensitive, id tie-breaker in the same direction; pages of 3.
	var seen []string
	for page := 1; page <= 3; page++ {
		m = want(t, f.req(t, "GET", p+fmt.Sprintf("/assets?sort=name&order=desc&page=%d&page_size=3", page), "admin", ""), 200)
		if m["page"] != float64(page) || m["total"] != 7.0 {
			t.Fatalf("page %d: %v", page, m)
		}
		seen = append(seen, tags(m, "id")...)
	}
	if want := []string{ids[4], ids[0], ids[6], ids[2], ids[3], ids[5], ids[1]}; strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Fatalf("name desc:\n%v\n%v", seen, want)
	}
	// Default direction of a field; a page beyond the end answers the last page.
	m = want(t, f.req(t, "GET", p+"/assets?sort=created_at&page=9&page_size=5", "admin", ""), 200)
	if m["order"] != "desc" || m["page"] != 2.0 || len(items(m)) != 2 || tags(m, "id")[1] != ids[0] {
		t.Fatalf("created_at clamp: %v", m)
	}
	// Warranty end: no warranty last in both directions.
	m = want(t, f.req(t, "GET", p+"/assets?sort=warranty_end&order=desc", "admin", ""), 200)
	if got := tags(m, "id"); got[0] != ids[5] || got[6] != ids[0] {
		t.Fatalf("warranty_end desc: %v", got)
	}
	// Filters apply before the count.
	m = want(t, f.req(t, "GET", p+"/assets?status=broken&page_size=2", "admin", ""), 200)
	if m["total"] != 4.0 || len(items(m)) != 2 {
		t.Fatalf("filter: %v", m)
	}
	if m = want(t, f.req(t, "GET", p+"/assets", "other", ""), 200); m["total"] != 0.0 || len(items(m)) != 0 {
		t.Fatalf("tenant isolation: %v", m)
	}
	// Legacy cursor / limit: the old shape plus total.
	m = want(t, f.req(t, "GET", p+"/assets?limit=2", "admin", ""), 200)
	if _, paged := m["page"]; paged || m["total"] != 7.0 || len(items(m)) != 2 || tags(m, "id")[0] != ids[6] {
		t.Fatalf("legacy limit: %v", m)
	}
	m = want(t, f.req(t, "GET", p+"/assets?status=broken&cursor="+url.QueryEscape(ids[4]), "admin", ""), 200)
	if _, paged := m["page"]; paged || m["total"] != 4.0 || len(items(m)) != 2 {
		t.Fatalf("legacy cursor: %v", m)
	}
	f.mem.FailNext("ListAssets")
	want(t, f.req(t, "GET", p+"/assets?limit=2", "admin", ""), 500)
	f.mem.FailNext("PageAssets")
	want(t, f.req(t, "GET", p+"/assets?limit=2", "admin", ""), 500) // the legacy total
}

func TestListContractOtherLists(t *testing.T) {
	f := newAPI(t, false)
	ids := f.seedList(t)
	for _, c := range []struct{ path, sort, order, key, first string }{
		{"/suppliers", "name", "asc", "name", "Supplier 0"},
		{"/consumables?sort=amount", "amount", "asc", "name", "Toner 6"},
		{"/licenses?sort=name&order=desc", "name", "desc", "name", "License 6"},
		{"/insurance-policies?page_size=2&page=4", "name", "asc", "name", "Policy 6"},
		{"/licenses?sort=valid_to", "valid_to", "asc", "name", "License 0"},
		{"/insurance-policies?sort=created_at", "created_at", "desc", "name", ""},
	} {
		m := want(t, f.req(t, "GET", p+c.path, "admin", ""), 200)
		if m["total"] != 7.0 || m["sort"] != c.sort || m["order"] != c.order {
			t.Fatalf("%s: %v", c.path, m)
		}
		if c.first != "" && tags(m, c.key)[0] != c.first {
			t.Fatalf("%s: first %v", c.path, tags(m, c.key))
		}
		legacy := want(t, f.req(t, "GET", p+strings.SplitN(c.path, "?", 2)[0]+"?limit=3", "admin", ""), 200)
		if _, paged := legacy["page"]; paged || legacy["total"] != 7.0 || len(items(legacy)) != 3 {
			t.Fatalf("%s legacy: %v", c.path, legacy)
		}
	}
	m := want(t, f.req(t, "GET", p+"/suppliers?query=supplier%203", "admin", ""), 200)
	if m["total"] != 1.0 {
		t.Fatalf("supplier filter: %v", m)
	}
	for _, method := range []string{"PageSuppliers", "PageConsumables", "PageLicenses", "PageInsurance"} {
		path := map[string]string{"PageSuppliers": "/suppliers", "PageConsumables": "/consumables", "PageLicenses": "/licenses", "PageInsurance": "/insurance-policies"}[method]
		f.mem.FailNext(method)
		want(t, f.req(t, "GET", p+path, "admin", ""), 500)
	}

	// Covered assets of a policy.
	policy := "0190f7c2-0001-7000-8000-000000000000"
	for _, id := range ids[:5] {
		want(t, f.req(t, "POST", p+"/insurance-policies/"+policy+"/assets", "admin", `{"asset_id":"`+id+`","covered_value":10}`), 201)
	}
	m = want(t, f.req(t, "GET", p+"/insurance-policies/"+policy+"/assets?page_size=2", "admin", ""), 200)
	if m["total"] != 5.0 || m["sort"] != "asset_tag" || strings.Join(tags(m, "asset_tag"), ",") != "T-02,T-03" {
		t.Fatalf("policy assets: %v", m)
	}
	m = want(t, f.req(t, "GET", p+"/insurance-policies/"+policy+"/assets?sort=name", "admin", ""), 200)
	if tags(m, "asset_name")[0] != "Alpha" {
		t.Fatalf("policy assets by name: %v", m)
	}
	want(t, f.req(t, "GET", p+"/insurance-policies/missing/assets", "admin", ""), 404)
	f.mem.FailNext("PagePolicyAssets")
	want(t, f.req(t, "GET", p+"/insurance-policies/"+policy+"/assets", "admin", ""), 500)
	m = want(t, f.req(t, "GET", p+"/insurance-policies/"+policy+"/assets?limit=1", "admin", ""), 200)
	if _, paged := m["page"]; paged || m["total"] != 5.0 || len(items(m)) != 5 {
		t.Fatalf("policy assets legacy: %v", m)
	}

	// Assignment history: newest first by default; returned_at sorts open ones last.
	asset := ids[1]
	want(t, f.req(t, "POST", p+"/assets/"+asset+"/assign", "admin", `{"user_id":"`+apiUser1+`"}`), 200)
	want(t, f.req(t, "POST", p+"/assets/"+asset+"/unassign", "admin", ``), 200)
	want(t, f.req(t, "POST", p+"/assets/"+asset+"/assign", "admin", `{"user_id":"`+apiMember+`"}`), 200)
	m = want(t, f.req(t, "GET", p+"/assets/"+asset+"/assignments", "admin", ""), 200)
	if m["sort"] != "assigned_at" || m["order"] != "desc" || m["total"] != 3.0 {
		t.Fatalf("assignments: %v", m)
	}
	m = want(t, f.req(t, "GET", p+"/assets/"+asset+"/assignments?sort=returned_at&order=asc", "admin", ""), 200)
	if tags(m, "user_id")[2] != apiMember {
		t.Fatalf("assignments by returned_at: %v", m)
	}
	m = want(t, f.req(t, "GET", p+"/assets/"+asset+"/assignments?limit=1", "admin", ""), 200)
	if _, paged := m["page"]; paged || m["total"] != 3.0 || len(items(m)) != 1 {
		t.Fatalf("assignments legacy: %v", m)
	}
	want(t, f.req(t, "GET", p+"/assets/missing/assignments", "admin", ""), 404)
	f.mem.FailNext("PageAssignments")
	want(t, f.req(t, "GET", p+"/assets/"+asset+"/assignments", "admin", ""), 500)
}

// searchPaper answers every search with the given paperless ids, best first.
type searchPaper struct {
	documents.Paper
	ids []string
}

func (s searchPaper) Search(_ context.Context, _, _ string, limit int) ([]documents.PaperHit, error) {
	var out []documents.PaperHit
	for i, id := range s.ids {
		if i == limit {
			break
		}
		out = append(out, documents.PaperHit{ID: id, Rank: float64(100 - i)})
	}
	return out, nil
}

func TestListContractDocumentSearch(t *testing.T) {
	f := newAPI(t, false)
	ids := f.seedList(t)
	var paper []string
	for i := range 5 {
		pid := fmt.Sprint(100 + i)
		paper = append(paper, pid, "foreign-"+pid) // non-asset hits are dropped
		if err := f.mem.InsertDocument(context.Background(), store.Document{TenantID: apiTenant, EntityType: store.EntityAsset, EntityID: ids[0],
			FileName: "f.pdf", StorageKey: store.PaperlessKeyPrefix + pid, PaperlessDocumentID: pid}); err != nil {
			t.Fatal(err)
		}
	}
	f.docs.SetPaperless(searchPaper{ids: paper})
	m := want(t, f.req(t, "GET", p+"/documents/search?q=invoice&page=2&page_size=2&order=asc", "admin", ""), 200)
	if m["total"] != 5.0 || m["page"] != 2.0 || m["sort"] != "rank" || m["order"] != "desc" || len(items(m)) != 2 {
		t.Fatalf("search page: %v", m)
	}
	if r := items(m)[0].(map[string]any)["rank"]; r != 96.0 {
		t.Fatalf("relevance order: %v", r)
	}
	m = want(t, f.req(t, "GET", p+"/documents/search?q=invoice&page=7", "admin", ""), 200)
	if m["page"] != 1.0 || len(items(m)) != 5 {
		t.Fatalf("search clamp: %v", m)
	}
	m = want(t, f.req(t, "GET", p+"/documents/search?q=invoice&limit=4", "admin", ""), 200)
	if _, paged := m["page"]; paged || m["total"] != 2.0 || len(items(m)) != 2 {
		t.Fatalf("search legacy: %v", m)
	}
	f.docs.SetPaperless(nil)
	want(t, f.req(t, "GET", p+"/documents/search?q=invoice&limit=4", "admin", ""), 503)
	want(t, f.req(t, "GET", p+"/documents/search?q=invoice", "admin", ""), 503)
}
