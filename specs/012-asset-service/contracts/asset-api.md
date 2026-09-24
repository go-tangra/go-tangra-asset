# Phase 1 Contracts: Asset Service

Surfaces: (A) browser/query HTTP API via the gateway under `/api/asset`; (B)
module-to-module gRPC (`asset.v1`, SPIFFE mTLS); plus (C) events and (D) the
external client interfaces (blob, inventory, user directory).

## A. Browser/query HTTP API — prefix `/api/asset/v1` (gateway-proxied)

Assets (assets:read / assets:manage; assign = assets:assign)
- `GET /assets` (filters status/category_id/supplier_id/location_id/user_id/query, page/page_size/order_by), `POST /assets`, `GET/PUT/DELETE /assets/{id}` (detail includes computed book value).
- `POST /assets/{id}/assign` {user_id, notes}, `POST /assets/{id}/unassign` {location_id?, notes}, `GET /assets/{id}/assignments`.
- `POST /assets/{id}/photo` (multipart), `DELETE /assets/{id}/photo`.
- `GET /assets/{id}/documents`, `POST /assets/{id}/documents` (multipart), `DELETE /assets/{id}/documents/{docId}`, `GET /assets/{id}/documents/{docId}/download`.
- `POST /assets/inventory-sync/preview` (inventory:sync), `POST /assets/inventory-sync/execute` {hostnames[]} (inventory:sync).

Categories (categories:manage): `GET/POST /categories`, `GET/PUT/DELETE /categories/{id}`, `GET /categories/tree`.
Suppliers (suppliers:manage): CRUD `/suppliers`.
Locations (locations:manage): CRUD `/locations`, `GET /locations/tree`.
Consumables (consumables:manage): CRUD `/consumables` + documents (list/upload/delete/download).
Licenses (licenses:manage): CRUD `/licenses` + documents.
Insurance (insurance:manage): CRUD `/insurance-policies`, `GET /insurance-policies/{id}/assets`, `POST /insurance-policies/{id}/assets` {asset_id, covered_value}, `DELETE /insurance-policies/{id}/assets/{assetId}`.
Users (assets:read): `GET /users` (from auth directory — assignee dropdown).
System: `GET /health` (public), `GET /stats` (stats:read).
Backup (backup:manage): `POST /backup/export`, `POST /backup/import` (mode skip|overwrite; full/cross-tenant → platform-admin).
Realtime: `GET /stream` (SSE, assets:read).

Mutating routes carry the platform CSRF header; multipart upload routes declare body
limits. No response includes object-store credentials or sealed contact PII.

## B. Module-to-module gRPC — `asset.v1` (SPIFFE mTLS, not gateway-proxied)
- `AssetService`: Create/Get/List/Update/Delete, Assign, Unassign, GetAssignmentHistory, UploadPhoto, DeletePhoto, List/Upload/Delete/DownloadDocument, InventorySyncPreview, InventorySyncExecute
- `CategoryService`: CRUD + GetTree
- `SupplierService`: CRUD
- `LocationService`: CRUD + GetTree
- `ConsumableService`: CRUD + document RPCs
- `LicenseService`: CRUD + document RPCs
- `InsurancePolicyService`: CRUD + ListPolicyAssets/AddAssetToPolicy/RemoveAssetFromPolicy
- `UserService`: ListUsers
- `SystemService`: Health, GetDashboardStats
Every request carries `tenant_id`; caller identity from the mTLS peer. Messages never
carry credentials or sealed contact PII. Full backup restore requires platform-admin.

## C. Event contract (platform bus `platform:events:<tenant>`)
- `asset.assigned` / `asset.unassigned` {asset_id, asset_tag, user_id}
- `asset.warranty.expiring` {asset_id, asset_tag, expires_at}
- `license.expiring` {license_id, name, valid_to} / `insurance.expiring` {policy_id, policy_number, valid_to}
- `consumable.low_stock` {consumable_id, name, amount, min_amount}
Consumed by the notification service + the gateway SSE hub → `/stream`. Payloads carry
no credentials/PII. Dedup via asset_notify_state so the same condition is not re-alerted.

## D. External client interfaces (internal, fronted by fakes)
- `blob.Store` — Put(streaming SHA-256)/Get/PresignGet/Delete/EnsureBucket (photos + documents; from paperless).
- `invclient.Client` — ListHosts(ctx, tenant) for inventory-sync (adapter over pkg/inventoryclient); Fake for tests.
- `userdir.Directory` — Resolve(ctx, userID)→(name,email), ListUsers(ctx)→[]User (over authclient); Fake for tests.
- `deprec.BookValue(cost, salvage, usefulLifeYears, rate float64, purchaseDate, now time.Time) float64` — pure DDB, floored at salvage.
Each has a Fake so lifecycle, assignment, sync, depreciation and authorization are
tested without the network.

## Authorization matrix
reads → assets:read/stats:read; entity writes → <entity>:manage; assign → assets:assign;
documents → documents:manage; inventory-sync → inventory:sync; backup → backup:manage;
full/cross-tenant restore → platform-admin. Every mutation/assignment/upload/sync/backup audited.
