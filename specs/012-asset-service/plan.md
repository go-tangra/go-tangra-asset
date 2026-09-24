# Implementation Plan: Asset Service

**Branch**: `012-asset-service` | **Date**: 2026-09-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/012-asset-service/spec.md`

## Summary

A tenant-scoped IT Asset Management platform module replicating and enhancing
go-tangra-asset: assets (with an assign/unassign lifecycle to platform users,
photos, documents, and depreciation), categories & locations (trees), suppliers,
consumables (stock), licenses, insurance policies (with per-asset coverage),
dashboard statistics and backup. It computes **depreciation** server-side, runs a
**lifecycle scheduler** (expiry + low-stock events, auto-expire), reconciles
against the platform **inventory** module (feature 010) via inventory-sync, stores
photos/documents in S3-compatible object storage, and publishes lifecycle events.

Freya adaptations: SPIFFE mTLS + gateway platform token (replacing the mTLS-CN +
`x-md-global-*` trust and LCM cert bootstrap); TimescaleDB + per-tenant RLS
(replacing ent app-level filtering, unique constraints preserved as the conflict
layer, tenant_id added to the assignment/document tables); sealed object-store
credentials; the assignee is a platform user resolved via auth; inventory-sync is
wired to the Freya inventory module over SPIFFE mTLS; lifecycle events + DDB
depreciation are net-new.

## Technical Context

**Language/Version**: Go 1.26 (matches every other Freya service).

**Primary Dependencies**: the Freya framework (`github.com/go-freya/freya`) for
transport (SPIFFE mTLS gRPC + OpenAPI-validated HTTP edge), identity, audit,
sealed envelopes, gateway registration; `pgx` + TimescaleDB for metadata;
`github.com/minio/minio-go/v7` for S3-compatible object storage (photos +
documents, reusing the paperless `blob` pattern with a self-provisioned bucket);
Valkey for the event bus; `github.com/go-freya/freya/services/inventory/pkg/
inventoryclient` for inventory-sync; `pkg/authclient` for platform-token
verification and assignee/user-directory resolution. UI: Vue 3 + Vite + Vuetify
(Materio) Module-Federation remote, mirroring services/inventory/ui.

**Storage**: TimescaleDB (Postgres) with per-tenant RLS on every `asset_*` table
(including the assignment, document and policy-asset tables the source omitted).
Ordinary relational tables; `asset_audit_events` is a hypertable. All source
unique constraints preserved: `(tenant_id,asset_tag)`, `(tenant_id,policy_number)`,
`(tenant_id,name)` on suppliers/locations, `(tenant_id,name,parent_id)` on
categories, `(policy_id,asset_id)`, document `storage_key` unique. Photos and
documents live only in object storage (keyed `tenants/<t>/assets|documents/<id>`,
SHA-256, presigned URLs); credentials are sealed.

**Depreciation**: a pure `deprec` package computes current book value via
double-declining-balance from cost, purchase date, useful-life, rate and salvage
(floored at salvage, never negative); fuzzed and unit-tested offline.

**Lifecycle scheduler**: a per-tenant worker (system subject) evaluates warranty
(purchase_date+warranty_months), license/insurance valid_to and consumable
amount<=min_amount on an interval, auto-transitions lapsed license/insurance to
expired, and publishes `asset.warranty.expiring` / `license.expiring` /
`insurance.expiring` / `consumable.low_stock` — with a per-condition dedup marker
so the same expiry is not re-alerted every pass.

**Inventory-sync**: `InventorySyncPreview` calls `inventoryclient.ListHosts` (over
SPIFFE mTLS) for the tenant's hosts and diffs them (match serial → hostname)
against assets into create/update changes; `InventorySyncExecute` applies selected
hosts (create auto-tagged assets from host hardware/OS, update changed fields). It
degrades gracefully (clear error, no writes) when inventory is unavailable.

**Events**: `asset.assigned/unassigned` + the lifecycle events publish to
`platform:events:<tenant>` for the notification service and the gateway SSE hub.

**Testing**: Go `testing` with a `testrt` runtime + `memstore` fake; the object
store, inventory client and auth user-directory are behind interfaces with fakes,
so lifecycle, assignment, depreciation, sync and authorization are unit-tested
offline; contract tests over the OpenAPI + proto; integration suite
(testcontainers: TimescaleDB, Valkey, RustFS) behind `//go:build integration`;
fuzz tests for depreciation math, asset-tag generation and the sync diff. Coverage
gate >=80% overall, 100% on sealed/authz/deprec.

**Target Platform**: Linux server container in `deploy/stack` behind the gateway,
with RustFS/Tika-free object storage; calls inventory + auth + warden over the mesh.

**Project Type**: Web service (Go backend + gRPC + OpenAPI HTTP) with an object-
storage path, cross-module inventory-sync, a lifecycle scheduler, and a Module-
Federation UI.

**Performance Goals**: asset list/tree/dashboard queries < 3s at 100k assets per
tenant; assign/create returns well under a second; a sync preview of a few hundred
hosts completes in a few seconds.

**Constraints**: per-tenant RLS; unique constraints enforce conflict detection;
object-store credentials sealed, never returned; object keys tenant-scoped;
presigned links short-lived; contact PII redacted; full/cross-tenant restore is
platform-admin only; the scheduler uses a scoped system subject; inventory-sync is
SPIFFE-mTLS (no shared secret).

**Scale/Scope**: 100k assets per tenant; six prioritized user stories (assets +
lifecycle, org records, photos/documents, consumables/licenses/insurance,
inventory-sync, depreciation/alerts/stats/backup).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Secure by Default**: zero-config refuses to start; object-store creds
  sealed; presigned links short-lived; full restore platform-admin-gated; TLS
  everywhere. PASS.
- **II. Zero Trust Service Communication**: mesh calls SPIFFE mTLS (inventory/
  auth/warden); browser via gateway platform token; no shared secrets. PASS.
- **III. Least Privilege & Tenant Isolation**: per-tenant RLS on every table incl.
  assignment/document/policy-asset; object keys tenant-scoped; scheduler uses a
  scoped system subject. PASS.
- **IV. Test-First with Security Verification (NON-NEGOTIABLE)**: contract/unit/
  integration tests precede implementation; object-store/inventory/auth behind
  fakes; negative + fuzz tests for depreciation, tagging, sync diff; redaction +
  authorization tests. PASS.
- **V. Defense in Depth & Observability**: gateway edge + module authz + RLS;
  bounded uploads; append-only audit of every mutation/assignment/upload/sync;
  health/readiness; lifecycle events for observability. PASS.
- **VI. Supply-Chain Integrity**: new dep (minio-go, already vetted in paperless)
  + intra-repo inventoryclient/authclient; `go.sum` pinned; `govulncheck` in CI. PASS.
- **VII. Simplicity & Explicitness**: explicit wiring; the enhancements
  (scheduler, depreciation, sync) are isolated behind explicit packages with
  interfaces. PASS.

No Constitution violations. Security Requirements (SR-001..006) map to research.md
decisions and to test tasks.

## Project Structure

### Documentation (this feature)

```
specs/012-asset-service/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (OpenAPI + proto + blob/inventory/auth ifaces + events)
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code (repository root)

```
services/asset/
├── go.mod                       # module github.com/go-freya/freya/services/asset (replaces ../.. ../auth ../gateway ../lcm ../inventory)
├── cmd/assetsvc/                # run + bootstrap/migrate
├── api/
│   ├── openapi/asset.yaml       # browser routes (x-freya-permission/x-freya-public)
│   └── proto/asset/v1/          # gRPC: Asset/Category/Supplier/Location/Consumable/License/Insurance/User/System services
├── internal/
│   ├── app/                     # wiring (freya.New, stores, blob, inventory client, scheduler, gateway reg)
│   ├── config/                  # config (db/valkey/kek/object_store/inventory/scheduler/gateway/enroll)
│   ├── store/ + repo/ + repodb/ # migrations (RLS + unique constraints + audit hypertable), models, SQL, repo iface
│   ├── memstore/                # in-memory repo fake
│   ├── sealed/ authz/ audit/    # envelope seal, permission + platform-admin checks, audit vocabulary
│   ├── blob/                    # S3/MinIO object store (photos+documents) + fake (from paperless)
│   ├── deprec/                  # pure double-declining-balance depreciation (fuzzed)
│   ├── assets/ categories/ suppliers/ locations/ consumables/ licenses/ insurance/  # domain services
│   ├── documents/               # polymorphic photo/document service over blob
│   ├── invsync/                 # inventory-sync (diff + apply) over inventoryclient
│   ├── userdir/                 # assignee/user-directory resolver over authclient (+ fake)
│   ├── scheduler/               # lifecycle worker (expiry + low-stock + auto-expire + events)
│   ├── stats/ backup/           # dashboard statistics + export/import
│   ├── events/ stream/          # platform event publisher + SSE relay
│   ├── httpapi/ grpcapi/        # mesh HTTP + gRPC surfaces
├── pkg/
│   ├── assetmanifest/           # gateway manifest (routes/permissions/abilities/nav) + SeedPermissions
│   └── assetclient/             # typed module-to-module gRPC client
├── ui/                          # Vue 3 + Vite + Vuetify MF remote (assets/categories/suppliers/locations/consumables/licenses/insurance/inventory-sync/dashboard)
├── deploy/                      # policy.yaml, kek.dev, README
├── Dockerfile Makefile buf.yaml buf.gen.yaml
```

**Structure Decision**: mirrors services/inventory/paperless (proven module +
object-storage layout) plus the asset-specific packages — `deprec` (pure
depreciation), `invsync` (cross-module reconciliation via inventoryclient),
`userdir` (assignee resolution via authclient), and `scheduler` (lifecycle events)
— each behind interfaces with fakes for offline tests.

## Complexity Tracking

The enhancements (lifecycle scheduler, DDB depreciation, inventory-sync) and the
object-storage + cross-module integrations add surface beyond a plain CRUD module,
but each is isolated behind an explicit, interface-fronted package with fakes; the
cross-tenant/full restore path is platform-admin gated. No Constitution violations.
