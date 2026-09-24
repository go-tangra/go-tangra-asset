// Package assetclient is a thin, typed Go client for the asset.v1
// service-to-service gRPC API, for other Freya modules that need to read or
// manage IT assets, their assign/unassign lifecycle, categories, suppliers,
// locations, consumables, licenses, insurance policies and dashboard
// statistics, or to trigger an inventory sync. It wraps the generated gRPC
// stubs so callers deal in ordinary Go values. The caller supplies a connected,
// SPIFFE-mTLS *grpc.ClientConn (e.g. from freya.App.Client(ctx, "asset")); this
// package does not dial or manage the connection. No response carries
// object-store credentials or sealed contact PII.
package assetclient

import (
	"context"
	"time"

	"google.golang.org/grpc"

	assetv1 "github.com/go-tangra/go-tangra-asset/v4/api/proto/asset/v1"
)

// Client calls the asset.v1 API over a caller-provided gRPC connection.
type Client struct {
	assets     assetv1.AssetServiceClient
	categories assetv1.CategoryServiceClient
	suppliers  assetv1.SupplierServiceClient
	locations  assetv1.LocationServiceClient
	system     assetv1.SystemServiceClient
	users      assetv1.UserServiceClient
}

// New builds a client from a connected (SPIFFE-mTLS) gRPC connection to the
// asset service.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{
		assets: assetv1.NewAssetServiceClient(conn), categories: assetv1.NewCategoryServiceClient(conn),
		suppliers: assetv1.NewSupplierServiceClient(conn), locations: assetv1.NewLocationServiceClient(conn),
		system: assetv1.NewSystemServiceClient(conn), users: assetv1.NewUserServiceClient(conn),
	}
}

// ---- plain Go values

// Asset is an IT asset as the module reports it (with the computed book value).
type Asset struct {
	ID, TenantID, AssetTag, Name, Serial, ModelName, ModelNumber string
	CategoryID, SupplierID, LocationID, UserID, AssigneeName     string
	Status                                                       string
	HasPhoto                                                     bool
	WarrantyMonths                                               int
	PurchaseDate                                                 *time.Time
	OrderNumber                                                  string
	PurchaseCost, SalvageValue, DepreciationRate, BookValue      float64
	UsefulLifeYears                                              int
	Notes                                                        string
	Tags                                                         map[string]string
	CreatedAt, UpdatedAt                                         time.Time
}

// AssetInput is the create/update body.
type AssetInput struct {
	AssetTag, Name, Serial, ModelName, ModelNumber string
	CategoryID, SupplierID, LocationID, Status     string
	WarrantyMonths                                 int
	PurchaseDate                                   *time.Time
	OrderNumber, Notes                             string
	PurchaseCost, SalvageValue, DepreciationRate   float64
	UsefulLifeYears                                int
	Tags                                           map[string]string
}

// AssetFilter constrains ListAssets. Empty fields match all.
type AssetFilter struct {
	Status, CategoryID, SupplierID, LocationID, UserID, Query string
	Limit                                                     int
	CursorID                                                  string
}

// Assignment is one check-out/check-in history row.
type Assignment struct {
	ID, AssetID, AssetName, UserID, UserName, Action, AssignedBy, Notes string
	AssignedAt                                                          time.Time
	ReturnedAt                                                          *time.Time
}

// Category is a node of the category tree.
type Category struct {
	ID, Name, Description, ParentID, Icon string
	AssetCount, ChildCount                int64
	Children                              []Category
}

// Supplier is a supplier (contact fields blank for service callers).
type Supplier struct {
	ID, Name, Code, City, Country, Status string
}

// Location is a node of the location tree.
type Location struct {
	ID, Name, Code, ParentID, Path, City, Country, Status string
	ChildCount, AssetCount                                int64
	Children                                              []Location
}

// User is a tenant member from the assignee directory.
type User struct{ ID, DisplayName string }

// Stats is the tenant dashboard rollup.
type Stats struct {
	TotalAssets, TotalConsumables, TotalSuppliers, TotalCategories, TotalLocations, TotalLicenses, TotalInsurancePolicies int64
	AssetsByStatus                                                                                                        map[string]int64
	TotalCost, TotalDepreciatedValue                                                                                      float64
	ExpiringSoon, LowStock, WarrantyExpiringSoon, LicensesExpiringSoon, InsuranceExpiringSoon, AssignedAssets             int64
}

// SyncChange is one inventory-sync decision.
type SyncChange struct {
	HostID, Hostname, Serial, Action, AssetID, AssetTag string
	Changes                                             map[string]string
}

// SyncPreview is a dry-run result.
type SyncPreview struct {
	Hosts, Create, Update, Unchanged int
	Changes                          []SyncChange
}

// SyncResult is an execute result.
type SyncResult struct {
	Created, Updated, Skipped int
	Errors                    []string
	Changes                   []SyncChange
}

// ---- assets

func (c *Client) CreateAsset(ctx context.Context, tenantID string, in AssetInput) (Asset, error) {
	a, err := c.assets.CreateAsset(ctx, &assetv1.CreateAssetRequest{TenantId: tenantID, Asset: inputToPB(in)})
	if err != nil {
		return Asset{}, err
	}
	return toAsset(a), nil
}

func (c *Client) GetAsset(ctx context.Context, tenantID, id string) (Asset, error) {
	a, err := c.assets.GetAsset(ctx, &assetv1.GetAssetRequest{TenantId: tenantID, Id: id})
	if err != nil {
		return Asset{}, err
	}
	return toAsset(a), nil
}

func (c *Client) ListAssets(ctx context.Context, tenantID string, f AssetFilter) ([]Asset, error) {
	res, err := c.assets.ListAssets(ctx, &assetv1.ListAssetsRequest{TenantId: tenantID, Status: f.Status, CategoryId: f.CategoryID, SupplierId: f.SupplierID,
		LocationId: f.LocationID, UserId: f.UserID, Query: f.Query, Limit: int32(f.Limit), CursorId: f.CursorID}) // #nosec G115 -- bounded page size
	if err != nil {
		return nil, err
	}
	out := make([]Asset, 0, len(res.GetAssets()))
	for _, a := range res.GetAssets() {
		out = append(out, toAsset(a))
	}
	return out, nil
}

func (c *Client) UpdateAsset(ctx context.Context, tenantID, id string, in AssetInput) (Asset, error) {
	a, err := c.assets.UpdateAsset(ctx, &assetv1.UpdateAssetRequest{TenantId: tenantID, Id: id, Asset: inputToPB(in)})
	if err != nil {
		return Asset{}, err
	}
	return toAsset(a), nil
}

func (c *Client) DeleteAsset(ctx context.Context, tenantID, id string) error {
	_, err := c.assets.DeleteAsset(ctx, &assetv1.DeleteAssetRequest{TenantId: tenantID, Id: id})
	return err
}

func (c *Client) AssignAsset(ctx context.Context, tenantID, id, userID, notes string) (Asset, error) {
	a, err := c.assets.AssignAsset(ctx, &assetv1.AssignAssetRequest{TenantId: tenantID, Id: id, UserId: userID, Notes: notes})
	if err != nil {
		return Asset{}, err
	}
	return toAsset(a), nil
}

func (c *Client) UnassignAsset(ctx context.Context, tenantID, id, locationID, notes string) (Asset, error) {
	a, err := c.assets.UnassignAsset(ctx, &assetv1.UnassignAssetRequest{TenantId: tenantID, Id: id, LocationId: locationID, Notes: notes})
	if err != nil {
		return Asset{}, err
	}
	return toAsset(a), nil
}

func (c *Client) GetAssignmentHistory(ctx context.Context, tenantID, id string, limit int) ([]Assignment, error) {
	res, err := c.assets.GetAssignmentHistory(ctx, &assetv1.GetAssignmentHistoryRequest{TenantId: tenantID, Id: id, Limit: int32(limit)}) // #nosec G115
	if err != nil {
		return nil, err
	}
	out := make([]Assignment, 0, len(res.GetAssignments()))
	for _, g := range res.GetAssignments() {
		out = append(out, Assignment{ID: g.GetId(), AssetID: g.GetAssetId(), AssetName: g.GetAssetName(), UserID: g.GetUserId(), UserName: g.GetUserName(),
			Action: g.GetAction(), AssignedBy: g.GetAssignedBy(), Notes: g.GetNotes(), AssignedAt: fromTS(g.GetAssignedAt()), ReturnedAt: fromTSP(g.GetReturnedAt())})
	}
	return out, nil
}

func (c *Client) InventorySyncPreview(ctx context.Context, tenantID string) (SyncPreview, error) {
	res, err := c.assets.InventorySyncPreview(ctx, &assetv1.InventorySyncPreviewRequest{TenantId: tenantID})
	if err != nil {
		return SyncPreview{}, err
	}
	return SyncPreview{Hosts: int(res.GetHosts()), Create: int(res.GetCreate()), Update: int(res.GetUpdate()), Unchanged: int(res.GetUnchanged()), Changes: toChanges(res.GetChanges())}, nil
}

func (c *Client) InventorySyncExecute(ctx context.Context, tenantID string, hostnames []string) (SyncResult, error) {
	res, err := c.assets.InventorySyncExecute(ctx, &assetv1.InventorySyncExecuteRequest{TenantId: tenantID, Hostnames: hostnames})
	if err != nil {
		return SyncResult{}, err
	}
	return SyncResult{Created: int(res.GetCreated()), Updated: int(res.GetUpdated()), Skipped: int(res.GetSkipped()), Errors: res.GetErrors(), Changes: toChanges(res.GetChanges())}, nil
}

// ---- org records

func (c *Client) GetCategoryTree(ctx context.Context, tenantID string) ([]Category, error) {
	res, err := c.categories.GetCategoryTree(ctx, &assetv1.GetCategoryTreeRequest{TenantId: tenantID})
	if err != nil {
		return nil, err
	}
	return toCategories(res.GetRoots()), nil
}

func (c *Client) ListSuppliers(ctx context.Context, tenantID, query string) ([]Supplier, error) {
	res, err := c.suppliers.ListSuppliers(ctx, &assetv1.ListSuppliersRequest{TenantId: tenantID, Query: query})
	if err != nil {
		return nil, err
	}
	out := make([]Supplier, 0, len(res.GetSuppliers()))
	for _, s := range res.GetSuppliers() {
		out = append(out, Supplier{ID: s.GetId(), Name: s.GetName(), Code: s.GetCode(), City: s.GetCity(), Country: s.GetCountry(), Status: s.GetStatus()})
	}
	return out, nil
}

func (c *Client) GetLocationTree(ctx context.Context, tenantID string) ([]Location, error) {
	res, err := c.locations.GetLocationTree(ctx, &assetv1.GetLocationTreeRequest{TenantId: tenantID})
	if err != nil {
		return nil, err
	}
	return toLocations(res.GetRoots()), nil
}

// ---- users + system

func (c *Client) ListUsers(ctx context.Context, tenantID, query string) ([]User, error) {
	res, err := c.users.ListUsers(ctx, &assetv1.ListUsersRequest{TenantId: tenantID, Query: query})
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(res.GetUsers()))
	for _, u := range res.GetUsers() {
		out = append(out, User{ID: u.GetId(), DisplayName: u.GetDisplayName()})
	}
	return out, nil
}

func (c *Client) GetDashboardStats(ctx context.Context, tenantID string) (Stats, error) {
	d, err := c.system.GetDashboardStats(ctx, &assetv1.GetDashboardStatsRequest{TenantId: tenantID})
	if err != nil {
		return Stats{}, err
	}
	return Stats{TotalAssets: d.GetTotalAssets(), TotalConsumables: d.GetTotalConsumables(), TotalSuppliers: d.GetTotalSuppliers(), TotalCategories: d.GetTotalCategories(),
		TotalLocations: d.GetTotalLocations(), TotalLicenses: d.GetTotalLicenses(), TotalInsurancePolicies: d.GetTotalInsurancePolicies(), AssetsByStatus: d.GetAssetsByStatus(),
		TotalCost: d.GetTotalCost(), TotalDepreciatedValue: d.GetTotalDepreciatedValue(), ExpiringSoon: d.GetExpiringSoon(), LowStock: d.GetLowStock(),
		WarrantyExpiringSoon: d.GetWarrantyExpiringSoon(), LicensesExpiringSoon: d.GetLicensesExpiringSoon(), InsuranceExpiringSoon: d.GetInsuranceExpiringSoon(), AssignedAssets: d.GetAssignedAssets()}, nil
}

// Health reports the module's health status.
func (c *Client) Health(ctx context.Context) (status string, components map[string]string, err error) {
	h, err := c.system.Health(ctx, &assetv1.HealthRequest{})
	if err != nil {
		return "", nil, err
	}
	return h.GetStatus(), h.GetComponents(), nil
}

// ---- mapping

func fromTS(t *assetv1.Timestamp) time.Time {
	if t == nil || t.GetUnix() == 0 {
		return time.Time{}
	}
	return time.Unix(t.GetUnix(), 0).UTC()
}

func fromTSP(t *assetv1.Timestamp) *time.Time {
	if t == nil || t.GetUnix() == 0 {
		return nil
	}
	v := time.Unix(t.GetUnix(), 0).UTC()
	return &v
}

func toTS(t *time.Time) *assetv1.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return &assetv1.Timestamp{Unix: t.Unix()}
}

func inputToPB(in AssetInput) *assetv1.AssetInput {
	return &assetv1.AssetInput{AssetTag: in.AssetTag, Name: in.Name, Serial: in.Serial, ModelName: in.ModelName, ModelNumber: in.ModelNumber,
		CategoryId: in.CategoryID, SupplierId: in.SupplierID, LocationId: in.LocationID, Status: in.Status, WarrantyMonths: int32(in.WarrantyMonths), // #nosec G115
		PurchaseDate: toTS(in.PurchaseDate), OrderNumber: in.OrderNumber, PurchaseCost: in.PurchaseCost, Notes: in.Notes, SalvageValue: in.SalvageValue,
		UsefulLifeYears: int32(in.UsefulLifeYears), DepreciationRate: in.DepreciationRate, Tags: in.Tags} // #nosec G115
}

func toAsset(a *assetv1.Asset) Asset {
	return Asset{ID: a.GetId(), TenantID: a.GetTenantId(), AssetTag: a.GetAssetTag(), Name: a.GetName(), Serial: a.GetSerial(), ModelName: a.GetModelName(), ModelNumber: a.GetModelNumber(),
		CategoryID: a.GetCategoryId(), SupplierID: a.GetSupplierId(), LocationID: a.GetLocationId(), UserID: a.GetUserId(), AssigneeName: a.GetAssigneeName(), Status: a.GetStatus(),
		HasPhoto: a.GetHasPhoto(), WarrantyMonths: int(a.GetWarrantyMonths()), PurchaseDate: fromTSP(a.GetPurchaseDate()), OrderNumber: a.GetOrderNumber(),
		PurchaseCost: a.GetPurchaseCost(), SalvageValue: a.GetSalvageValue(), DepreciationRate: a.GetDepreciationRate(), BookValue: a.GetBookValue(),
		UsefulLifeYears: int(a.GetUsefulLifeYears()), Notes: a.GetNotes(), Tags: a.GetTags(), CreatedAt: fromTS(a.GetCreatedAt()), UpdatedAt: fromTS(a.GetUpdatedAt())}
}

func toChanges(in []*assetv1.SyncChange) []SyncChange {
	out := make([]SyncChange, 0, len(in))
	for _, c := range in {
		out = append(out, SyncChange{HostID: c.GetHostId(), Hostname: c.GetHostname(), Serial: c.GetSerial(), Action: c.GetAction(), AssetID: c.GetAssetId(), AssetTag: c.GetAssetTag(), Changes: c.GetChanges()})
	}
	return out
}

func toCategories(in []*assetv1.Category) []Category {
	out := make([]Category, 0, len(in))
	for _, c := range in {
		out = append(out, Category{ID: c.GetId(), Name: c.GetName(), Description: c.GetDescription(), ParentID: c.GetParentId(), Icon: c.GetIcon(),
			AssetCount: c.GetAssetCount(), ChildCount: c.GetChildCount(), Children: toCategories(c.GetChildren())})
	}
	return out
}

func toLocations(in []*assetv1.Location) []Location {
	out := make([]Location, 0, len(in))
	for _, l := range in {
		out = append(out, Location{ID: l.GetId(), Name: l.GetName(), Code: l.GetCode(), ParentID: l.GetParentId(), Path: l.GetPath(), City: l.GetCity(), Country: l.GetCountry(),
			Status: l.GetStatus(), ChildCount: l.GetChildCount(), AssetCount: l.GetAssetCount(), Children: toLocations(l.GetChildren())})
	}
	return out
}
