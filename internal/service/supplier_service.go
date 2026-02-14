package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type SupplierService struct {
	assetV1.UnimplementedSupplierServiceServer

	log          *log.Helper
	supplierRepo *data.SupplierRepo
}

func NewSupplierService(ctx *bootstrap.Context, supplierRepo *data.SupplierRepo) *SupplierService {
	return &SupplierService{
		log:          ctx.NewLoggerHelper("asset/service/supplier"),
		supplierRepo: supplierRepo,
	}
}

func (s *SupplierService) CreateSupplier(ctx context.Context, req *assetV1.CreateSupplierRequest) (*assetV1.CreateSupplierResponse, error) {
	opts := []func(*ent.SupplierCreate){}

	if req.Code != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetCode(*req.Code) })
	}
	if req.Address != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetAddress(*req.Address) })
	}
	if req.City != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetCity(*req.City) })
	}
	if req.State != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetState(*req.State) })
	}
	if req.Country != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetCountry(*req.Country) })
	}
	if req.PostalCode != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetPostalCode(*req.PostalCode) })
	}
	if req.ContactPerson != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetContactPerson(*req.ContactPerson) })
	}
	if req.Telephone != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetTelephone(*req.Telephone) })
	}
	if req.Email != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetEmail(*req.Email) })
	}
	if req.Website != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetWebsite(*req.Website) })
	}
	if req.Notes != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetNotes(*req.Notes) })
	}
	if req.Tags != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetTags(*req.Tags) })
	}
	if req.Metadata != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetMetadata(*req.Metadata) })
	}
	if req.Status != nil {
		opts = append(opts, func(c *ent.SupplierCreate) { c.SetStatus(*req.Status) })
	}

	entity, err := s.supplierRepo.Create(ctx, req.GetTenantId(), req.GetName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateSupplierResponse{
		Supplier: supplierToProto(entity),
	}, nil
}

func (s *SupplierService) GetSupplier(ctx context.Context, req *assetV1.GetSupplierRequest) (*assetV1.GetSupplierResponse, error) {
	entity, err := s.supplierRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorSupplierNotFound("supplier not found")
	}

	return &assetV1.GetSupplierResponse{
		Supplier: supplierToProto(entity),
	}, nil
}

func (s *SupplierService) ListSuppliers(ctx context.Context, req *assetV1.ListSuppliersRequest) (*assetV1.ListSuppliersResponse, error) {
	filters := make(map[string]interface{})
	if req.Status != nil {
		filters["status"] = *req.Status
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

	entities, total, err := s.supplierRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.Supplier, len(entities))
	for i, e := range entities {
		items[i] = supplierToProto(e)
	}

	return &assetV1.ListSuppliersResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *SupplierService) UpdateSupplier(ctx context.Context, req *assetV1.UpdateSupplierRequest) (*assetV1.UpdateSupplierResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.Code != nil {
			updates["code"] = *req.Data.Code
		}
		if req.Data.Address != nil {
			updates["address"] = *req.Data.Address
		}
		if req.Data.City != nil {
			updates["city"] = *req.Data.City
		}
		if req.Data.State != nil {
			updates["state"] = *req.Data.State
		}
		if req.Data.Country != nil {
			updates["country"] = *req.Data.Country
		}
		if req.Data.PostalCode != nil {
			updates["postal_code"] = *req.Data.PostalCode
		}
		if req.Data.ContactPerson != nil {
			updates["contact_person"] = *req.Data.ContactPerson
		}
		if req.Data.Telephone != nil {
			updates["telephone"] = *req.Data.Telephone
		}
		if req.Data.Email != nil {
			updates["email"] = *req.Data.Email
		}
		if req.Data.Website != nil {
			updates["website"] = *req.Data.Website
		}
		if req.Data.Notes != nil {
			updates["notes"] = *req.Data.Notes
		}
		if req.Data.Tags != nil {
			updates["tags"] = *req.Data.Tags
		}
		if req.Data.Metadata != nil {
			updates["metadata"] = *req.Data.Metadata
		}
		if req.Data.Status != nil {
			updates["status"] = *req.Data.Status
		}
	}

	entity, err := s.supplierRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateSupplierResponse{
		Supplier: supplierToProto(entity),
	}, nil
}

func (s *SupplierService) DeleteSupplier(ctx context.Context, req *assetV1.DeleteSupplierRequest) (*emptypb.Empty, error) {
	err := s.supplierRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func supplierToProto(e *ent.Supplier) *assetV1.Supplier {
	if e == nil {
		return nil
	}

	result := &assetV1.Supplier{
		Id:            &e.ID,
		TenantId:      e.TenantID,
		Name:          ptrString(e.Name),
		Code:          ptrString(e.Code),
		Address:       ptrString(e.Address),
		City:          ptrString(e.City),
		State:         ptrString(e.State),
		Country:       ptrString(e.Country),
		PostalCode:    ptrString(e.PostalCode),
		ContactPerson: ptrString(e.ContactPerson),
		Telephone:     ptrString(e.Telephone),
		Email:         ptrString(e.Email),
		Website:       ptrString(e.Website),
		Notes:         ptrString(e.Notes),
		Tags:          ptrString(e.Tags),
		Metadata:      ptrString(e.Metadata),
		Status:        ptrInt32(e.Status),
		CreatedBy:     e.CreateBy,
		UpdatedBy:     e.UpdateBy,
	}

	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}
	if e.UpdateTime != nil {
		result.UpdatedAt = timestamppb.New(*e.UpdateTime)
	}

	return result
}
