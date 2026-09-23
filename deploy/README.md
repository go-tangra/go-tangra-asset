# Asset service — deployment notes

The **asset** service is a tenant-scoped IT Asset Management (ITAM) module of
the Freya platform: assets with an **assign/unassign lifecycle** to platform
users, **photos and documents** in S3-compatible object storage, **categories**
and **locations** (trees), **suppliers**, **consumables** (stock), **licenses**,
**insurance policies** with per-asset coverage, server-side **depreciation**
(double-declining balance), a **lifecycle scheduler** (warranty/license/
insurance expiry + low-stock events, auto-expire), **inventory-sync** against the
`inventory` module, dashboard **statistics** and tenant **backup**.

Module id `asset`; browser API under `/api/asset/v1` (gateway-proxied, platform
token); module gRPC surface `asset.v1` on the SPIFFE mTLS mesh (not proxied).

## Server operations

| Item | Value |
|---|---|
| Binary | `assetsvc -config deploy/container.yaml` (applies migrations, then serves) |
| Bootstrap only | `assetsvc bootstrap -config …` (migrations, then exit) |
| Listeners | gRPC `server.grpc_addr`, HTTP `server.http_addr` (mesh, mTLS), admin `admin.addr` (`/healthz`, `/readyz`) |
| Store | TimescaleDB, database `asset`, app role `asset_app` (NOBYPASSRLS); `db.migrate_dsn` for the migration role |
| Event bus | Valkey user `asset` (Streams `platform:events:<tenant>`) |
| KEK | 32-byte key (`kek.source: file|env`) sealing supplier/location contact PII |
| Mesh identity | enrolls with lcm through the gateway edge (`mesh_enroll`), stores its SVID in `/state` |
| Gateway | registers its manifest (routes, permissions, abilities, nav) on a lease; seeds role grants into auth |

Every `asset_*` table carries `tenant_id` under **row-level security**; the
scheduler and audit writer run under the pinned system scope. All unique
constraints (`(tenant,asset_tag)`, `(tenant,policy_number)`, `(tenant,name)`
on suppliers/locations/insurance, `(tenant,name,parent)` on categories,
`(policy,asset)`, document `storage_key`) are the conflict-detection guards.

## Object storage (photos + documents)

`object_store` names an S3-compatible endpoint (RustFS/MinIO in `deploy/stack`).
The bucket is self-provisioned at startup (non-fatal if the store is briefly
down). Object keys are tenant-prefixed (`tenants/<tenant>/assets/<id>/photo.<ext>`,
`tenants/<tenant>/documents/<docID>`) and every read/delete verifies the prefix.
Bytes are streamed through the service (`…/download`) or via a short-lived
presigned link (`object_store.presign_ttl_seconds`). Credentials are never part
of a response, log, audit row or backup. Uploads are bounded by
`uploads.max_size_bytes` (and the route's declared body limit); photos must be
PNG/JPEG/GIF/WebP.

## Inventory-sync

`POST /assets/inventory-sync/preview` lists the tenant's hosts from the
`inventory` module (`discovery.static.inventory`, SPIFFE mTLS — the inventory
policy must allow `svc/asset` on `InventoryHostService/ListHosts`) and diffs
them against the assets (match order: `inventory_host_id` tag → serial →
hostname) into create/update/unchanged. `…/execute` applies the selected
hostnames: new assets are auto-tagged (`AST-xxxxxx`) from the host's name,
serial, model and OS (as tags); matched assets get the sync-owned fields
updated. When inventory is unreachable the request fails with
`temporarily_unavailable` and writes nothing.

## Lifecycle events

The scheduler (`scheduler.interval_seconds`, per-tenant sweep) publishes to the
tenant bus and the gateway SSE relay (`GET /api/asset/v1/stream`):

| Event | Condition | Dedup key |
|---|---|---|
| `asset.warranty.expiring` | purchase_date + warranty_months within `warranty_soon_days` (or past) | `warranty:<asset>` |
| `license.expiring` | valid_to within `license_soon_days` or past (status → `expired`) | `license:<id>` |
| `insurance.expiring` | valid_to within `insurance_soon_days` or past (status → `expired`) | `insurance:<id>` |
| `consumable.low_stock` | amount ≤ min_amount (min_amount > 0) | `low_stock:<id>` |
| `asset.assigned` / `asset.unassigned` | assign/unassign | — |

`asset_notify_state` records the last notified state per condition so a pass
never re-alerts an unchanged condition; a condition that clears and recurs
alerts again.

## Backup

`POST /backup/export` returns every record of the caller's tenant, FK-ordered,
ids preserved, **without** sealed contact fields or document bytes (document
metadata references tenant-prefixed keys only). `POST /backup/import`
(`mode: skip|overwrite`) restores into the caller's tenant; `tenant_id` (a
different tenant) or `full: true` (wipe then load) require a **platform admin**.
A cross-tenant restore gets fresh ids with the foreign keys rewritten.

## Security

- Every route declares an `x-freya-permission` (`assets:read|manage|assign`,
  `categories|suppliers|locations|consumables|licenses|insurance|documents:manage`,
  `inventory:sync`, `stats:read`, `backup:manage`); `/health` is public.
- Supplier/location contact fields (contact person, telephone, e-mail) are
  sealed with the KEK envelope, bound to the row and field, and returned in clear
  only to tenant admins; service (gRPC) callers never see them.
- Object-store credentials are config-only; object keys never cross tenants.
- Every mutation, assignment, upload, sync and backup is audited (redacted
  details) into the `asset_audit_events` hypertable.
- `make cover` enforces ≥80% overall and 100% on `internal/authz`,
  `internal/sealed`, `internal/deprec`; `make vuln` runs govulncheck.

## Dev stack

`deploy/stack` adds the `asset` database/role, Valkey user, gateway allow-list
entry (`spiffe://example.org/svc/asset=/api/asset;asset`), the enrollment-token
mint (`asset-token`) and the `asset` service (`configs/asset.yaml`, sharing the
RustFS dev credentials with its own `asset` bucket). The UI remote is served
under `/m/asset/` with nav entries Assets, Categories, Suppliers, Locations,
Consumables, Licenses, Insurance, Inventory Sync, Dashboard.

## UI

The remote under `services/asset/ui` is built on the shared kit `@freya/ui` (FlyonUI + Zod,
see `docs/frontend.md`): forms validate through Zod schemas in `src/schemas/`, the
shell provides the theme and shared singletons, and `npm run lint` runs
`check-no-legacy`. Rebuild the image after UI changes; the Dockerfile builds `ui/kit`
first.
