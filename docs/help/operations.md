---
title: Operate the Assets module
summary: Configuration keys and defaults, dependencies, the service policies the module needs, optional inventory, paperless and scheduler integrations, and fixes for common problems.
kind: reference
order: 90
tags: [asset, operations, configuration, policy, troubleshooting]
audience: operator
---
The module runs as `assetsvc` (image `ghcr.io/go-tangra/go-tangra-asset`,
user `app`, uid 10001).

```sh
assetsvc -config deploy/container.yaml             # serve; applies migrations
assetsvc bootstrap -config deploy/container.yaml   # apply migrations and exit
assetsvc version
```

At start it enrolls for its SVID `spiffe://<trust domain>/svc/asset` (when
`mesh_enroll.enabled`), migrates its TimescaleDB schema, creates the object
store bucket if it is missing, serves the browser API and the `asset.v1`
gRPC services, registers with the gateway, and registers its permissions,
roles and built-in grants with auth (at start and every five minutes).

## Dependencies

| Service | Needed for | Required |
|---|---|:-:|
| TimescaleDB | all data; database `asset`, app role `asset_app` (no `BYPASSRLS`) | yes |
| Valkey (platform event bus) | live events, user `asset` | yes |
| S3-compatible object store (RustFS, MinIO) | photos; documents when paperless is off | yes |
| auth | token verification, user names, the assignee list | yes |
| gateway | browser traffic, registration | yes |
| inventory | inventory sync | no |
| paperless | document storage and full-text search | no |
| scheduler | the `asset:inventory-sync` task | no |

The module starts without inventory, paperless and the scheduler; the
features that use them fail with `temporarily_unavailable` until they are
reachable.

## Configuration

The file is YAML. Unknown keys are refused, so a typo stops the service at
start. Besides the framework keys (`service_name`, `trust_domain`, `env`,
`identity`, `authz`, `server`, `admin`, `discovery`, `limits`) the module
reads:

| Key | Default | Notes |
|---|---|---|
| `db.dsn` | — | required; in production `sslmode=verify-full` (or `verify-ca`) |
| `db.migrate_dsn` | `db.dsn` | owner connection used for migrations |
| `db.max_conns` | 16 | |
| `valkey.addresses` | — | required |
| `valkey.username`, `valkey.password`, `valkey.ca_file` | — | Valkey user `asset` over TLS |
| `valkey.allow_plaintext` | false | development only; refused in production |
| `kek.source` | `file` | `file` (with `kek.path`) or `env` (with `kek.env`): the 32-byte key (raw or base64) that encrypts supplier and location contact details |
| `object_store.endpoint`, `object_store.bucket` | — | required |
| `object_store.region` | `us-east-1` | |
| `object_store.use_ssl` | false | required in production |
| `object_store.access_key`, `object_store.secret_key` | — | |
| `object_store.presign_ttl_seconds` | 300 | 30–3600 |
| `uploads.max_size_bytes` | 20 MiB | 1 KiB–1 GiB; the gateway route still caps photo and document uploads at 20 MiB |
| `scheduler.interval_seconds` | 3600 | 10–86400; expiry and low-stock check |
| `scheduler.warranty_soon_days`, `license_soon_days`, `insurance_soon_days` | 30 | 1–3650 |
| `events.enabled` | true | publish live events |
| `inventory.service` | `inventory` | mesh name of the inventory module |
| `paperless.enabled` | true | store documents in paperless |
| `paperless.service` | `paperless` | |
| `task_scheduler.enabled` | false | register `asset:inventory-sync` with the scheduler |
| `task_scheduler.service` | `scheduler` | |
| `gateway.service` | `gateway` | mesh name of the gateway; the only peer allowed to fetch the help pages |
| `gateway.issuer` | — | required, an `https://` origin: the platform token issuer |
| `mesh_enroll.*` | — | `enabled`, `enroll_url`, `lcm_grpc`, `tenant_id`, `token_file`, `state_file`, `insecure` (refused in production) |
| `limits_asset.max_request_bytes` | 1 MiB | 1 KiB–64 MiB; range-checked at start |
| `limits_asset.max_backup_bytes` | 32 MiB | 1 KiB–256 MiB; range-checked at start |

At start the log lists accepted insecure settings (`valkey.allow_plaintext`,
`object_store.use_ssl=false`, `mesh_enroll.insecure`).

The gateway's add-module bundle sets `paperless.enabled: false` and
`task_scheduler.enabled: false` and lists only the gateway and auth in
`discovery.static`: inventory sync, paperless storage and the scheduled task
are off until you add those services.

## Service policies

The module's own policy must let the gateway in. The usual rule also covers
the help pages (`GET /docs/v1/bundle`):

```yaml
rules:
  - id: gateway-forwards
    from: ["spiffe://<trust domain>/svc/gateway"]
    to: ["asset"]
    operations: ["*"]
    effect: allow
  - id: scheduler-execute
    from: ["spiffe://<trust domain>/svc/scheduler"]
    to: ["asset"]
    operations: ["/scheduler.v1.TaskExecutor/ExecuteTask"]
    effect: allow
```

Other modules may read `GetAsset`, `ListAssets`, `GetCategoryTree`,
`ListSuppliers`, `GetLocationTree`, `Health` and `GetDashboardStats` of
`asset.v1` (rule `modules-read`).

Other services must admit `spiffe://<trust domain>/svc/asset`:

| Service | Operations | Why |
|---|---|---|
| auth | `/auth.v1.Profiles/Lookup`, `/auth.v1.Profiles/ListMembers` | assignee names and the user list |
| inventory | `InventoryHostService/ListHosts` | inventory sync |
| inventory | `HostReportService/ListHostReports`; `host_reports.consumers` must list `asset` | VM and container filter |
| paperless | its `asset-documents` rule | document storage and search |
| scheduler | `asset` in `modules-register`, and `discovery.static.asset` | registering and running the sync task |

Add `discovery.static.inventory`, `discovery.static.paperless` and
`discovery.static.scheduler` to the asset configuration for the services you
use.

## Documents in paperless

With `paperless.enabled`, new documents of assets, consumables and licenses
are stored in paperless under the category `Assets/<asset tag>`,
`Assets/Consumables/<name>` or `Assets/Licenses/<name>`. Paperless users see
them through their access to the `Assets` category. Documents still in the
object store are moved a minute after start and then hourly, checksum
verified; a document that fails stays where it is and is retried. Photos
stay in the object store.

## Troubleshooting

**The service refuses to start with `config: ...`.** The message names the
key and its allowed range, for example `db.dsn must use sslmode=verify-full
(or verify-ca) in production`, `object_store.use_ssl is required in
production` or `gateway.issuer must be an https origin`.

**It refuses to start with `kek: ...`.** The key file or variable is missing
or is not 32 bytes. Use the same key as before: contact details encrypted
with another key are shown empty.

**It refuses to start with `asset: help docs: ...`.** The help pages built
into the binary are invalid. CI catches this with `tangra-docs lint`; rebuild
from a commit that passed CI.

**Inventory sync answers `temporarily_unavailable`.** inventory is not in
`discovery.static`, is down, or its policy does not admit `svc/asset` for
`ListHosts`. With the VM or container filter on, `ListHostReports` must be
allowed too.

**Document search answers `temporarily_unavailable`.** `paperless.enabled` is
false, or paperless is unreachable.

**The scheduled sync task is not offered.** Set `task_scheduler.enabled:
true`, add `discovery.static.scheduler`, and let the scheduler register
`svc/asset` and reach it (`discovery.static.asset`).

**Uploads fail.** The log shows `object store: ensure bucket` at start when
the store was unreachable; check `object_store.*` and its TLS. Files over the
upload limit get `body_too_large`.

**Roles are missing in auth.** Look for `auth registration failed; retrying`
in the log. The module retries until auth accepts, then registers every five
minutes.
