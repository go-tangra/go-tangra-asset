package service

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type LicenseService struct {
	assetV1.UnimplementedLicenseServiceServer

	log           *log.Helper
	licenseRepo   *data.LicenseRepo
	documentRepo  *data.DocumentRepo
	storageClient *data.StorageClient
}

func NewLicenseService(
	ctx *bootstrap.Context,
	licenseRepo *data.LicenseRepo,
	documentRepo *data.DocumentRepo,
	storageClient *data.StorageClient,
) *LicenseService {
	return &LicenseService{
		log:           ctx.NewLoggerHelper("asset/service/license"),
		licenseRepo:   licenseRepo,
		documentRepo:  documentRepo,
		storageClient: storageClient,
	}
}

func (s *LicenseService) CreateLicense(ctx context.Context, req *assetV1.CreateLicenseRequest) (*assetV1.CreateLicenseResponse, error) {
	opts := []func(*ent.LicenseCreate){}

	if req.SupplierId != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetSupplierID(*req.SupplierId) })
	}
	if req.PurchaseDate != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetPurchaseDate(req.PurchaseDate.AsTime()) })
	}
	if req.PurchaseCost != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetPurchaseCost(*req.PurchaseCost) })
	}
	if req.OrderNumber != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetOrderNumber(*req.OrderNumber) })
	}
	if req.ValidFrom != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetValidFrom(req.ValidFrom.AsTime()) })
	}
	if req.ValidTo != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetValidTo(req.ValidTo.AsTime()) })
	}
	if req.Notes != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetNotes(*req.Notes) })
	}
	if req.Status != nil {
		opts = append(opts, func(c *ent.LicenseCreate) { c.SetStatus(licenseStatusToString(*req.Status)) })
	}

	entity, err := s.licenseRepo.Create(ctx, req.GetTenantId(), req.GetName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateLicenseResponse{
		License: licenseToProto(entity),
	}, nil
}

func (s *LicenseService) GetLicense(ctx context.Context, req *assetV1.GetLicenseRequest) (*assetV1.GetLicenseResponse, error) {
	entity, err := s.licenseRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorLicenseNotFound("license not found")
	}

	return &assetV1.GetLicenseResponse{
		License: licenseToProto(entity),
	}, nil
}

func (s *LicenseService) ListLicenses(ctx context.Context, req *assetV1.ListLicensesRequest) (*assetV1.ListLicensesResponse, error) {
	filters := make(map[string]interface{})
	if req.SupplierId != nil {
		filters["supplier_id"] = *req.SupplierId
	}
	if req.Status != nil && *req.Status != assetV1.LicenseStatus_LICENSE_STATUS_UNSPECIFIED {
		filters["status"] = licenseStatusToString(*req.Status)
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

	entities, total, err := s.licenseRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.License, len(entities))
	for i, e := range entities {
		items[i] = licenseToProto(e)
	}

	return &assetV1.ListLicensesResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *LicenseService) UpdateLicense(ctx context.Context, req *assetV1.UpdateLicenseRequest) (*assetV1.UpdateLicenseResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.SupplierId != nil {
			updates["supplier_id"] = *req.Data.SupplierId
		}
		if req.Data.PurchaseDate != nil {
			updates["purchase_date"] = req.Data.PurchaseDate.AsTime()
		}
		if req.Data.PurchaseCost != nil {
			updates["purchase_cost"] = *req.Data.PurchaseCost
		}
		if req.Data.OrderNumber != nil {
			updates["order_number"] = *req.Data.OrderNumber
		}
		if req.Data.ValidFrom != nil {
			updates["valid_from"] = req.Data.ValidFrom.AsTime()
		}
		if req.Data.ValidTo != nil {
			updates["valid_to"] = req.Data.ValidTo.AsTime()
		}
		if req.Data.Notes != nil {
			updates["notes"] = *req.Data.Notes
		}
		if req.Data.Status != nil {
			updates["status"] = licenseStatusToString(*req.Data.Status)
		}
	}

	entity, err := s.licenseRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateLicenseResponse{
		License: licenseToProto(entity),
	}, nil
}

func (s *LicenseService) DeleteLicense(ctx context.Context, req *assetV1.DeleteLicenseRequest) (*emptypb.Empty, error) {
	err := s.licenseRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *LicenseService) ListDocuments(ctx context.Context, req *assetV1.ListLicenseDocumentsRequest) (*assetV1.ListLicenseDocumentsResponse, error) {
	entities, err := s.documentRepo.ListByEntity(ctx, "license", req.GetId())
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.AssetDocument, len(entities))
	for i, e := range entities {
		items[i] = documentToProto(e)
	}

	return &assetV1.ListLicenseDocumentsResponse{
		Items: items,
	}, nil
}

func (s *LicenseService) UploadDocument(ctx context.Context, req *assetV1.UploadLicenseDocumentRequest) (*assetV1.UploadLicenseDocumentResponse, error) {
	l, err := s.licenseRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, assetV1.ErrorLicenseNotFound("license not found")
	}

	docID := fmt.Sprintf("%d", timestamppb.Now().GetSeconds())
	key := fmt.Sprintf("%d/licenses/%s/docs/%s/%s", l.TenantID, l.ID, docID, req.GetFileName())

	result, err := s.storageClient.Upload(ctx, key, req.GetContent(), req.GetMimeType())
	if err != nil {
		return nil, assetV1.ErrorStorageError("upload document failed: %v", err)
	}

	userID := getUserID(ctx)
	var uploadedBy *uint32
	if userID > 0 {
		uploadedBy = &userID
	}

	entity, err := s.documentRepo.Create(ctx, "license", l.ID, req.GetFileName(), result.Size, req.GetMimeType(), result.Key, result.Checksum, req.GetDescription(), uploadedBy)
	if err != nil {
		return nil, err
	}

	return &assetV1.UploadLicenseDocumentResponse{
		Document: documentToProto(entity),
	}, nil
}

func (s *LicenseService) DeleteDocument(ctx context.Context, req *assetV1.DeleteLicenseDocumentRequest) (*emptypb.Empty, error) {
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

func (s *LicenseService) DownloadDocument(ctx context.Context, req *assetV1.DownloadLicenseDocumentRequest) (*assetV1.DownloadDocumentResponse, error) {
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

func licenseToProto(e *ent.License) *assetV1.License {
	if e == nil {
		return nil
	}

	result := &assetV1.License{
		Id:          &e.ID,
		TenantId:    e.TenantID,
		Name:        ptrString(e.Name),
		SupplierId:  ptrString(e.SupplierID),
		OrderNumber: ptrString(e.OrderNumber),
		Notes:       ptrString(e.Notes),
		Status:      licenseStatusFromString(e.Status).Enum(),
		CreatedBy:   e.CreateBy,
		UpdatedBy:   e.UpdateBy,
	}

	if e.PurchaseCost != nil {
		result.PurchaseCost = e.PurchaseCost
	}
	if e.PurchaseDate != nil {
		result.PurchaseDate = timestamppb.New(*e.PurchaseDate)
	}
	if e.ValidFrom != nil {
		result.ValidFrom = timestamppb.New(*e.ValidFrom)
	}
	if e.ValidTo != nil {
		result.ValidTo = timestamppb.New(*e.ValidTo)
	}
	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}
	if e.UpdateTime != nil {
		result.UpdatedAt = timestamppb.New(*e.UpdateTime)
	}

	return result
}

func licenseStatusToString(s assetV1.LicenseStatus) string {
	switch s {
	case assetV1.LicenseStatus_LICENSE_STATUS_ACTIVE:
		return "ACTIVE"
	case assetV1.LicenseStatus_LICENSE_STATUS_EXPIRED:
		return "EXPIRED"
	case assetV1.LicenseStatus_LICENSE_STATUS_SUSPENDED:
		return "SUSPENDED"
	default:
		return "ACTIVE"
	}
}

func licenseStatusFromString(s string) assetV1.LicenseStatus {
	switch s {
	case "ACTIVE":
		return assetV1.LicenseStatus_LICENSE_STATUS_ACTIVE
	case "EXPIRED":
		return assetV1.LicenseStatus_LICENSE_STATUS_EXPIRED
	case "SUSPENDED":
		return assetV1.LicenseStatus_LICENSE_STATUS_SUSPENDED
	default:
		return assetV1.LicenseStatus_LICENSE_STATUS_UNSPECIFIED
	}
}
