---
title: Reference
summary: Asset and record fields, statuses, events, permissions, roles, backup contents, API routes, errors and limits.
kind: reference
order: 60
tags: [asset, reference, fields, statuses, permissions, backup, api, limits]
---
## Asset fields

| Field | Meaning |
|---|---|
| `asset_tag` | unique in the tenant; 1–64 characters, no spaces; generated as `AST-` + 6 hex characters when blank |
| `name` | required, at most 200 characters |
| `serial`, `model_name`, `model_number` | free text |
| `category_id`, `supplier_id`, `location_id` | references to existing records; the location is cleared while the asset is assigned |
| `status` | see [asset statuses](#asset-statuses) |
| `user_id`, `assignee_name` | the assigned user and their display name |
| `purchase_date`, `purchase_cost`, `order_number` | purchase data; amounts may not be negative |
| `warranty_months` | warranty length from the purchase date |
| `useful_life_years`, `salvage_value`, `depreciation_rate` | depreciation inputs; rate 0–1, default `0.40` |
| `book_value` | current depreciated value, computed on every read |
| `has_photo` | a photo is stored |
| `notes` | free text |
| `tags` | up to 64 key/value pairs |

## Asset statuses

| Status | Set by | Can be assigned |
|---|---|:-:|
| `deployable` | default; **Unassign**; edit | yes |
| `assigned` | **Assign** only | no |
| `broken` | edit | no |
| `archived` | edit | no |

## Assignment history

Each **Assign** adds an `assigned` row (user, time, who assigned it, note).
Each **Unassign** closes the open row (sets its returned time) and adds an
`unassigned` row. The history is listed newest first.

## Other records

| Record | Unique | Statuses |
|---|---|---|
| Category | name under the same parent | — |
| Location | name in the tenant | `active`, `planned`, `decommissioned` |
| Supplier | name in the tenant | `active`, `inactive` |
| Consumable | — | low stock when `amount` ≤ `min_amount` and `min_amount` > 0 |
| License | — | `active`, `expired`, `suspended` |
| Insurance policy | name and policy number in the tenant | `active`, `expired`, `cancelled` |

Coverage types: `all_risk`, `fire_theft`, `liability`, `equipment_breakdown`,
`cyber`. An asset can be covered by a policy once, with an optional covered
value.

Deleting a category or location needs it to have no children and no assets;
deleting a supplier needs it to have no assets. Deleting a consumable or
license removes its documents.

## Documents and photos

| | Limit |
|---|---|
| upload size | 20 MiB (operator setting) |
| photo types | PNG, JPEG, GIF, WebP |
| document types | any |
| file name | at most 255 characters |
| documents belong to | an asset, a consumable or a license |

## Events

Published on the platform event bus and streamed live to open pages.

| Event | When |
|---|---|
| `asset.assigned` | an asset is assigned |
| `asset.unassigned` | an asset is unassigned |
| `asset.warranty.expiring` | warranty end within the warning window, or past |
| `license.expiring` | license **Valid to** within the window, or past |
| `insurance.expiring` | policy **Valid to** within the window, or past |
| `consumable.low_stock` | stock at or below the threshold |

The expiry and stock checks run at start and then on an interval (default
hourly); the windows default to 30 days. See
[warnings](guides/records.md#expiry-and-low-stock-warnings).

## Permissions and roles

| Permission | Allows |
|---|---|
| `assets:read` | read assets, records, documents, users and the live stream; search documents |
| `assets:manage` | create, change and delete assets |
| `assets:assign` | assign and unassign assets |
| `categories:manage` | manage the category tree |
| `suppliers:manage` | manage suppliers |
| `locations:manage` | manage the location tree |
| `consumables:manage` | manage consumables |
| `licenses:manage` | manage licenses |
| `insurance:manage` | manage policies and the assets they cover |
| `documents:manage` | upload and delete photos and documents |
| `inventory:sync` | preview and run inventory sync, change the sync filter |
| `stats:read` | read the dashboard statistics |
| `backup:manage` | export and import the tenant's asset data |

| Module role | Permissions |
|---|---|
| Assets administrator | all thirteen |
| Assets editor | `assets:read`, `assets:manage`, `assets:assign`, `stats:read` |
| Assets viewer | `assets:read`, `stats:read` |

The platform's built-in roles also receive asset permissions: `owner` and
`admin` all; `operator` all but `backup:manage`; `member` and `auditor`
`assets:read` and `stats:read`.

Supplier and location contact details are shown only to holders of the
tenant `owner` or `admin` role, whatever their asset permissions.

## Backup

On the **Dashboard**, **Export tenant data** downloads a JSON file
(`asset-backup-<date>.json`) with suppliers, locations, categories, assets,
assignments, consumables, licenses, insurance policies, covered assets and
document metadata. It does **not** contain supplier and location contact
details, nor the bytes of photos and documents. The inventory-sync filter is
not exported.

**Import** reads such a file into your tenant. **On conflict** decides what
happens to records that already exist (same ID): `skip` keeps them,
`overwrite` replaces them. The request is limited to 32 MiB.

Through the API an import can also wipe the tenant first (`full: true`) or
target another tenant (`tenant_id`); both need a platform administrator.
Records restored into another tenant get new IDs.

## API

The browser API is served by the gateway under `/api/asset/v1`. Every change
needs the `X-CSRF-Token` header. Lists take `page`, `page_size` (1–200),
`sort` and `order` (`asc`/`desc`); asset, supplier, consumable, license and
policy lists also take `query`.

| Route | Permission |
|---|---|
| `GET /assets`, `GET /assets/{id}`, `GET /assets/{id}/assignments` | `assets:read` |
| `POST /assets`, `PUT /assets/{id}`, `DELETE /assets/{id}` | `assets:manage` |
| `POST /assets/{id}/assign`, `POST /assets/{id}/unassign` | `assets:assign` |
| `GET /assets/{id}/photo` | `assets:read` |
| `POST`, `DELETE /assets/{id}/photo` | `documents:manage` |
| `GET /{assets,consumables,licenses}/{id}/documents`, `.../{doc_id}/download` | `assets:read` |
| `POST /{assets,consumables,licenses}/{id}/documents`, `DELETE .../{doc_id}` | `documents:manage` |
| `GET /documents/search?q=` | `assets:read` |
| `GET /categories`, `/categories/tree`, `/locations`, `/locations/tree`, `/suppliers`, `/consumables`, `/licenses`, `/insurance-policies` and their `/{id}` | `assets:read` |
| `POST`, `PUT`, `DELETE` on those | the record's `:manage` permission |
| `GET`, `POST /insurance-policies/{id}/assets`, `DELETE .../assets/{asset_id}` | `assets:read` / `insurance:manage` |
| `POST /assets/inventory-sync/preview`, `/execute`; `GET`, `PUT /assets/inventory-sync/settings` | `inventory:sync` |
| `GET /users` | `assets:read` |
| `GET /stats` | `stats:read` |
| `POST /backup/export`, `POST /backup/import` | `backup:manage` |
| `GET /stream` (server-sent events) | `assets:read` |
| `GET /health` | public |

`POST /assets/inventory-sync/execute` takes `{"hostnames": [...]}`; an empty
list syncs every host.

## Errors

| Reason | Status | Typical cause |
|---|---|---|
| `validation_failed` | 422 | a bad field, an unknown reference, an invalid filter pattern |
| `conflict` | 409 | duplicate tag, name or policy number; assign/unassign in the wrong status; deleting a record still in use |
| `not_found` | 404 | the record does not exist in your tenant |
| `forbidden` | 403 | missing permission, or a full or cross-tenant import without platform administrator |
| `body_too_large` | 413 | upload or import over its limit |
| `unsupported_media_type` | 415 | a photo that is not PNG, JPEG, GIF or WebP |
| `temporarily_unavailable` | 503 | inventory or paperless unreachable |
