---
title: Assets
summary: Track IT assets and who has them, with categories, locations, suppliers, consumables, licenses, insurance, depreciation and inventory sync.
kind: guide
order: 0
tags: [asset, itam, overview]
---
The Assets module is your tenant's IT asset register. It records each piece
of equipment, who it is assigned to or where it is stored, what it cost and
what it is worth today. Around the assets it keeps the records you need to
manage them: categories, locations, suppliers, consumable stock, software
licenses and insurance policies.

## Concepts

- **Asset**: one item, identified by an **asset tag** that is unique in the
  tenant (for example `AST-3F09C2`). It has a status, an optional photo,
  documents, and purchase, warranty and depreciation data.
- **Assignment**: an asset is either *assigned* to a platform user or kept at
  a location. Every assign and unassign is recorded in the asset's
  assignment history.
- **Categories** and **locations**: two trees you file assets under, for
  example *Laptops / Developer laptops* or *HQ / Floor 2*.
- **Suppliers**: vendors you buy from. Their contact details are encrypted
  and shown only to tenant owners and administrators.
- **Consumables**: stock items (toner, cables) with an amount and a reorder
  threshold.
- **Licenses** and **insurance policies**: records with a validity period.
  A policy also lists the assets it covers.
- **Documents**: files (invoices, warranty PDFs) attached to an asset, a
  consumable or a license.

The module also computes each asset's **book value** (double-declining
balance), warns when warranties, licenses and insurance are about to expire
or stock runs low, and can create or update assets from the hosts the
**inventory** module discovers.

## Asset statuses

| Status | Meaning |
|---|---|
| `deployable` | available; kept at a location; can be assigned |
| `assigned` | checked out to a user (set only by **Assign**) |
| `broken` | out of service |
| `archived` | retired; no more warranty warnings |

## Pages in the portal

| Page | Shows | Needs |
|---|---|---|
| Assets | the asset list; open an asset for its details, photo, documents and history | `assets:read` |
| Categories, Suppliers, Locations | the reference records | `assets:read` |
| Consumables, Licenses, Insurance | stock, licenses and policies | `assets:read` |
| Document search | full-text search in attached documents | `assets:read` |
| Inventory Sync | create or update assets from inventory hosts | `inventory:sync` |
| Dashboard | totals, value, expiring items, low stock, backup | `stats:read` |

Reading is open to everyone with `assets:read`; each kind of change needs its
own permission. See [permissions and roles](reference.md#permissions-and-roles).

## Where to go next

- [Get started](getting-started.md): file and assign your first asset.
- [Track and assign assets](guides/assets.md)
- [Categories, locations, suppliers, stock, licenses and insurance](guides/records.md)
- [Sync assets from inventory](guides/inventory-sync.md)
- [Example: a laptop's book value over five years](examples/depreciation.md)
- [Reference](reference.md): fields, statuses, events, permissions, backup and limits.
