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

type ConsumableService struct {
	assetV1.UnimplementedConsumableServiceServer

	log            *log.Helper
	consumableRepo *data.ConsumableRepo
	documentRepo   *data.DocumentRepo
	storageClient  *data.StorageClient
}

func NewConsumableService(
	ctx *bootstrap.Context,
	consumableRepo *data.ConsumableRepo,
	documentRepo *data.DocumentRepo,
	storageClient *data.StorageClient,
) *ConsumableService {
	return &ConsumableService{
		log:            ctx.NewLoggerHelper("asset/service/consumable"),
		consumableRepo: consumableRepo,
		documentRepo:   documentRepo,
		storageClient:  storageClient,
	}
}

func (s *ConsumableService) CreateConsumable(ctx context.Context, req *assetV1.CreateConsumableRequest) (*assetV1.CreateConsumableResponse, error) {
	opts := []func(*ent.ConsumableCreate){}

	if req.Description != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetDescription(*req.Description) })
	}
	if req.CategoryId != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetCategoryID(*req.CategoryId) })
	}
	if req.SupplierId != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetSupplierID(*req.SupplierId) })
	}
	if req.LocationId != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetLocationID(*req.LocationId) })
	}
	if req.ModelName != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetModelName(*req.ModelName) })
	}
	if req.ModelNumber != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetModelNumber(*req.ModelNumber) })
	}
	if req.Amount != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetAmount(*req.Amount) })
	}
	if req.MinAmount != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetMinAmount(*req.MinAmount) })
	}
	if req.PurchaseDate != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetPurchaseDate(req.PurchaseDate.AsTime()) })
	}
	if req.PurchaseCost != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetPurchaseCost(*req.PurchaseCost) })
	}
	if req.OrderNumber != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetOrderNumber(*req.OrderNumber) })
	}
	if req.Notes != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetNotes(*req.Notes) })
	}
	if req.Tags != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetTags(*req.Tags) })
	}
	if req.Metadata != nil {
		opts = append(opts, func(c *ent.ConsumableCreate) { c.SetMetadata(req.Metadata.AsMap()) })
	}

	entity, err := s.consumableRepo.Create(ctx, req.GetTenantId(), req.GetName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateConsumableResponse{
		Consumable: consumableToProto(entity),
	}, nil
}

func (s *ConsumableService) GetConsumable(ctx context.Context, req *assetV1.GetConsumableRequest) (*assetV1.GetConsumableResponse, error) {
	entity, err := s.consumableRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorConsumableNotFound("consumable not found")
	}

	return &assetV1.GetConsumableResponse{
		Consumable: consumableToProto(entity),
	}, nil
}

func (s *ConsumableService) ListConsumables(ctx context.Context, req *assetV1.ListConsumablesRequest) (*assetV1.ListConsumablesResponse, error) {
	filters := make(map[string]interface{})
	if req.CategoryId != nil {
		filters["category_id"] = *req.CategoryId
	}
	if req.SupplierId != nil {
		filters["supplier_id"] = *req.SupplierId
	}
	if req.LocationId != nil {
		filters["location_id"] = *req.LocationId
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

	entities, total, err := s.consumableRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.Consumable, len(entities))
	for i, e := range entities {
		items[i] = consumableToProto(e)
	}

	return &assetV1.ListConsumablesResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *ConsumableService) UpdateConsumable(ctx context.Context, req *assetV1.UpdateConsumableRequest) (*assetV1.UpdateConsumableResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.Description != nil {
			updates["description"] = *req.Data.Description
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
		if req.Data.ModelName != nil {
			updates["model_name"] = *req.Data.ModelName
		}
		if req.Data.ModelNumber != nil {
			updates["model_number"] = *req.Data.ModelNumber
		}
		if req.Data.Amount != nil {
			updates["amount"] = *req.Data.Amount
		}
		if req.Data.MinAmount != nil {
			updates["min_amount"] = *req.Data.MinAmount
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
		if req.Data.Notes != nil {
			updates["notes"] = *req.Data.Notes
		}
		if req.Data.Tags != nil {
			updates["tags"] = *req.Data.Tags
		}
		if req.Data.Metadata != nil {
			updates["metadata"] = req.Data.Metadata.AsMap()
		}
	}

	entity, err := s.consumableRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateConsumableResponse{
		Consumable: consumableToProto(entity),
	}, nil
}

func (s *ConsumableService) DeleteConsumable(ctx context.Context, req *assetV1.DeleteConsumableRequest) (*emptypb.Empty, error) {
	err := s.consumableRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ConsumableService) ListDocuments(ctx context.Context, req *assetV1.ListConsumableDocumentsRequest) (*assetV1.ListConsumableDocumentsResponse, error) {
	entities, err := s.documentRepo.ListByEntity(ctx, "consumable", req.GetId())
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.AssetDocument, len(entities))
	for i, e := range entities {
		items[i] = documentToProto(e)
	}

	return &assetV1.ListConsumableDocumentsResponse{
		Items: items,
	}, nil
}

func (s *ConsumableService) UploadDocument(ctx context.Context, req *assetV1.UploadConsumableDocumentRequest) (*assetV1.UploadConsumableDocumentResponse, error) {
	c, err := s.consumableRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, assetV1.ErrorConsumableNotFound("consumable not found")
	}

	docID := fmt.Sprintf("%d", timestamppb.Now().GetSeconds())
	key := fmt.Sprintf("%d/consumables/%s/docs/%s/%s", c.TenantID, c.ID, docID, req.GetFileName())

	result, err := s.storageClient.Upload(ctx, key, req.GetContent(), req.GetMimeType())
	if err != nil {
		return nil, assetV1.ErrorStorageError("upload document failed: %v", err)
	}

	userID := getUserID(ctx)
	var uploadedBy *uint32
	if userID > 0 {
		uploadedBy = &userID
	}

	entity, err := s.documentRepo.Create(ctx, "consumable", c.ID, req.GetFileName(), result.Size, req.GetMimeType(), result.Key, result.Checksum, req.GetDescription(), uploadedBy)
	if err != nil {
		return nil, err
	}

	return &assetV1.UploadConsumableDocumentResponse{
		Document: documentToProto(entity),
	}, nil
}

func (s *ConsumableService) DeleteDocument(ctx context.Context, req *assetV1.DeleteConsumableDocumentRequest) (*emptypb.Empty, error) {
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

func (s *ConsumableService) DownloadDocument(ctx context.Context, req *assetV1.DownloadConsumableDocumentRequest) (*assetV1.DownloadDocumentResponse, error) {
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

func consumableToProto(e *ent.Consumable) *assetV1.Consumable {
	if e == nil {
		return nil
	}

	result := &assetV1.Consumable{
		Id:          &e.ID,
		TenantId:    e.TenantID,
		Name:        ptrString(e.Name),
		Description: ptrString(e.Description),
		CategoryId:  ptrString(e.CategoryID),
		SupplierId:  ptrString(e.SupplierID),
		LocationId:  ptrString(e.LocationID),
		ModelName:   ptrString(e.ModelName),
		ModelNumber: ptrString(e.ModelNumber),
		Amount:      ptrInt32(e.Amount),
		MinAmount:   ptrInt32(e.MinAmount),
		OrderNumber: ptrString(e.OrderNumber),
		Notes:       ptrString(e.Notes),
		Tags:        ptrString(e.Tags),
		Metadata:    mapToStruct(e.Metadata),
		CreatedBy:   e.CreateBy,
		UpdatedBy:   e.UpdateBy,
	}

	if e.PurchaseCost != nil {
		result.PurchaseCost = e.PurchaseCost
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
