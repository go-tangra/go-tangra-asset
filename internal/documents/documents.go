// Package documents is the polymorphic attachment service: an asset's photo and
// the documents (invoices, manuals, ...) attached to an asset, consumable or
// license. Bytes live only in object storage under tenant-prefixed keys
// (tenants/<tenant>/assets/<id>/photo.<ext>, tenants/<tenant>/documents/<docID>);
// the store keeps the metadata row with the SHA-256 checksum. Downloads stream
// through the service (or a short-lived presigned URL); the object-store
// credentials are never part of any response. The concrete store enforces per-
// tenant RLS; every object key is verified to carry the caller's tenant prefix
// before it is read or deleted.
package documents

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/blob"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Errors.
var (
	ErrNotFound    = errors.New("documents: not found")
	ErrTooLarge    = errors.New("documents: file exceeds the upload limit")
	ErrMediaType   = errors.New("documents: unsupported media type")
	ErrEntityType  = errors.New("documents: unsupported entity type")
	ErrCrossTenant = errors.New("documents: object key outside the caller's tenant")
)

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "documents: " + e.Msg }

// Service manages photos and documents.
type Service struct {
	st         repo.Store
	blobs      blob.Store
	aud        audit.Recorder
	maxSize    int64
	presignTTL time.Duration
	now        func() time.Time
}

// New builds the service. maxSize bounds uploads (bytes); presignTTL is the
// lifetime of a presigned download link.
func New(st repo.Store, blobs blob.Store, aud audit.Recorder, maxSize int64, presignTTL time.Duration) *Service {
	if maxSize <= 0 {
		maxSize = 20 << 20
	}
	if presignTTL <= 0 {
		presignTTL = 5 * time.Minute
	}
	return &Service{st: st, blobs: blobs, aud: aud, maxSize: maxSize, presignTTL: presignTTL, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// MaxSize is the configured upload bound.
func (s *Service) MaxSize() int64 { return s.maxSize }

// UploadInput describes an inbound file.
type UploadInput struct {
	FileName    string
	MimeType    string
	Size        int64
	Reader      io.Reader
	Description string
}

func mapErr(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// tenantPrefix is the object-key prefix every object of a tenant lives under.
func tenantPrefix(tenantID string) string { return "tenants/" + tenantID + "/" }

// PhotoKey is the object key of an asset's photo.
func PhotoKey(tenantID, assetID, ext string) string {
	return tenantPrefix(tenantID) + "assets/" + assetID + "/photo" + ext
}

// DocumentKey is the object key of a document.
func DocumentKey(tenantID, docID string) string { return tenantPrefix(tenantID) + "documents/" + docID }

func checkKey(tenantID, key string) error {
	if !strings.HasPrefix(key, tenantPrefix(tenantID)) || strings.Contains(key, "..") {
		return ErrCrossTenant
	}
	return nil
}

var photoExt = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}

// PhotoMime derives the content type of a stored photo from its key.
func PhotoMime(key string) string {
	switch path.Ext(key) {
	case ".png":
		return "image/png"
	case ".jpg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

// entityExists verifies the owning record is in the caller's tenant.
func (s *Service) entityExists(ctx context.Context, tenantID, entityType, entityID string) error {
	var err error
	switch entityType {
	case store.EntityAsset:
		_, err = s.st.GetAsset(ctx, tenantID, entityID)
	case store.EntityConsumable:
		_, err = s.st.GetConsumable(ctx, tenantID, entityID)
	case store.EntityLicense:
		_, err = s.st.GetLicense(ctx, tenantID, entityID)
	default:
		return ErrEntityType
	}
	return mapErr(err)
}

func (s *Service) emit(ctx context.Context, subj authz.Subjects, typ audit.EventType, id string, details map[string]any) {
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: typ, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectDocument, SubjectID: id, Outcome: audit.OutcomeOK, Details: details})
}

// UploadPhoto stores (or replaces) an asset's photo and returns the updated asset.
func (s *Service) UploadPhoto(ctx context.Context, subj authz.Subjects, assetID string, in UploadInput) (store.Asset, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Asset{}, err
	}
	ext, ok := photoExt[strings.ToLower(strings.TrimSpace(strings.Split(in.MimeType, ";")[0]))]
	if !ok {
		return store.Asset{}, ErrMediaType
	}
	if in.Size > s.maxSize {
		return store.Asset{}, ErrTooLarge
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, assetID)
	if err != nil {
		return store.Asset{}, mapErr(err)
	}
	key := PhotoKey(subj.TenantID, assetID, ext)
	if _, err := s.blobs.Put(ctx, key, io.LimitReader(in.Reader, s.maxSize+1), in.Size, in.MimeType); err != nil {
		return store.Asset{}, err
	}
	if a.PhotoKey != "" && a.PhotoKey != key {
		_ = s.blobs.Delete(ctx, a.PhotoKey)
	}
	a.PhotoKey, a.UpdatedBy, a.UpdatedAt = key, subj.ActorID(), s.now()
	if err := s.st.UpdateAsset(ctx, a); err != nil {
		_ = s.blobs.Delete(ctx, key)
		return store.Asset{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.PhotoUploaded, assetID, map[string]any{"asset_id": assetID})
	return a, nil
}

// GetPhoto streams an asset's photo with its content type.
func (s *Service) GetPhoto(ctx context.Context, subj authz.Subjects, assetID string) (io.ReadCloser, string, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, "", err
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, assetID)
	if err != nil {
		return nil, "", mapErr(err)
	}
	if a.PhotoKey == "" {
		return nil, "", ErrNotFound
	}
	if err := checkKey(subj.TenantID, a.PhotoKey); err != nil {
		return nil, "", err
	}
	rc, err := s.blobs.Get(ctx, a.PhotoKey)
	if err != nil {
		return nil, "", ErrNotFound
	}
	return rc, PhotoMime(a.PhotoKey), nil
}

// DeletePhoto removes an asset's photo object and clears the reference.
func (s *Service) DeletePhoto(ctx context.Context, subj authz.Subjects, assetID string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, assetID)
	if err != nil {
		return mapErr(err)
	}
	if a.PhotoKey == "" {
		return ErrNotFound
	}
	if err := checkKey(subj.TenantID, a.PhotoKey); err != nil {
		return err
	}
	if err := s.blobs.Delete(ctx, a.PhotoKey); err != nil {
		return err
	}
	a.PhotoKey, a.UpdatedBy, a.UpdatedAt = "", subj.ActorID(), s.now()
	if err := s.st.UpdateAsset(ctx, a); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.PhotoDeleted, assetID, map[string]any{"asset_id": assetID})
	return nil
}

// Upload attaches a document to an asset, consumable or license.
func (s *Service) Upload(ctx context.Context, subj authz.Subjects, entityType, entityID string, in UploadInput) (store.Document, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Document{}, err
	}
	if strings.TrimSpace(in.FileName) == "" || len(in.FileName) > 255 {
		return store.Document{}, ValidationError{"file name is required (max 255)"}
	}
	if in.Size > s.maxSize {
		return store.Document{}, ErrTooLarge
	}
	if err := s.entityExists(ctx, subj.TenantID, entityType, entityID); err != nil {
		return store.Document{}, err
	}
	id := store.NewID()
	key := DocumentKey(subj.TenantID, id)
	mime := strings.TrimSpace(in.MimeType)
	if mime == "" {
		mime = "application/octet-stream"
	}
	sum, err := s.blobs.Put(ctx, key, io.LimitReader(in.Reader, s.maxSize+1), in.Size, mime)
	if err != nil {
		return store.Document{}, err
	}
	d := store.Document{ID: id, TenantID: subj.TenantID, EntityType: entityType, EntityID: entityID, FileName: path.Base(strings.TrimSpace(in.FileName)),
		FileSize: in.Size, MimeType: mime, StorageKey: key, Checksum: sum, Description: in.Description, UploadedBy: subj.ActorID(), CreatedAt: s.now()}
	if err := s.st.InsertDocument(ctx, d); err != nil {
		_ = s.blobs.Delete(ctx, key)
		return store.Document{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.DocumentUploaded, d.ID, map[string]any{"entity_type": entityType, "entity_id": entityID, "file_size": in.Size})
	return d, nil
}

// List returns the documents attached to an entity.
func (s *Service) List(ctx context.Context, subj authz.Subjects, entityType, entityID string) ([]store.Document, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	if err := s.entityExists(ctx, subj.TenantID, entityType, entityID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListDocuments(ctx, subj.TenantID, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []store.Document{}
	}
	return rows, nil
}

// get loads a document and verifies it belongs to the entity and the tenant's key space.
func (s *Service) get(ctx context.Context, subj authz.Subjects, entityType, entityID, docID string) (store.Document, error) {
	d, err := s.st.GetDocument(ctx, subj.TenantID, docID)
	if err != nil {
		return store.Document{}, mapErr(err)
	}
	if d.EntityType != entityType || d.EntityID != entityID {
		return store.Document{}, ErrNotFound
	}
	if err := checkKey(subj.TenantID, d.StorageKey); err != nil {
		return store.Document{}, err
	}
	return d, nil
}

// Download streams a document's bytes with its metadata.
func (s *Service) Download(ctx context.Context, subj authz.Subjects, entityType, entityID, docID string) (io.ReadCloser, store.Document, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, store.Document{}, err
	}
	d, err := s.get(ctx, subj, entityType, entityID, docID)
	if err != nil {
		return nil, store.Document{}, err
	}
	rc, err := s.blobs.Get(ctx, d.StorageKey)
	if err != nil {
		return nil, store.Document{}, ErrNotFound
	}
	return rc, d, nil
}

// DownloadURL returns a short-lived presigned link to a document.
func (s *Service) DownloadURL(ctx context.Context, subj authz.Subjects, entityType, entityID, docID string) (string, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return "", err
	}
	d, err := s.get(ctx, subj, entityType, entityID, docID)
	if err != nil {
		return "", err
	}
	return s.blobs.PresignGet(ctx, d.StorageKey, s.presignTTL)
}

// Delete removes a document row and its object.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, entityType, entityID, docID string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	d, err := s.get(ctx, subj, entityType, entityID, docID)
	if err != nil {
		return err
	}
	if err := s.blobs.Delete(ctx, d.StorageKey); err != nil {
		return err
	}
	if err := s.st.DeleteDocument(ctx, subj.TenantID, docID); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.DocumentDeleted, docID, map[string]any{"entity_type": entityType, "entity_id": entityID})
	return nil
}

// Purge removes every document (rows + objects) of an entity; used when the
// owning consumable/license is deleted. Errors on individual objects are
// tolerated so a stale object never blocks the delete.
func (s *Service) Purge(ctx context.Context, tenantID, entityType, entityID string) {
	rows, err := s.st.ListDocuments(ctx, tenantID, entityType, entityID)
	if err != nil {
		return
	}
	s.PurgeRows(ctx, tenantID, rows)
}

// PurgeRows removes the given document rows and their objects (the rows were
// listed before the owning entity was deleted, so a store cascade cannot hide
// them). Keys outside the tenant's prefix are never touched.
func (s *Service) PurgeRows(ctx context.Context, tenantID string, rows []store.Document) {
	for _, d := range rows {
		if d.TenantID != tenantID {
			continue
		}
		if checkKey(tenantID, d.StorageKey) == nil {
			_ = s.blobs.Delete(ctx, d.StorageKey)
		}
		_ = s.st.DeleteDocument(ctx, tenantID, d.ID)
	}
}
