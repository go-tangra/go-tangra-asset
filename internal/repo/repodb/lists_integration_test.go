//go:build integration

// List-contract pages (go-tangra specs/032-server-side-tables) against a real
// database: every sort field × direction pages each record exactly once and
// in the same order as the memstore (same semantics as SQL), filters apply
// before the count, a page beyond the end answers the last page, tenants are
// isolated, and the cursor lists the gRPC RPCs and the backup walk keep their
// id-DESC keyset. Run with:
//
//	go test -tags integration -run TestListPages ./internal/repo/repodb/
package repodb_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// listIDs is a stable id per seeded record (uuid v7 shaped, ordered by n).
func listID(kind, n int) string { return fmt.Sprintf("0190f7c2-%04x-7000-8000-%012x", kind, n) }

// seedLists writes the same records into st (the database or the memstore).
func seedLists(t *testing.T, st repo.Store) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	cats := []string{"Laptops", "monitors", "Phones"}
	for i, n := range cats {
		must(st.CreateCategory(ctx, store.Category{ID: listID(1, i), TenantID: tenantA, Name: n}))
	}
	locs := []struct{ name, path string }{{"Sofia", "Sofia"}, {"Floor 2", "Sofia / Floor 2"}, {"plovdiv", ""}}
	for i, l := range locs {
		must(st.CreateLocation(ctx, store.Location{ID: listID(2, i), TenantID: tenantA, Name: l.name, Path: l.path}))
	}
	names := []string{"alpha", "Bravo", "charlie", "Delta", "echo", "alpha", "Foxtrot", "golf", "Hotel", "india", "Bravo", "juliet", ""}
	statuses := []string{store.AssetDeployable, store.AssetAssigned, store.AssetBroken, store.AssetArchived}
	for i := range 23 {
		a := store.Asset{ID: listID(3, i), TenantID: tenantA, AssetTag: fmt.Sprintf("AST-%03d", (i*7)%23), Name: names[i%len(names)],
			Status: statuses[i%len(statuses)], CreatedAt: base.Add(time.Duration(i%9) * time.Hour)}
		if i%4 != 0 {
			a.CategoryID = listID(1, i%len(cats))
		}
		if i%5 != 0 {
			a.LocationID = listID(2, i%len(locs))
		}
		if i%3 != 0 {
			pd := base.AddDate(0, -(i % 6), 0)
			a.PurchaseDate = &pd
			a.WarrantyMonths = (i % 4) * 12
		}
		must(st.CreateAsset(ctx, a))
	}
	for i := range 4 { // another tenant's assets never appear in tenant A's pages
		must(st.CreateAsset(ctx, store.Asset{ID: listID(4, i), TenantID: tenantB, AssetTag: fmt.Sprintf("AST-%03d", i), Name: "other"}))
	}
	for i := range 11 {
		created := base.Add(time.Duration(i%4) * time.Minute)
		must(st.CreateSupplier(ctx, store.Supplier{ID: listID(5, i), TenantID: tenantA, Name: fmt.Sprintf("%s supplier %02d", names[i%5], i), CreatedAt: created}))
		must(st.CreateConsumable(ctx, store.Consumable{ID: listID(6, i), TenantID: tenantA, Name: names[i%6], Amount: i % 3, CreatedAt: created}))
		lic := store.License{ID: listID(7, i), TenantID: tenantA, Name: names[i%4], CreatedAt: created}
		pol := store.InsurancePolicy{ID: listID(8, i), TenantID: tenantA, Name: fmt.Sprintf("%s policy %02d", names[i%4], i), PolicyNumber: fmt.Sprintf("P-%02d", i), CreatedAt: created}
		if i%3 != 0 {
			vt := base.AddDate(0, i%5, 0)
			lic.ValidTo, pol.ValidTo = &vt, &vt
		}
		must(st.CreateLicense(ctx, lic))
		must(st.CreateInsurance(ctx, pol))
	}
	for i := range 9 {
		must(st.AddPolicyAsset(ctx, store.PolicyAsset{ID: listID(9, i), TenantID: tenantA, PolicyID: listID(8, 0), AssetID: listID(3, i),
			AssetTag: fmt.Sprintf("AST-%03d", (i*7)%23), AssetName: names[i%len(names)]}))
	}
	for i := range 8 {
		g := store.Assignment{ID: listID(10, i), TenantID: tenantA, AssetID: listID(3, 1), UserID: "u", Action: store.ActionAssigned,
			AssignedAt: base.Add(time.Duration(i%3) * time.Hour)}
		if i%2 == 0 {
			rt := base.Add(time.Duration(i) * time.Hour)
			g.ReturnedAt = &rt
		}
		must(st.InsertAssignment(ctx, g))
	}
}

// pager fetches one page of a list.
type pager func(req listquery.Request) ([]string, int, listquery.Request, error)

// walk pages through a list with size 7 and returns the ids in order; every
// page reports the same total and the request it answered.
func walk(t *testing.T, name string, get pager) ([]string, int) {
	t.Helper()
	var ids []string
	total := -1
	for p := 1; ; p++ {
		got, n, applied, err := get(listquery.Request{Page: p, PageSize: 7})
		if err != nil {
			t.Fatalf("%s page %d: %v", name, p, err)
		}
		if total >= 0 && n != total {
			t.Fatalf("%s: total changed %d → %d", name, total, n)
		}
		total = n
		if applied.Page != p && len(got) > 0 {
			t.Fatalf("%s: page %d answered as %d", name, p, applied.Page)
		}
		ids = append(ids, got...)
		if p*7 >= n {
			break
		}
	}
	return ids, total
}

func ids[T any](rows []T, id func(T) string) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, id(r))
	}
	return out
}

func TestListPages(t *testing.T) {
	ctx := context.Background()
	db := openRepo(t)
	mem := memstore.New()
	seedLists(t, db)
	seedLists(t, mem)

	type list struct {
		name string
		spec listquery.Spec
		page func(st repo.Store, sort string, dir listquery.Dir) pager
		want int
	}
	sorted := func(fn func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error)) func(repo.Store, string, listquery.Dir) pager {
		return func(st repo.Store, sort string, dir listquery.Dir) pager {
			return func(req listquery.Request) ([]string, int, listquery.Request, error) {
				req.Sort, req.Order = sort, dir
				return fn(st, req)
			}
		}
	}
	lists := []list{
		{"assets", store.AssetList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PageAssets(ctx, tenantA, store.AssetFilter{}, req)
			return ids(r, func(x store.Asset) string { return x.ID }), n, a, err
		}), 23},
		{"suppliers", store.SupplierList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PageSuppliers(ctx, tenantA, store.ListOpts{}, req)
			return ids(r, func(x store.Supplier) string { return x.ID }), n, a, err
		}), 11},
		{"consumables", store.ConsumableList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PageConsumables(ctx, tenantA, store.ListOpts{}, req)
			return ids(r, func(x store.Consumable) string { return x.ID }), n, a, err
		}), 11},
		{"licenses", store.LicenseList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PageLicenses(ctx, tenantA, store.ListOpts{}, req)
			return ids(r, func(x store.License) string { return x.ID }), n, a, err
		}), 11},
		{"insurance", store.InsuranceList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PageInsurance(ctx, tenantA, store.ListOpts{}, req)
			return ids(r, func(x store.InsurancePolicy) string { return x.ID }), n, a, err
		}), 11},
		{"policy assets", store.PolicyAssetList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PagePolicyAssets(ctx, tenantA, listID(8, 0), req)
			return ids(r, func(x store.PolicyAsset) string { return x.ID }), n, a, err
		}), 9},
		{"assignments", store.AssignmentList, sorted(func(st repo.Store, req listquery.Request) ([]string, int, listquery.Request, error) {
			r, n, a, err := st.PageAssignments(ctx, tenantA, listID(3, 1), req)
			return ids(r, func(x store.Assignment) string { return x.ID }), n, a, err
		}), 8},
	}
	for _, l := range lists {
		for field := range l.spec.Fields {
			for _, dir := range []listquery.Dir{listquery.Asc, listquery.Desc} {
				name := fmt.Sprintf("%s/%s/%s", l.name, field, dir)
				got, total := walk(t, name, l.page(db, field, dir))
				if total != l.want || len(got) != l.want {
					t.Fatalf("%s: total %d, rows %d, want %d", name, total, len(got), l.want)
				}
				seen := map[string]bool{}
				for _, id := range got {
					if seen[id] {
						t.Fatalf("%s: %s paged twice", name, id)
					}
					seen[id] = true
				}
				want, _ := walk(t, name+" (memstore)", l.page(mem, field, dir))
				if !slices.Equal(got, want) {
					t.Fatalf("%s: database order\n%v\nmemstore order\n%v", name, got, want)
				}
			}
		}
	}

	t.Run("filters before count, clamp, tenant isolation", func(t *testing.T) {
		rows, total, applied, err := db.PageAssets(ctx, tenantA, store.AssetFilter{Status: store.AssetBroken}, listquery.Request{Page: 99, PageSize: 2})
		if err != nil || total != 6 || applied.Page != 3 || len(rows) != 2 {
			t.Fatalf("status filter: total %d page %d rows %d err %v", total, applied.Page, len(rows), err)
		}
		for _, a := range rows {
			if a.Status != store.AssetBroken {
				t.Fatalf("filtered page holds %s", a.Status)
			}
		}
		_, total, _, err = db.PageAssets(ctx, tenantA, store.AssetFilter{Query: "BRAVO"}, listquery.Request{})
		if err != nil || total != 3 {
			t.Fatalf("text filter total %d err %v", total, err)
		}
		_, total, _, err = db.PageAssets(ctx, tenantA, store.AssetFilter{CategoryID: listID(1, 1)}, listquery.Request{})
		if err != nil || total != 6 {
			t.Fatalf("category filter total %d err %v", total, err)
		}
		rows, total, _, err = db.PageAssets(ctx, tenantB, store.AssetFilter{}, listquery.Request{})
		if err != nil || total != 4 || len(rows) != 4 || rows[0].TenantID != tenantB {
			t.Fatalf("tenant B: total %d rows %d err %v", total, len(rows), err)
		}
		_, total, applied, err = db.PageSuppliers(ctx, tenantB, store.ListOpts{}, listquery.Request{Page: 5})
		if err != nil || total != 0 || applied.Page != 1 {
			t.Fatalf("tenant B suppliers: total %d page %d err %v", total, applied.Page, err)
		}
		_, total, _, err = db.PageSuppliers(ctx, tenantA, store.ListOpts{Query: "supplier 0"}, listquery.Request{})
		if err != nil || total != 10 {
			t.Fatalf("supplier text filter total %d err %v", total, err)
		}
		// A zero request takes the defaults (asset tag ascending).
		rows, _, applied, err = db.PageAssets(ctx, tenantA, store.AssetFilter{}, listquery.Request{})
		if err != nil || applied != (listquery.Request{Page: 1, PageSize: 25, Sort: "asset_tag", Order: listquery.Asc}) || rows[0].AssetTag != "AST-000" {
			t.Fatalf("defaults: %+v %v", applied, err)
		}
	})

	t.Run("cursor lists unchanged (gRPC RPCs, backup walk)", func(t *testing.T) {
		var got []string
		cursor := ""
		for {
			page, err := db.ListAssets(ctx, tenantA, store.AssetFilter{Limit: 5, CursorID: cursor})
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, ids(page, func(a store.Asset) string { return a.ID })...)
			if len(page) < 5 {
				break
			}
			cursor = page[len(page)-1].ID
		}
		if len(got) != 23 || !slices.IsSortedFunc(got, func(a, b string) int { return -compare(a, b) }) {
			t.Fatalf("asset keyset walk: %d rows, id DESC %v", len(got), got)
		}
		sup, err := db.ListSuppliers(ctx, tenantA, store.ListOpts{Limit: 4, CursorID: listID(5, 6)})
		if err != nil || len(sup) != 4 || sup[0].ID != listID(5, 5) || sup[3].ID != listID(5, 2) {
			t.Fatalf("supplier keyset page: %v %v", ids(sup, func(s store.Supplier) string { return s.ID }), err)
		}
		all, err := db.AllAssets(ctx, tenantA)
		if err != nil || len(all) != 23 || all[0].ID != listID(3, 22) {
			t.Fatalf("AllAssets: %d %v", len(all), err)
		}
	})
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
