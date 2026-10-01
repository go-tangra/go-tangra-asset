package memstore

import (
	"context"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// List-contract pages (go-tangra specs/032-server-side-tables): the same
// filters as the cursor lists, sorted by the store.*List fields with the same
// semantics as SQL (listquery.SortSlice) and windowed with the total.

func contains(v, q string) bool { return strings.Contains(strings.ToLower(v), q) }

// matchAssets is the tenant's assets matching f (cursor/limit ignored).
func (m *Mem) matchAssets(tenantID string, f store.AssetFilter) []store.Asset {
	q := strings.ToLower(f.Query)
	var out []store.Asset
	for _, a := range m.assets {
		switch {
		case a.TenantID != tenantID,
			f.Status != "" && a.Status != f.Status,
			f.CategoryID != "" && a.CategoryID != f.CategoryID,
			f.SupplierID != "" && a.SupplierID != f.SupplierID,
			f.LocationID != "" && a.LocationID != f.LocationID,
			f.UserID != "" && a.UserID != f.UserID,
			q != "" && !contains(a.AssetTag, q) && !contains(a.Name, q) && !contains(a.Serial, q) && !contains(a.ModelName, q):
			continue
		}
		out = append(out, a)
	}
	return out
}

func (m *Mem) matchSuppliers(tenantID, query string) []store.Supplier {
	q := strings.ToLower(query)
	var out []store.Supplier
	for _, s := range m.suppliers {
		if s.TenantID == tenantID && (q == "" || contains(s.Name, q) || contains(s.Code, q)) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Mem) matchConsumables(tenantID, query string) []store.Consumable {
	q := strings.ToLower(query)
	var out []store.Consumable
	for _, c := range m.consumables {
		if c.TenantID == tenantID && (q == "" || contains(c.Name, q) || contains(c.ModelName, q)) {
			out = append(out, c)
		}
	}
	return out
}

func (m *Mem) matchLicenses(tenantID, query string) []store.License {
	q := strings.ToLower(query)
	var out []store.License
	for _, l := range m.licenses {
		if l.TenantID == tenantID && (q == "" || contains(l.Name, q)) {
			out = append(out, l)
		}
	}
	return out
}

func (m *Mem) matchInsurance(tenantID, query string) []store.InsurancePolicy {
	q := strings.ToLower(query)
	var out []store.InsurancePolicy
	for _, p := range m.insurance {
		if p.TenantID == tenantID && (q == "" || contains(p.Name, q) || contains(p.PolicyNumber, q)) {
			out = append(out, p)
		}
	}
	return out
}

// timeKey is a nullable time as a sort key (nil sorts last, like SQL NULL).
func timeKey(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// warrantyEnd mirrors store.WarrantyEndExpr.
func warrantyEnd(a store.Asset) any {
	if a.PurchaseDate == nil || a.WarrantyMonths <= 0 {
		return nil
	}
	return a.PurchaseDate.AddDate(0, a.WarrantyMonths, 0)
}

// assetKey is the value of a store.AssetList sort field (category / location:
// the referenced record's name / path, nil when unset).
func (m *Mem) assetKey(a store.Asset, field string) any {
	switch field {
	case "name":
		return a.Name
	case "status":
		return a.Status
	case "category":
		if c, ok := m.categories[a.CategoryID]; ok && a.CategoryID != "" {
			return c.Name
		}
		return nil
	case "location":
		if l, ok := m.locations[a.LocationID]; ok && a.LocationID != "" {
			if l.Path != "" {
				return l.Path
			}
			return l.Name
		}
		return nil
	case "purchase_date":
		return timeKey(a.PurchaseDate)
	case "warranty_end":
		return warrantyEnd(a)
	case "created_at":
		return a.CreatedAt
	default:
		return a.AssetTag
	}
}

// window sorts and pages items.
func window[T any](items []T, req listquery.Request, spec listquery.Spec, key func(T, string) any, id func(T) string) ([]T, int, listquery.Request) {
	req = store.ListRequest(req, spec)
	listquery.SortSlice(items, req, key, id)
	pg, total, applied := listquery.Window(items, req)
	return append([]T{}, pg...), total, applied
}

// PageAssets implements repo.Store.
func (m *Mem) PageAssets(_ context.Context, tenantID string, f store.AssetFilter, req listquery.Request) ([]store.Asset, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PageAssets"); err != nil {
		return nil, 0, req, err
	}
	out, total, applied := window(m.matchAssets(tenantID, f), req, store.AssetList, m.assetKey, func(a store.Asset) string { return a.ID })
	for i := range out {
		m.fillAsset(&out[i])
	}
	return out, total, applied, nil
}

// PageAssignments implements repo.Store.
func (m *Mem) PageAssignments(_ context.Context, tenantID, assetID string, req listquery.Request) ([]store.Assignment, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PageAssignments"); err != nil {
		return nil, 0, req, err
	}
	var all []store.Assignment
	for _, a := range m.assignments {
		if a.TenantID == tenantID && a.AssetID == assetID {
			all = append(all, a)
		}
	}
	out, total, applied := window(all, req, store.AssignmentList, func(a store.Assignment, field string) any {
		if field == "returned_at" {
			return timeKey(a.ReturnedAt)
		}
		return a.AssignedAt
	}, func(a store.Assignment) string { return a.ID })
	return out, total, applied, nil
}

// nameKey sorts the simple lists by name or created_at.
func nameKey(name string, created time.Time, field string) any {
	if field == "created_at" {
		return created
	}
	return name
}

// PageSuppliers implements repo.Store.
func (m *Mem) PageSuppliers(_ context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.Supplier, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PageSuppliers"); err != nil {
		return nil, 0, req, err
	}
	out, total, applied := window(m.matchSuppliers(tenantID, f.Query), req, store.SupplierList,
		func(s store.Supplier, field string) any { return nameKey(s.Name, s.CreatedAt, field) }, func(s store.Supplier) string { return s.ID })
	return out, total, applied, nil
}

// PageConsumables implements repo.Store.
func (m *Mem) PageConsumables(_ context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.Consumable, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PageConsumables"); err != nil {
		return nil, 0, req, err
	}
	out, total, applied := window(m.matchConsumables(tenantID, f.Query), req, store.ConsumableList, func(c store.Consumable, field string) any {
		if field == "amount" {
			return c.Amount
		}
		return nameKey(c.Name, c.CreatedAt, field)
	}, func(c store.Consumable) string { return c.ID })
	return out, total, applied, nil
}

// PageLicenses implements repo.Store.
func (m *Mem) PageLicenses(_ context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.License, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PageLicenses"); err != nil {
		return nil, 0, req, err
	}
	out, total, applied := window(m.matchLicenses(tenantID, f.Query), req, store.LicenseList, func(l store.License, field string) any {
		if field == "valid_to" {
			return timeKey(l.ValidTo)
		}
		return nameKey(l.Name, l.CreatedAt, field)
	}, func(l store.License) string { return l.ID })
	return out, total, applied, nil
}

// PageInsurance implements repo.Store.
func (m *Mem) PageInsurance(_ context.Context, tenantID string, f store.ListOpts, req listquery.Request) ([]store.InsurancePolicy, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PageInsurance"); err != nil {
		return nil, 0, req, err
	}
	out, total, applied := window(m.matchInsurance(tenantID, f.Query), req, store.InsuranceList, func(p store.InsurancePolicy, field string) any {
		if field == "valid_to" {
			return timeKey(p.ValidTo)
		}
		return nameKey(p.Name, p.CreatedAt, field)
	}, func(p store.InsurancePolicy) string { return p.ID })
	for i := range out {
		m.fillInsurance(&out[i])
	}
	return out, total, applied, nil
}

// PagePolicyAssets implements repo.Store.
func (m *Mem) PagePolicyAssets(_ context.Context, tenantID, policyID string, req listquery.Request) ([]store.PolicyAsset, int, listquery.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PagePolicyAssets"); err != nil {
		return nil, 0, req, err
	}
	var all []store.PolicyAsset
	for _, pa := range m.policyAssets {
		if pa.TenantID == tenantID && pa.PolicyID == policyID {
			all = append(all, pa)
		}
	}
	out, total, applied := window(all, req, store.PolicyAssetList, func(pa store.PolicyAsset, field string) any {
		if field == "name" {
			return pa.AssetName
		}
		return pa.AssetTag
	}, func(pa store.PolicyAsset) string { return pa.ID })
	return out, total, applied, nil
}
