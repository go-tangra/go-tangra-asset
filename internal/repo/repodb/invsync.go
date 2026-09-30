package repodb

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// GetInvSyncSettings implements repo.Store.
func (d *DB) GetInvSyncSettings(ctx context.Context, tenantID string) (out store.InvSyncSettings, found bool, err error) {
	out.TenantID = tenantID
	err = d.tenant(ctx, tenantID, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `SELECT exclude_vms, exclude_containers, skip_stale, skip_retired, hostname_include, hostname_exclude,
			os_include, os_exclude, updated_by, updated_at FROM asset_invsync_settings WHERE tenant_id=$1`, tenantID).
			Scan(&out.ExcludeVMs, &out.ExcludeContainers, &out.SkipStale, &out.SkipRetired, &out.HostnameInclude, &out.HostnameExclude,
				&out.OSInclude, &out.OSExclude, &out.UpdatedBy, &out.UpdatedAt)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		return nil
	})
	return
}

// PutInvSyncSettings implements repo.Store.
func (d *DB) PutInvSyncSettings(ctx context.Context, s store.InvSyncSettings) error {
	return d.tenant(ctx, s.TenantID, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `INSERT INTO asset_invsync_settings (tenant_id, exclude_vms, exclude_containers, skip_stale, skip_retired,
			hostname_include, hostname_exclude, os_include, os_exclude, updated_by, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (tenant_id) DO UPDATE SET exclude_vms=EXCLUDED.exclude_vms, exclude_containers=EXCLUDED.exclude_containers,
			  skip_stale=EXCLUDED.skip_stale, skip_retired=EXCLUDED.skip_retired, hostname_include=EXCLUDED.hostname_include,
			  hostname_exclude=EXCLUDED.hostname_exclude, os_include=EXCLUDED.os_include, os_exclude=EXCLUDED.os_exclude,
			  updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
			s.TenantID, s.ExcludeVMs, s.ExcludeContainers, s.SkipStale, s.SkipRetired, nonNil(s.HostnameInclude), nonNil(s.HostnameExclude),
			nonNil(s.OSInclude), nonNil(s.OSExclude), s.UpdatedBy, s.UpdatedAt)
		return mapErr(e)
	})
}
