# Feature Specification: Asset Service

**Feature Branch**: `012-asset-service`

**Created**: 2026-09-21

**Status**: Draft

**Input**: replicate and enhance go-tangra-asset as a Freya module — a tenant-scoped IT Asset Management (ITAM) platform (assets with assign/unassign lifecycle, photos, documents, depreciation; categories, suppliers, locations, consumables, licenses, insurance; stats and backup), wired to the Freya inventory module for asset↔inventory sync, and enhanced with server-side depreciation + lifecycle expiry/low-stock events.

## Overview

The **asset** service is a tenant-scoped **IT Asset Management** platform module.
It catalogs IT **assets** (with an assign/unassign lifecycle to platform users,
photos, attached documents, and depreciation), and the supporting records —
**categories** and **locations** (hierarchical trees), **suppliers**,
**consumables** (stock), software **licenses**, and **insurance policies** (with
per-asset coverage). It computes **depreciation** server-side, runs a **lifecycle
scheduler** that flags expiring warranties/licenses/insurance and low-stock
consumables (publishing events for notification and live UI, and auto-expiring
lapsed licenses/insurance), reconciles assets against the platform **inventory**
module (feature 010), and provides dashboard statistics and backup. Photos and
documents live in S3-compatible object storage; metadata lives in TimescaleDB
with per-tenant row-level security. It registers with the application gateway,
exposes service-to-service gRPC, and ships a Module-Federation UI.

Freya adaptations (vs the source): SPIFFE mTLS + gateway platform token replace
the mTLS-CN + gateway-metadata trust and LCM cert bootstrap; TimescaleDB + RLS
replace ent app-level tenant filtering (unique constraints preserved as the
conflict-detection layer); object-store credentials are sealed; the assignee is a
platform user (resolved via auth); inventory-sync is wired to the Freya inventory
module over SPIFFE mTLS; and lifecycle events + server-side depreciation are new.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Catalog assets and manage the assign/unassign lifecycle (Priority: P1)

An asset manager creates an asset (auto-tagged if left blank), assigns it to a
person (moving it to "assigned" and recording who/when), later checks it back in
(returning it to "deployable" at a location), and reviews its assignment history.

**Why this priority**: This is the MVP and the core of ITAM — a conflict-free
asset catalog with an auditable assignment lifecycle. Everything else supports it.

**Independent Test**: Create an asset, assign it to a user (status gate enforced),
unassign it, and read its assignment history showing both events.

**Acceptance Scenarios**:

1. **Given** a new asset with no tag, **When** it is created, **Then** a unique per-tenant asset tag is generated (e.g. AST-xxxxxx) and a duplicate explicit tag is rejected.
2. **Given** a deployable asset, **When** it is assigned to a user, **Then** its status becomes assigned, its location is cleared, the assignee is recorded (name resolved from the user directory), and an assignment-history row + an assigned event are created.
3. **Given** an already-assigned asset, **When** assignment is attempted, **Then** it is refused (already assigned); an unassigned asset cannot be unassigned.
4. **Given** an assigned asset, **When** it is unassigned (optionally to a location), **Then** its status returns to deployable, the assignee is cleared, the open assignment is closed (returned), and an unassigned event is published.

---

### User Story 2 - Organize with categories, suppliers, and locations (Priority: P1)

A manager models asset categories and physical locations as hierarchies (viewable
as trees) and records suppliers, then classifies assets against them and is
prevented from deleting a category/location/supplier still in use.

**Why this priority**: Assets reference categories/suppliers/locations; the
organizational records are required to classify and report on the catalog.

**Independent Test**: Build a category tree and a location tree, create a
supplier, classify an asset, and confirm a non-empty category/location/supplier
refuses deletion.

**Acceptance Scenarios**:

1. **Given** categories/locations with parents, **When** the tree is read, **Then** the nested hierarchy with asset/child counts is returned; names are unique per tenant (category by name+parent).
2. **Given** a category/location/supplier referenced by assets or children, **When** deletion is attempted, **Then** it is refused (has children / has assets).
3. **Given** supplier/location contact fields, **When** records are read, **Then** contact PII is redacted per the caller's authorization and never leaks into logs, audit or backups.

---

### User Story 3 - Attach photos and documents (Priority: P2)

A manager uploads a photo for an asset and attaches documents (invoices, warranty
PDFs) to assets, consumables or licenses, then downloads them via short-lived links.

**Why this priority**: Document/photo attachment is expected of ITAM but builds on
the core catalog; it also introduces the object-storage path.

**Independent Test**: Upload a photo and a document to an asset, list the
documents, download one via a presigned URL, and delete them.

**Acceptance Scenarios**:

1. **Given** an asset, **When** a photo/document is uploaded, **Then** the bytes are stored in object storage with a checksum, the metadata recorded, and the object key never exposes credentials.
2. **Given** an attached document, **When** it is downloaded, **Then** a short-lived link (or stream) returns the exact bytes; deleting the record removes the object.
3. **Given** documents on assets, consumables and licenses, **When** listed for an entity, **Then** only that entity's documents are returned, tenant-isolated.

---

### User Story 4 - Track consumables, licenses, and insurance coverage (Priority: P2)

A manager tracks consumable stock (with reorder thresholds), software licenses
(with validity), and insurance policies, and records which assets a policy covers.

**Why this priority**: These supporting inventories complete the ITAM picture but
are independent of the core asset lifecycle.

**Independent Test**: Create a consumable with a min level, a license with a
validity window, and an insurance policy; add an asset to the policy and list its
covered assets.

**Acceptance Scenarios**:

1. **Given** a consumable, **When** its stock falls to or below its minimum, **Then** it is flagged low-stock (and a low-stock event is published).
2. **Given** a license/insurance policy past its validity, **When** the lifecycle scheduler runs, **Then** its status auto-transitions to expired and an expiring/expired event is published ahead of the date.
3. **Given** a policy, **When** an asset is added with a covered value, **Then** it appears in the policy's covered assets (unique per policy+asset) and the policy's asset count reflects it; removing it updates the count.

---

### User Story 5 - Reconcile assets with discovered inventory (Priority: P2)

A manager previews the difference between the platform's discovered inventory
(hosts) and the asset catalog, then imports selected hosts as assets (creating or
updating), keeping the catalog in step with what is actually deployed.

**Why this priority**: Inventory reconciliation is a high-value cross-module
capability, but depends on the core asset model and the inventory service.

**Independent Test**: With inventory hosts present, run a sync preview showing
create/update changes, execute the sync for selected hosts, and confirm assets
were created/updated accordingly.

**Acceptance Scenarios**:

1. **Given** inventory hosts and existing assets, **When** a sync preview runs, **Then** it lists per-host create/update changes (matched by hostname/serial) with the fields that differ, scoped to the caller's tenant.
2. **Given** selected hosts, **When** the sync is executed, **Then** new assets are created (auto-tagged, populated from host hardware/OS) and matched assets are updated on the changed fields only.
3. **Given** the inventory module is unavailable, **When** a sync is requested, **Then** it fails gracefully with a clear message and changes nothing.

---

### User Story 6 - Depreciation, lifecycle alerts, statistics, and backup (Priority: P3)

A manager sees each asset's current book value (depreciated), a dashboard of fleet
value and lifecycle risks (expiring/low-stock), and can export/import the tenant's
asset data.

**Why this priority**: Reporting, financial value and portability round out the
product but are not required for core cataloging.

**Independent Test**: Read an asset's depreciated value, view dashboard statistics
(totals, value, expiring/low-stock counts), and export then re-import the tenant.

**Acceptance Scenarios**:

1. **Given** an asset with cost, purchase date and useful life, **When** it is read, **Then** its current book value is computed (double-declining-balance, not below salvage value).
2. **Given** data exists, **When** the dashboard is read, **Then** totals by status, entity counts, total cost, total depreciated value, expiring-soon counts and low-stock count are returned for the tenant.
3. **Given** a tenant export, **When** it is imported (skip or overwrite; full/cross-tenant restore only for administrators), **Then** categories, suppliers, locations, assets, consumables, licenses, assignments, documents (by reference), insurance and coverage are recreated in FK-safe order with no secrets and no cross-tenant leakage.

### Edge Cases

- Assigning a non-deployable asset, or unassigning a non-assigned asset, is refused with a clear reason.
- A duplicate asset tag, policy number, supplier name, category (name+parent) or location name is rejected.
- Deleting a category/location with children, or any of them still referencing assets, is refused unless forced/reassigned.
- An asset with no cost or useful life reports no depreciation (book value = cost or unset), never a negative value.
- A license/insurance already past validity is expired on the next scheduler pass; events for the same expiry are not spammed repeatedly.
- Inventory-sync matches an existing asset by serial first then hostname; ambiguous matches are surfaced, not silently merged.
- Deleting an asset removes its assignment history, its documents (and their objects) and its photo, and detaches it from policies.
- Object storage or the inventory/auth module being unavailable degrades gracefully (uploads/sync/name-resolution fail clearly; the catalog stays consistent).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST let users CRUD assets with a per-tenant unique asset tag (auto-generated when blank), classification (category/supplier/location), status, purchase/warranty/cost and depreciation fields, and free-text search + filtered listing.
- **FR-002**: System MUST support the assign/unassign lifecycle: assign a deployable asset to a platform user (→ assigned, clear location, record assignee resolved from the user directory), unassign an assigned asset (→ deployable, optional location, clear user), with status-gate enforcement and an assignment-history record per action.
- **FR-003**: Users MUST be able to read an asset's assignment history (newest first).
- **FR-004**: System MUST store asset photos and polymorphic documents (for assets, consumables and licenses) in object storage with checksums, list them per entity, download them via short-lived links, and delete them (removing the object).
- **FR-005**: Users MUST be able to CRUD categories and locations as self-referential trees (read nested with counts) and CRUD suppliers, with per-tenant unique names and delete guards (has-children / has-assets).
- **FR-006**: Users MUST be able to CRUD consumables (with stock amount and reorder threshold), licenses (with validity), and insurance policies (with validity and coverage), and manage a policy's covered assets (add/remove with covered value, list).
- **FR-007**: System MUST compute each asset's current book value server-side via double-declining-balance from cost, purchase date, useful life, rate and salvage value (never below salvage), and aggregate total depreciated value in statistics.
- **FR-008**: System MUST run a lifecycle scheduler (per tenant, on an interval) that detects expiring warranties, licenses and insurance and low-stock consumables, publishes the corresponding events, and auto-transitions lapsed licenses/insurance to expired.
- **FR-009**: System MUST publish lifecycle and assignment events (asset.assigned/unassigned, asset.warranty.expiring, license.expiring, insurance.expiring, consumable.low_stock) to the platform event bus for notification and live UI, without repeated duplicate alerts for the same condition.
- **FR-010**: System MUST provide inventory-sync: a preview diffing the platform inventory's hosts against assets (matched by serial/hostname) into create/update changes, and an execute that applies selected hosts (create auto-tagged assets, update changed fields), degrading gracefully when inventory is unavailable.
- **FR-011**: System MUST resolve assignee display names from the platform user directory and expose a user-list for the assignment UI.
- **FR-012**: System MUST provide dashboard statistics: asset counts by status, entity totals, total cost, total depreciated value, expiring-soon counts and low-stock count, per tenant.
- **FR-013**: System MUST support per-tenant export/import of all asset data in FK-safe order with schema versioning and skip/overwrite handling; full/cross-tenant restore MUST be restricted to platform administrators; secrets are never exported.
- **FR-014**: System MUST register routes, API permissions, UI abilities and navigation with the application gateway and expose service-to-service APIs for other modules.
- **FR-015**: System MUST enforce API permissions (read; asset/category/supplier/location/consumable/license/insurance/document management; assign; inventory-sync; stats; backup) and gate full backup restore to platform administrators.
- **FR-016**: System MUST record an append-only audit entry for every mutation, assignment, upload, sync and backup, with contact PII and object-store credentials redacted.

### Security Requirements *(mandatory — Constitution: Development Workflow)*

- **Trust boundaries crossed**: browser API via the application gateway; service-to-service mesh gRPC (inventory, auth, warden); object storage; the shared event bus.
- **Data classification**: asset inventory and financials (tenant-confidential); supplier/location contact details (PII); object-store credentials (secret); audit records (tamper-evident).
- **Authentication/Authorization**: browser callers use the gateway platform token; module callers use SPIFFE mTLS; API permissions gate every operation; full/cross-tenant backup restore requires platform-admin; RLS isolates tenants.
- **Threat scenarios**: cross-tenant read/restore of another tenant's assets; leakage of contact PII or object-store credentials; a forged tenant/user claim; object keys used to reach another tenant's files.
- **SR-001**: All asset data MUST be isolated per tenant by row-level security (including the assignment, document and policy-asset tables); unique constraints MUST enforce conflict detection (duplicate tag/policy-number/name).
- **SR-002**: Object-store credentials MUST be sealed at rest and never returned in any response, log, audit entry or backup; object keys MUST be tenant-scoped so a caller cannot reach another tenant's files; presigned links MUST be short-lived.
- **SR-003**: Supplier/location contact PII MUST be redacted from logs, audit detail and backups and returned only to authorized callers.
- **SR-004**: Full and cross-tenant backup restore MUST require platform-administrator authority; a non-admin import MUST land only in the caller's own tenant.
- **SR-005**: The lifecycle scheduler and other trusted worker paths MUST run under a scoped system subject, never an unauthenticated bypass; the inventory-sync client MUST authenticate to the inventory module over SPIFFE mTLS (no shared secret).
- **SR-006**: All operations MUST be recorded in an append-only, tamper-evident audit trail with actor, tenant, subject and outcome.

### Key Entities *(include if feature involves data)*

- **Asset**: an IT asset in a tenant; tag (unique), classification, status, purchase/warranty/cost, depreciation inputs, assignee or location, photo.
- **Assignment**: a check-out/check-in history record for an asset (user, action, assigned/returned time).
- **Document**: a file attached to an asset, consumable or license, stored in object storage.
- **Category / Location**: self-referential hierarchies classifying assets; unique names per tenant.
- **Supplier**: a vendor with contact details (PII).
- **Consumable**: a stock item with quantity and reorder threshold.
- **License**: a software license with a validity window and status.
- **Insurance policy / policy-asset**: a coverage policy and its covered assets (with covered value).
- **Platform user**: the external assignee, resolved from the user directory (no local table).

## Success Criteria *(mandatory)*

- **SC-001**: A manager can create and assign an asset in under 1 minute, with the status gate preventing any double-assignment (0 assets ever assigned to two people at once).
- **SC-002**: 100% of duplicate-tag, duplicate-policy-number and in-use-delete attempts are rejected.
- **SC-003**: An asset's depreciated book value is computed correctly (double-declining-balance, floored at salvage) in 100% of test cases and never negative.
- **SC-004**: The lifecycle scheduler surfaces every warranty/license/insurance expiry and low-stock condition within one evaluation interval, without duplicate alerts for the same condition.
- **SC-005**: An inventory-sync preview correctly classifies each host as create/update/unchanged against the catalog in 100% of test cases, and never reaches or reads another tenant's data.
- **SC-006**: Object-store credentials and supplier/location contact PII never appear in any response, log, audit entry or backup (verified by inspection); object keys never resolve across tenants.
- **SC-007**: The system manages at least 100,000 assets per tenant with asset list, tree and dashboard queries returning in under 3 seconds.
- **SC-008**: A tenant export re-imports to an equivalent state (all entities, FK-ordered) with no cross-tenant leakage and no secrets; a non-admin restore lands only in the caller's tenant.

## Assumptions

- The assignee is a platform user identified by id; display names are resolved from the platform user directory and cached best-effort.
- Depreciation defaults to double-declining-balance with a 40% rate when unspecified; assets without cost/useful-life report no depreciation.
- Inventory-sync matches on serial first, then hostname; the inventory module is the source of discovered hosts and is optional (sync degrades gracefully when absent).
- The platform provides tenant identity, the gateway, the event bus, the audit trail, certificate/identity issuance, the object store, and the inventory and auth modules; this feature consumes them.
- Expiry "soon" windows (warranty/license/insurance) are configurable with sensible defaults.

## Out of Scope

- Software license seat metering/allocation and automated compliance enforcement (licenses are tracked, not metered).
- Automated consumable decrement on any workflow (stock is adjusted explicitly).
- Procurement/purchase-order workflows and financial ledger integration (cost is recorded, not accounted).
- Being the source of discovered inventory (the inventory module is; this service reconciles against it).
