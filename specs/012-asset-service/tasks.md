# Tasks: Asset Service

**Feature**: 012-asset-service | **Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Organized by phase; user-story phases are independently testable. Tests are
MANDATORY and precede implementation (Constitution IV). `[P]` = parallelizable.
Module path: `github.com/go-freya/freya/services/asset`. Mirror services/inventory
(module + object-storage layout, reusing paperless's blob) and add the asset-specific
packages (deprec/invsync/userdir/scheduler) behind interfaces + fakes.

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Create the module skeleton `services/asset/` per plan.md (cmd/assetsvc, api/{openapi,proto/asset/v1}, internal/{app,config,store,repo,memstore,sealed,authz,audit,blob,deprec,assets,categories,suppliers,locations,consumables,licenses,insurance,documents,invsync,userdir,scheduler,stats,backup,events,stream,httpapi,grpcapi}, pkg/{assetmanifest,assetclient}, ui, deploy).
- [X] T002 Add `services/asset/go.mod` (module .../services/asset, Go 1.26) with replaces for ../.. ../auth ../gateway ../lcm ../inventory; add minio-go/v7; seed go.sum from services/inventory (+ paperless blob deps).
- [ ] T003 [P] Add buf.yaml/buf.gen.yaml + `api/proto/asset/v1/*.proto` stubs (Asset/Category/Supplier/Location/Consumable/License/Insurance/User/System services) + Makefile codegen (mirror inventory).
- [ ] T004 [P] Add `services/asset/Dockerfile` (build UI, embed -tags ui, build assetsvc) + `Makefile` (test/cover/vuln/generate/build/image) mirroring inventory.
- [ ] T005 [P] Scaffold `services/asset/ui/` (Vue 3 + Vite + Vuetify MF remote named `asset`) from services/inventory/ui (package.json, vite base /m/asset/, main.ts, api client BASE /api/asset/v1, remote/{routes,nav}, embed.go/embed_stub.go).

## Phase 2: Foundational (Blocking Prerequisites)

- [ ] T006 Typed, validated config `internal/config/config.go` (+ test) — sections db, valkey, kek, object_store (endpoint/bucket/region/access/secret_key sealed/presign_ttl), inventory (service — for sync), scheduler (interval, warranty/license/insurance "soon" windows), uploads (max_size), events, gateway, mesh_enroll, limits_asset. Default/Load/Validate/Warnings + duration helpers.
- [ ] T007 Store migrations `internal/store/migrations/`: 0001_schema.sql (all asset_* tables from data-model.md with ALL unique constraints; tenant_id ADDED to asset_assignments + asset_documents + asset_insurance_policy_assets + asset_notify_state), 0002_audit.sql (asset_audit_events hypertable), 0003_rls.sql (per-tenant RLS on EVERY asset_* table + asset_app grants).
- [X] T008 Store models + repos `internal/store/{models.go,repos.go}` + interface `internal/repo/repo.go` (all entities; asset upsert-by-tag, assign/unassign state + assignment rows, category/location trees, polymorphic documents, policy-asset M2M, consumable/license/insurance, notify-state dedup, stats aggregations, tenant ids, audit).
- [ ] T009 [P] In-memory `internal/memstore/memstore.go` implementing repo.Store (filters, unique-conflict, trees, documents, policy-assets, notify-state, FailNext) for tests.
- [ ] T010 [P] `internal/sealed/` envelope-seal/open + redaction helpers + `sealed_test.go` (100%).
- [ ] T011 [P] `internal/authz/` Subjects{TenantID,UserID,Roles,ActorKind} + IsAdmin (admin/owner) + IsPlatformAdmin + RequireTenant/RequirePlatformAdmin + API-permission checks + `authz_test.go` (100%).
- [ ] T012 [P] `internal/deprec/` — PURE double-declining-balance: `BookValue(cost, salvage, usefulLifeYears, rate float64, purchaseDate, now time.Time) float64` (floored at salvage, never negative; no cost/life → cost or 0) + `deprec_test.go` + `deprec_fuzz_test.go` (arbitrary inputs never panic; result in [salvage,cost]). 100%.
- [ ] T013 [P] `internal/blob/` — S3/MinIO object store (Put streaming+SHA-256, Get, PresignGet, Delete, EnsureBucket) + Fake + `blob_test.go` (COPY from services/paperless/internal/blob, module-path sed'd; never logs bytes/creds).
- [ ] T014 [P] `internal/audit/` writer adapter (action vocabulary per data-model) + redaction of credential/secret/contact/email/phone/telephone fields.
- [ ] T015 `internal/events/` publisher (asset.assigned/unassigned + asset.warranty.expiring + license.expiring + insurance.expiring + consumable.low_stock to platform:events:<tenant>) + `internal/stream/` SSE relay (copy inventory).
- [ ] T016 `internal/userdir/` — assignee/user-directory resolver interface over pkg/authclient (Resolve(userID)→name/email, ListUsers) + a Fake for tests. `internal/invsync` will use `internal/invclient` — an adapter over services/inventory/pkg/inventoryclient (ListHosts) + Fake; add `internal/invclient/invclient.go` here.
- [ ] T017 App build/wire/run `internal/app/app.go` — freya.New + mesh enroll, store/KEK, authz, blob (EnsureBucket), inventory client, userdir, events, gateway registration via pkg/assetmanifest, mesh HTTP+gRPC surfaces, the lifecycle scheduler worker, admin health/readiness; refuses to start insecure.
- [ ] T018 `cmd/assetsvc/main.go` + bootstrap subcommand (config, migrate, run) with ui.Remote() wiring (the paperless GUI-404 lesson).
- [ ] T019 [P] `pkg/assetmanifest/manifest.go` — routes/permissions (assets:read/manage/assign, categories/suppliers/locations/consumables/licenses/insurance/documents:manage, inventory:sync, stats:read, backup:manage) + Grants/Abilities/Nav (Assets, Categories, Suppliers, Locations, Consumables, Licenses, Insurance, Inventory Sync, Dashboard) + Routes() from OpenAPI + SeedPermissions.
- [ ] T020 `api/openapi/asset.yaml` (all routes w/ x-freya-permission; /stream no oversized timeout) + embed.go; contract test `tests/contract/openapi_test.go` (parses, every mounted route declared).
- [ ] T021 `internal/repo/repodb/` implementing the store over TimescaleDB + integration test (testcontainers) for schema/RLS/unique-constraints/trees/documents/policy-assets (`//go:build integration`).

## Phase 3: User Story 1 — Assets & assign/unassign lifecycle (Priority: P1) 🎯 MVP

### Tests (write first, must fail)
- [ ] T022 [P] [US1] Contract test `tests/contract/assets_test.go` — asset CRUD, assign/unassign, assignment history shapes; duplicate tag rejected; status-gate enforced; tenant isolation.
- [ ] T023 [P] [US1] Unit test `internal/assets/assets_test.go` — auto asset_tag generation + uniqueness; Assign (deployable→assigned, clears location, writes history, publishes event), Unassign (assigned→deployable, closes history, clears user), status-gate refusals; book value via deprec on read.
- [ ] T024 [P] [US1] Unit test — assignee name resolved via a fake userdir; assigned_by = actor.

### Implementation
- [ ] T025 [US1] `internal/assets/assets.go` — Service: CRUD (auto-tag, filters/search), Assign/Unassign (status gate + assignment rows + events), GetAssignmentHistory; wire deprec book value into the View; userdir for names.
- [ ] T026 [US1] HTTP handlers `internal/httpapi/assets.go` (CRUD, assign, unassign, assignments) + register + OpenAPI.
- [ ] T027 [P] [US1] gRPC `internal/grpcapi/` AssetService (CRUD/Assign/Unassign/GetAssignmentHistory) + register.
- [ ] T028 [P] [US1] UI: `ui/src/views/assets/` (list + detail with assign/history + depreciation) + store + api client.

**Checkpoint**: US1 demoable — catalog assets, assign/unassign with status gate + history (MVP).

## Phase 4: User Story 2 — Categories, suppliers, locations (Priority: P1)

### Tests (write first, must fail)
- [ ] T029 [P] [US2] Contract test `tests/contract/org_test.go` — category/location tree + supplier CRUD; unique names; delete guards.
- [ ] T030 [P] [US2] Unit test `internal/{categories,suppliers,locations}/*_test.go` — tree build + counts, unique name(+parent), delete-guard (has children/assets), PII redaction on supplier/location reads.

### Implementation
- [ ] T031 [US2] `internal/categories/categories.go` (CRUD + GetTree + guards) + `internal/suppliers/suppliers.go` (CRUD + guard, PII sealed) + `internal/locations/locations.go` (CRUD + GetTree + path + guards, PII sealed).
- [ ] T032 [US2] HTTP handlers `internal/httpapi/{categories,suppliers,locations}.go` + OpenAPI; gRPC Category/Supplier/Location services.
- [ ] T033 [P] [US2] UI: `ui/src/views/{categories,suppliers,locations}/` (trees + forms) + stores.

## Phase 5: User Story 3 — Photos & documents (Priority: P2)

### Tests (write first, must fail)
- [ ] T034 [P] [US3] Contract test `tests/contract/documents_test.go` — photo upload/delete, document upload/list/download/delete; per-entity isolation; no credential leak.
- [ ] T035 [P] [US3] Unit test `internal/documents/documents_test.go` (with blob.Fake) — polymorphic upload (asset/consumable/license), checksum, presigned download, delete removes object; object keys tenant-prefixed.
- [ ] T036 [P] [US3] Security test — object-store credentials never in responses/logs/audit; object keys never cross tenants (SR-002).

### Implementation
- [ ] T037 [US3] `internal/documents/documents.go` — Service over blob: UploadPhoto/DeletePhoto (asset), Upload/List/Delete/DownloadDocument (polymorphic), presigned URLs, tenant-prefixed keys.
- [ ] T038 [US3] HTTP handlers `internal/httpapi/documents.go` (multipart upload + streaming download) + OpenAPI; gRPC document RPCs on Asset/Consumable/License.
- [ ] T039 [P] [US3] UI: photo + documents tabs on the asset (and consumable/license) detail views.

## Phase 6: User Story 4 — Consumables, licenses, insurance (Priority: P2)

### Tests (write first, must fail)
- [ ] T040 [P] [US4] Contract test `tests/contract/inventory_records_test.go` — consumable/license/insurance CRUD + policy-asset add/remove/list shapes.
- [ ] T041 [P] [US4] Unit test `internal/{consumables,licenses,insurance}/*_test.go` — CRUD, low-stock flag (amount<=min), unique policy_number, policy-asset M2M (unique, counts), delete guards.

### Implementation
- [ ] T042 [US4] `internal/consumables/consumables.go` + `internal/licenses/licenses.go` + `internal/insurance/insurance.go` (CRUD; insurance ListPolicyAssets/AddAssetToPolicy(covered_value)/RemoveAssetFromPolicy + asset_count).
- [ ] T043 [US4] HTTP handlers `internal/httpapi/{consumables,licenses,insurance}.go` + OpenAPI; gRPC Consumable/License/InsurancePolicy services.
- [ ] T044 [P] [US4] UI: `ui/src/views/{consumables,licenses,insurance}/` (insurance incl. asset-coverage select) + stores.

## Phase 7: User Story 5 — Inventory sync (Priority: P2)

### Tests (write first, must fail)
- [ ] T045 [P] [US5] Unit test `internal/invsync/invsync_test.go` (with a fake inventory client + memstore) — preview classifies create/update/unchanged (match serial→hostname); execute creates auto-tagged assets + updates changed fields; unavailable inventory → clean error, no writes.
- [ ] T046 [P] [US5] Fuzz test `internal/invsync/diff_fuzz_test.go` — the host↔asset diff never panics on arbitrary host/asset sets.
- [ ] T047 [P] [US5] Contract test `tests/contract/invsync_test.go` — preview/execute shapes; tenant-scoped.

### Implementation
- [ ] T048 [US5] `internal/invsync/invsync.go` — Service using internal/invclient (over inventoryclient): Preview (ListHosts → diff vs assets) + Execute(selected hostnames → create/update assets); tenant-scoped; graceful when inventory unavailable.
- [ ] T049 [US5] HTTP handlers `internal/httpapi/invsync.go` (preview/execute) + OpenAPI; gRPC Asset InventorySyncPreview/Execute.
- [ ] T050 [P] [US5] UI: `ui/src/views/inventory-sync/` (preview diff table + select + execute).

## Phase 8: User Story 6 — Depreciation, lifecycle alerts, stats, backup (Priority: P3)

### Tests (write first, must fail)
- [ ] T051 [P] [US6] Unit test `internal/scheduler/scheduler_test.go` — evaluates warranty/license/insurance expiry + low-stock, publishes events, auto-expires lapsed license/insurance, dedups via notify-state (no repeat alerts).
- [ ] T052 [P] [US6] Unit test `internal/stats/stats_test.go` — totals by status, entity counts, total cost, total depreciated value (via deprec), expiring/low-stock counts.
- [ ] T053 [P] [US6] Unit test `internal/backup/backup_test.go` — FK-ordered export/import round-trip; ids preserved; skip vs overwrite; schema version; full/cross-tenant restore requires platform-admin; NO secrets/PII.
- [ ] T054 [P] [US6] Contract test `tests/contract/stats_backup_test.go` — /stats + /backup shapes; non-admin full restore refused.

### Implementation
- [ ] T055 [US6] `internal/scheduler/scheduler.go` — per-tenant lifecycle worker (system subject): evaluate + publish + auto-expire + notify-state dedup; wired into app workers.
- [ ] T056 [US6] `internal/stats/stats.go` + HTTP `internal/httpapi/statistics.go` + gRPC SystemService (Health/GetDashboardStats) + OpenAPI (incl. total depreciated value via deprec).
- [ ] T057 [US6] `internal/backup/backup.go` + HTTP `internal/httpapi/backup.go` (export/import mode; platform-admin for full/cross-tenant) + OpenAPI; secrets/object-bytes excluded.
- [ ] T058 [P] [US6] UI: `ui/src/views/dashboard/` (stats widgets: value, expiring, low-stock) + backup controls.

## Phase 9: Platform integration & polish

- [ ] T059 Stack wiring `deploy/stack/`: add an `asset` DB + `asset_app` role to init-db.sql; an `asset` Valkey user; an `asset` compose service (enrolls, mounts policy+kek, uses RustFS, depends on inventory) + an `asset-token` mint init + `configs/asset.yaml`.
- [ ] T060 Gateway allow-list: add `spiffe://example.org/svc/asset=/api/asset;asset` to gateway-bootstrap; confirm registers (registered:true) + the Asset menu renders.
- [ ] T061 `services/asset/deploy/policy.yaml` (gateway-forwards; module callers of asset.v1; asset calls inventory/auth) + `deploy/kek.dev`.
- [ ] T062 [P] `services/asset/deploy/README.md` (server ops, object storage, inventory-sync, lifecycle events, security) + update `deploy/stack/README.md` to list asset.
- [ ] T063 [P] `pkg/assetclient/` typed Go client for module-to-module use + test.
- [ ] T064 Coverage gate: `go -C services/asset test ./...` ≥80% overall (with the integration harness), 100% on sealed/authz/deprec; `govulncheck` clean; wire scripts/coverage-gate.sh into make cover.
- [ ] T065 Stack smoke test: bring up the stack, confirm asset registers (registered:true), create an asset + assign it end-to-end, and (inventory up) run a sync preview.
- [ ] T066 [P] Fuzz + negative tests for depreciation math (deprec), asset-tag generation, the inventory-sync diff, and the backup import parser (Constitution IV).

## Dependencies & sequencing

- Setup (P1) + Foundational (P2) block everything.
- US1 (P1, assets+lifecycle) is the MVP. US2 (P1) provides the classification records
  US1 references. US3 (P2, photos/documents) adds the object-storage path. US4 (P2)
  adds the supporting inventories. US5 (P2, inventory-sync) reconciles US1 against the
  inventory module. US6 (P3) adds depreciation/alerts/stats/backup.
- The enhancements (deprec, scheduler, invsync via inventoryclient) + object storage
  + userdir are the asset-specific novel work vs a plain CRUD module.

## Implementation strategy

MVP first: Setup + Foundational + US1 (assets + assign lifecycle) → demoable. Then US2
(org records) → US3 (photos/documents) → US4 (consumables/licenses/insurance) → US5
(inventory-sync) → US6 (depreciation/alerts/stats/backup) → Phase 9 (stack + coverage +
smoke + fuzz).

## Summary

- **Total tasks**: 66 across 9 phases.
- **Per story**: US1=7 (T022-T028), US2=5 (T029-T033), US3=6 (T034-T039), US4=5 (T040-T044), US5=6 (T045-T050), US6=8 (T051-T058).
- **Parallelizable**: [P]-marked tasks (distinct files) — tests, UI views, client, docs.
- **MVP scope**: Setup + Foundational + US1.
- **Novel vs prior modules**: pure deprec (DDB) lib, lifecycle scheduler + events, inventory-sync wired to the inventory module via inventoryclient, userdir over authclient, object storage (reused from paperless).
