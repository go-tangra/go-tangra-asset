-- +goose Up
-- Core asset (ITAM) tables. Every table carries tenant_id and is RLS-protected
-- (policies + grants applied in 0003_rls.sql). The append-only audit hypertable
-- lives in 0002_audit.sql. Every UNIQUE constraint below is a conflict-detection
-- guard and MUST be preserved. Contact PII and object-store credentials are never
-- stored in plain columns (sealed/redacted upstream); photo/document bytes live
-- only in object storage (tenant-prefixed keys). Referenced-parent tables are
-- created first so the foreign keys resolve.

-- Suppliers. Referenced (SET NULL) by assets/consumables/licenses.
CREATE TABLE asset_suppliers (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  name           text NOT NULL DEFAULT '',
  code           text NOT NULL DEFAULT '',
  address        text NOT NULL DEFAULT '',
  city           text NOT NULL DEFAULT '',
  state          text NOT NULL DEFAULT '',
  country        text NOT NULL DEFAULT '',
  postal_code    text NOT NULL DEFAULT '',
  contact_person text NOT NULL DEFAULT '',
  telephone      text NOT NULL DEFAULT '',
  email          text NOT NULL DEFAULT '',
  website        text NOT NULL DEFAULT '',
  notes          text NOT NULL DEFAULT '',
  status         text NOT NULL DEFAULT 'active',
  tags           jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by     text NOT NULL DEFAULT '',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);
CREATE INDEX suppliers_tenant ON asset_suppliers (tenant_id);
CREATE INDEX suppliers_status ON asset_suppliers (tenant_id, status);

-- Location tree (self-referential). Referenced (SET NULL) by assets/consumables.
CREATE TABLE asset_locations (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  name        text NOT NULL DEFAULT '',
  code        text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  parent_id   uuid REFERENCES asset_locations(id) ON DELETE SET NULL,
  path        text NOT NULL DEFAULT '',
  address     text NOT NULL DEFAULT '',
  city        text NOT NULL DEFAULT '',
  state       text NOT NULL DEFAULT '',
  country     text NOT NULL DEFAULT '',
  postal_code text NOT NULL DEFAULT '',
  contact     text NOT NULL DEFAULT '',
  phone       text NOT NULL DEFAULT '',
  email       text NOT NULL DEFAULT '',
  status      text NOT NULL DEFAULT 'active',
  tags        jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by  text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);
CREATE INDEX locations_tenant ON asset_locations (tenant_id);
CREATE INDEX locations_parent ON asset_locations (tenant_id, parent_id);
CREATE INDEX locations_status ON asset_locations (tenant_id, status);

-- Category tree (self-referential). Referenced (SET NULL) by assets/consumables.
CREATE TABLE asset_categories (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  name        text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  parent_id   uuid REFERENCES asset_categories(id) ON DELETE SET NULL,
  icon        text NOT NULL DEFAULT '',
  tags        jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by  text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
-- (tenant_id,name,parent_id) UNIQUE. NULL parent_id would defeat a plain UNIQUE
-- (NULL <> NULL), so the nil uuid stands in for a missing parent — the same
-- COALESCE trick ipam used for its nullable code.
CREATE UNIQUE INDEX categories_uniq_name_parent
  ON asset_categories (tenant_id, name, COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid));
CREATE INDEX categories_tenant ON asset_categories (tenant_id);
CREATE INDEX categories_parent ON asset_categories (tenant_id, parent_id);

-- Assets. The asset_tag guard is the primary conflict-detection unique.
CREATE TABLE asset_assets (
  id                uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  asset_tag         text NOT NULL DEFAULT '',
  name              text NOT NULL DEFAULT '',
  serial            text NOT NULL DEFAULT '',
  model_name        text NOT NULL DEFAULT '',
  model_number      text NOT NULL DEFAULT '',
  category_id       uuid REFERENCES asset_categories(id) ON DELETE SET NULL,
  supplier_id       uuid REFERENCES asset_suppliers(id) ON DELETE SET NULL,
  location_id       uuid REFERENCES asset_locations(id) ON DELETE SET NULL,
  user_id           text NOT NULL DEFAULT '',
  status            text NOT NULL DEFAULT 'deployable',
  photo_key         text NOT NULL DEFAULT '',
  warranty_months   int  NOT NULL DEFAULT 0,
  purchase_date     timestamptz,
  order_number      text NOT NULL DEFAULT '',
  purchase_cost     double precision NOT NULL DEFAULT 0,
  notes             text NOT NULL DEFAULT '',
  salvage_value     double precision NOT NULL DEFAULT 0,
  useful_life_years int  NOT NULL DEFAULT 0,
  depreciation_rate double precision NOT NULL DEFAULT 0.40,
  tags              jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by        text NOT NULL DEFAULT '',
  updated_by        text NOT NULL DEFAULT '',
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, asset_tag)
);
CREATE INDEX assets_tenant   ON asset_assets (tenant_id);
CREATE INDEX assets_serial   ON asset_assets (tenant_id, serial);
CREATE INDEX assets_status   ON asset_assets (tenant_id, status);
CREATE INDEX assets_category ON asset_assets (tenant_id, category_id);
CREATE INDEX assets_supplier ON asset_assets (tenant_id, supplier_id);
CREATE INDEX assets_user     ON asset_assets (tenant_id, user_id);
CREATE INDEX assets_location ON asset_assets (tenant_id, location_id);

-- Assignment history (check-out/check-in). tenant_id ADDED vs source.
CREATE TABLE asset_assignments (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  asset_id    uuid NOT NULL REFERENCES asset_assets(id) ON DELETE CASCADE,
  asset_name  text NOT NULL DEFAULT '',
  user_id     text NOT NULL DEFAULT '',
  user_name   text NOT NULL DEFAULT '',
  action      text NOT NULL DEFAULT 'assigned',
  assigned_at timestamptz NOT NULL DEFAULT now(),
  returned_at timestamptz,
  assigned_by text NOT NULL DEFAULT '',
  notes       text NOT NULL DEFAULT ''
);
CREATE INDEX assignments_asset_returned ON asset_assignments (asset_id, returned_at);
CREATE INDEX assignments_asset          ON asset_assignments (tenant_id, asset_id);
CREATE INDEX assignments_user           ON asset_assignments (tenant_id, user_id);
CREATE INDEX assignments_tenant         ON asset_assignments (tenant_id);

-- Consumables. Stock amount + reorder threshold (low-stock via scheduler).
CREATE TABLE asset_consumables (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  name          text NOT NULL DEFAULT '',
  description   text NOT NULL DEFAULT '',
  category_id   uuid REFERENCES asset_categories(id) ON DELETE SET NULL,
  supplier_id   uuid REFERENCES asset_suppliers(id) ON DELETE SET NULL,
  location_id   uuid REFERENCES asset_locations(id) ON DELETE SET NULL,
  model_name    text NOT NULL DEFAULT '',
  model_number  text NOT NULL DEFAULT '',
  amount        int  NOT NULL DEFAULT 0,
  min_amount    int  NOT NULL DEFAULT 0,
  purchase_date timestamptz,
  purchase_cost double precision NOT NULL DEFAULT 0,
  order_number  text NOT NULL DEFAULT '',
  notes         text NOT NULL DEFAULT '',
  tags          jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by    text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX consumables_name     ON asset_consumables (tenant_id, name);
CREATE INDEX consumables_tenant   ON asset_consumables (tenant_id);
CREATE INDEX consumables_category ON asset_consumables (tenant_id, category_id);
CREATE INDEX consumables_supplier ON asset_consumables (tenant_id, supplier_id);
CREATE INDEX consumables_location ON asset_consumables (tenant_id, location_id);

-- Licenses. Auto-expire past valid_to (scheduler).
CREATE TABLE asset_licenses (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  name          text NOT NULL DEFAULT '',
  supplier_id   uuid REFERENCES asset_suppliers(id) ON DELETE SET NULL,
  purchase_date timestamptz,
  purchase_cost double precision NOT NULL DEFAULT 0,
  order_number  text NOT NULL DEFAULT '',
  valid_from    timestamptz,
  valid_to      timestamptz,
  notes         text NOT NULL DEFAULT '',
  status        text NOT NULL DEFAULT 'active',
  metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by    text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX licenses_name     ON asset_licenses (tenant_id, name);
CREATE INDEX licenses_tenant   ON asset_licenses (tenant_id);
CREATE INDEX licenses_supplier ON asset_licenses (tenant_id, supplier_id);
CREATE INDEX licenses_status   ON asset_licenses (tenant_id, status);

-- Insurance policies. policy_number is the primary conflict guard; name is also
-- unique per tenant.
CREATE TABLE asset_insurance_policies (
  id             uuid PRIMARY KEY,
  tenant_id      uuid NOT NULL,
  name           text NOT NULL DEFAULT '',
  policy_number  text NOT NULL DEFAULT '',
  provider       text NOT NULL DEFAULT '',
  coverage_type  text NOT NULL DEFAULT '',
  premium_amount double precision NOT NULL DEFAULT 0,
  deductible     double precision NOT NULL DEFAULT 0,
  coverage_limit double precision NOT NULL DEFAULT 0,
  valid_from     timestamptz,
  valid_to       timestamptz,
  status         text NOT NULL DEFAULT 'active',
  notes          text NOT NULL DEFAULT '',
  metadata       jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by     text NOT NULL DEFAULT '',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, policy_number),
  UNIQUE (tenant_id, name)
);
CREATE INDEX insurance_tenant ON asset_insurance_policies (tenant_id);
CREATE INDEX insurance_status ON asset_insurance_policies (tenant_id, status);

-- Insurance policy <-> asset M2M join. tenant_id ADDED vs source. The
-- (policy_id,asset_id) unique is the duplicate-link guard.
CREATE TABLE asset_insurance_policy_assets (
  id            uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL,
  policy_id     uuid NOT NULL REFERENCES asset_insurance_policies(id) ON DELETE CASCADE,
  asset_id      uuid NOT NULL REFERENCES asset_assets(id) ON DELETE CASCADE,
  covered_value double precision NOT NULL DEFAULT 0,
  notes         text NOT NULL DEFAULT '',
  asset_tag     text NOT NULL DEFAULT '',
  asset_name    text NOT NULL DEFAULT '',
  model_name    text NOT NULL DEFAULT '',
  created_by    text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (policy_id, asset_id)
);
CREATE INDEX policy_assets_policy ON asset_insurance_policy_assets (tenant_id, policy_id);
CREATE INDEX policy_assets_asset  ON asset_insurance_policy_assets (tenant_id, asset_id);
CREATE INDEX policy_assets_tenant ON asset_insurance_policy_assets (tenant_id);

-- Documents (polymorphic: asset|consumable|license). tenant_id ADDED vs source.
-- No FK (entity is polymorphic); asset deletion removes its documents explicitly
-- in the store layer. storage_key is globally unique (object-store key guard).
CREATE TABLE asset_documents (
  id          uuid PRIMARY KEY,
  tenant_id   uuid NOT NULL,
  entity_type text NOT NULL DEFAULT '',
  entity_id   uuid NOT NULL,
  file_name   text NOT NULL DEFAULT '',
  file_size   bigint NOT NULL DEFAULT 0,
  mime_type   text NOT NULL DEFAULT '',
  storage_key text NOT NULL,
  checksum    text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  uploaded_by text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (storage_key)
);
CREATE INDEX documents_entity ON asset_documents (tenant_id, entity_type, entity_id);
CREATE INDEX documents_tenant ON asset_documents (tenant_id);

-- Scheduler dedup state. (tenant_id,condition_key) unique prevents duplicate
-- lifecycle alerts (warranty/license/insurance expiry, low stock).
CREATE TABLE asset_notify_state (
  id               uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL,
  condition_key    text NOT NULL,
  last_notified_at timestamptz NOT NULL DEFAULT now(),
  last_state       text NOT NULL DEFAULT '',
  UNIQUE (tenant_id, condition_key)
);
CREATE INDEX notify_state_tenant ON asset_notify_state (tenant_id);

-- +goose Down
DROP TABLE IF EXISTS asset_notify_state;
DROP TABLE IF EXISTS asset_documents;
DROP TABLE IF EXISTS asset_insurance_policy_assets;
DROP TABLE IF EXISTS asset_insurance_policies;
DROP TABLE IF EXISTS asset_licenses;
DROP TABLE IF EXISTS asset_consumables;
DROP TABLE IF EXISTS asset_assignments;
DROP TABLE IF EXISTS asset_assets;
DROP TABLE IF EXISTS asset_categories;
DROP TABLE IF EXISTS asset_locations;
DROP TABLE IF EXISTS asset_suppliers;
