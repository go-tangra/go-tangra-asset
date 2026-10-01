//go:build integration

// Index-backed list sorting (go-tangra specs/032-server-side-tables perf.md):
// with the NotNull sort fields the ORDER BY carries no NULLS LAST, so the 0006
// (tenant_id, col, id) indexes serve the default and descending sorts by a
// forward or backward index scan instead of a full top-N sort; free-text
// search matches LIKE metacharacters literally (security-review F-4). Run with:
//
//	go test -tags integration -run 'TestListIndexPlans|TestListSearchLiteral' ./internal/repo/repodb/
package repodb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/go-tangra/go-tangra/v4/listquery"
	"github.com/jackc/pgx/v5"

	"github.com/go-tangra/go-tangra-asset/v4/internal/repo/repodb"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// migrated starts a database, migrates it and returns the admin connection
// and the application store.
func migrated(t *testing.T) (*pgx.Conn, *store.Store) {
	t.Helper()
	ctx := context.Background()
	adminDSN, appDSN := startDB(t)
	if err := store.Migrate(ctx, adminDSN); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	st, err := store.Open(ctx, appDSN, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(st.Close)
	return admin, st
}

func TestListIndexPlans(t *testing.T) {
	ctx := context.Background()
	admin, st := migrated(t)
	assetID := "0190f7c2-0000-7000-8000-00000000a551"
	for _, q := range []string{
		`INSERT INTO asset_assets (id, tenant_id, asset_tag, name, status, created_at)
		 SELECT gen_random_uuid(), t, 'AST-' || g, 'Asset ' || (g % 977), 'deployable', now() - g * interval '1 minute'
		 FROM generate_series(1, 20000) g, (VALUES ('` + tenantA + `'::uuid), ('` + tenantB + `'::uuid)) v(t)`,
		`INSERT INTO asset_assets (id, tenant_id, asset_tag, name) VALUES ('` + assetID + `', '` + tenantA + `', 'AST-X', 'x')`,
		`INSERT INTO asset_assignments (id, tenant_id, asset_id, assigned_at)
		 SELECT gen_random_uuid(), '` + tenantA + `', '` + assetID + `', now() - g * interval '1 hour' FROM generate_series(1, 5000) g`,
		`INSERT INTO asset_suppliers (id, tenant_id, name, created_at)
		 SELECT gen_random_uuid(), t, 'Supplier ' || g, now() - g * interval '1 minute'
		 FROM generate_series(1, 10000) g, (VALUES ('` + tenantA + `'::uuid), ('` + tenantB + `'::uuid)) v(t)`,
		`ANALYZE`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	const assetFrom = "asset_assets a" +
		" LEFT JOIN asset_categories c ON c.tenant_id = a.tenant_id AND c.id = a.category_id" +
		" LEFT JOIN asset_locations l ON l.tenant_id = a.tenant_id AND l.id = a.location_id"
	for _, c := range []struct {
		name, from, where string
		spec              listquery.Spec
		sort              string
		dir               listquery.Dir
		index             string
	}{
		{"assets default", assetFrom, "a.tenant_id=$1", store.AssetList, "asset_tag", listquery.Asc, "assets_tag_order"},
		{"assets asset_tag desc", assetFrom, "a.tenant_id=$1", store.AssetList, "asset_tag", listquery.Desc, "assets_tag_order"},
		{"assets name desc", assetFrom, "a.tenant_id=$1", store.AssetList, "name", listquery.Desc, "assets_name_order"},
		{"assets created_at desc", assetFrom, "a.tenant_id=$1", store.AssetList, "created_at", listquery.Desc, "assets_created_order"},
		{"assets created_at asc", assetFrom, "a.tenant_id=$1", store.AssetList, "created_at", listquery.Asc, "assets_created_order"},
		{"assignments default", "asset_assignments", "tenant_id=$1 AND asset_id='" + assetID + "'", store.AssignmentList, "assigned_at", listquery.Desc, "assignments_asset_order"},
		{"suppliers name desc", "asset_suppliers", "tenant_id=$1", store.SupplierList, "name", listquery.Desc, "suppliers_name_order"},
	} {
		req := store.ListRequest(listquery.Request{Sort: c.sort, Order: c.dir}, c.spec)
		q := "EXPLAIN SELECT 1 FROM " + c.from + " WHERE " + c.where + " ORDER BY " + req.OrderBy(c.spec) + " LIMIT 25"
		var plan []string
		err := st.Tx(ctx, store.Scope{TenantID: tenantA}, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, q, tenantA)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					return err
				}
				plan = append(plan, line)
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		text := strings.Join(plan, "\n")
		if !strings.Contains(text, " using "+c.index+" ") || strings.Contains(text, "Sort") {
			t.Fatalf("%s: want an index scan on %s without a sort\n%s\n%s", c.name, c.index, q, text)
		}
		t.Logf("%s: %s", c.name, strings.TrimSpace(plan[1]))
	}
}

func TestListSearchLiteral(t *testing.T) {
	ctx := context.Background()
	_, st := migrated(t)
	db := repodb.New(st)
	for i, n := range []string{"100% sure", "a_b", `back\slash`, "plain"} {
		if err := db.CreateAsset(ctx, store.Asset{ID: listID(20, i), TenantID: tenantA, AssetTag: n, Name: n}); err != nil {
			t.Fatal(err)
		}
	}
	for q, want := range map[string]int{"%": 1, "_": 1, `\`: 1, "a_": 1, "PLAIN": 1, "%%": 0, strings.Repeat("x", 5000): 0} {
		rows, total, _, err := db.PageAssets(ctx, tenantA, store.AssetFilter{Query: q}, listquery.Request{})
		if err != nil || total != want || len(rows) != want {
			t.Fatalf("query %.10q: total %d rows %d want %d err %v", q, total, len(rows), want, err)
		}
	}
	// The cursor list (gRPC) shares the escaping.
	rows, err := db.ListAssets(ctx, tenantA, store.AssetFilter{Query: "%", Limit: 10})
	if err != nil || len(rows) != 1 || rows[0].Name != "100% sure" {
		t.Fatalf("cursor list %%: %d %v", len(rows), err)
	}
}
