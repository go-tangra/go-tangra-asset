---
title: Get started
summary: Create a category and a location, add your first asset and assign it to a colleague.
kind: guide
order: 10
tags: [asset, getting-started]
requires: assets:manage
---
This walk-through files one laptop and hands it to a user. You need
`categories:manage`, `locations:manage`, `assets:manage` and `assets:assign`.
The *Assets administrator* role has all of them; the *Assets editor* role can
do steps 3 and 4 once the category and location exist.

## 1. Create a category

1. Open **Categories** and choose **New category**.
2. Enter the name `Laptops` and choose **Save**.

To make a sub-category later, select `Laptops` and choose **Add child**.

## 2. Create a location

1. Open **Locations** and choose **New location**.
2. Enter the name `HQ store room`, leave **Status** at `active` and choose
   **Save**.

## 3. Add the asset

1. Open **Assets** and choose **New asset**.
2. Fill in:
   - **Name**: `Developer laptop`
   - **Asset tag**: leave it blank. The module generates one, such as
     `AST-3F09C2`.
   - **Serial**: the serial number from the device
   - **Category**: `Laptops`, **Location**: `HQ store room`
   - **Purchase date**, **Purchase cost**, **Warranty (months)** and
     **Useful life (years)**, if you know them
3. Choose **Save**. The asset appears in the list with status `deployable`
   and a **Book value**.

## 4. Assign it

1. Choose the asset in the list to open it.
2. Choose **Assign**, pick the **User** and add a note if you like, for
   example `New starter`.
3. Choose **Assign**. The status changes to `assigned`, the location is
   cleared, and the **Assignment history** shows the new entry with who
   assigned it.

When the laptop comes back, open it, choose **Unassign** and pick the
location it returns to.

## Next steps

- Attach the invoice and a photo: [track and assign assets](guides/assets.md).
- Record the supplier and the warranty: [records](guides/records.md).
- Import the machines inventory already knows: [inventory sync](guides/inventory-sync.md).
