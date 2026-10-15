---
title: "Example: a laptop's book value over five years"
summary: How the module computes an asset's current book value with double-declining balance, worked through for a laptop bought for 1500.
kind: example
order: 50
tags: [asset, example, depreciation, book-value]
requires: assets:read
---
**Goal.** Your finance team writes laptops off over five years and expects
each one to be worth about 100 at the end. You want the **Book value** column
and the dashboard's **Depreciated value** to reflect that.

## The asset

Create or edit the laptop with:

| Field | Value |
|---|---|
| **Purchase date** | the day it was bought |
| **Purchase cost** | `1500` |
| **Useful life (years)** | `5` |
| **Salvage value** | `100` |
| **Depreciation rate (0–1)** | blank (the default `0.40`) |

The module stores no currency: use the same one everywhere.

## How the value is computed

The book value is recomputed every time the asset is read:

```text
book value = purchase cost × (1 − rate) ^ years since purchase
```

`years since purchase` is fractional, so the value falls a little every day
rather than once a year. The result never drops below the salvage value, and
from the end of the useful life on it is exactly the salvage value.

For this laptop:

| Age | Book value |
|---|---|
| new | 1500.00 |
| 6 months | 1161.90 |
| 1 year | 900.00 |
| 2 years | 540.00 |
| 3 years | 324.00 |
| 4 years | 194.40 |
| 5 years and later | 100.00 |

The asset page shows this as **Current book value** in the **Depreciation**
card, next to the purchase cost, salvage value, useful life, the rate and the
**Warranty until** date.

## Variations

- **A slower write-down.** Set the rate to `0.25`: after three years the
  laptop is worth 632.81, after four 474.61, and it still reaches 100 at five
  years.
- **No depreciation.** Leave **Purchase cost**, **Purchase date** or **Useful
  life** empty (or 0) and the book value equals the purchase cost.
- **Salvage above cost** is treated as the cost, so the value never changes.

## On the dashboard

The **Asset dashboard** shows **Total purchase cost**, the sum of all purchase
costs, and **Depreciated value**, the sum of the current book values of all
assets, archived ones included.
