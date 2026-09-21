# Quickstart: Asset Service

Validation scenarios proving the feature end to end. Assumes the `deploy/stack`
platform is up (TimescaleDB, Valkey, RustFS, gateway, auth, inventory) with the asset
service registered.

## Prerequisites
- asset service running (gateway `/api/asset`; object store reachable).
- An operator signed in with assets:read + the relevant :manage permissions; a
  platform-admin for the full-backup scenario.
- For inventory-sync: the inventory module up with some hosts for the tenant.

## Scenario 1 — Assets & assign/unassign lifecycle (US1)
1. Create an asset with no tag → a unique AST-xxxxxx tag is generated; a duplicate explicit tag is rejected.
2. Assign it to a user → status assigned, location cleared, assignee name resolved, an assignment row + asset.assigned event. Re-assign → refused (already assigned).
3. Unassign (to a location) → status deployable, user cleared, assignment closed, asset.unassigned event. Read assignment history → both events newest-first.

## Scenario 2 — Categories, suppliers, locations (US2)
1. Build a category tree and a location tree; read them nested with counts. Duplicate name/name+parent → rejected.
2. Classify an asset; deleting an in-use category/location/supplier → refused (has children/assets). Read a supplier → contact PII redacted per authorization.

## Scenario 3 — Photos & documents (US3)
1. Upload a photo and a document (invoice) to an asset; the bytes land in object storage with a checksum; the object key never exposes credentials.
2. List the asset's documents; download one via a short-lived link (exact bytes); delete it (object removed). Attach a document to a consumable and a license; each entity lists only its own.

## Scenario 4 — Consumables, licenses, insurance (US4)
1. Create a consumable with min_amount; drop stock to <= min → low-stock flagged + consumable.low_stock event.
2. Create a license/insurance with a past valid_to → the scheduler auto-expires it and emits an expiring/expired event.
3. Create a policy; add an asset with a covered value; list covered assets; the policy asset_count reflects it; remove it → count updates.

## Scenario 5 — Inventory sync (US5)
1. With inventory hosts present, run a sync preview → per-host create/update/unchanged changes with changed fields, tenant-scoped.
2. Execute the sync for selected hosts → new assets created (auto-tagged from host hardware/OS), matched assets updated on changed fields.
3. Stop the inventory module → a sync request fails gracefully with a clear message and writes nothing.

## Scenario 6 — Depreciation, stats, backup (US6)
1. Read an asset with cost/purchase date/useful life → current book value is computed (DDB, floored at salvage, never negative).
2. Read the dashboard → totals by status, entity counts, total cost, total depreciated value, expiring-soon + low-stock counts.
3. Export the tenant; import into a clean tenant (skip vs overwrite; full/cross-tenant restore only as platform-admin) → all entities round-trip FK-ordered with no secrets and no cross-tenant leakage.

## Security checks (cross-cutting)
- Object-store credentials and supplier/location contact PII never appear in any response, log, audit or backup.
- Object keys never resolve across tenants; presigned links are short-lived.
- A cross-tenant read/restore returns nothing / is refused; a non-admin import lands only in the caller's tenant; full restore refused for non-admins.
