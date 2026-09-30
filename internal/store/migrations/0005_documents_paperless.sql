-- +goose Up
-- Documents held by the paperless module (feature 030): the asset row keeps
-- the metadata and the paperless document id; storage_key is
-- "paperless:<id>" for those (still unique).
ALTER TABLE asset_documents ADD COLUMN paperless_document_id text NOT NULL DEFAULT '';
CREATE INDEX documents_paperless ON asset_documents (tenant_id, paperless_document_id) WHERE paperless_document_id <> '';

-- +goose Down
DROP INDEX IF EXISTS documents_paperless;
ALTER TABLE asset_documents DROP COLUMN IF EXISTS paperless_document_id;
