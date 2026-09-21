package grpcapi

import (
	"bytes"
	"context"
	"strings"

	assetv1 "github.com/go-freya/freya/services/asset/api/proto/asset/v1"
	"github.com/go-freya/freya/services/asset/internal/documents"
	"github.com/go-freya/freya/services/asset/internal/store"
)

// ---- documents (shared by Asset/Consumable/License services)

func (d Deps) listDocuments(ctx context.Context, typ string, req *assetv1.ListDocumentsRequest) (*assetv1.ListDocumentsResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := d.Documents.List(ctx, subj, typ, req.GetEntityId())
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListDocumentsResponse{Documents: make([]*assetv1.Document, 0, len(rows))}
	for _, r := range rows {
		out.Documents = append(out.Documents, documentToPB(r))
	}
	return out, nil
}

func (d Deps) uploadDocument(ctx context.Context, typ string, req *assetv1.UploadDocumentRequest) (*assetv1.Document, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	content := req.GetContent()
	doc, err := d.Documents.Upload(ctx, subj, typ, req.GetEntityId(), documents.UploadInput{
		FileName: req.GetFileName(), MimeType: req.GetMimeType(), Size: int64(len(content)), Reader: bytes.NewReader(content), Description: req.GetDescription(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return documentToPB(doc), nil
}

func (d Deps) deleteDocument(ctx context.Context, typ string, req *assetv1.DeleteDocumentRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := d.Documents.Delete(ctx, subj, typ, req.GetEntityId(), req.GetDocumentId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

func (d Deps) downloadDocument(ctx context.Context, typ string, req *assetv1.DownloadDocumentRequest) (*assetv1.DownloadDocumentResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	url, err := d.Documents.DownloadURL(ctx, subj, typ, req.GetEntityId(), req.GetDocumentId())
	if err != nil {
		return nil, grpcError(err)
	}
	rc, doc, err := d.Documents.Download(ctx, subj, typ, req.GetEntityId(), req.GetDocumentId())
	if err != nil {
		return nil, grpcError(err)
	}
	_ = rc.Close()
	return &assetv1.DownloadDocumentResponse{Document: documentToPB(doc), Url: url}, nil
}

// ---- AssetService

// AssetServer implements asset.v1.AssetService.
type AssetServer struct {
	assetv1.UnimplementedAssetServiceServer
	d Deps
}

func (s *AssetServer) CreateAsset(ctx context.Context, req *assetv1.CreateAssetRequest) (*assetv1.Asset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assets.Create(ctx, subj, assetFromPB(req.GetAsset()))
	if err != nil {
		return nil, grpcError(err)
	}
	return assetToPB(v), nil
}

func (s *AssetServer) GetAsset(ctx context.Context, req *assetv1.GetAssetRequest) (*assetv1.Asset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assets.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return assetToPB(v), nil
}

func (s *AssetServer) ListAssets(ctx context.Context, req *assetv1.ListAssetsRequest) (*assetv1.ListAssetsResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Assets.List(ctx, subj, store.AssetFilter{Status: req.GetStatus(), CategoryID: req.GetCategoryId(), SupplierID: req.GetSupplierId(),
		LocationID: req.GetLocationId(), UserID: req.GetUserId(), Query: req.GetQuery(), Limit: int(req.GetLimit()), CursorID: req.GetCursorId()})
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListAssetsResponse{Assets: make([]*assetv1.Asset, 0, len(rows))}
	for _, v := range rows {
		out.Assets = append(out.Assets, assetToPB(v))
	}
	return out, nil
}

func (s *AssetServer) UpdateAsset(ctx context.Context, req *assetv1.UpdateAssetRequest) (*assetv1.Asset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assets.Update(ctx, subj, req.GetId(), assetFromPB(req.GetAsset()))
	if err != nil {
		return nil, grpcError(err)
	}
	return assetToPB(v), nil
}

func (s *AssetServer) DeleteAsset(ctx context.Context, req *assetv1.DeleteAssetRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Assets.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

func (s *AssetServer) AssignAsset(ctx context.Context, req *assetv1.AssignAssetRequest) (*assetv1.Asset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assets.Assign(ctx, subj, req.GetId(), req.GetUserId(), req.GetNotes())
	if err != nil {
		return nil, grpcError(err)
	}
	return assetToPB(v), nil
}

func (s *AssetServer) UnassignAsset(ctx context.Context, req *assetv1.UnassignAssetRequest) (*assetv1.Asset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Assets.Unassign(ctx, subj, req.GetId(), req.GetLocationId(), req.GetNotes())
	if err != nil {
		return nil, grpcError(err)
	}
	return assetToPB(v), nil
}

func (s *AssetServer) GetAssignmentHistory(ctx context.Context, req *assetv1.GetAssignmentHistoryRequest) (*assetv1.GetAssignmentHistoryResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Assets.Assignments(ctx, subj, req.GetId(), int(req.GetLimit()))
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.GetAssignmentHistoryResponse{Assignments: make([]*assetv1.Assignment, 0, len(rows))}
	for _, g := range rows {
		out.Assignments = append(out.Assignments, assignmentToPB(g))
	}
	return out, nil
}

func (s *AssetServer) UploadPhoto(ctx context.Context, req *assetv1.UploadPhotoRequest) (*assetv1.Asset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	content := req.GetContent()
	if _, err := s.d.Documents.UploadPhoto(ctx, subj, req.GetId(), documents.UploadInput{FileName: "photo", MimeType: req.GetMimeType(), Size: int64(len(content)), Reader: bytes.NewReader(content)}); err != nil {
		return nil, grpcError(err)
	}
	v, err := s.d.Assets.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return assetToPB(v), nil
}

func (s *AssetServer) DeletePhoto(ctx context.Context, req *assetv1.DeletePhotoRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Documents.DeletePhoto(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

func (s *AssetServer) ListDocuments(ctx context.Context, req *assetv1.ListDocumentsRequest) (*assetv1.ListDocumentsResponse, error) {
	return s.d.listDocuments(ctx, store.EntityAsset, req)
}
func (s *AssetServer) UploadDocument(ctx context.Context, req *assetv1.UploadDocumentRequest) (*assetv1.Document, error) {
	return s.d.uploadDocument(ctx, store.EntityAsset, req)
}
func (s *AssetServer) DeleteDocument(ctx context.Context, req *assetv1.DeleteDocumentRequest) (*assetv1.Empty, error) {
	return s.d.deleteDocument(ctx, store.EntityAsset, req)
}
func (s *AssetServer) DownloadDocument(ctx context.Context, req *assetv1.DownloadDocumentRequest) (*assetv1.DownloadDocumentResponse, error) {
	return s.d.downloadDocument(ctx, store.EntityAsset, req)
}

func (s *AssetServer) InventorySyncPreview(ctx context.Context, req *assetv1.InventorySyncPreviewRequest) (*assetv1.InventorySyncPreviewResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if s.d.Sync == nil {
		return nil, grpcError(errUnavailableSync)
	}
	p, err := s.d.Sync.Preview(ctx, subj)
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.InventorySyncPreviewResponse{Hosts: int32(p.Hosts), Create: int32(p.Create), Update: int32(p.Update), Unchanged: int32(p.Unchanged)} // #nosec G115 -- counts fit
	for _, c := range p.Changes {
		out.Changes = append(out.Changes, syncChangeToPB(c))
	}
	return out, nil
}

func (s *AssetServer) InventorySyncExecute(ctx context.Context, req *assetv1.InventorySyncExecuteRequest) (*assetv1.InventorySyncExecuteResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if s.d.Sync == nil {
		return nil, grpcError(errUnavailableSync)
	}
	r, err := s.d.Sync.Execute(ctx, subj, req.GetHostnames())
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.InventorySyncExecuteResponse{Created: int32(r.Created), Updated: int32(r.Updated), Skipped: int32(r.Skipped), Errors: r.Errors} // #nosec G115 -- counts fit
	for _, c := range r.Changes {
		out.Changes = append(out.Changes, syncChangeToPB(c))
	}
	return out, nil
}

// ---- CategoryService

// CategoryServer implements asset.v1.CategoryService.
type CategoryServer struct {
	assetv1.UnimplementedCategoryServiceServer
	d Deps
}

func (s *CategoryServer) CreateCategory(ctx context.Context, req *assetv1.CreateCategoryRequest) (*assetv1.Category, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	c, err := s.d.Categories.Create(ctx, subj, categoryFromPB(req.GetCategory()))
	if err != nil {
		return nil, grpcError(err)
	}
	return categoryToPB(c), nil
}

func (s *CategoryServer) GetCategory(ctx context.Context, req *assetv1.GetCategoryRequest) (*assetv1.Category, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	c, err := s.d.Categories.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return categoryToPB(c), nil
}

func (s *CategoryServer) ListCategories(ctx context.Context, req *assetv1.ListCategoriesRequest) (*assetv1.ListCategoriesResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Categories.List(ctx, subj)
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListCategoriesResponse{}
	for _, c := range rows {
		out.Categories = append(out.Categories, categoryToPB(c))
	}
	return out, nil
}

func (s *CategoryServer) UpdateCategory(ctx context.Context, req *assetv1.UpdateCategoryRequest) (*assetv1.Category, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	c, err := s.d.Categories.Update(ctx, subj, req.GetId(), categoryFromPB(req.GetCategory()))
	if err != nil {
		return nil, grpcError(err)
	}
	return categoryToPB(c), nil
}

func (s *CategoryServer) DeleteCategory(ctx context.Context, req *assetv1.DeleteCategoryRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Categories.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

func (s *CategoryServer) GetCategoryTree(ctx context.Context, req *assetv1.GetCategoryTreeRequest) (*assetv1.GetCategoryTreeResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	tree, err := s.d.Categories.Tree(ctx, subj)
	if err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.GetCategoryTreeResponse{Roots: categoryTreeToPB(tree)}, nil
}

// ---- SupplierService

// SupplierServer implements asset.v1.SupplierService.
type SupplierServer struct {
	assetv1.UnimplementedSupplierServiceServer
	d Deps
}

func (s *SupplierServer) CreateSupplier(ctx context.Context, req *assetv1.CreateSupplierRequest) (*assetv1.Supplier, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Suppliers.Create(ctx, subj, supplierFromPB(req.GetSupplier()))
	if err != nil {
		return nil, grpcError(err)
	}
	return supplierToPB(v), nil
}

func (s *SupplierServer) GetSupplier(ctx context.Context, req *assetv1.GetSupplierRequest) (*assetv1.Supplier, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Suppliers.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return supplierToPB(v), nil
}

func (s *SupplierServer) ListSuppliers(ctx context.Context, req *assetv1.ListSuppliersRequest) (*assetv1.ListSuppliersResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Suppliers.List(ctx, subj, store.ListOpts{Query: req.GetQuery(), Limit: int(req.GetLimit()), CursorID: req.GetCursorId()})
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListSuppliersResponse{}
	for _, v := range rows {
		out.Suppliers = append(out.Suppliers, supplierToPB(v))
	}
	return out, nil
}

func (s *SupplierServer) UpdateSupplier(ctx context.Context, req *assetv1.UpdateSupplierRequest) (*assetv1.Supplier, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Suppliers.Update(ctx, subj, req.GetId(), supplierFromPB(req.GetSupplier()))
	if err != nil {
		return nil, grpcError(err)
	}
	return supplierToPB(v), nil
}

func (s *SupplierServer) DeleteSupplier(ctx context.Context, req *assetv1.DeleteSupplierRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Suppliers.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

// ---- LocationService

// LocationServer implements asset.v1.LocationService.
type LocationServer struct {
	assetv1.UnimplementedLocationServiceServer
	d Deps
}

func (s *LocationServer) CreateLocation(ctx context.Context, req *assetv1.CreateLocationRequest) (*assetv1.Location, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Locations.Create(ctx, subj, locationFromPB(req.GetLocation()))
	if err != nil {
		return nil, grpcError(err)
	}
	return locationToPB(v), nil
}

func (s *LocationServer) GetLocation(ctx context.Context, req *assetv1.GetLocationRequest) (*assetv1.Location, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Locations.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return locationToPB(v), nil
}

func (s *LocationServer) ListLocations(ctx context.Context, req *assetv1.ListLocationsRequest) (*assetv1.ListLocationsResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Locations.List(ctx, subj)
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListLocationsResponse{}
	for _, v := range rows {
		out.Locations = append(out.Locations, locationToPB(v))
	}
	return out, nil
}

func (s *LocationServer) UpdateLocation(ctx context.Context, req *assetv1.UpdateLocationRequest) (*assetv1.Location, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Locations.Update(ctx, subj, req.GetId(), locationFromPB(req.GetLocation()))
	if err != nil {
		return nil, grpcError(err)
	}
	return locationToPB(v), nil
}

func (s *LocationServer) DeleteLocation(ctx context.Context, req *assetv1.DeleteLocationRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Locations.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

func (s *LocationServer) GetLocationTree(ctx context.Context, req *assetv1.GetLocationTreeRequest) (*assetv1.GetLocationTreeResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	tree, err := s.d.Locations.Tree(ctx, subj)
	if err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.GetLocationTreeResponse{Roots: locationTreeToPB(tree)}, nil
}

// ---- ConsumableService

// ConsumableServer implements asset.v1.ConsumableService.
type ConsumableServer struct {
	assetv1.UnimplementedConsumableServiceServer
	d Deps
}

func (s *ConsumableServer) CreateConsumable(ctx context.Context, req *assetv1.CreateConsumableRequest) (*assetv1.Consumable, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Consumables.Create(ctx, subj, consumableFromPB(req.GetConsumable()))
	if err != nil {
		return nil, grpcError(err)
	}
	return consumableToPB(v), nil
}

func (s *ConsumableServer) GetConsumable(ctx context.Context, req *assetv1.GetConsumableRequest) (*assetv1.Consumable, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Consumables.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return consumableToPB(v), nil
}

func (s *ConsumableServer) ListConsumables(ctx context.Context, req *assetv1.ListConsumablesRequest) (*assetv1.ListConsumablesResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Consumables.List(ctx, subj, store.ListOpts{Query: req.GetQuery(), Limit: int(req.GetLimit()), CursorID: req.GetCursorId()})
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListConsumablesResponse{}
	for _, v := range rows {
		out.Consumables = append(out.Consumables, consumableToPB(v))
	}
	return out, nil
}

func (s *ConsumableServer) UpdateConsumable(ctx context.Context, req *assetv1.UpdateConsumableRequest) (*assetv1.Consumable, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Consumables.Update(ctx, subj, req.GetId(), consumableFromPB(req.GetConsumable()))
	if err != nil {
		return nil, grpcError(err)
	}
	return consumableToPB(v), nil
}

func (s *ConsumableServer) DeleteConsumable(ctx context.Context, req *assetv1.DeleteConsumableRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	var docs []store.Document
	if s.d.Documents != nil {
		docs, _ = s.d.Documents.List(ctx, subj, store.EntityConsumable, req.GetId())
	}
	if err := s.d.Consumables.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	if s.d.Documents != nil {
		s.d.Documents.PurgeRows(ctx, subj.TenantID, docs)
	}
	return &assetv1.Empty{}, nil
}

func (s *ConsumableServer) ListDocuments(ctx context.Context, req *assetv1.ListDocumentsRequest) (*assetv1.ListDocumentsResponse, error) {
	return s.d.listDocuments(ctx, store.EntityConsumable, req)
}
func (s *ConsumableServer) UploadDocument(ctx context.Context, req *assetv1.UploadDocumentRequest) (*assetv1.Document, error) {
	return s.d.uploadDocument(ctx, store.EntityConsumable, req)
}
func (s *ConsumableServer) DeleteDocument(ctx context.Context, req *assetv1.DeleteDocumentRequest) (*assetv1.Empty, error) {
	return s.d.deleteDocument(ctx, store.EntityConsumable, req)
}
func (s *ConsumableServer) DownloadDocument(ctx context.Context, req *assetv1.DownloadDocumentRequest) (*assetv1.DownloadDocumentResponse, error) {
	return s.d.downloadDocument(ctx, store.EntityConsumable, req)
}

// ---- LicenseService

// LicenseServer implements asset.v1.LicenseService.
type LicenseServer struct {
	assetv1.UnimplementedLicenseServiceServer
	d Deps
}

func (s *LicenseServer) CreateLicense(ctx context.Context, req *assetv1.CreateLicenseRequest) (*assetv1.License, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Licenses.Create(ctx, subj, licenseFromPB(req.GetLicense()))
	if err != nil {
		return nil, grpcError(err)
	}
	return licenseToPB(v), nil
}

func (s *LicenseServer) GetLicense(ctx context.Context, req *assetv1.GetLicenseRequest) (*assetv1.License, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Licenses.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return licenseToPB(v), nil
}

func (s *LicenseServer) ListLicenses(ctx context.Context, req *assetv1.ListLicensesRequest) (*assetv1.ListLicensesResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Licenses.List(ctx, subj, store.ListOpts{Query: req.GetQuery(), Limit: int(req.GetLimit()), CursorID: req.GetCursorId()})
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListLicensesResponse{}
	for _, v := range rows {
		out.Licenses = append(out.Licenses, licenseToPB(v))
	}
	return out, nil
}

func (s *LicenseServer) UpdateLicense(ctx context.Context, req *assetv1.UpdateLicenseRequest) (*assetv1.License, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Licenses.Update(ctx, subj, req.GetId(), licenseFromPB(req.GetLicense()))
	if err != nil {
		return nil, grpcError(err)
	}
	return licenseToPB(v), nil
}

func (s *LicenseServer) DeleteLicense(ctx context.Context, req *assetv1.DeleteLicenseRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	var docs []store.Document
	if s.d.Documents != nil {
		docs, _ = s.d.Documents.List(ctx, subj, store.EntityLicense, req.GetId())
	}
	if err := s.d.Licenses.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	if s.d.Documents != nil {
		s.d.Documents.PurgeRows(ctx, subj.TenantID, docs)
	}
	return &assetv1.Empty{}, nil
}

func (s *LicenseServer) ListDocuments(ctx context.Context, req *assetv1.ListDocumentsRequest) (*assetv1.ListDocumentsResponse, error) {
	return s.d.listDocuments(ctx, store.EntityLicense, req)
}
func (s *LicenseServer) UploadDocument(ctx context.Context, req *assetv1.UploadDocumentRequest) (*assetv1.Document, error) {
	return s.d.uploadDocument(ctx, store.EntityLicense, req)
}
func (s *LicenseServer) DeleteDocument(ctx context.Context, req *assetv1.DeleteDocumentRequest) (*assetv1.Empty, error) {
	return s.d.deleteDocument(ctx, store.EntityLicense, req)
}
func (s *LicenseServer) DownloadDocument(ctx context.Context, req *assetv1.DownloadDocumentRequest) (*assetv1.DownloadDocumentResponse, error) {
	return s.d.downloadDocument(ctx, store.EntityLicense, req)
}

// ---- InsurancePolicyService

// InsuranceServer implements asset.v1.InsurancePolicyService.
type InsuranceServer struct {
	assetv1.UnimplementedInsurancePolicyServiceServer
	d Deps
}

func (s *InsuranceServer) CreateInsurancePolicy(ctx context.Context, req *assetv1.CreateInsurancePolicyRequest) (*assetv1.InsurancePolicy, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Insurance.Create(ctx, subj, policyFromPB(req.GetPolicy()))
	if err != nil {
		return nil, grpcError(err)
	}
	return policyToPB(v), nil
}

func (s *InsuranceServer) GetInsurancePolicy(ctx context.Context, req *assetv1.GetInsurancePolicyRequest) (*assetv1.InsurancePolicy, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Insurance.Get(ctx, subj, req.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return policyToPB(v), nil
}

func (s *InsuranceServer) ListInsurancePolicies(ctx context.Context, req *assetv1.ListInsurancePoliciesRequest) (*assetv1.ListInsurancePoliciesResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Insurance.List(ctx, subj, store.ListOpts{Query: req.GetQuery(), Limit: int(req.GetLimit()), CursorID: req.GetCursorId()})
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListInsurancePoliciesResponse{}
	for _, v := range rows {
		out.Policies = append(out.Policies, policyToPB(v))
	}
	return out, nil
}

func (s *InsuranceServer) UpdateInsurancePolicy(ctx context.Context, req *assetv1.UpdateInsurancePolicyRequest) (*assetv1.InsurancePolicy, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	v, err := s.d.Insurance.Update(ctx, subj, req.GetId(), policyFromPB(req.GetPolicy()))
	if err != nil {
		return nil, grpcError(err)
	}
	return policyToPB(v), nil
}

func (s *InsuranceServer) DeleteInsurancePolicy(ctx context.Context, req *assetv1.DeleteInsurancePolicyRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Insurance.Delete(ctx, subj, req.GetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

func (s *InsuranceServer) ListPolicyAssets(ctx context.Context, req *assetv1.ListPolicyAssetsRequest) (*assetv1.ListPolicyAssetsResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Insurance.ListPolicyAssets(ctx, subj, req.GetPolicyId())
	if err != nil {
		return nil, grpcError(err)
	}
	out := &assetv1.ListPolicyAssetsResponse{}
	for _, pa := range rows {
		out.Assets = append(out.Assets, policyAssetToPB(pa))
	}
	return out, nil
}

func (s *InsuranceServer) AddAssetToPolicy(ctx context.Context, req *assetv1.AddAssetToPolicyRequest) (*assetv1.PolicyAsset, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	pa, err := s.d.Insurance.AddAssetToPolicy(ctx, subj, req.GetPolicyId(), req.GetAssetId(), req.GetCoveredValue(), req.GetNotes())
	if err != nil {
		return nil, grpcError(err)
	}
	return policyAssetToPB(pa), nil
}

func (s *InsuranceServer) RemoveAssetFromPolicy(ctx context.Context, req *assetv1.RemoveAssetFromPolicyRequest) (*assetv1.Empty, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if err := s.d.Insurance.RemoveAssetFromPolicy(ctx, subj, req.GetPolicyId(), req.GetAssetId()); err != nil {
		return nil, grpcError(err)
	}
	return &assetv1.Empty{}, nil
}

// ---- UserService + SystemService

// UserServer implements asset.v1.UserService.
type UserServer struct {
	assetv1.UnimplementedUserServiceServer
	d Deps
}

func (s *UserServer) ListUsers(ctx context.Context, req *assetv1.ListUsersRequest) (*assetv1.ListUsersResponse, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	out := &assetv1.ListUsersResponse{}
	if s.d.Users == nil {
		return out, nil
	}
	users, err := s.d.Users.ListUsers(ctx, subj.TenantID)
	if err != nil {
		return nil, grpcError(err)
	}
	q := strings.ToLower(req.GetQuery())
	for _, u := range users {
		if q != "" && !strings.Contains(strings.ToLower(u.DisplayName), q) && !strings.Contains(strings.ToLower(u.ID), q) {
			continue
		}
		out.Users = append(out.Users, &assetv1.User{Id: u.ID, DisplayName: u.DisplayName, AvatarUrl: u.AvatarURL})
	}
	return out, nil
}

// SystemServer implements asset.v1.SystemService.
type SystemServer struct {
	assetv1.UnimplementedSystemServiceServer
	d Deps
}

func (s *SystemServer) Health(_ context.Context, _ *assetv1.HealthRequest) (*assetv1.HealthResponse, error) {
	out := &assetv1.HealthResponse{Status: "ok"}
	if s.d.Health != nil {
		out.Components = s.d.Health()
		for _, v := range out.Components {
			if v != "ok" {
				out.Status = "degraded"
			}
		}
	}
	return out, nil
}

func (s *SystemServer) GetDashboardStats(ctx context.Context, req *assetv1.GetDashboardStatsRequest) (*assetv1.DashboardStats, error) {
	subj, err := caller(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}
	if s.d.Stats == nil {
		return nil, grpcError(errUnavailableSync)
	}
	d, err := s.d.Stats.Get(ctx, subj)
	if err != nil {
		return nil, grpcError(err)
	}
	return statsToPB(d), nil
}
