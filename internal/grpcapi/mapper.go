package grpcapi

import (
	"time"

	assetv1 "github.com/go-freya/freya/services/asset/api/proto/asset/v1"
	"github.com/go-freya/freya/services/asset/internal/assets"
	"github.com/go-freya/freya/services/asset/internal/categories"
	"github.com/go-freya/freya/services/asset/internal/consumables"
	"github.com/go-freya/freya/services/asset/internal/insurance"
	"github.com/go-freya/freya/services/asset/internal/invsync"
	"github.com/go-freya/freya/services/asset/internal/licenses"
	"github.com/go-freya/freya/services/asset/internal/locations"
	"github.com/go-freya/freya/services/asset/internal/stats"
	"github.com/go-freya/freya/services/asset/internal/store"
	"github.com/go-freya/freya/services/asset/internal/suppliers"
)

func ts(t time.Time) *assetv1.Timestamp {
	if t.IsZero() {
		return nil
	}
	return &assetv1.Timestamp{Unix: t.Unix()}
}

func tsp(t *time.Time) *assetv1.Timestamp {
	if t == nil {
		return nil
	}
	return ts(*t)
}

func fromTS(t *assetv1.Timestamp) *time.Time {
	if t == nil || t.GetUnix() == 0 {
		return nil
	}
	v := time.Unix(t.GetUnix(), 0).UTC()
	return &v
}

func assetToPB(v assets.View) *assetv1.Asset {
	a := v.Asset
	return &assetv1.Asset{
		Id: a.ID, TenantId: a.TenantID, AssetTag: a.AssetTag, Name: a.Name, Serial: a.Serial, ModelName: a.ModelName, ModelNumber: a.ModelNumber,
		CategoryId: a.CategoryID, SupplierId: a.SupplierID, LocationId: a.LocationID, UserId: a.UserID, AssigneeName: v.AssigneeName, Status: a.Status,
		HasPhoto: v.HasPhoto, WarrantyMonths: int32(a.WarrantyMonths), PurchaseDate: tsp(a.PurchaseDate), OrderNumber: a.OrderNumber, // #nosec G115 -- bounded domain value
		PurchaseCost: a.PurchaseCost, Notes: a.Notes, SalvageValue: a.SalvageValue, UsefulLifeYears: int32(a.UsefulLifeYears), // #nosec G115
		DepreciationRate: a.DepreciationRate, Tags: a.Tags, BookValue: a.BookValue, CreatedBy: a.CreatedBy, UpdatedBy: a.UpdatedBy,
		CreatedAt: ts(a.CreatedAt), UpdatedAt: ts(a.UpdatedAt),
	}
}

func assetFromPB(in *assetv1.AssetInput) assets.Input {
	if in == nil {
		return assets.Input{}
	}
	return assets.Input{
		AssetTag: in.GetAssetTag(), Name: in.GetName(), Serial: in.GetSerial(), ModelName: in.GetModelName(), ModelNumber: in.GetModelNumber(),
		CategoryID: in.GetCategoryId(), SupplierID: in.GetSupplierId(), LocationID: in.GetLocationId(), Status: in.GetStatus(),
		WarrantyMonths: int(in.GetWarrantyMonths()), PurchaseDate: fromTS(in.GetPurchaseDate()), OrderNumber: in.GetOrderNumber(),
		PurchaseCost: in.GetPurchaseCost(), Notes: in.GetNotes(), SalvageValue: in.GetSalvageValue(), UsefulLifeYears: int(in.GetUsefulLifeYears()),
		DepreciationRate: in.GetDepreciationRate(), Tags: in.GetTags(),
	}
}

func assignmentToPB(g store.Assignment) *assetv1.Assignment {
	return &assetv1.Assignment{Id: g.ID, AssetId: g.AssetID, AssetName: g.AssetName, UserId: g.UserID, UserName: g.UserName, Action: g.Action,
		AssignedAt: ts(g.AssignedAt), ReturnedAt: tsp(g.ReturnedAt), AssignedBy: g.AssignedBy, Notes: g.Notes}
}

func documentToPB(d store.Document) *assetv1.Document {
	return &assetv1.Document{Id: d.ID, EntityType: d.EntityType, EntityId: d.EntityID, FileName: d.FileName, FileSize: d.FileSize, MimeType: d.MimeType,
		Checksum: d.Checksum, Description: d.Description, UploadedBy: d.UploadedBy, CreatedAt: ts(d.CreatedAt)}
}

func syncChangeToPB(c invsync.Change) *assetv1.SyncChange {
	out := &assetv1.SyncChange{HostId: c.HostID, Hostname: c.Hostname, Serial: c.Serial, Action: c.Action, AssetId: c.AssetID, AssetTag: c.AssetTag}
	if len(c.Changes) > 0 {
		out.Changes = map[string]string{}
		for k, v := range c.Changes {
			out.Changes[k] = v.Old + " → " + v.New
		}
	}
	return out
}

func categoryToPB(c store.Category) *assetv1.Category {
	return &assetv1.Category{Id: c.ID, Name: c.Name, Description: c.Description, ParentId: c.ParentID, Icon: c.Icon, Tags: c.Tags,
		AssetCount: c.AssetCount, ChildCount: c.ChildCount, CreatedAt: ts(c.CreatedAt), UpdatedAt: ts(c.UpdatedAt)}
}

func categoryTreeToPB(nodes []*categories.Node) []*assetv1.Category {
	out := make([]*assetv1.Category, 0, len(nodes))
	for _, n := range nodes {
		c := categoryToPB(n.Category)
		c.Children = categoryTreeToPB(n.Children)
		out = append(out, c)
	}
	return out
}

func categoryFromPB(in *assetv1.CategoryInput) categories.Input {
	return categories.Input{Name: in.GetName(), Description: in.GetDescription(), ParentID: in.GetParentId(), Icon: in.GetIcon(), Tags: in.GetTags()}
}

func supplierToPB(s store.Supplier) *assetv1.Supplier {
	return &assetv1.Supplier{Id: s.ID, Name: s.Name, Code: s.Code, Address: s.Address, City: s.City, State: s.State, Country: s.Country, PostalCode: s.PostalCode,
		ContactPerson: s.ContactPerson, Telephone: s.Telephone, Email: s.Email, Website: s.Website, Notes: s.Notes, Status: s.Status, Tags: s.Tags,
		CreatedAt: ts(s.CreatedAt), UpdatedAt: ts(s.UpdatedAt)}
}

func supplierFromPB(in *assetv1.SupplierInput) suppliers.Input {
	return suppliers.Input{Name: in.GetName(), Code: in.GetCode(), Address: in.GetAddress(), City: in.GetCity(), State: in.GetState(), Country: in.GetCountry(),
		PostalCode: in.GetPostalCode(), ContactPerson: in.GetContactPerson(), Telephone: in.GetTelephone(), Email: in.GetEmail(), Website: in.GetWebsite(),
		Notes: in.GetNotes(), Status: in.GetStatus(), Tags: in.GetTags()}
}

func locationToPB(l store.Location) *assetv1.Location {
	return &assetv1.Location{Id: l.ID, Name: l.Name, Code: l.Code, Description: l.Description, ParentId: l.ParentID, Path: l.Path, Address: l.Address, City: l.City,
		State: l.State, Country: l.Country, PostalCode: l.PostalCode, Contact: l.Contact, Phone: l.Phone, Email: l.Email, Status: l.Status, Tags: l.Tags,
		ChildCount: l.ChildCount, AssetCount: l.AssetCount, CreatedAt: ts(l.CreatedAt), UpdatedAt: ts(l.UpdatedAt)}
}

func locationTreeToPB(nodes []*locations.Node) []*assetv1.Location {
	out := make([]*assetv1.Location, 0, len(nodes))
	for _, n := range nodes {
		l := locationToPB(n.Location)
		l.Children = locationTreeToPB(n.Children)
		out = append(out, l)
	}
	return out
}

func locationFromPB(in *assetv1.LocationInput) locations.Input {
	return locations.Input{Name: in.GetName(), Code: in.GetCode(), Description: in.GetDescription(), ParentID: in.GetParentId(), Address: in.GetAddress(),
		City: in.GetCity(), State: in.GetState(), Country: in.GetCountry(), PostalCode: in.GetPostalCode(), Contact: in.GetContact(), Phone: in.GetPhone(),
		Email: in.GetEmail(), Status: in.GetStatus(), Tags: in.GetTags()}
}

func consumableToPB(v consumables.View) *assetv1.Consumable {
	c := v.Consumable
	return &assetv1.Consumable{Id: c.ID, Name: c.Name, Description: c.Description, CategoryId: c.CategoryID, SupplierId: c.SupplierID, LocationId: c.LocationID,
		ModelName: c.ModelName, ModelNumber: c.ModelNumber, Amount: int32(c.Amount), MinAmount: int32(c.MinAmount), PurchaseDate: tsp(c.PurchaseDate), // #nosec G115
		PurchaseCost: c.PurchaseCost, OrderNumber: c.OrderNumber, Notes: c.Notes, Tags: c.Tags, LowStock: v.LowStock, CreatedAt: ts(c.CreatedAt), UpdatedAt: ts(c.UpdatedAt)}
}

func consumableFromPB(in *assetv1.ConsumableInput) consumables.Input {
	return consumables.Input{Name: in.GetName(), Description: in.GetDescription(), CategoryID: in.GetCategoryId(), SupplierID: in.GetSupplierId(),
		LocationID: in.GetLocationId(), ModelName: in.GetModelName(), ModelNumber: in.GetModelNumber(), Amount: int(in.GetAmount()), MinAmount: int(in.GetMinAmount()),
		PurchaseDate: fromTS(in.GetPurchaseDate()), PurchaseCost: in.GetPurchaseCost(), OrderNumber: in.GetOrderNumber(), Notes: in.GetNotes(), Tags: in.GetTags()}
}

func licenseToPB(l store.License) *assetv1.License {
	return &assetv1.License{Id: l.ID, Name: l.Name, SupplierId: l.SupplierID, PurchaseDate: tsp(l.PurchaseDate), PurchaseCost: l.PurchaseCost, OrderNumber: l.OrderNumber,
		ValidFrom: tsp(l.ValidFrom), ValidTo: tsp(l.ValidTo), Notes: l.Notes, Status: l.Status, Metadata: l.Metadata, CreatedAt: ts(l.CreatedAt), UpdatedAt: ts(l.UpdatedAt)}
}

func licenseFromPB(in *assetv1.LicenseInput) licenses.Input {
	return licenses.Input{Name: in.GetName(), SupplierID: in.GetSupplierId(), PurchaseDate: fromTS(in.GetPurchaseDate()), PurchaseCost: in.GetPurchaseCost(),
		OrderNumber: in.GetOrderNumber(), ValidFrom: fromTS(in.GetValidFrom()), ValidTo: fromTS(in.GetValidTo()), Notes: in.GetNotes(), Status: in.GetStatus(), Metadata: in.GetMetadata()}
}

func policyToPB(p store.InsurancePolicy) *assetv1.InsurancePolicy {
	return &assetv1.InsurancePolicy{Id: p.ID, Name: p.Name, PolicyNumber: p.PolicyNumber, Provider: p.Provider, CoverageType: p.CoverageType, PremiumAmount: p.PremiumAmount,
		Deductible: p.Deductible, CoverageLimit: p.CoverageLimit, ValidFrom: tsp(p.ValidFrom), ValidTo: tsp(p.ValidTo), Status: p.Status, Notes: p.Notes, Metadata: p.Metadata,
		AssetCount: p.AssetCount, CreatedAt: ts(p.CreatedAt), UpdatedAt: ts(p.UpdatedAt)}
}

func policyFromPB(in *assetv1.InsurancePolicyInput) insurance.Input {
	return insurance.Input{Name: in.GetName(), PolicyNumber: in.GetPolicyNumber(), Provider: in.GetProvider(), CoverageType: in.GetCoverageType(),
		PremiumAmount: in.GetPremiumAmount(), Deductible: in.GetDeductible(), CoverageLimit: in.GetCoverageLimit(), ValidFrom: fromTS(in.GetValidFrom()),
		ValidTo: fromTS(in.GetValidTo()), Status: in.GetStatus(), Notes: in.GetNotes(), Metadata: in.GetMetadata()}
}

func policyAssetToPB(pa store.PolicyAsset) *assetv1.PolicyAsset {
	return &assetv1.PolicyAsset{Id: pa.ID, PolicyId: pa.PolicyID, AssetId: pa.AssetID, CoveredValue: pa.CoveredValue, Notes: pa.Notes, AssetTag: pa.AssetTag,
		AssetName: pa.AssetName, ModelName: pa.ModelName, CreatedBy: pa.CreatedBy, CreatedAt: ts(pa.CreatedAt)}
}

func statsToPB(d stats.Dashboard) *assetv1.DashboardStats {
	return &assetv1.DashboardStats{TotalAssets: d.TotalAssets, AssetsByStatus: d.AssetsByStatus, TotalConsumables: d.TotalConsumables, TotalSuppliers: d.TotalSuppliers,
		TotalCategories: d.TotalCategories, TotalLocations: d.TotalLocations, TotalLicenses: d.TotalLicenses, TotalInsurancePolicies: d.TotalInsurance,
		TotalCost: d.TotalCost, TotalDepreciatedValue: d.TotalDepreciatedValue, ExpiringSoon: d.ExpiringSoon, LowStock: d.LowStock,
		WarrantyExpiringSoon: d.WarrantyExpiringSoon, LicensesExpiringSoon: d.LicensesExpiringSoon, InsuranceExpiringSoon: d.InsuranceExpiringSoon, AssignedAssets: d.AssignedAssets}
}
