// Package repo defines the storage contract for the asset (ITAM) service.
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrNotEmpty = errors.New("not empty")
)

// Stats is a per-tenant dashboard rollup.
type Stats struct {
	TotalAssets           int64            `json:"total_assets"`
	AssetsByStatus        map[string]int64 `json:"assets_by_status"`
	TotalConsumables      int64            `json:"total_consumables"`
	TotalSuppliers        int64            `json:"total_suppliers"`
	TotalCategories       int64            `json:"total_categories"`
	TotalLocations        int64            `json:"total_locations"`
	TotalLicenses         int64            `json:"total_licenses"`
	TotalInsurance        int64            `json:"total_insurance_policies"`
	TotalCost             float64          `json:"total_cost"`
	TotalDepreciatedValue float64          `json:"total_depreciated_value"`
	ExpiringSoon          int64            `json:"expiring_soon"`
	LowStock              int64            `json:"low_stock"`
}

// Store is the asset persistence contract. Methods are tenant-scoped; the system
// scope (nil-uuid pin) is used only by the lifecycle scheduler + maintenance.
type Store interface {
	// Assets
	CreateAsset(ctx context.Context, a store.Asset) error // ErrConflict on duplicate tag
	GetAsset(ctx context.Context, tenantID, id string) (store.Asset, error)
	FindAssetByTag(ctx context.Context, tenantID, tag string) (store.Asset, error)
	FindAssetBySerial(ctx context.Context, tenantID, serial string) (store.Asset, error)
	ListAssets(ctx context.Context, tenantID string, f store.AssetFilter) ([]store.Asset, error)
	UpdateAsset(ctx context.Context, a store.Asset) error
	DeleteAsset(ctx context.Context, tenantID, id string) error // cascades assignments/documents/policy links
	CountAssetsByCategory(ctx context.Context, tenantID, categoryID string) (int64, error)
	CountAssetsBySupplier(ctx context.Context, tenantID, supplierID string) (int64, error)
	CountAssetsByLocation(ctx context.Context, tenantID, locationID string) (int64, error)
	AllAssets(ctx context.Context, tenantID string) ([]store.Asset, error) // scheduler/stats/sync

	// Assignments
	InsertAssignment(ctx context.Context, a store.Assignment) error
	CloseActiveAssignment(ctx context.Context, tenantID, assetID string, at time.Time) error
	ListAssignments(ctx context.Context, tenantID, assetID string, limit int) ([]store.Assignment, error)

	// Documents (polymorphic)
	InsertDocument(ctx context.Context, d store.Document) error
	GetDocument(ctx context.Context, tenantID, id string) (store.Document, error)
	ListDocuments(ctx context.Context, tenantID, entityType, entityID string) ([]store.Document, error)
	DeleteDocument(ctx context.Context, tenantID, id string) error

	// Categories
	CreateCategory(ctx context.Context, c store.Category) error
	GetCategory(ctx context.Context, tenantID, id string) (store.Category, error)
	ListCategories(ctx context.Context, tenantID string) ([]store.Category, error)
	UpdateCategory(ctx context.Context, c store.Category) error
	DeleteCategory(ctx context.Context, tenantID, id string) error
	CountChildCategories(ctx context.Context, tenantID, id string) (int64, error)

	// Suppliers
	CreateSupplier(ctx context.Context, s store.Supplier) error
	GetSupplier(ctx context.Context, tenantID, id string) (store.Supplier, error)
	ListSuppliers(ctx context.Context, tenantID string, f store.ListOpts) ([]store.Supplier, error)
	UpdateSupplier(ctx context.Context, s store.Supplier) error
	DeleteSupplier(ctx context.Context, tenantID, id string) error

	// Locations
	CreateLocation(ctx context.Context, l store.Location) error
	GetLocation(ctx context.Context, tenantID, id string) (store.Location, error)
	ListLocations(ctx context.Context, tenantID string) ([]store.Location, error)
	UpdateLocation(ctx context.Context, l store.Location) error
	DeleteLocation(ctx context.Context, tenantID, id string) error
	CountChildLocations(ctx context.Context, tenantID, id string) (int64, error)

	// Consumables
	CreateConsumable(ctx context.Context, c store.Consumable) error
	GetConsumable(ctx context.Context, tenantID, id string) (store.Consumable, error)
	ListConsumables(ctx context.Context, tenantID string, f store.ListOpts) ([]store.Consumable, error)
	UpdateConsumable(ctx context.Context, c store.Consumable) error
	DeleteConsumable(ctx context.Context, tenantID, id string) error
	AllConsumables(ctx context.Context, tenantID string) ([]store.Consumable, error) // scheduler

	// Licenses
	CreateLicense(ctx context.Context, l store.License) error
	GetLicense(ctx context.Context, tenantID, id string) (store.License, error)
	ListLicenses(ctx context.Context, tenantID string, f store.ListOpts) ([]store.License, error)
	UpdateLicense(ctx context.Context, l store.License) error
	DeleteLicense(ctx context.Context, tenantID, id string) error
	AllLicenses(ctx context.Context, tenantID string) ([]store.License, error) // scheduler

	// Insurance policies + policy-assets
	CreateInsurance(ctx context.Context, p store.InsurancePolicy) error
	GetInsurance(ctx context.Context, tenantID, id string) (store.InsurancePolicy, error)
	ListInsurance(ctx context.Context, tenantID string, f store.ListOpts) ([]store.InsurancePolicy, error)
	UpdateInsurance(ctx context.Context, p store.InsurancePolicy) error
	DeleteInsurance(ctx context.Context, tenantID, id string) error
	AllInsurance(ctx context.Context, tenantID string) ([]store.InsurancePolicy, error) // scheduler
	AddPolicyAsset(ctx context.Context, pa store.PolicyAsset) error                     // ErrConflict on dup
	RemovePolicyAsset(ctx context.Context, tenantID, policyID, assetID string) error
	ListPolicyAssets(ctx context.Context, tenantID, policyID string) ([]store.PolicyAsset, error)

	// Notify-state (scheduler dedup)
	GetNotifyState(ctx context.Context, tenantID, conditionKey string) (store.NotifyState, error)
	UpsertNotifyState(ctx context.Context, n store.NotifyState) error

	// Statistics
	TenantStats(ctx context.Context, tenantID string) (Stats, error)
	TenantIDs(ctx context.Context) ([]string, error) // system scope

	// Audit
	AppendAudit(ctx context.Context, row store.AuditRow) error
}
