-- +goose Up
-- Per-tenant row-level security on every asset_* table. asset_app is NOBYPASSRLS;
-- every statement runs with app.tenant_id set to the caller's tenant. Trusted
-- worker/maintenance paths (lifecycle scheduler, audit writer, tenant enumeration)
-- set app.system='on' (with app.tenant_id pinned to the nil uuid so the uuid cast
-- stays valid) so the policy admits their cross-tenant access.
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'asset_suppliers','asset_locations','asset_categories','asset_assets',
    'asset_assignments','asset_consumables','asset_licenses','asset_insurance_policies',
    'asset_insurance_policy_assets','asset_documents','asset_notify_state','asset_audit_events'
  ]
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($p$CREATE POLICY tenant_isolation ON %I USING (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.system', true) = 'on') WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid OR current_setting('app.system', true) = 'on')$p$, t);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON %I TO asset_app', t);
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'asset_suppliers','asset_locations','asset_categories','asset_assets',
    'asset_assignments','asset_consumables','asset_licenses','asset_insurance_policies',
    'asset_insurance_policy_assets','asset_documents','asset_notify_state','asset_audit_events'
  ]
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;
-- +goose StatementEnd
