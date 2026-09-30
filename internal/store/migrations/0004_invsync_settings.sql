-- +goose Up
-- Persisted inventory-sync filters per tenant (feature 030).
CREATE TABLE asset_invsync_settings (
  tenant_id          uuid PRIMARY KEY,
  exclude_vms        boolean NOT NULL DEFAULT false,
  exclude_containers boolean NOT NULL DEFAULT false,
  skip_stale         boolean NOT NULL DEFAULT false,
  skip_retired       boolean NOT NULL DEFAULT false,
  hostname_include   text[] NOT NULL DEFAULT '{}',
  hostname_exclude   text[] NOT NULL DEFAULT '{}',
  os_include         text[] NOT NULL DEFAULT '{}',
  os_exclude         text[] NOT NULL DEFAULT '{}',
  updated_by         text NOT NULL DEFAULT '',
  updated_at         timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE asset_invsync_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_invsync_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON asset_invsync_settings
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.system', true) = 'on')
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.system', true) = 'on');

GRANT SELECT, INSERT, UPDATE, DELETE ON asset_invsync_settings TO asset_app;

-- +goose Down
DROP TABLE IF EXISTS asset_invsync_settings;
