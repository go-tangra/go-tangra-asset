---
title: Sync assets from inventory
summary: Preview the hosts the inventory module knows, create or update assets from them, filter out VMs, containers and unwanted hosts, and run the sync on a schedule.
kind: guide
order: 40
tags: [asset, inventory, sync, filter, scheduler]
requires: inventory:sync
---
The inventory module discovers your machines through its agents. **Inventory
Sync** compares those hosts with your assets and creates or updates assets
from them. It never deletes assets. You need `inventory:sync`.

## How hosts are matched

Each host is matched to an existing asset by, in order:

1. the asset tag `inventory_host_id` (set on every asset the sync created),
2. the serial number (case-insensitive),
3. the hostname against the asset name (case-insensitive).

If two hosts match the same asset, the second one is treated as new.

A matched asset can have these fields updated from the host: **name** (the
hostname), **serial** and **model** (only when the host reports them), and the
tags `inventory_host_id`, `manufacturer`, `os_name`, `os_version` and
`os_arch`. Nothing else is touched: status, category, location, assignee,
purchase and depreciation data stay as they are.

A new asset is created `deployable` with a generated `AST-` tag, the hostname
as its name (or the serial when there is no hostname), and the serial, model
and tags from the host.

> The sync sets the asset name to the hostname. If you renamed a synced asset
> by hand, the next sync proposes to change it back.

## Preview and execute

1. Open **Inventory Sync** and choose **Preview**. Nothing is written.
2. The tiles count the **Hosts** and how many are **To create**, **To
   update**, **Unchanged** and **Excluded**. The table lists every host with
   its action and, for updates, each field's old and new value.
3. All hosts to create or update are selected. Clear the ones you do not
   want.
4. Choose **Execute**. The result reports how many assets were created,
   updated, skipped and excluded, and any per-host errors. The preview then
   runs again.

If the inventory module cannot be reached, **Preview** and **Execute** fail
with *temporarily unavailable* and nothing is changed.

## Sync filter

The **Sync filter** card below the table decides which hosts are never
created or updated. It applies to previews, manual syncs and the scheduled
sync. Choose **Save filter** to store it for the tenant.

| Setting | Skips hosts |
|---|---|
| **Skip virtual machines** | the inventory reports as virtual machines |
| **Skip containers** | the inventory reports as containers |
| **Skip stale hosts** | with inventory status `stale` |
| **Skip retired hosts** | with inventory status `retired` |
| **Only hostnames matching** | whose hostname matches none of the patterns (empty: all) |
| **Skip hostnames matching** | whose hostname matches a pattern |
| **Only operating systems matching** | whose OS name matches none of the patterns (empty: all) |
| **Skip operating systems matching** | whose OS name matches a pattern |

Patterns are case-insensitive, one per line, with `*` and `?` as wildcards,
for example `*.lab.*` or `windows server*`. Each list takes up to 50 patterns
of up to 200 characters.

Excluded hosts appear in the preview as `excluded` with the reason, such as
*Skipped: virtual machine*. Their existing assets are kept unchanged.

The VM and container switches need agents that report virtualization. Hosts
whose role is unknown are imported. If either switch is on and the roles
cannot be read from inventory, the sync fails rather than import hosts the
filter should have skipped.

## Scheduled sync

When the platform scheduler is connected, it offers the task **Sync assets
from inventory** (`asset:inventory-sync`, suggested schedule `0 3 * * *`).
It runs the whole filtered sync for your tenant. Its payload chooses what to
apply:

```json
{"create": true, "update": true}
```

Both default to `true`. Set `"create": false` to only update assets that
already exist. Ask your operator if the task is not offered.
