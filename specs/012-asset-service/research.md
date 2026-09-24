# Phase 0 Research: Asset Service

Decisions resolving the Technical Context. Scope confirmed with the requester:
full ITAM replica + inventory-sync WIRED to the Freya inventory module (010) +
ENHANCEMENTS (server-side DDB depreciation, lifecycle expiry/low-stock events).

## D1. Storage & conflict detection (TimescaleDB + RLS, unique constraints preserved)
**Decision**: TimescaleDB with per-tenant RLS on every `asset_*` table. The source
omitted tenant_id on the assignment and document tables; the replica ADDS it so RLS
covers them. All source unique constraints are preserved as the conflict layer:
`(tenant_id,asset_tag)`, `(tenant_id,policy_number)`, `(tenant_id,name)` on
suppliers/locations, `(tenant_id,name,parent_id)` on categories, document
`storage_key` unique, `(policy_id,asset_id)` unique. `asset_audit_events` is a
hypertable; the rest are ordinary relational RLS tables.
**Rationale**: matches the source's DB-level conflict detection while replacing the
ent SystemViewer bypass with RLS + a scoped system subject for the scheduler.
**Alternatives**: ent app-level filtering (source) — rejected, bypass risk.

## D2. Object storage for photos + documents (reuse the paperless blob pattern)
**Decision**: Photos and polymorphic documents (asset|consumable|license) live only
in S3-compatible object storage (RustFS in the dev stack), keyed
`tenants/<tenant>/{assets,documents}/<id>`, with a streaming SHA-256 checksum and
short-lived presigned download URLs. The blob client (minio-go) self-provisions its
bucket at startup (idempotent `EnsureBucket`), reusing the proven paperless `blob`
package. Object-store credentials are sealed (KEK) and never returned/logged.
**Rationale**: identical concern to paperless documents; reuse the proven, live-
verified code; keeps bytes out of the metadata DB and credentials out of responses.
Object keys are tenant-prefixed so a caller cannot reach another tenant's files.

## D3. Assign/unassign lifecycle & assignee resolution
**Decision**: Assign requires status `deployable`→`assigned` (else refuse
"already assigned"), sets user_id, clears location, writes an assignment-history
row (action=assigned, assigned_by=actor), publishes `asset.assigned`. Unassign
requires `assigned`→`deployable`, closes the open assignment (returned_at=now),
clears user, optionally sets a location, publishes `asset.unassigned`. The assignee
display name is resolved from the platform user directory via `userdir` (an
interface over authclient with a fake); resolution is best-effort (id always
recorded, name cached). `GetAssignmentHistory` lists newest-first.
**Rationale**: matches the source's status-gated lifecycle + denormalized history;
the userdir interface keeps auth coupling testable and optional.

## D4. Depreciation (pure double-declining-balance, fuzzed)
**Decision**: A pure `deprec` package computes current book value:
DDB with `rate` (default 0.40) per year on the declining balance from
`purchase_cost` over `useful_life_years`, floored at `salvage_value`, never below
salvage or negative; assets without cost/useful-life report no depreciation
(book value = cost or unset). Computed on asset read and aggregated (total
depreciated value) in stats. Fuzzed (arbitrary inputs never panic; result is in
[salvage, cost]).
**Rationale**: the source stores the inputs but never computes; the requester chose
the enhancement. A pure package is deterministic and offline-testable.

## D5. Lifecycle scheduler + events (enhancement)
**Decision**: A per-tenant worker (system subject) runs on an interval and, for each
tenant: flags assets whose warranty (purchase_date+warranty_months) is within the
"soon" window, licenses/insurance whose valid_to is soon or past (auto-transitioning
lapsed ones to `expired`), and consumables with amount<=min_amount; publishes
`asset.warranty.expiring` / `license.expiring` / `insurance.expiring` /
`consumable.low_stock` to `platform:events:<tenant>`. A per-condition dedup marker
(e.g. last-notified timestamp/state) prevents re-alerting the same condition each
pass. `asset.assigned/unassigned` publish on those actions.
**Rationale**: the requester chose lifecycle alerts; the notification service + SSE
hub consume the events. Dedup avoids alert spam.
**Alternatives**: alert every pass — rejected (spam); external cron — rejected,
kept in-module for tenant-scoping + audit.

## D6. Inventory-sync wired to the Freya inventory module (010)
**Decision**: `invsync` depends on `inventoryclient` (the inventory module's typed
gRPC client) over SPIFFE mTLS. Preview lists the tenant's hosts, matches each to an
asset (serial first, then hostname), and produces create/update/unchanged changes
with changed_fields; execute applies selected hosts (create auto-tagged assets from
host hardware/OS + management IP, update changed fields). The inventory client is an
interface with a fake; when inventory is unavailable, preview/execute fail with a
clear error and write nothing.
**Rationale**: the collector the source synced against is our inventory module;
reusing its client is the faithful Freya wiring. The interface keeps it testable and
the feature optional.

## D7. Auth, assignee directory, and platform-admin
**Decision**: Browser callers authenticate with the gateway platform token
(authclient); module callers use SPIFFE mTLS. `authz.Subjects` carries tenant + user
+ roles; `IsPlatformAdmin` gates full/cross-tenant backup restore. `UserService.
ListUsers` and assignee name resolution proxy the platform user directory via auth.
**Rationale**: matches the other replicas; replaces the source's `x-md-global-*`
trust + LCM certs with SPIFFE + platform token.

## D8. Backup (FK-ordered, platform-admin for full restore)
**Decision**: Export/import all `asset_*` entities in FK-safe order (categories →
suppliers → locations → assets → consumables → licenses → assignments → documents(by
reference) → insurance → policy-assets), schema-versioned, skip/overwrite. Secrets
and object bytes are never exported (documents referenced by key). Full/cross-tenant
restore requires platform-admin; a non-admin import lands only in the caller's tenant.

## Supply-chain note (Constitution VI)
New third-party dep: `minio/minio-go/v7` (already vetted + live in paperless).
Intra-repo deps: inventoryclient, authclient. Pinned in `go.sum`; `govulncheck` in CI.

## STRIDE summary
- **Spoofing**: forged tenant/user/platform-admin → gateway platform-token
  verification (authclient); mesh peers SPIFFE-identified; inventory-sync SPIFFE mTLS.
- **Tampering**: cross-tenant edits / duplicate records → RLS + unique-constraint
  guards + append-only audit (SR-001/006).
- **Repudiation**: append-only tamper-evident audit of every mutation/assignment/
  upload/sync/backup with actor+tenant+subject+outcome (SR-006).
- **Information disclosure**: object-store creds, contact PII, cross-tenant files →
  sealed creds never returned, tenant-prefixed object keys, PII redaction, per-tenant
  RLS, short-lived presigned links (SR-002/003).
- **Denial of service**: oversized uploads / sync floods → bounded upload size,
  bounded sync batches, scheduler intervals.
- **Elevation of privilege**: full/cross-tenant restore → platform-admin only; the
  scheduler uses a scoped system subject, never an unauthenticated bypass (SR-004/005).
