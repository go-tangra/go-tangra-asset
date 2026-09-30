package repodb

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// ListDocumentsByPaperlessIDs implements repo.Store.
func (d *DB) ListDocumentsByPaperlessIDs(ctx context.Context, tenantID string, ids []string) (out []store.Document, err error) {
	if len(ids) == 0 {
		return nil, nil
	}
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, "SELECT "+documentCols+" FROM asset_documents WHERE tenant_id=$1 AND paperless_document_id = ANY($2)", tenantID, ids)
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

// ListUnmigratedDocuments implements repo.Store (system scope).
func (d *DB) ListUnmigratedDocuments(ctx context.Context, limit int) (out []store.Document, err error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	err = d.system(ctx, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, "SELECT "+documentCols+" FROM asset_documents WHERE paperless_document_id = '' AND storage_key LIKE 'tenants/%' ORDER BY created_at, id LIMIT $1", limit)
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

// MoveDocumentToPaperless implements repo.Store.
func (d *DB) MoveDocumentToPaperless(ctx context.Context, tenantID, id, paperlessID string) error {
	return d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		ct, e := tx.Exec(ctx, `UPDATE asset_documents SET paperless_document_id=$3, storage_key=$4 WHERE tenant_id=$1 AND id=$2`,
			tenantID, id, paperlessID, store.PaperlessKeyPrefix+paperlessID)
		if e != nil {
			return mapWriteErr(e)
		}
		if ct.RowsAffected() == 0 {
			return repo.ErrNotFound
		}
		return nil
	})
}
