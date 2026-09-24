# go-tangra-asset

Tenant IT asset management (ITAM) service for the
[go-tangra v4 platform](https://github.com/go-tangra/go-tangra).

It tracks **assets** with an assign/unassign lifecycle to platform users,
**photos and documents** (metadata in TimescaleDB, bytes in S3-compatible object
storage such as RustFS or MinIO), **categories** and **locations** (trees),
**suppliers**, **consumables** (stock), **licenses**, **insurance policies** with
per-asset coverage, and server-side **depreciation** (double-declining balance).
A lifecycle scheduler raises warranty, license, insurance and low-stock events on
the platform event bus. **Inventory sync** diffs the hosts reported by the
inventory service against the assets and creates or updates them. Supplier and
location contact data is sealed at rest (AES-256-GCM envelope) and returned in
clear only to tenant admins.

Operations: [`deploy/README.md`](deploy/README.md).
Design history: `specs/012-asset-service`.

## Place in the platform

```
go-tangra/go-tangra          platform module + @go-tangra/ui kit
        |
go-tangra-auth  <---->  go-tangra-portal (gateway)  <---->  go-tangra-lcm
                                  |
                          go-tangra-asset  ---->  object storage (S3)
                                  |
                                  +---- mTLS ---->  go-tangra-inventory (host list)
```

- Built on `github.com/go-tangra/go-tangra/v4` (mTLS transports, identity,
  service policy, audit, observability).
- Verifies platform tokens, resolves people and checks permissions through the
  auth SDK (`github.com/go-tangra/go-tangra-auth/sdk/v4`).
- Registers with the gateway through the portal SDK
  (`github.com/go-tangra/go-tangra-portal/sdk/v4`), which fronts the browser API
  (`/api/asset`) and the federated UI remote.
- Enrolls for its SVID with lcm over the network
  (`github.com/go-tangra/go-tangra-lcm/sdk/v4`), as the platform stack does.
- Reads hosts from inventory over the SPIFFE mTLS mesh through the inventory SDK
  (`github.com/go-tangra/go-tangra-inventory/sdk/v4`). The inventory policy must
  allow `svc/asset` on `InventoryHostService/ListHosts`.

The repository holds one Go module, `github.com/go-tangra/go-tangra-asset/v4`.
Other services call it through `pkg/assetclient` and the `asset.v1` protos
(not proxied by the gateway).

## Layout

| Path | What |
|------|------|
| `api/openapi/asset.yaml` | browser API contract (served under `/api/asset`) |
| `api/proto/asset/v1/` | asset gRPC services |
| `internal/config` | configuration + validation (secure defaults, named opt-outs) |
| `internal/store`, `internal/repo` | TimescaleDB schema (RLS), repositories; `internal/memstore` is the in-memory test double |
| `internal/blob` | S3-compatible object storage (minio-go), presigned downloads |
| `internal/sealed` | envelope encryption of contact PII (KEK -> DEK) |
| `internal/authz` | per-route permission checks |
| `internal/assets`, `internal/categories`, `internal/locations`, `internal/suppliers` | assets and their reference data |
| `internal/consumables`, `internal/licenses`, `internal/insurance`, `internal/documents` | stock, licenses, insurance coverage, documents |
| `internal/deprec` | pure depreciation library |
| `internal/invclient`, `internal/invsync` | inventory client adapter and the sync preview/execute |
| `internal/scheduler`, `internal/events`, `internal/stream` | lifecycle sweeps, events on the platform bus, live stream |
| `internal/backup`, `internal/stats`, `internal/audit`, `internal/userdir` | backup export/import, statistics, audit, user lookups |
| `internal/httpapi`, `internal/grpcapi` | browser and service APIs |
| `internal/app`, `cmd/assetsvc` | wiring and the service binary (serve, `bootstrap`, `version`) |
| `pkg/assetmanifest` | gateway manifest and built-in role grants |
| `pkg/assetclient` | Go client other services use |
| `deploy` | service policy and operations notes |
| `ui/` | Vue 3 + FlyonUI federated remote on `@go-tangra/ui` |

## Build and test

You need Go 1.26, Node 22, Docker (for the integration suite and the image), and a
GitHub token with `read:packages` to install `@go-tangra/ui` from GitHub Packages.

```bash
go build ./... && go vet ./... && go test -race ./...
buf lint
make test-integration                     # -tags integration, TimescaleDB + Valkey via testcontainers (needs Docker)
make lint cover vuln

cd ui
export NODE_AUTH_TOKEN=$(gh auth token)   # ui/.npmrc only references this variable
npm ci && npm run lint && npm run test:unit && npm run build
```

The unit coverage gate requires at least 80 % overall and 100 % for the
authorization, sealing and depreciation packages. Generated code, SQL bindings and
wiring are covered by the integration suite instead. The Playwright specs in
`ui/tests/e2e` need a running platform and operator credentials; they skip
otherwise.

## Run

The service runs in the go-tangra platform stack (`deploy/stack` in
[go-tangra](https://github.com/go-tangra/go-tangra)), next to TimescaleDB, Valkey,
RustFS and the inventory service. The stack mounts its configuration at
`/app/deploy/container.yaml` and the development key-encryption key at
`/app/deploy/kek.dev`. `deploy/kek.dev` in this repository is a development key
only; it is excluded from the image.

```bash
assetsvc bootstrap -config deploy/container.yaml    # apply migrations and exit
assetsvc -config deploy/container.yaml              # serve (applies migrations)
```

See `specs/012-asset-service/quickstart.md` for the end-to-end walkthrough.

## Container image

The image is `ghcr.io/go-tangra/go-tangra-asset`, built by
`.github/workflows/ci.yaml`. It carries `assetsvc` with the embedded UI remote.

```bash
docker buildx build --secret id=npm_token,env=NODE_AUTH_TOKEN \
  --build-arg APP_VERSION=4.0.0 -t go-tangra-asset:dev .
docker run --rm go-tangra-asset:dev version
```

The image runs `assetsvc -config deploy/container.yaml` as user `app`
(uid 10001). It contains no configuration and no key material: deployments mount
their own `deploy/container.yaml` and key-encryption key. The object store and
inventory are separate services the configuration points at
(`object_store.endpoint`, `discovery.static.inventory`); the bucket is created on
start when missing.

## API permissions

`assets:read/manage/assign`, `categories:manage`, `suppliers:manage`,
`locations:manage`, `consumables:manage`, `licenses:manage`, `insurance:manage`,
`documents:manage`, `inventory:sync`, `stats:read`, `backup:manage`. The gateway
enforces the per-route permission from the manifest; the module checks it again.
Built-in role grants are seeded by the module (`pkg/assetmanifest`).

## Versioning

- Releases are tagged `vX.Y.Z`. CI publishes the image as `X.Y.Z`, `X.Y`, `X`
  and `sha-<short>`. There is no `latest` tag.
- v4.0.0 rebuilds the service on the go-tangra v4 platform. The v3 line stays on
  the `v3` branch and its `v3.x` tags.
