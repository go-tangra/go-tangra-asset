package memstore

import (
	"context"
	"sort"
	"strings"

	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// ListDocumentsByPaperlessIDs implements repo.Store.
func (m *Mem) ListDocumentsByPaperlessIDs(_ context.Context, tenantID string, ids []string) ([]store.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListDocumentsByPaperlessIDs"); err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []store.Document
	for _, d := range m.documents {
		if d.TenantID == tenantID && d.PaperlessDocumentID != "" && want[d.PaperlessDocumentID] {
			out = append(out, d)
		}
	}
	return out, nil
}

// ListUnmigratedDocuments implements repo.Store.
func (m *Mem) ListUnmigratedDocuments(_ context.Context, limit int) ([]store.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListUnmigratedDocuments"); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var out []store.Document
	for _, d := range m.documents {
		if d.PaperlessDocumentID == "" && strings.HasPrefix(d.StorageKey, "tenants/") {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// MoveDocumentToPaperless implements repo.Store.
func (m *Mem) MoveDocumentToPaperless(_ context.Context, tenantID, id, paperlessID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("MoveDocumentToPaperless"); err != nil {
		return err
	}
	d, ok := m.documents[id]
	if !ok || d.TenantID != tenantID {
		return repo.ErrNotFound
	}
	d.PaperlessDocumentID, d.StorageKey = paperlessID, store.PaperlessKeyPrefix+paperlessID
	m.documents[id] = d
	return nil
}
