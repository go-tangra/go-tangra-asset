// Package repodb binds repo.Store to TimescaleDB via *store.Store. Tenant-scoped
// calls run in a tenant transaction (RLS); system-scoped calls (tenant
// enumeration for the scheduler, audit) run under the system-scope pin. Every
// UNIQUE constraint of the schema is mapped to repo.ErrConflict; the category/
// location delete guards and referenced-supplier checks surface repo.ErrNotEmpty.
package repodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// DB implements repo.Store over *store.Store.
type DB struct {
	St *store.Store
}

// New wraps the store.
func New(st *store.Store) *DB { return &DB{St: st} }

// Close releases the underlying pool.
func (d *DB) Close() { d.St.Close() }

func (d *DB) tenant(ctx context.Context, tid string, fn func(tx pgx.Tx) error) error {
	return d.St.Tx(ctx, store.Scope{TenantID: tid}, fn)
}
func (d *DB) system(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return d.St.Tx(ctx, store.Scope{System: true}, fn)
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface{ Scan(dest ...any) error }

// mapErr maps store errors for reads and deletes: no row → ErrNotFound, a
// unique violation → ErrConflict, a foreign-key violation (dependents exist) →
// ErrNotEmpty.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return repo.ErrConflict
		case "23503":
			return repo.ErrNotEmpty
		}
	}
	return err
}

// mapWriteErr is mapErr for inserts/updates, where a foreign-key violation
// means a referenced row (category, supplier, location, parent) is missing.
func mapWriteErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return repo.ErrNotFound
	}
	return mapErr(err)
}

func mustJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// nullID maps "" to NULL for nullable uuid columns.
func nullID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func tags(b []byte) map[string]string {
	out := map[string]string{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

// affected returns ErrNotFound when no row was touched.
func affected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// affectedWrite is affected for updates (foreign-key violations → ErrNotFound).
func affectedWrite(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return mapWriteErr(err)
	}
	return affected(tag, nil)
}

func now() time.Time { return time.Now().UTC() }

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return now()
	}
	return t
}

// Statement verbs.
const (
	sqlRemove = "DELETE FROM "
	sqlInsert = "INSERT INTO "
	sqlUpdate = "UPDATE "
)

// ---- assets

const assetCols = `id, tenant_id, asset_tag, name, serial, model_name, model_number, category_id, supplier_id, location_id,
	user_id, status, photo_key, warranty_months, purchase_date, order_number, purchase_cost, notes, salvage_value,
	useful_life_years, depreciation_rate, tags, created_by, updated_by, created_at, updated_at`

func scanAsset(sc scanner) (store.Asset, error) {
	var a store.Asset
	var cat, sup, loc *string
	var tg []byte
	if err := sc.Scan(&a.ID, &a.TenantID, &a.AssetTag, &a.Name, &a.Serial, &a.ModelName, &a.ModelNumber, &cat, &sup, &loc,
		&a.UserID, &a.Status, &a.PhotoKey, &a.WarrantyMonths, &a.PurchaseDate, &a.OrderNumber, &a.PurchaseCost, &a.Notes, &a.SalvageValue,
		&a.UsefulLifeYears, &a.DepreciationRate, &tg, &a.CreatedBy, &a.UpdatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return store.Asset{}, err
	}
	a.CategoryID, a.SupplierID, a.LocationID = deref(cat), deref(sup), deref(loc)
	a.Tags = tags(tg)
	return a, nil
}

func (d *DB) CreateAsset(ctx context.Context, a store.Asset) error {
	return d.tenant(ctx, a.TenantID, func(tx pgx.Tx) error {
		if a.ID == "" {
			a.ID = store.NewID()
		}
		if a.Status == "" {
			a.Status = store.AssetDeployable
		}
		if a.DepreciationRate == 0 {
			a.DepreciationRate = 0.40
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_assets (`+assetCols+`) VALUES
			($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)`,
			a.ID, a.TenantID, a.AssetTag, a.Name, a.Serial, a.ModelName, a.ModelNumber, nullID(a.CategoryID), nullID(a.SupplierID), nullID(a.LocationID),
			a.UserID, a.Status, a.PhotoKey, a.WarrantyMonths, a.PurchaseDate, a.OrderNumber, a.PurchaseCost, a.Notes, a.SalvageValue,
			a.UsefulLifeYears, a.DepreciationRate, mustJSON(a.Tags), a.CreatedBy, a.UpdatedBy, orNow(a.CreatedAt), orNow(a.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) getAssetWhere(ctx context.Context, tenantID, where string, arg any) (out store.Asset, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanAsset(tx.QueryRow(ctx, "SELECT "+assetCols+" FROM asset_assets WHERE tenant_id=$1 AND "+where, tenantID, arg))
		return mapErr(e)
	})
	return
}

func (d *DB) GetAsset(ctx context.Context, tenantID, id string) (store.Asset, error) {
	return d.getAssetWhere(ctx, tenantID, "id=$2", id)
}

func (d *DB) FindAssetByTag(ctx context.Context, tenantID, tag string) (store.Asset, error) {
	return d.getAssetWhere(ctx, tenantID, "asset_tag=$2", tag)
}

func (d *DB) FindAssetBySerial(ctx context.Context, tenantID, serial string) (store.Asset, error) {
	return d.getAssetWhere(ctx, tenantID, "serial=$2 AND serial<>''", serial)
}

func (d *DB) ListAssets(ctx context.Context, tenantID string, f store.AssetFilter) (out []store.Asset, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var b strings.Builder
		b.WriteString("SELECT " + assetCols + " FROM asset_assets WHERE tenant_id=$1")
		args := []any{tenantID}
		add := func(cond string, val any) {
			args = append(args, val)
			b.WriteString(fmt.Sprintf(cond, len(args)))
		}
		if f.Status != "" {
			add(" AND status=$%d", f.Status)
		}
		if f.CategoryID != "" {
			add(" AND category_id=$%d", f.CategoryID)
		}
		if f.SupplierID != "" {
			add(" AND supplier_id=$%d", f.SupplierID)
		}
		if f.LocationID != "" {
			add(" AND location_id=$%d", f.LocationID)
		}
		if f.UserID != "" {
			add(" AND user_id=$%d", f.UserID)
		}
		if f.Query != "" {
			add(" AND (name ILIKE $%d OR asset_tag ILIKE $%[1]d OR serial ILIKE $%[1]d OR model_name ILIKE $%[1]d)", "%"+f.Query+"%")
		}
		if f.CursorID != "" {
			add(" AND id < $%d", f.CursorID)
		}
		b.WriteString(" ORDER BY id DESC")
		if f.Limit > 0 {
			add(" LIMIT $%d", f.Limit)
		}
		rows, e := tx.Query(ctx, b.String(), args...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			a, e := scanAsset(rows)
			if e != nil {
				return e
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateAsset(ctx context.Context, a store.Asset) error {
	return d.tenant(ctx, a.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_assets SET asset_tag=$3, name=$4, serial=$5, model_name=$6, model_number=$7, category_id=$8, supplier_id=$9,
			location_id=$10, user_id=$11, status=$12, photo_key=$13, warranty_months=$14, purchase_date=$15, order_number=$16, purchase_cost=$17, notes=$18,
			salvage_value=$19, useful_life_years=$20, depreciation_rate=$21, tags=$22, updated_by=$23, updated_at=$24 WHERE tenant_id=$1 AND id=$2`,
			a.TenantID, a.ID, a.AssetTag, a.Name, a.Serial, a.ModelName, a.ModelNumber, nullID(a.CategoryID), nullID(a.SupplierID), nullID(a.LocationID),
			a.UserID, a.Status, a.PhotoKey, a.WarrantyMonths, a.PurchaseDate, a.OrderNumber, a.PurchaseCost, a.Notes, a.SalvageValue, a.UsefulLifeYears,
			a.DepreciationRate, mustJSON(a.Tags), a.UpdatedBy, orNow(a.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

// DeleteAsset removes the asset row (assignments and policy links cascade by
// FK) and its polymorphic document rows (no FK).
func (d *DB) DeleteAsset(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, sqlRemove+`asset_documents WHERE tenant_id=$1 AND entity_type='asset' AND entity_id=$2`, tenantID, id); err != nil {
			return mapErr(err)
		}
		tag, err := tx.Exec(ctx, sqlRemove+`asset_assets WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

func (d *DB) countAssets(ctx context.Context, tenantID, col, id string) (n int64, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT count(*) FROM asset_assets WHERE tenant_id=$1 AND "+col+"=$2", tenantID, id).Scan(&n)
	})
	return
}

func (d *DB) CountAssetsByCategory(ctx context.Context, tenantID, id string) (int64, error) {
	return d.countAssets(ctx, tenantID, "category_id", id)
}
func (d *DB) CountAssetsBySupplier(ctx context.Context, tenantID, id string) (int64, error) {
	return d.countAssets(ctx, tenantID, "supplier_id", id)
}
func (d *DB) CountAssetsByLocation(ctx context.Context, tenantID, id string) (int64, error) {
	return d.countAssets(ctx, tenantID, "location_id", id)
}

func (d *DB) AllAssets(ctx context.Context, tenantID string) ([]store.Asset, error) {
	return d.ListAssets(ctx, tenantID, store.AssetFilter{})
}

// ---- assignments

const assignmentCols = `id, tenant_id, asset_id, asset_name, user_id, user_name, action, assigned_at, returned_at, assigned_by, notes`

func scanAssignment(sc scanner) (store.Assignment, error) {
	var g store.Assignment
	err := sc.Scan(&g.ID, &g.TenantID, &g.AssetID, &g.AssetName, &g.UserID, &g.UserName, &g.Action, &g.AssignedAt, &g.ReturnedAt, &g.AssignedBy, &g.Notes)
	return g, err
}

func (d *DB) InsertAssignment(ctx context.Context, g store.Assignment) error {
	return d.tenant(ctx, g.TenantID, func(tx pgx.Tx) error {
		if g.ID == "" {
			g.ID = store.NewID()
		}
		if g.Action == "" {
			g.Action = store.ActionAssigned
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_assignments (`+assignmentCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			g.ID, g.TenantID, g.AssetID, g.AssetName, g.UserID, g.UserName, g.Action, orNow(g.AssignedAt), g.ReturnedAt, g.AssignedBy, g.Notes)
		return mapWriteErr(err)
	})
}

func (d *DB) CloseActiveAssignment(ctx context.Context, tenantID, assetID string, at time.Time) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sqlUpdate+`asset_assignments SET returned_at=$3 WHERE tenant_id=$1 AND asset_id=$2 AND returned_at IS NULL`, tenantID, assetID, orNow(at))
		return mapWriteErr(err)
	})
}

func (d *DB) ListAssignments(ctx context.Context, tenantID, assetID string, limit int) (out []store.Assignment, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		q := "SELECT " + assignmentCols + " FROM asset_assignments WHERE tenant_id=$1 AND asset_id=$2 ORDER BY assigned_at DESC, id DESC"
		args := []any{tenantID, assetID}
		if limit > 0 {
			q += " LIMIT $3"
			args = append(args, limit)
		}
		rows, e := tx.Query(ctx, q, args...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			g, e := scanAssignment(rows)
			if e != nil {
				return e
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return
}

// ---- documents

const documentCols = `id, tenant_id, entity_type, entity_id, file_name, file_size, mime_type, storage_key, checksum, description, uploaded_by, created_at`

func scanDocument(sc scanner) (store.Document, error) {
	var x store.Document
	err := sc.Scan(&x.ID, &x.TenantID, &x.EntityType, &x.EntityID, &x.FileName, &x.FileSize, &x.MimeType, &x.StorageKey, &x.Checksum, &x.Description, &x.UploadedBy, &x.CreatedAt)
	return x, err
}

func (d *DB) InsertDocument(ctx context.Context, x store.Document) error {
	return d.tenant(ctx, x.TenantID, func(tx pgx.Tx) error {
		if x.ID == "" {
			x.ID = store.NewID()
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_documents (`+documentCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			x.ID, x.TenantID, x.EntityType, x.EntityID, x.FileName, x.FileSize, x.MimeType, x.StorageKey, x.Checksum, x.Description, x.UploadedBy, orNow(x.CreatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetDocument(ctx context.Context, tenantID, id string) (out store.Document, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanDocument(tx.QueryRow(ctx, "SELECT "+documentCols+" FROM asset_documents WHERE tenant_id=$1 AND id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

func (d *DB) ListDocuments(ctx context.Context, tenantID, entityType, entityID string) (out []store.Document, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, "SELECT "+documentCols+" FROM asset_documents WHERE tenant_id=$1 AND entity_type=$2 AND entity_id=$3 ORDER BY id DESC", tenantID, entityType, entityID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			x, e := scanDocument(rows)
			if e != nil {
				return e
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return
}

func (d *DB) DeleteDocument(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlRemove+`asset_documents WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

// ---- categories

const categoryCols = `c.id, c.tenant_id, c.name, c.description, c.parent_id, c.icon, c.tags, c.created_by, c.created_at, c.updated_at,
	(SELECT count(*) FROM asset_assets a WHERE a.category_id=c.id) AS asset_count,
	(SELECT count(*) FROM asset_categories k WHERE k.parent_id=c.id) AS child_count`

func scanCategory(sc scanner) (store.Category, error) {
	var c store.Category
	var parent *string
	var tg []byte
	if err := sc.Scan(&c.ID, &c.TenantID, &c.Name, &c.Description, &parent, &c.Icon, &tg, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.AssetCount, &c.ChildCount); err != nil {
		return store.Category{}, err
	}
	c.ParentID, c.Tags = deref(parent), tags(tg)
	return c, nil
}

func (d *DB) CreateCategory(ctx context.Context, c store.Category) error {
	return d.tenant(ctx, c.TenantID, func(tx pgx.Tx) error {
		if c.ID == "" {
			c.ID = store.NewID()
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_categories (id, tenant_id, name, description, parent_id, icon, tags, created_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, c.ID, c.TenantID, c.Name, c.Description, nullID(c.ParentID), c.Icon, mustJSON(c.Tags), c.CreatedBy, orNow(c.CreatedAt), orNow(c.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetCategory(ctx context.Context, tenantID, id string) (out store.Category, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanCategory(tx.QueryRow(ctx, "SELECT "+categoryCols+" FROM asset_categories c WHERE c.tenant_id=$1 AND c.id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

func (d *DB) ListCategories(ctx context.Context, tenantID string) (out []store.Category, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, "SELECT "+categoryCols+" FROM asset_categories c WHERE c.tenant_id=$1 ORDER BY c.name", tenantID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			c, e := scanCategory(rows)
			if e != nil {
				return e
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateCategory(ctx context.Context, c store.Category) error {
	return d.tenant(ctx, c.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_categories SET name=$3, description=$4, parent_id=$5, icon=$6, tags=$7, updated_at=$8 WHERE tenant_id=$1 AND id=$2`,
			c.TenantID, c.ID, c.Name, c.Description, nullID(c.ParentID), c.Icon, mustJSON(c.Tags), orNow(c.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

// DeleteCategory refuses (ErrNotEmpty) while children or assets reference it.
func (d *DB) DeleteCategory(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var n int64
		if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM asset_categories WHERE tenant_id=$1 AND parent_id=$2) + (SELECT count(*) FROM asset_assets WHERE tenant_id=$1 AND category_id=$2)`, tenantID, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return repo.ErrNotEmpty
		}
		tag, err := tx.Exec(ctx, sqlRemove+`asset_categories WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

func (d *DB) CountChildCategories(ctx context.Context, tenantID, id string) (n int64, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM asset_categories WHERE tenant_id=$1 AND parent_id=$2`, tenantID, id).Scan(&n)
	})
	return
}

// ---- suppliers

const supplierCols = `id, tenant_id, name, code, address, city, state, country, postal_code, contact_person, telephone, email, website, notes, status, tags, created_by, created_at, updated_at`

func scanSupplier(sc scanner) (store.Supplier, error) {
	var s store.Supplier
	var tg []byte
	if err := sc.Scan(&s.ID, &s.TenantID, &s.Name, &s.Code, &s.Address, &s.City, &s.State, &s.Country, &s.PostalCode, &s.ContactPerson, &s.Telephone, &s.Email,
		&s.Website, &s.Notes, &s.Status, &tg, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return store.Supplier{}, err
	}
	s.Tags = tags(tg)
	return s, nil
}

func (d *DB) CreateSupplier(ctx context.Context, s store.Supplier) error {
	return d.tenant(ctx, s.TenantID, func(tx pgx.Tx) error {
		if s.ID == "" {
			s.ID = store.NewID()
		}
		if s.Status == "" {
			s.Status = store.SupActive
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_suppliers (`+supplierCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			s.ID, s.TenantID, s.Name, s.Code, s.Address, s.City, s.State, s.Country, s.PostalCode, s.ContactPerson, s.Telephone, s.Email, s.Website, s.Notes, s.Status,
			mustJSON(s.Tags), s.CreatedBy, orNow(s.CreatedAt), orNow(s.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetSupplier(ctx context.Context, tenantID, id string) (out store.Supplier, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanSupplier(tx.QueryRow(ctx, "SELECT "+supplierCols+" FROM asset_suppliers WHERE tenant_id=$1 AND id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

// listOpts appends the query/cursor/limit clauses shared by the simple lists.
// idCol names the id column (qualified when the FROM clause is aliased).
func listOpts(b *strings.Builder, args *[]any, f store.ListOpts, idCol string, searchCols []string) {
	add := func(cond string, val any) {
		*args = append(*args, val)
		b.WriteString(fmt.Sprintf(cond, len(*args)))
	}
	if f.Query != "" {
		parts := make([]string, 0, len(searchCols))
		*args = append(*args, "%"+f.Query+"%")
		n := len(*args)
		for _, c := range searchCols {
			parts = append(parts, fmt.Sprintf("%s ILIKE $%d", c, n))
		}
		b.WriteString(" AND (" + strings.Join(parts, " OR ") + ")")
	}
	if f.CursorID != "" {
		add(" AND "+idCol+" < $%d", f.CursorID)
	}
	b.WriteString(" ORDER BY " + idCol + " DESC")
	if f.Limit > 0 {
		add(" LIMIT $%d", f.Limit)
	}
}

func (d *DB) ListSuppliers(ctx context.Context, tenantID string, f store.ListOpts) (out []store.Supplier, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var b strings.Builder
		b.WriteString("SELECT " + supplierCols + " FROM asset_suppliers WHERE tenant_id=$1")
		args := []any{tenantID}
		listOpts(&b, &args, f, "id", []string{"name", "code"})
		rows, e := tx.Query(ctx, b.String(), args...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			s, e := scanSupplier(rows)
			if e != nil {
				return e
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateSupplier(ctx context.Context, s store.Supplier) error {
	return d.tenant(ctx, s.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_suppliers SET name=$3, code=$4, address=$5, city=$6, state=$7, country=$8, postal_code=$9, contact_person=$10,
			telephone=$11, email=$12, website=$13, notes=$14, status=$15, tags=$16, updated_at=$17 WHERE tenant_id=$1 AND id=$2`,
			s.TenantID, s.ID, s.Name, s.Code, s.Address, s.City, s.State, s.Country, s.PostalCode, s.ContactPerson, s.Telephone, s.Email, s.Website, s.Notes, s.Status,
			mustJSON(s.Tags), orNow(s.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

func (d *DB) DeleteSupplier(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlRemove+`asset_suppliers WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

// ---- locations

const locationCols = `l.id, l.tenant_id, l.name, l.code, l.description, l.parent_id, l.path, l.address, l.city, l.state, l.country, l.postal_code,
	l.contact, l.phone, l.email, l.status, l.tags, l.created_by, l.created_at, l.updated_at,
	(SELECT count(*) FROM asset_locations k WHERE k.parent_id=l.id) AS child_count,
	(SELECT count(*) FROM asset_assets a WHERE a.location_id=l.id) AS asset_count`

func scanLocation(sc scanner) (store.Location, error) {
	var l store.Location
	var parent *string
	var tg []byte
	if err := sc.Scan(&l.ID, &l.TenantID, &l.Name, &l.Code, &l.Description, &parent, &l.Path, &l.Address, &l.City, &l.State, &l.Country, &l.PostalCode,
		&l.Contact, &l.Phone, &l.Email, &l.Status, &tg, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &l.ChildCount, &l.AssetCount); err != nil {
		return store.Location{}, err
	}
	l.ParentID, l.Tags = deref(parent), tags(tg)
	return l, nil
}

func (d *DB) CreateLocation(ctx context.Context, l store.Location) error {
	return d.tenant(ctx, l.TenantID, func(tx pgx.Tx) error {
		if l.ID == "" {
			l.ID = store.NewID()
		}
		if l.Status == "" {
			l.Status = store.LocActive
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_locations (id, tenant_id, name, code, description, parent_id, path, address, city, state, country, postal_code,
			contact, phone, email, status, tags, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
			l.ID, l.TenantID, l.Name, l.Code, l.Description, nullID(l.ParentID), l.Path, l.Address, l.City, l.State, l.Country, l.PostalCode,
			l.Contact, l.Phone, l.Email, l.Status, mustJSON(l.Tags), l.CreatedBy, orNow(l.CreatedAt), orNow(l.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetLocation(ctx context.Context, tenantID, id string) (out store.Location, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanLocation(tx.QueryRow(ctx, "SELECT "+locationCols+" FROM asset_locations l WHERE l.tenant_id=$1 AND l.id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

func (d *DB) ListLocations(ctx context.Context, tenantID string) (out []store.Location, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, "SELECT "+locationCols+" FROM asset_locations l WHERE l.tenant_id=$1 ORDER BY l.path", tenantID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			l, e := scanLocation(rows)
			if e != nil {
				return e
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateLocation(ctx context.Context, l store.Location) error {
	return d.tenant(ctx, l.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_locations SET name=$3, code=$4, description=$5, parent_id=$6, path=$7, address=$8, city=$9, state=$10, country=$11,
			postal_code=$12, contact=$13, phone=$14, email=$15, status=$16, tags=$17, updated_at=$18 WHERE tenant_id=$1 AND id=$2`,
			l.TenantID, l.ID, l.Name, l.Code, l.Description, nullID(l.ParentID), l.Path, l.Address, l.City, l.State, l.Country, l.PostalCode,
			l.Contact, l.Phone, l.Email, l.Status, mustJSON(l.Tags), orNow(l.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

// DeleteLocation refuses (ErrNotEmpty) while children or assets reference it.
func (d *DB) DeleteLocation(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var n int64
		if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM asset_locations WHERE tenant_id=$1 AND parent_id=$2) + (SELECT count(*) FROM asset_assets WHERE tenant_id=$1 AND location_id=$2)`, tenantID, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return repo.ErrNotEmpty
		}
		tag, err := tx.Exec(ctx, sqlRemove+`asset_locations WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

func (d *DB) CountChildLocations(ctx context.Context, tenantID, id string) (n int64, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM asset_locations WHERE tenant_id=$1 AND parent_id=$2`, tenantID, id).Scan(&n)
	})
	return
}

// ---- consumables

const consumableCols = `id, tenant_id, name, description, category_id, supplier_id, location_id, model_name, model_number, amount, min_amount,
	purchase_date, purchase_cost, order_number, notes, tags, created_by, created_at, updated_at`

func scanConsumable(sc scanner) (store.Consumable, error) {
	var c store.Consumable
	var cat, sup, loc *string
	var tg []byte
	if err := sc.Scan(&c.ID, &c.TenantID, &c.Name, &c.Description, &cat, &sup, &loc, &c.ModelName, &c.ModelNumber, &c.Amount, &c.MinAmount,
		&c.PurchaseDate, &c.PurchaseCost, &c.OrderNumber, &c.Notes, &tg, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return store.Consumable{}, err
	}
	c.CategoryID, c.SupplierID, c.LocationID, c.Tags = deref(cat), deref(sup), deref(loc), tags(tg)
	return c, nil
}

func (d *DB) CreateConsumable(ctx context.Context, c store.Consumable) error {
	return d.tenant(ctx, c.TenantID, func(tx pgx.Tx) error {
		if c.ID == "" {
			c.ID = store.NewID()
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_consumables (`+consumableCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
			c.ID, c.TenantID, c.Name, c.Description, nullID(c.CategoryID), nullID(c.SupplierID), nullID(c.LocationID), c.ModelName, c.ModelNumber, c.Amount, c.MinAmount,
			c.PurchaseDate, c.PurchaseCost, c.OrderNumber, c.Notes, mustJSON(c.Tags), c.CreatedBy, orNow(c.CreatedAt), orNow(c.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetConsumable(ctx context.Context, tenantID, id string) (out store.Consumable, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanConsumable(tx.QueryRow(ctx, "SELECT "+consumableCols+" FROM asset_consumables WHERE tenant_id=$1 AND id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

func (d *DB) ListConsumables(ctx context.Context, tenantID string, f store.ListOpts) (out []store.Consumable, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var b strings.Builder
		b.WriteString("SELECT " + consumableCols + " FROM asset_consumables WHERE tenant_id=$1")
		args := []any{tenantID}
		listOpts(&b, &args, f, "id", []string{"name", "model_name"})
		rows, e := tx.Query(ctx, b.String(), args...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			c, e := scanConsumable(rows)
			if e != nil {
				return e
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateConsumable(ctx context.Context, c store.Consumable) error {
	return d.tenant(ctx, c.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_consumables SET name=$3, description=$4, category_id=$5, supplier_id=$6, location_id=$7, model_name=$8, model_number=$9,
			amount=$10, min_amount=$11, purchase_date=$12, purchase_cost=$13, order_number=$14, notes=$15, tags=$16, updated_at=$17 WHERE tenant_id=$1 AND id=$2`,
			c.TenantID, c.ID, c.Name, c.Description, nullID(c.CategoryID), nullID(c.SupplierID), nullID(c.LocationID), c.ModelName, c.ModelNumber, c.Amount, c.MinAmount,
			c.PurchaseDate, c.PurchaseCost, c.OrderNumber, c.Notes, mustJSON(c.Tags), orNow(c.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

func (d *DB) DeleteConsumable(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, sqlRemove+`asset_documents WHERE tenant_id=$1 AND entity_type='consumable' AND entity_id=$2`, tenantID, id); err != nil {
			return mapErr(err)
		}
		tag, err := tx.Exec(ctx, sqlRemove+`asset_consumables WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

func (d *DB) AllConsumables(ctx context.Context, tenantID string) ([]store.Consumable, error) {
	return d.ListConsumables(ctx, tenantID, store.ListOpts{})
}

// ---- licenses

const licenseCols = `id, tenant_id, name, supplier_id, purchase_date, purchase_cost, order_number, valid_from, valid_to, notes, status, metadata, created_by, created_at, updated_at`

func scanLicense(sc scanner) (store.License, error) {
	var l store.License
	var sup *string
	var md []byte
	if err := sc.Scan(&l.ID, &l.TenantID, &l.Name, &sup, &l.PurchaseDate, &l.PurchaseCost, &l.OrderNumber, &l.ValidFrom, &l.ValidTo, &l.Notes, &l.Status, &md, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return store.License{}, err
	}
	l.SupplierID, l.Metadata = deref(sup), tags(md)
	return l, nil
}

func (d *DB) CreateLicense(ctx context.Context, l store.License) error {
	return d.tenant(ctx, l.TenantID, func(tx pgx.Tx) error {
		if l.ID == "" {
			l.ID = store.NewID()
		}
		if l.Status == "" {
			l.Status = store.LicActive
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_licenses (`+licenseCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			l.ID, l.TenantID, l.Name, nullID(l.SupplierID), l.PurchaseDate, l.PurchaseCost, l.OrderNumber, l.ValidFrom, l.ValidTo, l.Notes, l.Status, mustJSON(l.Metadata), l.CreatedBy, orNow(l.CreatedAt), orNow(l.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetLicense(ctx context.Context, tenantID, id string) (out store.License, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanLicense(tx.QueryRow(ctx, "SELECT "+licenseCols+" FROM asset_licenses WHERE tenant_id=$1 AND id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

func (d *DB) ListLicenses(ctx context.Context, tenantID string, f store.ListOpts) (out []store.License, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var b strings.Builder
		b.WriteString("SELECT " + licenseCols + " FROM asset_licenses WHERE tenant_id=$1")
		args := []any{tenantID}
		listOpts(&b, &args, f, "id", []string{"name"})
		rows, e := tx.Query(ctx, b.String(), args...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			l, e := scanLicense(rows)
			if e != nil {
				return e
			}
			out = append(out, l)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateLicense(ctx context.Context, l store.License) error {
	return d.tenant(ctx, l.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_licenses SET name=$3, supplier_id=$4, purchase_date=$5, purchase_cost=$6, order_number=$7, valid_from=$8, valid_to=$9,
			notes=$10, status=$11, metadata=$12, updated_at=$13 WHERE tenant_id=$1 AND id=$2`,
			l.TenantID, l.ID, l.Name, nullID(l.SupplierID), l.PurchaseDate, l.PurchaseCost, l.OrderNumber, l.ValidFrom, l.ValidTo, l.Notes, l.Status, mustJSON(l.Metadata), orNow(l.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

func (d *DB) DeleteLicense(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, sqlRemove+`asset_documents WHERE tenant_id=$1 AND entity_type='license' AND entity_id=$2`, tenantID, id); err != nil {
			return mapErr(err)
		}
		tag, err := tx.Exec(ctx, sqlRemove+`asset_licenses WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

func (d *DB) AllLicenses(ctx context.Context, tenantID string) ([]store.License, error) {
	return d.ListLicenses(ctx, tenantID, store.ListOpts{})
}

// ---- insurance

const insuranceCols = `p.id, p.tenant_id, p.name, p.policy_number, p.provider, p.coverage_type, p.premium_amount, p.deductible, p.coverage_limit, p.valid_from, p.valid_to,
	p.status, p.notes, p.metadata, p.created_by, p.created_at, p.updated_at, (SELECT count(*) FROM asset_insurance_policy_assets x WHERE x.policy_id=p.id) AS asset_count`

func scanInsurance(sc scanner) (store.InsurancePolicy, error) {
	var p store.InsurancePolicy
	var md []byte
	if err := sc.Scan(&p.ID, &p.TenantID, &p.Name, &p.PolicyNumber, &p.Provider, &p.CoverageType, &p.PremiumAmount, &p.Deductible, &p.CoverageLimit, &p.ValidFrom, &p.ValidTo,
		&p.Status, &p.Notes, &md, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt, &p.AssetCount); err != nil {
		return store.InsurancePolicy{}, err
	}
	p.Metadata = tags(md)
	return p, nil
}

func (d *DB) CreateInsurance(ctx context.Context, p store.InsurancePolicy) error {
	return d.tenant(ctx, p.TenantID, func(tx pgx.Tx) error {
		if p.ID == "" {
			p.ID = store.NewID()
		}
		if p.Status == "" {
			p.Status = store.InsActive
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_insurance_policies (id, tenant_id, name, policy_number, provider, coverage_type, premium_amount, deductible, coverage_limit,
			valid_from, valid_to, status, notes, metadata, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			p.ID, p.TenantID, p.Name, p.PolicyNumber, p.Provider, p.CoverageType, p.PremiumAmount, p.Deductible, p.CoverageLimit, p.ValidFrom, p.ValidTo, p.Status, p.Notes,
			mustJSON(p.Metadata), p.CreatedBy, orNow(p.CreatedAt), orNow(p.UpdatedAt))
		return mapWriteErr(err)
	})
}

func (d *DB) GetInsurance(ctx context.Context, tenantID, id string) (out store.InsurancePolicy, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scanInsurance(tx.QueryRow(ctx, "SELECT "+insuranceCols+" FROM asset_insurance_policies p WHERE p.tenant_id=$1 AND p.id=$2", tenantID, id))
		return mapErr(e)
	})
	return
}

func (d *DB) ListInsurance(ctx context.Context, tenantID string, f store.ListOpts) (out []store.InsurancePolicy, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		var b strings.Builder
		b.WriteString("SELECT " + insuranceCols + " FROM asset_insurance_policies p WHERE p.tenant_id=$1")
		args := []any{tenantID}
		listOpts(&b, &args, f, "p.id", []string{"p.name", "p.policy_number", "p.provider"})
		rows, e := tx.Query(ctx, b.String(), args...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			p, e := scanInsurance(rows)
			if e != nil {
				return e
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return
}

func (d *DB) UpdateInsurance(ctx context.Context, p store.InsurancePolicy) error {
	return d.tenant(ctx, p.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlUpdate+`asset_insurance_policies SET name=$3, policy_number=$4, provider=$5, coverage_type=$6, premium_amount=$7, deductible=$8, coverage_limit=$9,
			valid_from=$10, valid_to=$11, status=$12, notes=$13, metadata=$14, updated_at=$15 WHERE tenant_id=$1 AND id=$2`,
			p.TenantID, p.ID, p.Name, p.PolicyNumber, p.Provider, p.CoverageType, p.PremiumAmount, p.Deductible, p.CoverageLimit, p.ValidFrom, p.ValidTo, p.Status, p.Notes,
			mustJSON(p.Metadata), orNow(p.UpdatedAt))
		return affectedWrite(tag, err)
	})
}

func (d *DB) DeleteInsurance(ctx context.Context, tenantID, id string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlRemove+`asset_insurance_policies WHERE tenant_id=$1 AND id=$2`, tenantID, id)
		return affected(tag, err)
	})
}

func (d *DB) AllInsurance(ctx context.Context, tenantID string) ([]store.InsurancePolicy, error) {
	return d.ListInsurance(ctx, tenantID, store.ListOpts{})
}

const policyAssetCols = `id, tenant_id, policy_id, asset_id, covered_value, notes, asset_tag, asset_name, model_name, created_by, created_at`

func (d *DB) AddPolicyAsset(ctx context.Context, pa store.PolicyAsset) error {
	return d.tenant(ctx, pa.TenantID, func(tx pgx.Tx) error {
		if pa.ID == "" {
			pa.ID = store.NewID()
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_insurance_policy_assets (`+policyAssetCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			pa.ID, pa.TenantID, pa.PolicyID, pa.AssetID, pa.CoveredValue, pa.Notes, pa.AssetTag, pa.AssetName, pa.ModelName, pa.CreatedBy, orNow(pa.CreatedAt))
		if err != nil {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && pg.Code == "23503" {
				return repo.ErrNotFound // policy or asset missing
			}
		}
		return mapWriteErr(err)
	})
}

func (d *DB) RemovePolicyAsset(ctx context.Context, tenantID, policyID, assetID string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, sqlRemove+`asset_insurance_policy_assets WHERE tenant_id=$1 AND policy_id=$2 AND asset_id=$3`, tenantID, policyID, assetID)
		return affected(tag, err)
	})
}

func (d *DB) ListPolicyAssets(ctx context.Context, tenantID, policyID string) (out []store.PolicyAsset, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, "SELECT "+policyAssetCols+" FROM asset_insurance_policy_assets WHERE tenant_id=$1 AND policy_id=$2 ORDER BY id DESC", tenantID, policyID)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var pa store.PolicyAsset
			if e := rows.Scan(&pa.ID, &pa.TenantID, &pa.PolicyID, &pa.AssetID, &pa.CoveredValue, &pa.Notes, &pa.AssetTag, &pa.AssetName, &pa.ModelName, &pa.CreatedBy, &pa.CreatedAt); e != nil {
				return e
			}
			out = append(out, pa)
		}
		return rows.Err()
	})
	return
}

// ---- notify state

func (d *DB) GetNotifyState(ctx context.Context, tenantID, key string) (out store.NotifyState, err error) {
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		return mapErr(tx.QueryRow(ctx, `SELECT id, tenant_id, condition_key, last_notified_at, last_state FROM asset_notify_state WHERE tenant_id=$1 AND condition_key=$2`, tenantID, key).
			Scan(&out.ID, &out.TenantID, &out.ConditionKey, &out.LastNotifiedAt, &out.LastState))
	})
	return
}

func (d *DB) UpsertNotifyState(ctx context.Context, n store.NotifyState) error {
	return d.tenant(ctx, n.TenantID, func(tx pgx.Tx) error {
		if n.ID == "" {
			n.ID = store.NewID()
		}
		_, err := tx.Exec(ctx, sqlInsert+`asset_notify_state (id, tenant_id, condition_key, last_notified_at, last_state) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (tenant_id, condition_key) DO UPDATE SET last_notified_at=EXCLUDED.last_notified_at, last_state=EXCLUDED.last_state`,
			n.ID, n.TenantID, n.ConditionKey, orNow(n.LastNotifiedAt), n.LastState)
		return mapWriteErr(err)
	})
}

// ---- statistics + tenants + audit

func (d *DB) TenantStats(ctx context.Context, tenantID string) (st repo.Stats, err error) {
	st.AssetsByStatus = map[string]int64{}
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT status, count(*) FROM asset_assets WHERE tenant_id=$1 GROUP BY status`, tenantID)
		if e != nil {
			return e
		}
		for rows.Next() {
			var s string
			var n int64
			if e := rows.Scan(&s, &n); e != nil {
				rows.Close()
				return e
			}
			st.AssetsByStatus[s] = n
			st.TotalAssets += n
		}
		rows.Close()
		if e := rows.Err(); e != nil {
			return e
		}
		return tx.QueryRow(ctx, `SELECT
			(SELECT COALESCE(sum(purchase_cost),0) FROM asset_assets WHERE tenant_id=$1),
			(SELECT count(*) FROM asset_consumables WHERE tenant_id=$1),
			(SELECT count(*) FROM asset_consumables WHERE tenant_id=$1 AND min_amount > 0 AND amount <= min_amount),
			(SELECT count(*) FROM asset_suppliers WHERE tenant_id=$1),
			(SELECT count(*) FROM asset_categories WHERE tenant_id=$1),
			(SELECT count(*) FROM asset_locations WHERE tenant_id=$1),
			(SELECT count(*) FROM asset_licenses WHERE tenant_id=$1),
			(SELECT count(*) FROM asset_insurance_policies WHERE tenant_id=$1)`, tenantID).
			Scan(&st.TotalCost, &st.TotalConsumables, &st.LowStock, &st.TotalSuppliers, &st.TotalCategories, &st.TotalLocations, &st.TotalLicenses, &st.TotalInsurance)
	})
	return
}

func (d *DB) TenantIDs(ctx context.Context) (out []string, err error) {
	err = d.system(ctx, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT DISTINCT tenant_id::text FROM (
			SELECT tenant_id FROM asset_assets UNION SELECT tenant_id FROM asset_licenses
			UNION SELECT tenant_id FROM asset_insurance_policies UNION SELECT tenant_id FROM asset_consumables) u ORDER BY 1`)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e := rows.Scan(&id); e != nil {
				return e
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return
}

func (d *DB) AppendAudit(ctx context.Context, row store.AuditRow) error {
	return d.system(ctx, func(tx pgx.Tx) error {
		id := row.ID
		if id == "" {
			id = store.NewID()
		}
		var detail any
		if row.Detail != nil {
			detail = mustJSON(row.Detail)
		}
		_, e := tx.Exec(ctx, sqlInsert+`asset_audit_events (id, tenant_id, at, actor_kind, actor_id, action, subject_kind, subject_id, outcome, reason, detail)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			id, row.TenantID, orNow(row.At), row.ActorKind, row.ActorID, row.Action, row.SubjectKind, row.SubjectID, row.Outcome, row.Reason, detail)
		return e
	})
}

var _ repo.Store = (*DB)(nil)
