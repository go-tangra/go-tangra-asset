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

type LocationService struct {
	assetV1.UnimplementedLocationServiceServer

	log          *log.Helper
	locationRepo *data.LocationRepo
}

func NewLocationService(ctx *bootstrap.Context, locationRepo *data.LocationRepo) *LocationService {
	return &LocationService{
		log:          ctx.NewLoggerHelper("asset/service/location"),
		locationRepo: locationRepo,
	}
}

func (s *LocationService) CreateLocation(ctx context.Context, req *assetV1.CreateLocationRequest) (*assetV1.CreateLocationResponse, error) {
	opts := []func(*ent.LocationCreate){}

	if req.Code != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetCode(*req.Code) })
	}
	if req.Description != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetDescription(*req.Description) })
	}
	if req.ParentId != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetParentID(*req.ParentId) })
	}
	if req.Address != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetAddress(*req.Address) })
	}
	if req.City != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetCity(*req.City) })
	}
	if req.State != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetState(*req.State) })
	}
	if req.Country != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetCountry(*req.Country) })
	}
	if req.PostalCode != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetPostalCode(*req.PostalCode) })
	}
	if req.Contact != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetContact(*req.Contact) })
	}
	if req.Phone != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetPhone(*req.Phone) })
	}
	if req.Email != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetEmail(*req.Email) })
	}
	if req.Status != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetStatus(int32(*req.Status)) })
	}
	if req.Tags != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetTags(*req.Tags) })
	}
	if req.Metadata != nil {
		opts = append(opts, func(c *ent.LocationCreate) { c.SetMetadata(req.Metadata.AsMap()) })
	}

	entity, err := s.locationRepo.Create(ctx, req.GetTenantId(), req.GetName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateLocationResponse{
		Location: locationToProto(entity),
	}, nil
}

func (s *LocationService) GetLocation(ctx context.Context, req *assetV1.GetLocationRequest) (*assetV1.GetLocationResponse, error) {
	entity, err := s.locationRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorLocationNotFound("location not found")
	}

	return &assetV1.GetLocationResponse{
		Location: locationToProto(entity),
	}, nil
}

func (s *LocationService) ListLocations(ctx context.Context, req *assetV1.ListLocationsRequest) (*assetV1.ListLocationsResponse, error) {
	filters := make(map[string]interface{})
	if req.ParentId != nil {
		filters["parent_id"] = *req.ParentId
	}
	if req.Status != nil {
		filters["status"] = int32(*req.Status)
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

	entities, total, err := s.locationRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.Location, len(entities))
	for i, e := range entities {
		items[i] = locationToProto(e)
	}

	return &assetV1.ListLocationsResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *LocationService) UpdateLocation(ctx context.Context, req *assetV1.UpdateLocationRequest) (*assetV1.UpdateLocationResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.Code != nil {
			updates["code"] = *req.Data.Code
		}
		if req.Data.Description != nil {
			updates["description"] = *req.Data.Description
		}
		if req.Data.ParentId != nil {
			updates["parent_id"] = *req.Data.ParentId
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
		if req.Data.Contact != nil {
			updates["contact"] = *req.Data.Contact
		}
		if req.Data.Phone != nil {
			updates["phone"] = *req.Data.Phone
		}
		if req.Data.Email != nil {
			updates["email"] = *req.Data.Email
		}
		if req.Data.Status != nil {
			updates["status"] = int32(*req.Data.Status)
		}
		if req.Data.Tags != nil {
			updates["tags"] = *req.Data.Tags
		}
		if req.Data.Metadata != nil {
			updates["metadata"] = req.Data.Metadata.AsMap()
		}
	}

	entity, err := s.locationRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateLocationResponse{
		Location: locationToProto(entity),
	}, nil
}

func (s *LocationService) DeleteLocation(ctx context.Context, req *assetV1.DeleteLocationRequest) (*emptypb.Empty, error) {
	err := s.locationRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *LocationService) GetLocationTree(ctx context.Context, req *assetV1.GetLocationTreeRequest) (*assetV1.GetLocationTreeResponse, error) {
	entities, err := s.locationRepo.GetTree(ctx, req.GetTenantId(), req.GetRootId())
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*assetV1.LocationTreeNode)
	for _, e := range entities {
		nodeMap[e.ID] = &assetV1.LocationTreeNode{
			Location: locationToProto(e),
			Children: []*assetV1.LocationTreeNode{},
		}
	}

	var roots []*assetV1.LocationTreeNode
	for _, e := range entities {
		node := nodeMap[e.ID]
		if e.ParentID == "" {
			roots = append(roots, node)
		} else {
			if parent, ok := nodeMap[e.ParentID]; ok {
				parent.Children = append(parent.Children, node)
			} else {
				roots = append(roots, node)
			}
		}
	}

	return &assetV1.GetLocationTreeResponse{
		Nodes: roots,
	}, nil
}

func locationToProto(e *ent.Location) *assetV1.Location {
	if e == nil {
		return nil
	}

	status := assetV1.LocationStatus(e.Status)

	result := &assetV1.Location{
		Id:          &e.ID,
		TenantId:    e.TenantID,
		Name:        ptrString(e.Name),
		Code:        ptrString(e.Code),
		Description: ptrString(e.Description),
		ParentId:    ptrString(e.ParentID),
		Path:        ptrString(e.Path),
		Address:     ptrString(e.Address),
		City:        ptrString(e.City),
		State:       ptrString(e.State),
		Country:     ptrString(e.Country),
		PostalCode:  ptrString(e.PostalCode),
		Contact:     ptrString(e.Contact),
		Phone:       ptrString(e.Phone),
		Email:       ptrString(e.Email),
		Status:      &status,
		Tags:        ptrString(e.Tags),
		Metadata:    mapToStruct(e.Metadata),
		CreatedBy:   e.CreateBy,
		UpdatedBy:   e.UpdateBy,
	}

	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}
	if e.UpdateTime != nil {
		result.UpdatedAt = timestamppb.New(*e.UpdateTime)
	}

	return result
}
