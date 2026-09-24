// Package stats computes the tenant dashboard rollup: assets by status, entity
// counts, total purchase cost, total depreciated (book) value via the deprec
// library, the number of warranty/license/insurance expiries within the
// configured windows and the low-stock consumable count.
package stats

import (
	"context"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/config"
	"github.com/go-tangra/go-tangra-asset/v4/internal/deprec"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Service computes statistics.
type Service struct {
	st      repo.Store
	windows config.SoonWindows
	now     func() time.Time
}

// New builds the service.
func New(st repo.Store, windows config.SoonWindows) *Service {
	day := 24 * time.Hour
	if windows.Warranty <= 0 {
		windows.Warranty = 30 * day
	}
	if windows.License <= 0 {
		windows.License = 30 * day
	}
	if windows.Insurance <= 0 {
		windows.Insurance = 30 * day
	}
	return &Service{st: st, windows: windows, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Dashboard is the tenant rollup with the expiry breakdown.
type Dashboard struct {
	repo.Stats
	WarrantyExpiringSoon  int64 `json:"warranty_expiring_soon"`
	LicensesExpiringSoon  int64 `json:"licenses_expiring_soon"`
	InsuranceExpiringSoon int64 `json:"insurance_expiring_soon"`
	AssignedAssets        int64 `json:"assigned_assets"`
}

// Get returns the caller's tenant dashboard.
func (s *Service) Get(ctx context.Context, subj authz.Subjects) (Dashboard, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return Dashboard{}, err
	}
	base, err := s.st.TenantStats(ctx, subj.TenantID)
	if err != nil {
		return Dashboard{}, err
	}
	if base.AssetsByStatus == nil {
		base.AssetsByStatus = map[string]int64{}
	}
	d := Dashboard{Stats: base}
	now := s.now()

	assets, err := s.st.AllAssets(ctx, subj.TenantID)
	if err != nil {
		return Dashboard{}, err
	}
	var cost, book float64
	for _, a := range assets {
		cost += a.PurchaseCost
		pd := time.Time{}
		if a.PurchaseDate != nil {
			pd = *a.PurchaseDate
		}
		book += deprec.BookValue(a.PurchaseCost, a.SalvageValue, a.UsefulLifeYears, a.DepreciationRate, pd, now)
		if a.Status == store.AssetAssigned {
			d.AssignedAssets++
		}
		if a.PurchaseDate != nil && a.WarrantyMonths > 0 && a.Status != store.AssetArchived {
			exp := a.PurchaseDate.AddDate(0, a.WarrantyMonths, 0)
			if !exp.Before(now) && exp.Sub(now) <= s.windows.Warranty {
				d.WarrantyExpiringSoon++
			}
		}
	}
	d.TotalCost, d.TotalDepreciatedValue = cost, book

	lics, err := s.st.AllLicenses(ctx, subj.TenantID)
	if err != nil {
		return Dashboard{}, err
	}
	for _, l := range lics {
		if l.ValidTo != nil && l.Status == store.LicActive && !l.ValidTo.Before(now) && l.ValidTo.Sub(now) <= s.windows.License {
			d.LicensesExpiringSoon++
		}
	}
	pols, err := s.st.AllInsurance(ctx, subj.TenantID)
	if err != nil {
		return Dashboard{}, err
	}
	for _, p := range pols {
		if p.ValidTo != nil && p.Status == store.InsActive && !p.ValidTo.Before(now) && p.ValidTo.Sub(now) <= s.windows.Insurance {
			d.InsuranceExpiringSoon++
		}
	}
	cons, err := s.st.AllConsumables(ctx, subj.TenantID)
	if err != nil {
		return Dashboard{}, err
	}
	d.LowStock = 0
	for _, c := range cons {
		if c.MinAmount > 0 && c.Amount <= c.MinAmount {
			d.LowStock++
		}
	}
	d.ExpiringSoon = d.WarrantyExpiringSoon + d.LicensesExpiringSoon + d.InsuranceExpiringSoon
	return d, nil
}
