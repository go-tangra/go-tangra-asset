// Package store holds the asset (ITAM) domain types and the SQL-backed store.
package store

import "time"

// --- enums (stored as text) ---
const (
	AssetDeployable, AssetAssigned, AssetBroken, AssetArchived     = "deployable", "assigned", "broken", "archived"
	ActionAssigned, ActionUnassigned, ActionTransferred            = "assigned", "unassigned", "transferred"
	LocActive, LocPlanned, LocDecommissioned                       = "active", "planned", "decommissioned"
	SupActive, SupInactive                                         = "active", "inactive"
	LicActive, LicExpired, LicSuspended                            = "active", "expired", "suspended"
	InsActive, InsExpired, InsCancelled                            = "active", "expired", "cancelled"
	CovAllRisk, CovFireTheft, CovLiability, CovEquipment, CovCyber = "all_risk", "fire_theft", "liability", "equipment_breakdown", "cyber"
	EntityAsset, EntityConsumable, EntityLicense                   = "asset", "consumable", "license"
)

// Asset is an IT asset. Unique (tenant_id,asset_tag).
type Asset struct {
	ID               string            `json:"id"`
	TenantID         string            `json:"tenant_id"`
	AssetTag         string            `json:"asset_tag"`
	Name             string            `json:"name,omitempty"`
	Serial           string            `json:"serial,omitempty"`
	ModelName        string            `json:"model_name,omitempty"`
	ModelNumber      string            `json:"model_number,omitempty"`
	CategoryID       string            `json:"category_id,omitempty"`
	SupplierID       string            `json:"supplier_id,omitempty"`
	LocationID       string            `json:"location_id,omitempty"`
	UserID           string            `json:"user_id,omitempty"`
	Status           string            `json:"status"`
	PhotoKey         string            `json:"photo_key,omitempty"`
	WarrantyMonths   int               `json:"warranty_months,omitempty"`
	PurchaseDate     *time.Time        `json:"purchase_date,omitempty"`
	OrderNumber      string            `json:"order_number,omitempty"`
	PurchaseCost     float64           `json:"purchase_cost,omitempty"`
	Notes            string            `json:"notes,omitempty"`
	SalvageValue     float64           `json:"salvage_value,omitempty"`
	UsefulLifeYears  int               `json:"useful_life_years,omitempty"`
	DepreciationRate float64           `json:"depreciation_rate,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
	CreatedBy        string            `json:"created_by,omitempty"`
	UpdatedBy        string            `json:"updated_by,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	// Computed (not stored):
	BookValue float64 `json:"book_value,omitempty"`
}

// Assignment is a check-out/check-in history record.
type Assignment struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	AssetID    string     `json:"asset_id"`
	AssetName  string     `json:"asset_name,omitempty"`
	UserID     string     `json:"user_id,omitempty"`
	UserName   string     `json:"user_name,omitempty"`
	Action     string     `json:"action"`
	AssignedAt time.Time  `json:"assigned_at"`
	ReturnedAt *time.Time `json:"returned_at,omitempty"`
	AssignedBy string     `json:"assigned_by,omitempty"`
	Notes      string     `json:"notes,omitempty"`
}

// Document is a polymorphic file attachment (asset|consumable|license).
type Document struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	MimeType    string    `json:"mime_type,omitempty"`
	StorageKey  string    `json:"storage_key"`
	Checksum    string    `json:"checksum,omitempty"`
	Description string    `json:"description,omitempty"`
	UploadedBy  string    `json:"uploaded_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Category is a self-referential tree. Unique (tenant_id,name,parent_id).
type Category struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedBy   string            `json:"created_by,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	AssetCount  int64             `json:"asset_count"`
	ChildCount  int64             `json:"child_count"`
}

// Supplier. Unique (tenant_id,name). Contact fields sealed/redacted.
type Supplier struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	Name          string            `json:"name"`
	Code          string            `json:"code,omitempty"`
	Address       string            `json:"address,omitempty"`
	City          string            `json:"city,omitempty"`
	State         string            `json:"state,omitempty"`
	Country       string            `json:"country,omitempty"`
	PostalCode    string            `json:"postal_code,omitempty"`
	ContactPerson string            `json:"contact_person,omitempty"`
	Telephone     string            `json:"telephone,omitempty"`
	Email         string            `json:"email,omitempty"`
	Website       string            `json:"website,omitempty"`
	Notes         string            `json:"notes,omitempty"`
	Status        string            `json:"status"`
	Tags          map[string]string `json:"tags,omitempty"`
	CreatedBy     string            `json:"created_by,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// Location is a self-referential tree. Unique (tenant_id,name). Contact sealed/redacted.
type Location struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Name        string            `json:"name"`
	Code        string            `json:"code,omitempty"`
	Description string            `json:"description,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Path        string            `json:"path,omitempty"`
	Address     string            `json:"address,omitempty"`
	City        string            `json:"city,omitempty"`
	State       string            `json:"state,omitempty"`
	Country     string            `json:"country,omitempty"`
	PostalCode  string            `json:"postal_code,omitempty"`
	Contact     string            `json:"contact,omitempty"`
	Phone       string            `json:"phone,omitempty"`
	Email       string            `json:"email,omitempty"`
	Status      string            `json:"status"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedBy   string            `json:"created_by,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ChildCount  int64             `json:"child_count"`
	AssetCount  int64             `json:"asset_count"`
}

// Consumable. Stock amount + reorder threshold.
type Consumable struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	CategoryID   string            `json:"category_id,omitempty"`
	SupplierID   string            `json:"supplier_id,omitempty"`
	LocationID   string            `json:"location_id,omitempty"`
	ModelName    string            `json:"model_name,omitempty"`
	ModelNumber  string            `json:"model_number,omitempty"`
	Amount       int               `json:"amount"`
	MinAmount    int               `json:"min_amount"`
	PurchaseDate *time.Time        `json:"purchase_date,omitempty"`
	PurchaseCost float64           `json:"purchase_cost,omitempty"`
	OrderNumber  string            `json:"order_number,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	CreatedBy    string            `json:"created_by,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// License. valid_to auto-expire.
type License struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	Name         string            `json:"name"`
	SupplierID   string            `json:"supplier_id,omitempty"`
	PurchaseDate *time.Time        `json:"purchase_date,omitempty"`
	PurchaseCost float64           `json:"purchase_cost,omitempty"`
	OrderNumber  string            `json:"order_number,omitempty"`
	ValidFrom    *time.Time        `json:"valid_from,omitempty"`
	ValidTo      *time.Time        `json:"valid_to,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedBy    string            `json:"created_by,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// InsurancePolicy. Unique (tenant_id,policy_number).
type InsurancePolicy struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	Name          string            `json:"name"`
	PolicyNumber  string            `json:"policy_number"`
	Provider      string            `json:"provider,omitempty"`
	CoverageType  string            `json:"coverage_type,omitempty"`
	PremiumAmount float64           `json:"premium_amount,omitempty"`
	Deductible    float64           `json:"deductible,omitempty"`
	CoverageLimit float64           `json:"coverage_limit,omitempty"`
	ValidFrom     *time.Time        `json:"valid_from,omitempty"`
	ValidTo       *time.Time        `json:"valid_to,omitempty"`
	Status        string            `json:"status"`
	Notes         string            `json:"notes,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedBy     string            `json:"created_by,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	AssetCount    int64             `json:"asset_count"`
}

// PolicyAsset is the M2M join. Unique (policy_id,asset_id).
type PolicyAsset struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	PolicyID     string    `json:"policy_id"`
	AssetID      string    `json:"asset_id"`
	CoveredValue float64   `json:"covered_value,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	AssetTag     string    `json:"asset_tag,omitempty"`
	AssetName    string    `json:"asset_name,omitempty"`
	ModelName    string    `json:"model_name,omitempty"`
	CreatedBy    string    `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// NotifyState dedups scheduler alerts. Unique (tenant_id,condition_key).
type NotifyState struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	ConditionKey   string    `json:"condition_key"`
	LastNotifiedAt time.Time `json:"last_notified_at"`
	LastState      string    `json:"last_state,omitempty"`
}

// AuditRow is an append-only audit event.
type AuditRow struct {
	ID          string
	TenantID    string
	At          time.Time
	ActorKind   string
	ActorID     string
	Action      string
	SubjectKind string
	SubjectID   string
	Outcome     string
	Reason      string
	Detail      map[string]any
}

// --- filters ---
type AssetFilter struct {
	Status, CategoryID, SupplierID, LocationID, UserID, Query string
	Limit                                                     int
	CursorID                                                  string
}
type ListOpts struct {
	Query    string
	Limit    int
	CursorID string
}
