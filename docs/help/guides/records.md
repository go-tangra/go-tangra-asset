---
title: Categories, locations, suppliers, stock, licenses and insurance
summary: Keep the records around your assets, track consumable stock, license and policy validity, and the expiry and low-stock warnings.
kind: guide
order: 30
tags: [asset, categories, locations, suppliers, consumables, licenses, insurance, expiry]
requires: assets:read
---
Everyone with `assets:read` can open these pages. Each kind of record has its
own permission for changes: `categories:manage`, `locations:manage`,
`suppliers:manage`, `consumables:manage`, `licenses:manage` and
`insurance:manage`.

## Categories and locations

Both are trees. Choose **New category** (or **New location**) for a top-level
entry; select an entry and choose **Add child** to create one beneath it.

- A category name must be unique under its parent, so *Laptops* can exist
  under two different parents.
- A location name must be unique in the whole tenant. Assets and consumables
  show a location by its path, for example `HQ / Floor 2`.
- A location has a status: `active`, `planned` or `decommissioned`, and an
  address and contact details.
- You cannot move an entry under one of its own descendants.

An entry that still has children or assets cannot be deleted: move or delete
them first. Consumables that use a deleted category or location simply lose
that reference.

## Suppliers

A supplier has a name (unique in the tenant), a code, a status (`active` or
`inactive`), an address, a website and contact details. A supplier used by
any asset cannot be deleted; consumables and licenses that use a deleted
supplier lose the reference.

**Contact details** (contact person, telephone and e-mail of suppliers, and
the contact, phone and e-mail of locations) are encrypted. Only tenant owners
and administrators see them; everyone else sees the fields empty, and they
are left out of backups.

## Consumables

A consumable is a stock item with an **Amount in stock** and a **Reorder
threshold**. It is *low* when the amount is at or below the threshold (a
threshold of 0 never warns). Stock is not decremented automatically: edit the
amount when you take items out or receive a delivery. Select a consumable to
see and upload its documents.

## Licenses

A license records a supplier, a validity period (**Valid from**, **Valid
to**), the purchase and a status: `active`, `expired` or `suspended`. Licenses
are tracked, not metered: there are no seats or allocations. Once **Valid
to** has passed, the module sets the status to `expired` by itself, unless the
license is `suspended`. Select a license to see and upload its documents.

## Insurance policies

A policy has a name and a policy number (both unique in the tenant), a
provider, a coverage type (`all_risk`, `fire_theft`, `liability`,
`equipment_breakdown`, `cyber`), the premium, deductible and coverage limit,
a validity period and a status: `active`, `expired` or `cancelled`. Like
licenses, a policy past its **Valid to** date is set to `expired`
automatically, unless it is `cancelled`.

To record what a policy covers, select it, pick an **Asset**, enter the
**Covered value** and choose **Add**. An asset can be on a policy only once.
**Remove** takes it off again.

## Expiry and low-stock warnings

The module checks every tenant at start and then on an interval (hourly
unless your operator changed it). It raises an event when:

| Event | When |
|---|---|
| `asset.warranty.expiring` | purchase date + warranty months is within 30 days, or past (archived assets are skipped) |
| `license.expiring` | **Valid to** is within 30 days, or past |
| `insurance.expiring` | **Valid to** is within 30 days, or past |
| `consumable.low_stock` | amount at or below the reorder threshold |

The 30-day windows are the defaults; your operator can change them. A
condition is reported once; it is reported again only after it changes (for
example from *expiring soon* to *expired*) or clears and comes back. A
low-stock consumable is also reported again when its amount changes while it
is still low.

The **Dashboard** counts items expiring soon and consumables low on stock,
and its **Recent lifecycle events** card lists the events that arrive while
the page is open.
