---
title: Track and assign assets
summary: Find assets, edit them, assign and unassign them, read the history, and attach photos and documents.
kind: guide
order: 20
tags: [asset, assign, documents, photos, search]
requires: assets:read
---
Open **Assets** to see the register. The list is paged and sorted on the
server; choose a column header to sort the whole list. It refreshes by itself
when an asset is assigned or unassigned elsewhere.

## Find an asset

- **Search** matches the name, asset tag, serial and model.
- **Status** and **Category** narrow the list. Choose **Filter** (or press
  Enter in the search box) to apply.

Choose a row to open the asset. The page shows its status, **Details**
(serial, model, category, supplier, location, assignee, order, notes, tags),
**Depreciation**, the **Photo**, the **Assignment history** and the
**Documents**.

## Add and edit

Creating, editing and deleting assets needs `assets:manage`.

- Leave **Asset tag** blank to get a generated `AST-` tag. A tag you type must
  be unique in the tenant and contain no spaces (at most 64 characters).
  Editing with a blank tag keeps the current one.
- **Status** can be `deployable`, `broken` or `archived`. `assigned` is set
  only by **Assign**; editing an assigned asset keeps it assigned to the same
  user.
- **Category**, **Supplier** and **Location** must be records that already
  exist.
- **Depreciation rate** is a fraction between 0 and 1. Left blank or 0 it is
  `0.40`. See [the depreciation example](../examples/depreciation.md).

**Delete** (the bin icon) removes the asset with its photo, its documents and
its assignment history, and takes it off any insurance policy. It cannot be
undone.

## Assign and unassign

You need `assets:assign`.

- **Assign** is offered only for a `deployable` asset. Pick the **User**
  (users of your tenant) and an optional note. The asset becomes `assigned`
  and its location is cleared.
- **Unassign** is offered only for an `assigned` asset. Pick the location it
  returns to (optional) and a note. The asset becomes `deployable` again.

Trying to assign an asset that is not `deployable`, or to unassign one that is
not `assigned`, is refused with a conflict. Every assign and unassign adds a
row to the **Assignment history** (action, user, time, returned time, who did
it and the note), newest first.

## Photos

The **Photo** card takes one PNG, JPEG, GIF or WebP image. Choose a file and
**Upload**; uploading again replaces it; **Remove** deletes it. Changing the
photo needs `documents:manage`.

## Documents

Assets, consumables and licenses each have a document list. Choose a **File**,
add a **Description** if you like, and **Upload**. Choose a document to
download it. Uploading and deleting need `documents:manage`; files are limited
to 20 MiB unless your operator changed the limit.

When the paperless module is connected, documents are stored there and their
text is indexed. **Document search** then finds documents by their content
(an invoice number, a serial, a word in a warranty letter). Each hit shows
the file, the asset, consumable or license it belongs to, and a snippet.
Without paperless, search answers *temporarily unavailable*.

## Live updates

The lists and the asset page reconnect to the module's event stream while
they are open. The badge next to the title shows whether the live connection
is up.
