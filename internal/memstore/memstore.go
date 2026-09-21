// Package memstore is an in-memory repo.Store for the asset (ITAM) service, used
// by tests and local dev. It filters by tenant (mirroring RLS), implements the
// full asset surface (assets with the duplicate asset-tag guard + find-by-tag/
// serial, assignment history with active-close, polymorphic documents with the
// unique storage-key guard, the category + location trees with computed child/
// asset counts and delete guards, suppliers, consumables, licenses, insurance
// policies + the policy-asset M2M with its duplicate-link guard, scheduler notify
// state, per-tenant statistics and audit), fills the computed fields on Get/List,
// and offers per-method error injection via FailNext.
package memstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-freya/freya/services/asset/internal/deprec"
	"github.com/go-freya/freya/services/asset/internal/repo"
	"github.com/go-freya/freya/services/asset/internal/store"
)

// injectedErr is the error FailNext arms for a given method.
type injectedErr struct{ method string }

func (e injectedErr) Error() string { return "memstore: injected failure in " + e.method }

// Mem is an in-memory store. It is safe for concurrent use.
type Mem struct {
	mu sync.Mutex

	assets       map[string]store.Asset
	assignments  map[string]store.Assignment
	documents    map[string]store.Document
	categories   map[string]store.Category
	suppliers    map[string]store.Supplier
	locations    map[string]store.Location
	consumables  map[string]store.Consumable
	licenses     map[string]store.License
	insurance    map[string]store.InsurancePolicy
	policyAssets map[string]store.PolicyAsset // keyed by id
	notify       map[string]store.NotifyState // keyed by id
	audit        []store.AuditRow

	failNext map[string]bool
	Now      func() time.Time
}

// New builds an empty store.
func New() *Mem {
	return &Mem{
		assets:       map[string]store.Asset{},
		assignments:  map[string]store.Assignment{},
		documents:    map[string]store.Document{},
		categories:   map[string]store.Category{},
		suppliers:    map[string]store.Supplier{},
		locations:    map[string]store.Location{},
		consumables:  map[string]store.Consumable{},
		licenses:     map[string]store.License{},
		insurance:    map[string]store.InsurancePolicy{},
		policyAssets: map[string]store.PolicyAsset{},
		notify:       map[string]store.NotifyState{},
		failNext:     map[string]bool{},
		Now:          func() time.Time { return time.Now().UTC() },
	}
}

// FailNext arms the next call to the named method to return an injected error.
func (m *Mem) FailNext(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext[method] = true
}

// fail reports (and disarms) an injected failure for method, if armed.
func (m *Mem) fail(method string) error {
	if m.failNext[method] {
		delete(m.failNext, method)
		return injectedErr{method}
	}
	return nil
}

// Close is a no-op for the in-memory store.
func (m *Mem) Close() {}

func (m *Mem) now() time.Time { return m.Now() }

// paginate sorts items newest-first by id (uuid v7 ids are time-ordered), applies
// the keyset cursor (last id of the previous page) and the limit.
func paginate[T any](items []T, id func(T) string, cursor string, limit int) []T {
	sort.Slice(items, func(i, j int) bool { return id(items[i]) > id(items[j]) })
	if cursor != "" {
		filtered := items[:0]
		for _, it := range items {
			if id(it) < cursor {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

// bookValue is the current double-declining-balance depreciated value (the
// deprec library), floored at the salvage value. It is computed, never stored.
func bookValue(a store.Asset, at time.Time) float64 {
	pd := time.Time{}
	if a.PurchaseDate != nil {
		pd = *a.PurchaseDate
	}
	return deprec.BookValue(a.PurchaseCost, a.SalvageValue, a.UsefulLifeYears, a.DepreciationRate, pd, at)
}

func autoTag() string { return "AST-" + strings.ToUpper(store.NewID()[:6]) }

// ---- assets

func (m *Mem) fillAsset(a *store.Asset) { a.BookValue = bookValue(*a, m.now()) }

func (m *Mem) CreateAsset(_ context.Context, a store.Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateAsset"); err != nil {
		return err
	}
	if a.AssetTag == "" {
		a.AssetTag = autoTag()
	}
	for _, ex := range m.assets {
		if ex.TenantID == a.TenantID && ex.AssetTag == a.AssetTag {
			return repo.ErrConflict
		}
	}
	if a.ID == "" {
		a.ID = store.NewID()
	}
	if _, dup := m.assets[a.ID]; dup {
		return repo.ErrConflict
	}
	if a.Status == "" {
		a.Status = store.AssetDeployable
	}
	if a.DepreciationRate == 0 {
		a.DepreciationRate = 0.40
	}
	t := m.now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = t
	}
	a.UpdatedAt = t
	m.assets[a.ID] = a
	return nil
}

func (m *Mem) GetAsset(_ context.Context, tenantID, id string) (store.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetAsset"); err != nil {
		return store.Asset{}, err
	}
	a, ok := m.assets[id]
	if !ok || a.TenantID != tenantID {
		return store.Asset{}, repo.ErrNotFound
	}
	m.fillAsset(&a)
	return a, nil
}

func (m *Mem) FindAssetByTag(_ context.Context, tenantID, tag string) (store.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("FindAssetByTag"); err != nil {
		return store.Asset{}, err
	}
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.AssetTag == tag {
			m.fillAsset(&a)
			return a, nil
		}
	}
	return store.Asset{}, repo.ErrNotFound
}

func (m *Mem) FindAssetBySerial(_ context.Context, tenantID, serial string) (store.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("FindAssetBySerial"); err != nil {
		return store.Asset{}, err
	}
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.Serial != "" && a.Serial == serial {
			m.fillAsset(&a)
			return a, nil
		}
	}
	return store.Asset{}, repo.ErrNotFound
}

func (m *Mem) ListAssets(_ context.Context, tenantID string, f store.AssetFilter) ([]store.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListAssets"); err != nil {
		return nil, err
	}
	var out []store.Asset
	for _, a := range m.assets {
		if a.TenantID != tenantID {
			continue
		}
		if f.Status != "" && a.Status != f.Status {
			continue
		}
		if f.CategoryID != "" && a.CategoryID != f.CategoryID {
			continue
		}
		if f.SupplierID != "" && a.SupplierID != f.SupplierID {
			continue
		}
		if f.LocationID != "" && a.LocationID != f.LocationID {
			continue
		}
		if f.UserID != "" && a.UserID != f.UserID {
			continue
		}
		if f.Query != "" {
			q := strings.ToLower(f.Query)
			if !strings.Contains(strings.ToLower(a.AssetTag), q) &&
				!strings.Contains(strings.ToLower(a.Name), q) &&
				!strings.Contains(strings.ToLower(a.Serial), q) &&
				!strings.Contains(strings.ToLower(a.ModelName), q) {
				continue
			}
		}
		out = append(out, a)
	}
	out = paginate(out, func(a store.Asset) string { return a.ID }, f.CursorID, f.Limit)
	for i := range out {
		m.fillAsset(&out[i])
	}
	return out, nil
}

func (m *Mem) UpdateAsset(_ context.Context, a store.Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateAsset"); err != nil {
		return err
	}
	ex, ok := m.assets[a.ID]
	if !ok || ex.TenantID != a.TenantID {
		return repo.ErrNotFound
	}
	if a.AssetTag == "" {
		a.AssetTag = ex.AssetTag
	}
	for _, o := range m.assets {
		if o.ID != a.ID && o.TenantID == a.TenantID && o.AssetTag == a.AssetTag {
			return repo.ErrConflict
		}
	}
	if a.DepreciationRate == 0 {
		a.DepreciationRate = 0.40
	}
	a.CreatedAt = ex.CreatedAt
	a.UpdatedAt = m.now()
	m.assets[a.ID] = a
	return nil
}

func (m *Mem) DeleteAsset(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteAsset"); err != nil {
		return err
	}
	a, ok := m.assets[id]
	if !ok || a.TenantID != tenantID {
		return repo.ErrNotFound
	}
	// Cascade assignments, polymorphic documents, and policy links.
	for aid, as := range m.assignments {
		if as.TenantID == tenantID && as.AssetID == id {
			delete(m.assignments, aid)
		}
	}
	for did, d := range m.documents {
		if d.TenantID == tenantID && d.EntityType == store.EntityAsset && d.EntityID == id {
			delete(m.documents, did)
		}
	}
	for pid, pa := range m.policyAssets {
		if pa.TenantID == tenantID && pa.AssetID == id {
			delete(m.policyAssets, pid)
		}
	}
	delete(m.assets, id)
	return nil
}

func (m *Mem) CountAssetsByCategory(_ context.Context, tenantID, categoryID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CountAssetsByCategory"); err != nil {
		return 0, err
	}
	var n int64
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.CategoryID == categoryID {
			n++
		}
	}
	return n, nil
}

func (m *Mem) CountAssetsBySupplier(_ context.Context, tenantID, supplierID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CountAssetsBySupplier"); err != nil {
		return 0, err
	}
	var n int64
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.SupplierID == supplierID {
			n++
		}
	}
	return n, nil
}

func (m *Mem) CountAssetsByLocation(_ context.Context, tenantID, locationID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CountAssetsByLocation"); err != nil {
		return 0, err
	}
	var n int64
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.LocationID == locationID {
			n++
		}
	}
	return n, nil
}

func (m *Mem) AllAssets(_ context.Context, tenantID string) ([]store.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("AllAssets"); err != nil {
		return nil, err
	}
	var out []store.Asset
	for _, a := range m.assets {
		if a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	out = paginate(out, func(a store.Asset) string { return a.ID }, "", 0)
	for i := range out {
		m.fillAsset(&out[i])
	}
	return out, nil
}

// ---- assignments

func (m *Mem) InsertAssignment(_ context.Context, a store.Assignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("InsertAssignment"); err != nil {
		return err
	}
	if a.ID == "" {
		a.ID = store.NewID()
	}
	if _, dup := m.assignments[a.ID]; dup {
		return repo.ErrConflict
	}
	if a.Action == "" {
		a.Action = store.ActionAssigned
	}
	if a.AssignedAt.IsZero() {
		a.AssignedAt = m.now()
	}
	m.assignments[a.ID] = a
	return nil
}

func (m *Mem) CloseActiveAssignment(_ context.Context, tenantID, assetID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CloseActiveAssignment"); err != nil {
		return err
	}
	if at.IsZero() {
		at = m.now()
	}
	for id, a := range m.assignments {
		if a.TenantID == tenantID && a.AssetID == assetID && a.ReturnedAt == nil {
			t := at
			a.ReturnedAt = &t
			m.assignments[id] = a
		}
	}
	return nil
}

func (m *Mem) ListAssignments(_ context.Context, tenantID, assetID string, limit int) ([]store.Assignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListAssignments"); err != nil {
		return nil, err
	}
	var out []store.Assignment
	for _, a := range m.assignments {
		if a.TenantID == tenantID && a.AssetID == assetID {
			out = append(out, a)
		}
	}
	// Newest first: by assignment time, then id (creation order).
	sort.Slice(out, func(i, j int) bool {
		if !out[i].AssignedAt.Equal(out[j].AssignedAt) {
			return out[i].AssignedAt.After(out[j].AssignedAt)
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---- documents (polymorphic)

func (m *Mem) InsertDocument(_ context.Context, d store.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("InsertDocument"); err != nil {
		return err
	}
	for _, ex := range m.documents {
		if ex.StorageKey == d.StorageKey {
			return repo.ErrConflict
		}
	}
	if d.ID == "" {
		d.ID = store.NewID()
	}
	if _, dup := m.documents[d.ID]; dup {
		return repo.ErrConflict
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = m.now()
	}
	m.documents[d.ID] = d
	return nil
}

func (m *Mem) GetDocument(_ context.Context, tenantID, id string) (store.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetDocument"); err != nil {
		return store.Document{}, err
	}
	d, ok := m.documents[id]
	if !ok || d.TenantID != tenantID {
		return store.Document{}, repo.ErrNotFound
	}
	return d, nil
}

func (m *Mem) ListDocuments(_ context.Context, tenantID, entityType, entityID string) ([]store.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListDocuments"); err != nil {
		return nil, err
	}
	var out []store.Document
	for _, d := range m.documents {
		if d.TenantID == tenantID && d.EntityType == entityType && d.EntityID == entityID {
			out = append(out, d)
		}
	}
	out = paginate(out, func(d store.Document) string { return d.ID }, "", 0)
	return out, nil
}

func (m *Mem) DeleteDocument(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteDocument"); err != nil {
		return err
	}
	d, ok := m.documents[id]
	if !ok || d.TenantID != tenantID {
		return repo.ErrNotFound
	}
	delete(m.documents, id)
	return nil
}

// ---- categories

func (m *Mem) fillCategory(c *store.Category) {
	var assets, children int64
	for _, a := range m.assets {
		if a.TenantID == c.TenantID && a.CategoryID == c.ID {
			assets++
		}
	}
	for _, o := range m.categories {
		if o.TenantID == c.TenantID && o.ParentID == c.ID {
			children++
		}
	}
	c.AssetCount = assets
	c.ChildCount = children
}

func (m *Mem) CreateCategory(_ context.Context, c store.Category) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateCategory"); err != nil {
		return err
	}
	for _, ex := range m.categories {
		if ex.TenantID == c.TenantID && ex.Name == c.Name && ex.ParentID == c.ParentID {
			return repo.ErrConflict
		}
	}
	if c.ID == "" {
		c.ID = store.NewID()
	}
	if _, dup := m.categories[c.ID]; dup {
		return repo.ErrConflict
	}
	t := m.now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = t
	}
	c.UpdatedAt = t
	m.categories[c.ID] = c
	return nil
}

func (m *Mem) GetCategory(_ context.Context, tenantID, id string) (store.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetCategory"); err != nil {
		return store.Category{}, err
	}
	c, ok := m.categories[id]
	if !ok || c.TenantID != tenantID {
		return store.Category{}, repo.ErrNotFound
	}
	m.fillCategory(&c)
	return c, nil
}

func (m *Mem) ListCategories(_ context.Context, tenantID string) ([]store.Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListCategories"); err != nil {
		return nil, err
	}
	var out []store.Category
	for _, c := range m.categories {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	out = paginate(out, func(c store.Category) string { return c.ID }, "", 0)
	for i := range out {
		m.fillCategory(&out[i])
	}
	return out, nil
}

func (m *Mem) UpdateCategory(_ context.Context, c store.Category) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateCategory"); err != nil {
		return err
	}
	ex, ok := m.categories[c.ID]
	if !ok || ex.TenantID != c.TenantID {
		return repo.ErrNotFound
	}
	for _, o := range m.categories {
		if o.ID != c.ID && o.TenantID == c.TenantID && o.Name == c.Name && o.ParentID == c.ParentID {
			return repo.ErrConflict
		}
	}
	c.CreatedAt = ex.CreatedAt
	c.UpdatedAt = m.now()
	m.categories[c.ID] = c
	return nil
}

func (m *Mem) DeleteCategory(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteCategory"); err != nil {
		return err
	}
	c, ok := m.categories[id]
	if !ok || c.TenantID != tenantID {
		return repo.ErrNotFound
	}
	for _, o := range m.categories {
		if o.TenantID == tenantID && o.ParentID == id {
			return repo.ErrNotEmpty
		}
	}
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.CategoryID == id {
			return repo.ErrNotEmpty
		}
	}
	// Consumable references are SET NULL by their FK on delete.
	for cid, cn := range m.consumables {
		if cn.TenantID == tenantID && cn.CategoryID == id {
			cn.CategoryID = ""
			m.consumables[cid] = cn
		}
	}
	delete(m.categories, id)
	return nil
}

func (m *Mem) CountChildCategories(_ context.Context, tenantID, id string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CountChildCategories"); err != nil {
		return 0, err
	}
	var n int64
	for _, c := range m.categories {
		if c.TenantID == tenantID && c.ParentID == id {
			n++
		}
	}
	return n, nil
}

// ---- suppliers

func (m *Mem) CreateSupplier(_ context.Context, s store.Supplier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateSupplier"); err != nil {
		return err
	}
	for _, ex := range m.suppliers {
		if ex.TenantID == s.TenantID && ex.Name == s.Name {
			return repo.ErrConflict
		}
	}
	if s.ID == "" {
		s.ID = store.NewID()
	}
	if _, dup := m.suppliers[s.ID]; dup {
		return repo.ErrConflict
	}
	if s.Status == "" {
		s.Status = store.SupActive
	}
	t := m.now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = t
	}
	s.UpdatedAt = t
	m.suppliers[s.ID] = s
	return nil
}

func (m *Mem) GetSupplier(_ context.Context, tenantID, id string) (store.Supplier, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetSupplier"); err != nil {
		return store.Supplier{}, err
	}
	s, ok := m.suppliers[id]
	if !ok || s.TenantID != tenantID {
		return store.Supplier{}, repo.ErrNotFound
	}
	return s, nil
}

func (m *Mem) ListSuppliers(_ context.Context, tenantID string, f store.ListOpts) ([]store.Supplier, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListSuppliers"); err != nil {
		return nil, err
	}
	var out []store.Supplier
	for _, s := range m.suppliers {
		if s.TenantID != tenantID {
			continue
		}
		if f.Query != "" {
			q := strings.ToLower(f.Query)
			if !strings.Contains(strings.ToLower(s.Name), q) && !strings.Contains(strings.ToLower(s.Code), q) {
				continue
			}
		}
		out = append(out, s)
	}
	out = paginate(out, func(s store.Supplier) string { return s.ID }, f.CursorID, f.Limit)
	return out, nil
}

func (m *Mem) UpdateSupplier(_ context.Context, s store.Supplier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateSupplier"); err != nil {
		return err
	}
	ex, ok := m.suppliers[s.ID]
	if !ok || ex.TenantID != s.TenantID {
		return repo.ErrNotFound
	}
	for _, o := range m.suppliers {
		if o.ID != s.ID && o.TenantID == s.TenantID && o.Name == s.Name {
			return repo.ErrConflict
		}
	}
	s.CreatedAt = ex.CreatedAt
	s.UpdatedAt = m.now()
	m.suppliers[s.ID] = s
	return nil
}

func (m *Mem) DeleteSupplier(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteSupplier"); err != nil {
		return err
	}
	s, ok := m.suppliers[id]
	if !ok || s.TenantID != tenantID {
		return repo.ErrNotFound
	}
	// References are SET NULL by their FKs on delete.
	for aid, a := range m.assets {
		if a.TenantID == tenantID && a.SupplierID == id {
			a.SupplierID = ""
			m.assets[aid] = a
		}
	}
	for cid, c := range m.consumables {
		if c.TenantID == tenantID && c.SupplierID == id {
			c.SupplierID = ""
			m.consumables[cid] = c
		}
	}
	for lid, l := range m.licenses {
		if l.TenantID == tenantID && l.SupplierID == id {
			l.SupplierID = ""
			m.licenses[lid] = l
		}
	}
	delete(m.suppliers, id)
	return nil
}

// ---- locations

func (m *Mem) fillLocation(l *store.Location) {
	var child, assets int64
	for _, o := range m.locations {
		if o.TenantID == l.TenantID && o.ParentID == l.ID {
			child++
		}
	}
	for _, a := range m.assets {
		if a.TenantID == l.TenantID && a.LocationID == l.ID {
			assets++
		}
	}
	l.ChildCount = child
	l.AssetCount = assets
}

func (m *Mem) CreateLocation(_ context.Context, l store.Location) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateLocation"); err != nil {
		return err
	}
	for _, ex := range m.locations {
		if ex.TenantID == l.TenantID && ex.Name == l.Name {
			return repo.ErrConflict
		}
	}
	if l.ID == "" {
		l.ID = store.NewID()
	}
	if _, dup := m.locations[l.ID]; dup {
		return repo.ErrConflict
	}
	if l.Status == "" {
		l.Status = store.LocActive
	}
	t := m.now()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = t
	}
	l.UpdatedAt = t
	m.locations[l.ID] = l
	return nil
}

func (m *Mem) GetLocation(_ context.Context, tenantID, id string) (store.Location, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetLocation"); err != nil {
		return store.Location{}, err
	}
	l, ok := m.locations[id]
	if !ok || l.TenantID != tenantID {
		return store.Location{}, repo.ErrNotFound
	}
	m.fillLocation(&l)
	return l, nil
}

func (m *Mem) ListLocations(_ context.Context, tenantID string) ([]store.Location, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListLocations"); err != nil {
		return nil, err
	}
	var out []store.Location
	for _, l := range m.locations {
		if l.TenantID == tenantID {
			out = append(out, l)
		}
	}
	out = paginate(out, func(l store.Location) string { return l.ID }, "", 0)
	for i := range out {
		m.fillLocation(&out[i])
	}
	return out, nil
}

func (m *Mem) UpdateLocation(_ context.Context, l store.Location) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateLocation"); err != nil {
		return err
	}
	ex, ok := m.locations[l.ID]
	if !ok || ex.TenantID != l.TenantID {
		return repo.ErrNotFound
	}
	for _, o := range m.locations {
		if o.ID != l.ID && o.TenantID == l.TenantID && o.Name == l.Name {
			return repo.ErrConflict
		}
	}
	l.CreatedAt = ex.CreatedAt
	l.UpdatedAt = m.now()
	m.locations[l.ID] = l
	return nil
}

func (m *Mem) DeleteLocation(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteLocation"); err != nil {
		return err
	}
	l, ok := m.locations[id]
	if !ok || l.TenantID != tenantID {
		return repo.ErrNotFound
	}
	for _, o := range m.locations {
		if o.TenantID == tenantID && o.ParentID == id {
			return repo.ErrNotEmpty
		}
	}
	for _, a := range m.assets {
		if a.TenantID == tenantID && a.LocationID == id {
			return repo.ErrNotEmpty
		}
	}
	// Consumable references are SET NULL by their FK on delete.
	for cid, c := range m.consumables {
		if c.TenantID == tenantID && c.LocationID == id {
			c.LocationID = ""
			m.consumables[cid] = c
		}
	}
	delete(m.locations, id)
	return nil
}

func (m *Mem) CountChildLocations(_ context.Context, tenantID, id string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CountChildLocations"); err != nil {
		return 0, err
	}
	var n int64
	for _, l := range m.locations {
		if l.TenantID == tenantID && l.ParentID == id {
			n++
		}
	}
	return n, nil
}

// ---- consumables

func (m *Mem) CreateConsumable(_ context.Context, c store.Consumable) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateConsumable"); err != nil {
		return err
	}
	if c.ID == "" {
		c.ID = store.NewID()
	}
	if _, dup := m.consumables[c.ID]; dup {
		return repo.ErrConflict
	}
	t := m.now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = t
	}
	c.UpdatedAt = t
	m.consumables[c.ID] = c
	return nil
}

func (m *Mem) GetConsumable(_ context.Context, tenantID, id string) (store.Consumable, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetConsumable"); err != nil {
		return store.Consumable{}, err
	}
	c, ok := m.consumables[id]
	if !ok || c.TenantID != tenantID {
		return store.Consumable{}, repo.ErrNotFound
	}
	return c, nil
}

func (m *Mem) ListConsumables(_ context.Context, tenantID string, f store.ListOpts) ([]store.Consumable, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListConsumables"); err != nil {
		return nil, err
	}
	var out []store.Consumable
	for _, c := range m.consumables {
		if c.TenantID != tenantID {
			continue
		}
		if f.Query != "" {
			q := strings.ToLower(f.Query)
			if !strings.Contains(strings.ToLower(c.Name), q) && !strings.Contains(strings.ToLower(c.ModelName), q) {
				continue
			}
		}
		out = append(out, c)
	}
	out = paginate(out, func(c store.Consumable) string { return c.ID }, f.CursorID, f.Limit)
	return out, nil
}

func (m *Mem) UpdateConsumable(_ context.Context, c store.Consumable) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateConsumable"); err != nil {
		return err
	}
	ex, ok := m.consumables[c.ID]
	if !ok || ex.TenantID != c.TenantID {
		return repo.ErrNotFound
	}
	c.CreatedAt = ex.CreatedAt
	c.UpdatedAt = m.now()
	m.consumables[c.ID] = c
	return nil
}

func (m *Mem) DeleteConsumable(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteConsumable"); err != nil {
		return err
	}
	c, ok := m.consumables[id]
	if !ok || c.TenantID != tenantID {
		return repo.ErrNotFound
	}
	for did, d := range m.documents {
		if d.TenantID == tenantID && d.EntityType == store.EntityConsumable && d.EntityID == id {
			delete(m.documents, did)
		}
	}
	delete(m.consumables, id)
	return nil
}

func (m *Mem) AllConsumables(_ context.Context, tenantID string) ([]store.Consumable, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("AllConsumables"); err != nil {
		return nil, err
	}
	var out []store.Consumable
	for _, c := range m.consumables {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	out = paginate(out, func(c store.Consumable) string { return c.ID }, "", 0)
	return out, nil
}

// ---- licenses

func (m *Mem) CreateLicense(_ context.Context, l store.License) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateLicense"); err != nil {
		return err
	}
	if l.ID == "" {
		l.ID = store.NewID()
	}
	if _, dup := m.licenses[l.ID]; dup {
		return repo.ErrConflict
	}
	if l.Status == "" {
		l.Status = store.LicActive
	}
	t := m.now()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = t
	}
	l.UpdatedAt = t
	m.licenses[l.ID] = l
	return nil
}

func (m *Mem) GetLicense(_ context.Context, tenantID, id string) (store.License, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetLicense"); err != nil {
		return store.License{}, err
	}
	l, ok := m.licenses[id]
	if !ok || l.TenantID != tenantID {
		return store.License{}, repo.ErrNotFound
	}
	return l, nil
}

func (m *Mem) ListLicenses(_ context.Context, tenantID string, f store.ListOpts) ([]store.License, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListLicenses"); err != nil {
		return nil, err
	}
	var out []store.License
	for _, l := range m.licenses {
		if l.TenantID != tenantID {
			continue
		}
		if f.Query != "" {
			q := strings.ToLower(f.Query)
			if !strings.Contains(strings.ToLower(l.Name), q) {
				continue
			}
		}
		out = append(out, l)
	}
	out = paginate(out, func(l store.License) string { return l.ID }, f.CursorID, f.Limit)
	return out, nil
}

func (m *Mem) UpdateLicense(_ context.Context, l store.License) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateLicense"); err != nil {
		return err
	}
	ex, ok := m.licenses[l.ID]
	if !ok || ex.TenantID != l.TenantID {
		return repo.ErrNotFound
	}
	l.CreatedAt = ex.CreatedAt
	l.UpdatedAt = m.now()
	m.licenses[l.ID] = l
	return nil
}

func (m *Mem) DeleteLicense(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteLicense"); err != nil {
		return err
	}
	l, ok := m.licenses[id]
	if !ok || l.TenantID != tenantID {
		return repo.ErrNotFound
	}
	for did, d := range m.documents {
		if d.TenantID == tenantID && d.EntityType == store.EntityLicense && d.EntityID == id {
			delete(m.documents, did)
		}
	}
	delete(m.licenses, id)
	return nil
}

func (m *Mem) AllLicenses(_ context.Context, tenantID string) ([]store.License, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("AllLicenses"); err != nil {
		return nil, err
	}
	var out []store.License
	for _, l := range m.licenses {
		if l.TenantID == tenantID {
			out = append(out, l)
		}
	}
	out = paginate(out, func(l store.License) string { return l.ID }, "", 0)
	return out, nil
}

// ---- insurance policies + policy-assets

func (m *Mem) fillInsurance(p *store.InsurancePolicy) {
	var n int64
	for _, pa := range m.policyAssets {
		if pa.TenantID == p.TenantID && pa.PolicyID == p.ID {
			n++
		}
	}
	p.AssetCount = n
}

func (m *Mem) CreateInsurance(_ context.Context, p store.InsurancePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("CreateInsurance"); err != nil {
		return err
	}
	for _, ex := range m.insurance {
		if ex.TenantID != p.TenantID {
			continue
		}
		if ex.PolicyNumber == p.PolicyNumber || ex.Name == p.Name {
			return repo.ErrConflict
		}
	}
	if p.ID == "" {
		p.ID = store.NewID()
	}
	if _, dup := m.insurance[p.ID]; dup {
		return repo.ErrConflict
	}
	if p.Status == "" {
		p.Status = store.InsActive
	}
	t := m.now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = t
	}
	p.UpdatedAt = t
	m.insurance[p.ID] = p
	return nil
}

func (m *Mem) GetInsurance(_ context.Context, tenantID, id string) (store.InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetInsurance"); err != nil {
		return store.InsurancePolicy{}, err
	}
	p, ok := m.insurance[id]
	if !ok || p.TenantID != tenantID {
		return store.InsurancePolicy{}, repo.ErrNotFound
	}
	m.fillInsurance(&p)
	return p, nil
}

func (m *Mem) ListInsurance(_ context.Context, tenantID string, f store.ListOpts) ([]store.InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListInsurance"); err != nil {
		return nil, err
	}
	var out []store.InsurancePolicy
	for _, p := range m.insurance {
		if p.TenantID != tenantID {
			continue
		}
		if f.Query != "" {
			q := strings.ToLower(f.Query)
			if !strings.Contains(strings.ToLower(p.Name), q) && !strings.Contains(strings.ToLower(p.PolicyNumber), q) {
				continue
			}
		}
		out = append(out, p)
	}
	out = paginate(out, func(p store.InsurancePolicy) string { return p.ID }, f.CursorID, f.Limit)
	for i := range out {
		m.fillInsurance(&out[i])
	}
	return out, nil
}

func (m *Mem) UpdateInsurance(_ context.Context, p store.InsurancePolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpdateInsurance"); err != nil {
		return err
	}
	ex, ok := m.insurance[p.ID]
	if !ok || ex.TenantID != p.TenantID {
		return repo.ErrNotFound
	}
	for _, o := range m.insurance {
		if o.ID == p.ID || o.TenantID != p.TenantID {
			continue
		}
		if o.PolicyNumber == p.PolicyNumber || o.Name == p.Name {
			return repo.ErrConflict
		}
	}
	p.CreatedAt = ex.CreatedAt
	p.UpdatedAt = m.now()
	m.insurance[p.ID] = p
	return nil
}

func (m *Mem) DeleteInsurance(_ context.Context, tenantID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("DeleteInsurance"); err != nil {
		return err
	}
	p, ok := m.insurance[id]
	if !ok || p.TenantID != tenantID {
		return repo.ErrNotFound
	}
	for pid, pa := range m.policyAssets {
		if pa.TenantID == tenantID && pa.PolicyID == id {
			delete(m.policyAssets, pid)
		}
	}
	delete(m.insurance, id)
	return nil
}

func (m *Mem) AllInsurance(_ context.Context, tenantID string) ([]store.InsurancePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("AllInsurance"); err != nil {
		return nil, err
	}
	var out []store.InsurancePolicy
	for _, p := range m.insurance {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	out = paginate(out, func(p store.InsurancePolicy) string { return p.ID }, "", 0)
	for i := range out {
		m.fillInsurance(&out[i])
	}
	return out, nil
}

func (m *Mem) AddPolicyAsset(_ context.Context, pa store.PolicyAsset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("AddPolicyAsset"); err != nil {
		return err
	}
	for _, ex := range m.policyAssets {
		if ex.PolicyID == pa.PolicyID && ex.AssetID == pa.AssetID {
			return repo.ErrConflict
		}
	}
	if pa.ID == "" {
		pa.ID = store.NewID()
	}
	if _, dup := m.policyAssets[pa.ID]; dup {
		return repo.ErrConflict
	}
	if pa.CreatedAt.IsZero() {
		pa.CreatedAt = m.now()
	}
	m.policyAssets[pa.ID] = pa
	return nil
}

func (m *Mem) RemovePolicyAsset(_ context.Context, tenantID, policyID, assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("RemovePolicyAsset"); err != nil {
		return err
	}
	for pid, pa := range m.policyAssets {
		if pa.TenantID == tenantID && pa.PolicyID == policyID && pa.AssetID == assetID {
			delete(m.policyAssets, pid)
			return nil
		}
	}
	return repo.ErrNotFound
}

func (m *Mem) ListPolicyAssets(_ context.Context, tenantID, policyID string) ([]store.PolicyAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("ListPolicyAssets"); err != nil {
		return nil, err
	}
	var out []store.PolicyAsset
	for _, pa := range m.policyAssets {
		if pa.TenantID == tenantID && pa.PolicyID == policyID {
			out = append(out, pa)
		}
	}
	out = paginate(out, func(pa store.PolicyAsset) string { return pa.ID }, "", 0)
	return out, nil
}

// ---- notify-state (scheduler dedup)

func (m *Mem) GetNotifyState(_ context.Context, tenantID, conditionKey string) (store.NotifyState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetNotifyState"); err != nil {
		return store.NotifyState{}, err
	}
	for _, n := range m.notify {
		if n.TenantID == tenantID && n.ConditionKey == conditionKey {
			return n, nil
		}
	}
	return store.NotifyState{}, repo.ErrNotFound
}

func (m *Mem) UpsertNotifyState(_ context.Context, n store.NotifyState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("UpsertNotifyState"); err != nil {
		return err
	}
	if n.LastNotifiedAt.IsZero() {
		n.LastNotifiedAt = m.now()
	}
	for id, ex := range m.notify {
		if ex.TenantID == n.TenantID && ex.ConditionKey == n.ConditionKey {
			n.ID = ex.ID
			m.notify[id] = n
			return nil
		}
	}
	if n.ID == "" {
		n.ID = store.NewID()
	}
	m.notify[n.ID] = n
	return nil
}

// ---- statistics

func (m *Mem) TenantStats(_ context.Context, tenantID string) (repo.Stats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("TenantStats"); err != nil {
		return repo.Stats{}, err
	}
	st := repo.Stats{AssetsByStatus: map[string]int64{}}
	at := m.now()
	for _, a := range m.assets {
		if a.TenantID != tenantID {
			continue
		}
		st.TotalAssets++
		st.AssetsByStatus[a.Status]++
		st.TotalCost += a.PurchaseCost
		st.TotalDepreciatedValue += bookValue(a, at)
	}
	for _, c := range m.consumables {
		if c.TenantID != tenantID {
			continue
		}
		st.TotalConsumables++
		if c.Amount <= c.MinAmount {
			st.LowStock++
		}
	}
	for _, s := range m.suppliers {
		if s.TenantID == tenantID {
			st.TotalSuppliers++
		}
	}
	for _, c := range m.categories {
		if c.TenantID == tenantID {
			st.TotalCategories++
		}
	}
	for _, l := range m.locations {
		if l.TenantID == tenantID {
			st.TotalLocations++
		}
	}
	for _, l := range m.licenses {
		if l.TenantID == tenantID {
			st.TotalLicenses++
		}
	}
	for _, p := range m.insurance {
		if p.TenantID == tenantID {
			st.TotalInsurance++
		}
	}
	// ExpiringSoon is evaluated by the scheduler; placeholder here.
	st.ExpiringSoon = 0
	return st, nil
}

func (m *Mem) TenantIDs(_ context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("TenantIDs"); err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, a := range m.assets {
		set[a.TenantID] = true
	}
	for _, c := range m.categories {
		set[c.TenantID] = true
	}
	for _, s := range m.suppliers {
		set[s.TenantID] = true
	}
	for _, l := range m.locations {
		set[l.TenantID] = true
	}
	for _, c := range m.consumables {
		set[c.TenantID] = true
	}
	for _, l := range m.licenses {
		set[l.TenantID] = true
	}
	for _, p := range m.insurance {
		set[p.TenantID] = true
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// ---- audit

func (m *Mem) AppendAudit(_ context.Context, row store.AuditRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("AppendAudit"); err != nil {
		return err
	}
	if row.ID == "" {
		row.ID = store.NewID()
	}
	if row.At.IsZero() {
		row.At = m.now()
	}
	m.audit = append(m.audit, row)
	return nil
}

// ensure the compile-time contract holds.
var _ repo.Store = (*Mem)(nil)

// silence unused import if fmt drops out during edits.
var _ = fmt.Sprintf
