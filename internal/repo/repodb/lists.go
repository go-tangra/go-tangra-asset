package repodb

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-tangra/go-tangra/v4/listquery"
	"github.com/jackc/pgx/v5"

	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// List-contract pages (go-tangra specs/032-server-side-tables): count the rows
// matching the filter, clamp the request to the last page, then select the
// page ORDER BY the Spec's constant expressions with the id tie-breaker. The
// cursor variants (ListX) stay for the gRPC RPCs and the backup walks.

// pageSQL names the parts of one page query; every part is a constant.
type pageSQL struct {
	cols       string // selected columns
	countFrom  string // FROM clause of the count (no joins needed by filters)
	selectFrom string // FROM clause of the page (with the joins sort fields use)
	where      string // filter, parameterised by args
}

// page runs the count and the page query of q in tx.
func page[T any](ctx context.Context, tx pgx.Tx, q pageSQL, args []any, spec listquery.Spec, req listquery.Request,
	scan func(scanner) (T, error)) ([]T, int, listquery.Request, error) {
	req = store.ListRequest(req, spec)
	var total int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+q.countFrom+" WHERE "+q.where, args...).Scan(&total); err != nil {
		return nil, 0, req, err
	}
	req = req.Clamp(total)
	rows, err := tx.Query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT %d OFFSET %d",
		q.cols, q.selectFrom, q.where, req.OrderBy(spec), req.Limit(), req.Offset()), args...)
	if err != nil {
		return nil, 0, req, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, 0, req, err
		}
		out = append(out, v)
	}
	return out, total, req, rows.Err()
}

// qualify prefixes every column of a comma-separated list.
func qualify(cols, prefix string) string {
	parts := strings.Split(cols, ",")
	for i, c := range parts {
		parts[i] = prefix + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}

// searchCond is the case-insensitive substring match of query over cols
// ("" when query is blank).
func searchCond(query string, cols []string, args *[]any) string {
	if query == "" {
		return ""
	}
	*args = append(*args, "%"+query+"%")
	n := len(*args)
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		parts = append(parts, fmt.Sprintf("%s ILIKE $%d", c, n))
	}
	return " AND (" + strings.Join(parts, " OR ") + ")"
}

// assetConds is the asset filter (status, references, assignee, text) as
// " AND …" conditions over columns prefixed with p.
func assetConds(f store.AssetFilter, p string, args *[]any) string {
	var b strings.Builder
	add := func(col string, v string) {
		if v == "" {
			return
		}
		*args = append(*args, v)
		fmt.Fprintf(&b, " AND %s%s=$%d", p, col, len(*args))
	}
	add("status", f.Status)
	add("category_id", f.CategoryID)
	add("supplier_id", f.SupplierID)
	add("location_id", f.LocationID)
	add("user_id", f.UserID)
	b.WriteString(searchCond(f.Query, []string{p + "name", p + "asset_tag", p + "serial", p + "model_name"}, args))
	return b.String()
}

var assetColsA = qualify(assetCols, "a.")

// PageAssets implements repo.Store.
func (d *DB) PageAssets(ctx context.Context, tenantID string, f store.AssetFilter, req listquery.Request) (out []store.Asset, total int, applied listquery.Request, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		args := []any{tenantID}
		where := "a.tenant_id=$1" + assetConds(f, "a.", &args)
		var e error
		out, total, applied, e = page(ctx, tx, pageSQL{cols: assetColsA, countFrom: "asset_assets a",
			selectFrom: "asset_assets a LEFT JOIN asset_categories c ON c.id = a.category_id LEFT JOIN asset_locations l ON l.id = a.location_id",
			where:      where}, args, store.AssetList, req, scanAsset)
		return e
	})
	return
}

// PageAssignments implements repo.Store.
func (d *DB) PageAssignments(ctx context.Context, tenantID, assetID string, req listquery.Request) (out []store.Assignment, total int, applied listquery.Request, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, total, applied, e = page(ctx, tx, pageSQL{cols: assignmentCols, countFrom: "asset_assignments", selectFrom: "asset_assignments",
			where: "tenant_id=$1 AND asset_id=$2"}, []any{tenantID, assetID}, store.AssignmentList, req, scanAssignment)
		return e
	})
	return
}

// pageSimple pages one of the simple lists (tenant + text search).
func pageSimple[T any](ctx context.Context, d *DB, tenantID string, f store.ListOpts, req listquery.Request, cols, from, alias string,
	searchCols []string, spec listquery.Spec, scan func(scanner) (T, error)) (out []T, total int, applied listquery.Request, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		args := []any{tenantID}
		where := alias + "tenant_id=$1" + searchCond(f.Query, searchCols, &args)
		var e error
		out, total, applied, e = page(ctx, tx, pageSQL{cols: cols, countFrom: from, selectFrom: from, where: where}, args, spec, req, scan)
		return e
	})
	return
}

// PageSuppliers implements repo.Store.
func (d *DB) PageSuppliers(ctx context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.Supplier, int, listquery.Request, error) {
	return pageSimple(ctx, d, tenantID, f, req, supplierCols, "asset_suppliers", "", []string{"name", "code"}, store.SupplierList, scanSupplier)
}

// PageConsumables implements repo.Store.
func (d *DB) PageConsumables(ctx context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.Consumable, int, listquery.Request, error) {
	return pageSimple(ctx, d, tenantID, f, req, consumableCols, "asset_consumables", "", []string{"name", "model_name"}, store.ConsumableList, scanConsumable)
}

// PageLicenses implements repo.Store.
func (d *DB) PageLicenses(ctx context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.License, int, listquery.Request, error) {
	return pageSimple(ctx, d, tenantID, f, req, licenseCols, "asset_licenses", "", []string{"name"}, store.LicenseList, scanLicense)
}

// PageInsurance implements repo.Store.
func (d *DB) PageInsurance(ctx context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.InsurancePolicy, int, listquery.Request, error) {
	return pageSimple(ctx, d, tenantID, f, req, insuranceCols, "asset_insurance_policies p", "p.", []string{"p.name", "p.policy_number", "p.provider"},
		store.InsuranceList, scanInsurance)
}

func scanPolicyAsset(sc scanner) (store.PolicyAsset, error) {
	var pa store.PolicyAsset
	err := sc.Scan(&pa.ID, &pa.TenantID, &pa.PolicyID, &pa.AssetID, &pa.CoveredValue, &pa.Notes, &pa.AssetTag, &pa.AssetName, &pa.ModelName, &pa.CreatedBy, &pa.CreatedAt)
	return pa, err
}

// PagePolicyAssets implements repo.Store.
func (d *DB) PagePolicyAssets(ctx context.Context, tenantID, policyID string, req listquery.Request) (out []store.PolicyAsset, total int, applied listquery.Request, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, total, applied, e = page(ctx, tx, pageSQL{cols: policyAssetCols, countFrom: "asset_insurance_policy_assets", selectFrom: "asset_insurance_policy_assets",
			where: "tenant_id=$1 AND policy_id=$2"}, []any{tenantID, policyID}, store.PolicyAssetList, req, scanPolicyAsset)
		return e
	})
	return
}
