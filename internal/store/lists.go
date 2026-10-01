package store

import (
	"unicode/utf8"

	"github.com/go-tangra/go-tangra/v4/listquery"
)

// List definitions of the asset tables (go-tangra specs/032-server-side-tables,
// contracts/sortable-fields.md "asset"). NotNull marks fields over NOT NULL
// columns (migration 0001), so their ORDER BY carries no NULLS LAST and the
// 0006 (tenant_id, col, id) indexes serve both directions; the nullable ones
// (purchase_date, warranty_end, valid_to, returned_at and the LEFT JOINed
// category / location) keep NULLS LAST. Sort fields map to constant SQL
// expressions only (the repodb page queries alias asset_assets as a, its
// category and location joins as c and l); the memstore sorts the same public
// names in Go. The gRPC list RPCs and the backup walks keep their id keyset.
var (
	// AssetList pages GET /assets: asset tag order by default. category and
	// location order by the referenced record's name / path (LEFT JOIN; assets
	// without one come last); warranty_end is purchase date + warranty months
	// (no warranty: last).
	AssetList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"asset_tag":     {Expr: "a.asset_tag", Text: true, NotNull: true},
			"name":          {Expr: "a.name", Text: true, NotNull: true},
			"status":        {Expr: "a.status", NotNull: true},
			"category":      {Expr: "c.name", Text: true},
			"location":      {Expr: "COALESCE(NULLIF(l.path, ''), l.name)", Text: true},
			"purchase_date": {Expr: "a.purchase_date", DefaultDir: listquery.Desc},
			"warranty_end":  {Expr: WarrantyEndExpr},
			"created_at":    {Expr: "a.created_at", DefaultDir: listquery.Desc, NotNull: true},
		},
		Default: "asset_tag", TieBreak: "a.id",
	}
	// SupplierList pages GET /suppliers.
	SupplierList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"name":       {Expr: "name", Text: true, NotNull: true},
			"created_at": {Expr: "created_at", DefaultDir: listquery.Desc, NotNull: true},
		},
		Default: "name", TieBreak: "id",
	}
	// ConsumableList pages GET /consumables (amount: the quantity in stock).
	ConsumableList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"name":       {Expr: "name", Text: true, NotNull: true},
			"amount":     {Expr: "amount", NotNull: true},
			"created_at": {Expr: "created_at", DefaultDir: listquery.Desc, NotNull: true},
		},
		Default: "name", TieBreak: "id",
	}
	// LicenseList pages GET /licenses (valid_to: the expiry date).
	LicenseList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"name":       {Expr: "name", Text: true, NotNull: true},
			"valid_to":   {Expr: "valid_to"},
			"created_at": {Expr: "created_at", DefaultDir: listquery.Desc, NotNull: true},
		},
		Default: "name", TieBreak: "id",
	}
	// InsuranceList pages GET /insurance-policies (valid_to: the end date).
	InsuranceList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"name":       {Expr: "p.name", Text: true, NotNull: true},
			"valid_to":   {Expr: "p.valid_to"},
			"created_at": {Expr: "p.created_at", DefaultDir: listquery.Desc, NotNull: true},
		},
		Default: "name", TieBreak: "p.id",
	}
	// PolicyAssetList pages GET /insurance-policies/{id}/assets (the asset tag
	// and name recorded on the coverage row).
	PolicyAssetList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"asset_tag": {Expr: "asset_tag", Text: true, NotNull: true},
			"name":      {Expr: "asset_name", Text: true, NotNull: true},
		},
		Default: "asset_tag", TieBreak: "id",
	}
	// AssignmentList pages GET /assets/{id}/assignments: newest first.
	AssignmentList = listquery.Spec{
		Fields: map[string]listquery.Field{
			"assigned_at": {Expr: "assigned_at", DefaultDir: listquery.Desc, NotNull: true},
			"returned_at": {Expr: "returned_at", DefaultDir: listquery.Desc},
		},
		Default: "assigned_at", TieBreak: "id",
	}
	// DocumentSearchList pages GET /documents/search: paperless relevance order
	// only (not SQL; the hits are windowed in Go), at most SearchWindow hits.
	DocumentSearchList = listquery.Spec{
		Fields:  map[string]listquery.Field{"rank": {Expr: "rank", DefaultDir: listquery.Desc}},
		Default: "rank", TieBreak: "id", MaxSize: SearchWindow,
	}
)

// WarrantyEndExpr is an asset's warranty end (NULL without a purchase date or
// warranty months).
const WarrantyEndExpr = "(CASE WHEN a.warranty_months > 0 THEN a.purchase_date + make_interval(months => a.warranty_months) END)"

// MaxQueryLen caps the free-text list search (runes): longer queries are
// rejected by the HTTP API and truncated by TrimQuery elsewhere (gRPC).
const MaxQueryLen = 200

// TrimQuery cuts a free-text search query to MaxQueryLen runes.
func TrimQuery(q string) string {
	if utf8.RuneCountInString(q) <= MaxQueryLen {
		return q
	}
	return string([]rune(q)[:MaxQueryLen])
}

// SearchWindow is the number of top paperless hits a document search pages over.
const SearchWindow = 100

// ListRequest completes r with the Spec's defaults (a zero Request from an
// internal caller pages with the defaults); an invalid hand-built Request
// falls back to the defaults entirely.
func ListRequest(r listquery.Request, s listquery.Spec) listquery.Request {
	out, err := listquery.New(r.Page, r.PageSize, r.Sort, r.Order, s)
	if err != nil {
		out, _ = listquery.New(0, 0, "", "", s)
	}
	return out
}
