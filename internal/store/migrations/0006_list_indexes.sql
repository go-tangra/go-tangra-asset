-- +goose Up
-- Server-side list sorting (go-tangra specs/032-server-side-tables): the
-- default and common sort orders of the paged lists, each with the id
-- tie-breaker, within a tenant. Other sort fields filter by tenant first.
CREATE INDEX IF NOT EXISTS assets_tag_order     ON asset_assets (tenant_id, lower(asset_tag), id);
CREATE INDEX IF NOT EXISTS assets_name_order    ON asset_assets (tenant_id, lower(name), id);
CREATE INDEX IF NOT EXISTS assets_created_order ON asset_assets (tenant_id, created_at, id);
CREATE INDEX IF NOT EXISTS suppliers_name_order   ON asset_suppliers (tenant_id, lower(name), id);
CREATE INDEX IF NOT EXISTS consumables_name_order ON asset_consumables (tenant_id, lower(name), id);
CREATE INDEX IF NOT EXISTS licenses_name_order    ON asset_licenses (tenant_id, lower(name), id);
CREATE INDEX IF NOT EXISTS insurance_name_order   ON asset_insurance_policies (tenant_id, lower(name), id);
CREATE INDEX IF NOT EXISTS assignments_asset_order ON asset_assignments (tenant_id, asset_id, assigned_at, id);

-- +goose Down
DROP INDEX IF EXISTS assignments_asset_order;
DROP INDEX IF EXISTS insurance_name_order;
DROP INDEX IF EXISTS licenses_name_order;
DROP INDEX IF EXISTS consumables_name_order;
DROP INDEX IF EXISTS suppliers_name_order;
DROP INDEX IF EXISTS assets_created_order;
DROP INDEX IF EXISTS assets_name_order;
DROP INDEX IF EXISTS assets_tag_order;
