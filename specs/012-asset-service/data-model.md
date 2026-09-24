# Phase 1 Data Model: Asset Service

All tables carry `tenant_id uuid NOT NULL` with per-tenant RLS (the ent
SystemViewer bypass is replaced by RLS + a scoped system subject for the scheduler).
IDs are application-generated strings. Timestamps `timestamptz`. tags/metadata
JSONB. Enums stored as small text. Contact PII is sealed/redacted. Photos/documents
live only in object storage (keys tenant-prefixed). Every unique constraint is a
conflict-detection guard and MUST be preserved.

## Entities

### asset_assets  (RLS)
- `id` (PK), `tenant_id`, `asset_tag` (auto AST-xxxxxx if blank), `name`, `serial`,
  `model_name`, `model_number`, `category_id` (FK), `supplier_id` (FK),
  `location_id` (FK, set when NOT assigned), `user_id` (platform user, set when assigned),
  `status` (deployable|assigned|broken|archived), `photo_key` (object key, null),
  `warranty_months` (int, null), `purchase_date` (null), `order_number`,
  `purchase_cost` (float, null), `notes`,
  depreciation: `salvage_value` (float,null), `useful_life_years` (int,null),
  `depreciation_rate` (float, default 0.40),
  tags/metadata, audit (created_by/updated_by/created_at/updated_at).
- **Unique**: `(tenant_id,asset_tag)`. Indexes: `(tenant_id,serial)`, tenant_id, status,
  category_id, supplier_id, user_id, location_id.
- book value (current depreciated) is COMPUTED (not stored).

### asset_assignments  (RLS — tenant_id ADDED vs source)
- `id` (PK), `tenant_id`, `asset_id` (FK), `asset_name` (denorm), `user_id`,
  `user_name` (denorm), `action` (assigned|unassigned|transferred),
  `assigned_at`, `returned_at` (null = active), `assigned_by`, `notes`.
- Indexes: `(asset_id,returned_at)`, asset_id, user_id, tenant_id.

### asset_documents  (RLS — tenant_id ADDED; polymorphic)
- `id` (PK), `tenant_id`, `entity_type` (asset|consumable|license), `entity_id`,
  `file_name`, `file_size`, `mime_type`, `storage_key` (object key), `checksum` (sha256),
  `description`, `uploaded_by`, `created_at`.
- **Unique**: `storage_key`. Indexes: `(tenant_id,entity_type,entity_id)`.

### asset_categories  (RLS, self-referential tree)
- `id, tenant_id, name, description, parent_id (self FK), icon, tags/metadata, audit`.
- asset_count/child_count COMPUTED. **Unique**: `(tenant_id,name,parent_id)`. Indexes: tenant_id, parent_id.

### asset_suppliers  (RLS)
- `id, tenant_id, name, code, address, city, state, country, postal_code,
  contact_person (sealed/redacted), telephone (sealed/redacted), email (sealed/redacted),
  website, notes, status (active|inactive), tags/metadata, audit`.
- **Unique**: `(tenant_id,name)`. Indexes: tenant_id, status.

### asset_locations  (RLS, self-referential tree)
- `id, tenant_id, name, code, description, parent_id (self FK), path, address, city,
  state, country, postal_code, contact/phone/email (sealed/redacted),
  status (active|planned|decommissioned), tags/metadata, audit`.
- child_count/asset_count COMPUTED. **Unique**: `(tenant_id,name)`. Indexes: tenant_id, parent_id, status.

### asset_consumables  (RLS)
- `id, tenant_id, name, description, category_id, supplier_id, location_id,
  model_name, model_number, amount (int stock), min_amount (int reorder threshold),
  purchase_date, purchase_cost, order_number, notes, tags/metadata, audit`.
- Indexes: `(tenant_id,name)`, tenant_id, category_id, supplier_id, location_id.
- low-stock (amount<=min_amount) evaluated by the scheduler.

### asset_licenses  (RLS)
- `id, tenant_id, name, supplier_id, purchase_date, purchase_cost, order_number,
  valid_from, valid_to, notes, status (active|expired|suspended), metadata, audit`.
- Indexes: `(tenant_id,name)`, tenant_id, supplier_id, status. Auto-expire past valid_to.

### asset_insurance_policies  (RLS)
- `id, tenant_id, name, policy_number, provider,
  coverage_type (all_risk|fire_theft|liability|equipment_breakdown|cyber),
  premium_amount, deductible, coverage_limit (floats, null), valid_from, valid_to,
  status (active|expired|cancelled), notes, metadata, audit`.
- asset_count COMPUTED. **Unique**: `(tenant_id,policy_number)`. Also `(tenant_id,name)`. Indexes: tenant_id, status.

### asset_insurance_policy_assets  (RLS, M2M)
- `id, tenant_id, policy_id (FK), asset_id (FK), covered_value (float,null),
  notes, denorm asset_tag/asset_name/model_name, created_by, created_at`.
- **Unique**: `(policy_id,asset_id)`. Indexes: policy_id, asset_id, tenant_id.

### asset_notify_state  (RLS — scheduler dedup)
- `id, tenant_id, condition_key (e.g. "warranty:<assetID>" / "license:<id>" /
  "insurance:<id>" / "low_stock:<consumableID>"), last_notified_at, last_state`.
- **Unique**: `(tenant_id,condition_key)`. Prevents duplicate lifecycle alerts.

### asset_audit_events  (append-only, hypertable)
- `id, tenant_id, at, actor_kind (user|service|system), actor_id, action,
  subject_kind (asset|assignment|document|category|supplier|location|consumable|
  license|insurance|policy_asset|sync|backup|system), subject_id, outcome
  (ok|refused|error), reason, detail (jsonb, redacted)`.

## Enums / vocabularies
AssetStatus(deployable|assigned|broken|archived), AssignmentAction(assigned|
unassigned|transferred), LocationStatus(active|planned|decommissioned),
SupplierStatus(active|inactive), LicenseStatus(active|expired|suspended),
InsuranceStatus(active|expired|cancelled), CoverageType(all_risk|fire_theft|
liability|equipment_breakdown|cyber), entity_type(asset|consumable|license),
RestoreMode(skip|overwrite).

**Permissions (API)**: assets:read, assets:manage, assets:assign, categories:manage,
suppliers:manage, locations:manage, consumables:manage, licenses:manage,
insurance:manage, documents:manage, inventory:sync, stats:read, backup:manage.

**AuditAction**: asset_/category_/supplier_/location_/consumable_/license_/insurance_
created/updated/deleted, asset_assigned, asset_unassigned, photo_uploaded/deleted,
document_uploaded/deleted, policy_asset_added/removed, inventory_sync_previewed/
executed, license_expired, insurance_expired, backup_exported/imported, access_refused.

## Relationships
Asset → category/supplier/location (FK); asset → assignments (1-N); asset →
documents (polymorphic); asset ↔ insurance policy (M2M via policy_assets). Category/
Location self-trees. Consumable/License → documents (polymorphic). Assignee is a
platform user id (no local table; resolved via auth).

## Redaction / sealing
Object-store credentials (config) sealed via KEK, never stored in `asset_*` columns/
returned. Supplier/location contact fields sealed/redacted. No credential or PII ever
appears in a response, log, audit detail or backup; document bytes are never exported
(referenced by key). Object keys are tenant-prefixed.
