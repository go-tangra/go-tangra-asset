package documents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Paper is the paperless module as the documents service uses it (feature
// 030): documents live in paperless (full-text search), filed under
// "Assets/<asset tag>", "Assets/Consumables/<name>" or
// "Assets/Licenses/<name>" and tagged with the owning entity. The asset
// service owns what it creates, so its searches find them; paperless users
// see them through their access to the Assets category.
type Paper interface {
	EnsureCategory(ctx context.Context, tenantID string, path []string) (string, error)
	Create(ctx context.Context, tenantID string, d PaperDocument) (string, error)
	Download(ctx context.Context, tenantID, id string) ([]byte, error)
	Delete(ctx context.Context, tenantID, id string) error
	Search(ctx context.Context, tenantID, query string, limit int) ([]PaperHit, error)
}

// PaperDocument is a document created in paperless.
type PaperDocument struct {
	CategoryID, Name, Description, FileName, MimeType string
	Tags                                              map[string]string
	Content                                           []byte
}

// PaperHit is one paperless search result.
type PaperHit struct {
	ID      string
	Snippet string
	Rank    float64
}

// ErrPaperNotFound is returned by Paper for unknown documents.
var ErrPaperNotFound = errors.New("documents: not found in paperless")

// ErrSearchUnavailable is returned by Search without paperless.
var ErrSearchUnavailable = errors.New("documents: search needs the paperless module")

// RootCategory is the paperless category asset documents are filed under.
const RootCategory = "Assets"

// SetPaperless stores new documents in paperless from now on.
func (s *Service) SetPaperless(p Paper) { s.paper = p }

// categoryPath is where an entity's documents are filed in paperless.
func categoryPath(entityType, label string) []string {
	label = strings.TrimSpace(strings.ReplaceAll(label, "/", "-"))
	if label == "" {
		label = "unnamed"
	}
	switch entityType {
	case store.EntityConsumable:
		return []string{RootCategory, "Consumables", label}
	case store.EntityLicense:
		return []string{RootCategory, "Licenses", label}
	}
	return []string{RootCategory, label}
}

// entityLabel returns the name documents of an entity are filed under.
func (s *Service) entityLabel(ctx context.Context, tenantID, entityType, entityID string) (string, error) {
	switch entityType {
	case store.EntityAsset:
		a, err := s.st.GetAsset(ctx, tenantID, entityID)
		if err != nil {
			return "", mapErr(err)
		}
		if a.AssetTag != "" {
			return a.AssetTag, nil
		}
		return a.Name, nil
	case store.EntityConsumable:
		c, err := s.st.GetConsumable(ctx, tenantID, entityID)
		return c.Name, mapErr(err)
	case store.EntityLicense:
		l, err := s.st.GetLicense(ctx, tenantID, entityID)
		return l.Name, mapErr(err)
	}
	return "", ErrEntityType
}

func paperTags(entityType, entityID, label string) map[string]string {
	return map[string]string{"asset_entity_type": entityType, "asset_entity_id": entityID, "asset_label": label}
}

// toPaperless files content for an entity in paperless and returns the
// paperless document id.
func (s *Service) toPaperless(ctx context.Context, tenantID, entityType, entityID, fileName, mime, description string, content []byte) (string, error) {
	label, err := s.entityLabel(ctx, tenantID, entityType, entityID)
	if err != nil {
		return "", err
	}
	cat, err := s.paper.EnsureCategory(ctx, tenantID, categoryPath(entityType, label))
	if err != nil {
		return "", err
	}
	return s.paper.Create(ctx, tenantID, PaperDocument{CategoryID: cat, Name: fileName, Description: description, FileName: fileName,
		MimeType: mime, Tags: paperTags(entityType, entityID, label), Content: content})
}

// uploadToPaperless is Upload when paperless is attached.
func (s *Service) uploadToPaperless(ctx context.Context, subj authz.Subjects, entityType, entityID string, in UploadInput, fileName, mime string) (store.Document, error) {
	content, err := io.ReadAll(io.LimitReader(in.Reader, s.maxSize+1))
	if err != nil {
		return store.Document{}, err
	}
	if int64(len(content)) > s.maxSize {
		return store.Document{}, ErrTooLarge
	}
	pid, err := s.toPaperless(ctx, subj.TenantID, entityType, entityID, fileName, mime, in.Description, content)
	if err != nil {
		return store.Document{}, err
	}
	sum := sha256.Sum256(content)
	d := store.Document{ID: store.NewID(), TenantID: subj.TenantID, EntityType: entityType, EntityID: entityID, FileName: fileName,
		FileSize: int64(len(content)), MimeType: mime, StorageKey: store.PaperlessKeyPrefix + pid, PaperlessDocumentID: pid,
		Checksum: hex.EncodeToString(sum[:]), Description: in.Description, UploadedBy: subj.ActorID(), CreatedAt: s.now()}
	if err := s.st.InsertDocument(ctx, d); err != nil {
		_ = s.paper.Delete(ctx, subj.TenantID, pid)
		return store.Document{}, mapErr(err)
	}
	return d, nil
}

// removeObject deletes a document's bytes wherever they live.
func (s *Service) removeObject(ctx context.Context, tenantID string, d store.Document) error {
	if d.PaperlessDocumentID != "" {
		if s.paper == nil {
			return ErrSearchUnavailable
		}
		if err := s.paper.Delete(ctx, tenantID, d.PaperlessDocumentID); err != nil && !errors.Is(err, ErrPaperNotFound) {
			return err
		}
		return nil
	}
	if checkKey(tenantID, d.StorageKey) != nil {
		return ErrCrossTenant
	}
	return s.blobs.Delete(ctx, d.StorageKey)
}

// SearchHit is one document found by full-text search.
type SearchHit struct {
	Document store.Document `json:"document"`
	Snippet  string         `json:"snippet"`
	Rank     float64        `json:"rank"`
}

// Search finds the tenant's asset, consumable and license documents by their
// text (paperless full-text search). Hits that are not asset documents are
// dropped.
func (s *Service) Search(ctx context.Context, subj authz.Subjects, query string, limit int) ([]SearchHit, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 200 {
		return nil, ValidationError{"query must be 1 to 200 characters"}
	}
	if s.paper == nil {
		return nil, ErrSearchUnavailable
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	hits, err := s.paper.Search(ctx, subj.TenantID, query, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	rows, err := s.st.ListDocumentsByPaperlessIDs(ctx, subj.TenantID, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]store.Document, len(rows))
	for _, d := range rows {
		byID[d.PaperlessDocumentID] = d
	}
	out := []SearchHit{}
	for _, h := range hits {
		if d, ok := byID[h.ID]; ok {
			out = append(out, SearchHit{Document: d, Snippet: h.Snippet, Rank: h.Rank})
		}
	}
	return out, nil
}

// MigrateResult counts one migration pass.
type MigrateResult struct {
	Migrated int
	Failed   int
}

// Migrate moves up to limit documents from the asset object store into
// paperless: copy (verified against the stored checksum), record the
// paperless id, then delete the old object. A failed document stays where it
// is and is retried on the next pass; the pass audits one summary per tenant.
func (s *Service) Migrate(ctx context.Context, limit int) (MigrateResult, error) {
	var res MigrateResult
	if s.paper == nil {
		return res, ErrSearchUnavailable
	}
	rows, err := s.st.ListUnmigratedDocuments(ctx, limit)
	if err != nil {
		return res, err
	}
	perTenant := map[string]int{}
	for _, d := range rows {
		if err := s.migrateOne(ctx, d); err != nil {
			res.Failed++
			continue
		}
		res.Migrated++
		perTenant[d.TenantID]++
	}
	for tenant, n := range perTenant {
		audit.Emit(ctx, s.aud, audit.Event{TenantID: tenant, EventType: audit.DocumentsMigrated, ActorKind: authz.ActorSystem, ActorID: "migration",
			SubjectKind: audit.SubjectDocument, SubjectID: "paperless", Outcome: audit.OutcomeOK, Details: map[string]any{"documents": n}})
	}
	return res, nil
}

func (s *Service) migrateOne(ctx context.Context, d store.Document) error {
	if checkKey(d.TenantID, d.StorageKey) != nil {
		return ErrCrossTenant
	}
	rc, err := s.blobs.Get(ctx, d.StorageKey)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(io.LimitReader(rc, s.maxSize*4+1))
	_ = rc.Close()
	if err != nil {
		return err
	}
	if d.Checksum != "" {
		if sum := sha256.Sum256(content); hex.EncodeToString(sum[:]) != d.Checksum {
			return fmt.Errorf("documents: %s: checksum mismatch", d.ID)
		}
	}
	mime := d.MimeType
	if mime == "" {
		mime = "application/octet-stream"
	}
	pid, err := s.toPaperless(ctx, d.TenantID, d.EntityType, d.EntityID, d.FileName, mime, d.Description, content)
	if err != nil {
		return err
	}
	if err := s.st.MoveDocumentToPaperless(ctx, d.TenantID, d.ID, pid); err != nil {
		_ = s.paper.Delete(ctx, d.TenantID, pid)
		return err
	}
	_ = s.blobs.Delete(ctx, d.StorageKey)
	return nil
}

// paperReader wraps downloaded bytes.
func paperReader(b []byte) io.ReadCloser { return io.NopCloser(bytes.NewReader(b)) }
