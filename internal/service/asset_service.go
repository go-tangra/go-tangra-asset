package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

// generateAssetTag creates a random asset tag in the format AST-xxxxxx (6 hex chars).
func generateAssetTag() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "AST-" + hex.EncodeToString(b)
}

type AssetService struct {
	assetV1.UnimplementedAssetServiceServer

	log             *log.Helper
	assetRepo       *data.AssetRepo
	assignmentRepo  *data.AssignmentRepo
	documentRepo    *data.DocumentRepo
	employeeRepo    *data.EmployeeRepo
	storageClient   *data.StorageClient
	inventoryClient *data.InventoryClient
}

func NewAssetService(
	ctx *bootstrap.Context,
	assetRepo *data.AssetRepo,
	assignmentRepo *data.AssignmentRepo,
	documentRepo *data.DocumentRepo,
	employeeRepo *data.EmployeeRepo,
	storageClient *data.StorageClient,
	inventoryClient *data.InventoryClient,
) *AssetService {
	return &AssetService{
		log:             ctx.NewLoggerHelper("asset/service/asset"),
		assetRepo:       assetRepo,
		assignmentRepo:  assignmentRepo,
		documentRepo:    documentRepo,
		employeeRepo:    employeeRepo,
		storageClient:   storageClient,
		inventoryClient: inventoryClient,
	}
}

func (s *AssetService) CreateAsset(ctx context.Context, req *assetV1.CreateAssetRequest) (*assetV1.CreateAssetResponse, error) {
	// Auto-generate asset tag if not provided
	assetTag := req.GetAssetTag()
	if assetTag == "" {
		assetTag = generateAssetTag()
	}

	name := req.GetName()

	opts := []func(*ent.AssetCreate){}

	if req.Serial != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetSerial(*req.Serial) })
	}
	if req.ModelName != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetModelName(*req.ModelName) })
	}
	if req.ModelNumber != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetModelNumber(*req.ModelNumber) })
	}
	if req.CategoryId != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetCategoryID(*req.CategoryId) })
	}
	if req.SupplierId != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetSupplierID(*req.SupplierId) })
	}
	if req.LocationId != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetLocationID(*req.LocationId) })
	}
	if req.Status != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetStatus(int32(*req.Status)) })
	}
	if req.WarrantyMonths != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetWarrantyMonths(*req.WarrantyMonths) })
	}
	if req.PurchaseDate != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetPurchaseDate(req.PurchaseDate.AsTime()) })
	}
	if req.OrderNumber != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetOrderNumber(*req.OrderNumber) })
	}
	if req.PurchaseCost != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetPurchaseCost(*req.PurchaseCost) })
	}
	if req.Notes != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetNotes(*req.Notes) })
	}
	if req.Tags != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetTags(*req.Tags) })
	}
	if req.Metadata != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetMetadata(req.Metadata.AsMap()) })
	}
	if req.SalvageValue != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetSalvageValue(*req.SalvageValue) })
	}
	if req.UsefulLifeYears != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetUsefulLifeYears(*req.UsefulLifeYears) })
	}
	if req.DepreciationRate != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetDepreciationRate(*req.DepreciationRate) })
	}

	entity, err := s.assetRepo.Create(ctx, req.GetTenantId(), assetTag, name, opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateAssetResponse{
		Asset: assetToProto(entity),
	}, nil
}

func (s *AssetService) GetAsset(ctx context.Context, req *assetV1.GetAssetRequest) (*assetV1.GetAssetResponse, error) {
	entity, err := s.assetRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}

	return &assetV1.GetAssetResponse{
		Asset: assetToProto(entity),
	}, nil
}

func (s *AssetService) ListAssets(ctx context.Context, req *assetV1.ListAssetsRequest) (*assetV1.ListAssetsResponse, error) {
	filters := make(map[string]interface{})
	if req.Status != nil {
		filters["status"] = int32(*req.Status)
	}
	if req.CategoryId != nil {
		filters["category_id"] = *req.CategoryId
	}
	if req.SupplierId != nil {
		filters["supplier_id"] = *req.SupplierId
	}
	if req.LocationId != nil {
		filters["location_id"] = *req.LocationId
	}
	if req.EmployeeId != nil {
		filters["employee_id"] = *req.EmployeeId
	}
	if req.Query != nil {
		filters["query"] = *req.Query
	}

	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())
	if req.GetNoPaging() {
		page = 0
		pageSize = 0
	}

	entities, total, err := s.assetRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.Asset, len(entities))
	for i, e := range entities {
		items[i] = assetToProto(e)
	}

	return &assetV1.ListAssetsResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *AssetService) UpdateAsset(ctx context.Context, req *assetV1.UpdateAssetRequest) (*assetV1.UpdateAssetResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.AssetTag != nil {
			updates["asset_tag"] = *req.Data.AssetTag
		}
		if req.Data.Serial != nil {
			updates["serial"] = *req.Data.Serial
		}
		if req.Data.ModelName != nil {
			updates["model_name"] = *req.Data.ModelName
		}
		if req.Data.ModelNumber != nil {
			updates["model_number"] = *req.Data.ModelNumber
		}
		if req.Data.CategoryId != nil {
			updates["category_id"] = *req.Data.CategoryId
		}
		if req.Data.SupplierId != nil {
			updates["supplier_id"] = *req.Data.SupplierId
		}
		if req.Data.LocationId != nil {
			updates["location_id"] = *req.Data.LocationId
		}
		if req.Data.Status != nil {
			updates["status"] = int32(*req.Data.Status)
		}
		if req.Data.WarrantyMonths != nil {
			updates["warranty_months"] = *req.Data.WarrantyMonths
		}
		if req.Data.PurchaseDate != nil {
			updates["purchase_date"] = req.Data.PurchaseDate.AsTime()
		}
		if req.Data.OrderNumber != nil {
			updates["order_number"] = *req.Data.OrderNumber
		}
		if req.Data.PurchaseCost != nil {
			updates["purchase_cost"] = *req.Data.PurchaseCost
		}
		if req.Data.Notes != nil {
			updates["notes"] = *req.Data.Notes
		}
		if req.Data.Tags != nil {
			updates["tags"] = *req.Data.Tags
		}
		if req.Data.Metadata != nil {
			updates["metadata"] = req.Data.Metadata.AsMap()
		}
		if req.Data.SalvageValue != nil {
			updates["salvage_value"] = *req.Data.SalvageValue
		}
		if req.Data.UsefulLifeYears != nil {
			updates["useful_life_years"] = *req.Data.UsefulLifeYears
		}
		if req.Data.DepreciationRate != nil {
			updates["depreciation_rate"] = *req.Data.DepreciationRate
		}
	}

	entity, err := s.assetRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateAssetResponse{
		Asset: assetToProto(entity),
	}, nil
}

func (s *AssetService) DeleteAsset(ctx context.Context, req *assetV1.DeleteAssetRequest) (*emptypb.Empty, error) {
	id := req.GetId()

	// Get asset first to access photo_key for S3 cleanup
	asset, err := s.assetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}

	// Delete assignment history
	if n, err := s.assignmentRepo.DeleteByAssetID(ctx, id); err != nil {
		s.log.Warnf("failed to delete assignments for asset %s: %v", id, err)
	} else if n > 0 {
		s.log.Infof("deleted %d assignments for asset %s", n, id)
	}

	// Delete documents and their S3 files
	docs, _ := s.documentRepo.ListByEntity(ctx, "asset", id)
	for _, doc := range docs {
		_ = s.storageClient.Delete(ctx, doc.StorageKey)
	}
	if n, err := s.documentRepo.DeleteByEntityID(ctx, "asset", id); err != nil {
		s.log.Warnf("failed to delete documents for asset %s: %v", id, err)
	} else if n > 0 {
		s.log.Infof("deleted %d documents for asset %s", n, id)
	}

	// Delete photo from S3
	if asset.PhotoKey != "" {
		_ = s.storageClient.Delete(ctx, asset.PhotoKey)
	}

	// Delete the asset
	if err := s.assetRepo.Delete(ctx, id); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *AssetService) AssignAsset(ctx context.Context, req *assetV1.AssignAssetRequest) (*assetV1.AssignAssetResponse, error) {
	// Get asset and verify status
	a, err := s.assetRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}
	if a.Status != 1 { // DEPLOYABLE
		return nil, assetV1.ErrorAssetAlreadyAssigned("asset is not deployable (current status: %d)", a.Status)
	}

	// Verify employee exists
	emp, err := s.employeeRepo.GetByID(ctx, req.GetEmployeeId())
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, assetV1.ErrorEmployeeNotFound("employee not found")
	}

	// Create assignment record
	employeeName := emp.FirstName + " " + emp.LastName
	userID := getUserID(ctx)
	var assignedBy *uint32
	if userID > 0 {
		assignedBy = &userID
	}

	_, err = s.assignmentRepo.Create(ctx, a.ID, a.Name, emp.ID, employeeName, 1, assignedBy, req.GetNotes())
	if err != nil {
		return nil, err
	}

	// Update asset
	updates := map[string]interface{}{
		"employee_id":      req.GetEmployeeId(),
		"status":           int32(2), // ASSIGNED
		"clear_location_id": true,
	}
	entity, err := s.assetRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.AssignAssetResponse{
		Asset: assetToProto(entity),
	}, nil
}

func (s *AssetService) UnassignAsset(ctx context.Context, req *assetV1.UnassignAssetRequest) (*assetV1.UnassignAssetResponse, error) {
	// Get asset and verify status
	a, err := s.assetRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}
	if a.Status != 2 { // ASSIGNED
		return nil, assetV1.ErrorAssetNotAssigned("asset is not assigned")
	}

	// Close active assignment
	err = s.assignmentRepo.CloseActiveAssignment(ctx, a.ID)
	if err != nil {
		return nil, err
	}

	// Update asset
	updates := map[string]interface{}{
		"status":            int32(1), // DEPLOYABLE
		"clear_employee_id": true,
	}
	if req.LocationId != nil {
		updates["location_id"] = *req.LocationId
	}

	entity, err := s.assetRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UnassignAssetResponse{
		Asset: assetToProto(entity),
	}, nil
}

func (s *AssetService) GetAssignmentHistory(ctx context.Context, req *assetV1.GetAssignmentHistoryRequest) (*assetV1.GetAssignmentHistoryResponse, error) {
	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())

	entities, total, err := s.assignmentRepo.ListByAssetID(ctx, req.GetId(), page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.AssetAssignment, len(entities))
	for i, e := range entities {
		action := assetV1.AssignmentAction(e.Action)
		items[i] = &assetV1.AssetAssignment{
			Id:           &e.ID,
			AssetId:      ptrString(e.AssetID),
			AssetName:    ptrString(e.AssetName),
			EmployeeId:   ptrString(e.EmployeeID),
			EmployeeName: ptrString(e.EmployeeName),
			Action:       &action,
			AssignedAt:   timestamppb.New(e.AssignedAt),
			AssignedBy:   e.AssignedBy,
			Notes:        ptrString(e.Notes),
		}
		if e.ReturnedAt != nil {
			items[i].ReturnedAt = timestamppb.New(*e.ReturnedAt)
		}
	}

	return &assetV1.GetAssignmentHistoryResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *AssetService) UploadPhoto(ctx context.Context, req *assetV1.UploadPhotoRequest) (*assetV1.UploadPhotoResponse, error) {
	a, err := s.assetRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}

	// Delete old photo if exists
	if a.PhotoKey != "" {
		_ = s.storageClient.Delete(ctx, a.PhotoKey)
	}

	key := fmt.Sprintf("%d/assets/%s/photo/%s", a.TenantID, a.ID, req.GetFileName())
	_, err = s.storageClient.Upload(ctx, key, req.GetContent(), req.GetMimeType())
	if err != nil {
		return nil, assetV1.ErrorStorageError("upload photo failed: %v", err)
	}

	updates := map[string]interface{}{
		"photo_key": key,
	}
	entity, err := s.assetRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UploadPhotoResponse{
		Asset: assetToProto(entity),
	}, nil
}

func (s *AssetService) DeletePhoto(ctx context.Context, req *assetV1.DeletePhotoRequest) (*emptypb.Empty, error) {
	a, err := s.assetRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}

	if a.PhotoKey != "" {
		_ = s.storageClient.Delete(ctx, a.PhotoKey)
	}

	updates := map[string]interface{}{
		"clear_photo_key": true,
	}
	_, err = s.assetRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *AssetService) ListDocuments(ctx context.Context, req *assetV1.ListAssetDocumentsRequest) (*assetV1.ListAssetDocumentsResponse, error) {
	entities, err := s.documentRepo.ListByEntity(ctx, "asset", req.GetId())
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.AssetDocument, len(entities))
	for i, e := range entities {
		items[i] = documentToProto(e)
	}

	return &assetV1.ListAssetDocumentsResponse{
		Items: items,
	}, nil
}

func (s *AssetService) UploadDocument(ctx context.Context, req *assetV1.UploadAssetDocumentRequest) (*assetV1.UploadAssetDocumentResponse, error) {
	a, err := s.assetRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, assetV1.ErrorAssetNotFound("asset not found")
	}

	docID := fmt.Sprintf("%d", timestamppb.Now().GetSeconds())
	key := fmt.Sprintf("%d/assets/%s/docs/%s/%s", a.TenantID, a.ID, docID, req.GetFileName())

	result, err := s.storageClient.Upload(ctx, key, req.GetContent(), req.GetMimeType())
	if err != nil {
		return nil, assetV1.ErrorStorageError("upload document failed: %v", err)
	}

	userID := getUserID(ctx)
	var uploadedBy *uint32
	if userID > 0 {
		uploadedBy = &userID
	}

	entity, err := s.documentRepo.Create(ctx, "asset", a.ID, req.GetFileName(), result.Size, req.GetMimeType(), result.Key, result.Checksum, req.GetDescription(), uploadedBy)
	if err != nil {
		return nil, err
	}

	return &assetV1.UploadAssetDocumentResponse{
		Document: documentToProto(entity),
	}, nil
}

func (s *AssetService) DeleteDocument(ctx context.Context, req *assetV1.DeleteAssetDocumentRequest) (*emptypb.Empty, error) {
	doc, err := s.documentRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, assetV1.ErrorDocumentNotFound("document not found")
	}

	_ = s.storageClient.Delete(ctx, doc.StorageKey)

	err = s.documentRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *AssetService) DownloadDocument(ctx context.Context, req *assetV1.DownloadAssetDocumentRequest) (*assetV1.DownloadDocumentResponse, error) {
	doc, err := s.documentRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, assetV1.ErrorDocumentNotFound("document not found")
	}

	content, err := s.storageClient.Download(ctx, doc.StorageKey)
	if err != nil {
		return nil, assetV1.ErrorStorageError("download document failed: %v", err)
	}

	return &assetV1.DownloadDocumentResponse{
		FileName: doc.FileName,
		Content:  content,
		MimeType: doc.MimeType,
	}, nil
}

func (s *AssetService) InventorySyncPreview(ctx context.Context, _ *assetV1.InventorySyncPreviewRequest) (*assetV1.InventorySyncPreviewResponse, error) {
	if !s.inventoryClient.IsConfigured() {
		return nil, assetV1.ErrorInventoryNotConfigured("inventory collector is not configured")
	}

	tenantID := getTenantID(ctx)
	preview, _, err := s.buildInventoryPreview(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return preview, nil
}

func (s *AssetService) InventorySyncExecute(ctx context.Context, req *assetV1.InventorySyncExecuteRequest) (*assetV1.InventorySyncExecuteResponse, error) {
	if !s.inventoryClient.IsConfigured() {
		return nil, assetV1.ErrorInventoryNotConfigured("inventory collector is not configured")
	}

	tenantID := getTenantID(ctx)
	preview, inventoryMap, err := s.buildInventoryPreview(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Build a set of selected hostnames for filtering
	selectedHostnames := make(map[string]bool)
	if len(req.GetSelectedHostnames()) > 0 {
		for _, h := range req.GetSelectedHostnames() {
			selectedHostnames[h] = true
		}
	}

	var (
		createdCount int32
		updatedCount int32
		skippedCount int32
		errorCount   int32
		syncErrors   []string
	)

	for _, change := range preview.Changes {
		hostname := change.Hostname

		// Filter by selected hostnames if specified
		if len(selectedHostnames) > 0 && !selectedHostnames[hostname] {
			skippedCount++
			continue
		}

		inv := inventoryMap[hostname]
		if inv == nil {
			skippedCount++
			continue
		}

		switch change.Action {
		case assetV1.InventorySyncChange_ACTION_CREATE:
			name := inv.GetHostname()
			if name == "" {
				name = "Unknown"
			}

			opts := buildInventoryCreateOpts(inv)
			_, createErr := s.assetRepo.Create(ctx, tenantID, generateAssetTag(), name, opts...)
			if createErr != nil {
				errorCount++
				syncErrors = append(syncErrors, "create "+hostname+": "+createErr.Error())
			} else {
				createdCount++
			}

		case assetV1.InventorySyncChange_ACTION_UPDATE:
			updates := buildInventoryUpdateMap(inv, change.ChangedFields)
			_, updateErr := s.assetRepo.Update(ctx, change.ExistingId, updates)
			if updateErr != nil {
				errorCount++
				syncErrors = append(syncErrors, "update "+hostname+": "+updateErr.Error())
			} else {
				updatedCount++
			}
		}
	}

	return &assetV1.InventorySyncExecuteResponse{
		CreatedCount: createdCount,
		UpdatedCount: updatedCount,
		SkippedCount: skippedCount,
		ErrorCount:   errorCount,
		Errors:       syncErrors,
	}, nil
}

func assetToProto(e *ent.Asset) *assetV1.Asset {
	if e == nil {
		return nil
	}

	status := assetV1.AssetStatus(e.Status)

	result := &assetV1.Asset{
		Id:           &e.ID,
		TenantId:     e.TenantID,
		AssetTag:     ptrString(e.AssetTag),
		Name:         ptrString(e.Name),
		Serial:       ptrString(e.Serial),
		ModelName:    ptrString(e.ModelName),
		ModelNumber:  ptrString(e.ModelNumber),
		CategoryId:   ptrString(e.CategoryID),
		SupplierId:   ptrString(e.SupplierID),
		LocationId:   ptrString(e.LocationID),
		EmployeeId:   ptrString(e.EmployeeID),
		Status:       &status,
		PhotoKey:     ptrString(e.PhotoKey),
		WarrantyMonths: e.WarrantyMonths,
		OrderNumber:  ptrString(e.OrderNumber),
		Notes:        ptrString(e.Notes),
		Tags:         ptrString(e.Tags),
		Metadata:     mapToStruct(e.Metadata),
		CreatedBy:    e.CreateBy,
		UpdatedBy:    e.UpdateBy,
	}

	if e.PurchaseCost != nil {
		result.PurchaseCost = e.PurchaseCost
	}
	if e.SalvageValue != nil {
		result.SalvageValue = e.SalvageValue
	}
	if e.UsefulLifeYears != nil {
		result.UsefulLifeYears = e.UsefulLifeYears
	}
	if e.DepreciationRate != nil {
		result.DepreciationRate = e.DepreciationRate
	}
	if e.PurchaseDate != nil {
		result.PurchaseDate = timestamppb.New(*e.PurchaseDate)
	}
	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}
	if e.UpdateTime != nil {
		result.UpdatedAt = timestamppb.New(*e.UpdateTime)
	}

	return result
}

func documentToProto(e *ent.AssetDocument) *assetV1.AssetDocument {
	if e == nil {
		return nil
	}

	result := &assetV1.AssetDocument{
		Id:          &e.ID,
		EntityType:  ptrString(e.EntityType),
		EntityId:    ptrString(e.EntityID),
		FileName:    ptrString(e.FileName),
		FileSize:    &e.FileSize,
		MimeType:    ptrString(e.MimeType),
		StorageKey:  ptrString(e.StorageKey),
		Checksum:    ptrString(e.Checksum),
		Description: ptrString(e.Description),
		CreatedBy:   e.UploadedBy,
	}

	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}

	return result
}
