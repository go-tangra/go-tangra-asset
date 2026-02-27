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

type CategoryService struct {
	assetV1.UnimplementedCategoryServiceServer

	log          *log.Helper
	categoryRepo *data.CategoryRepo
}

func NewCategoryService(ctx *bootstrap.Context, categoryRepo *data.CategoryRepo) *CategoryService {
	return &CategoryService{
		log:          ctx.NewLoggerHelper("asset/service/category"),
		categoryRepo: categoryRepo,
	}
}

func (s *CategoryService) CreateCategory(ctx context.Context, req *assetV1.CreateCategoryRequest) (*assetV1.CreateCategoryResponse, error) {
	opts := []func(*ent.CategoryCreate){}

	if req.Description != nil {
		opts = append(opts, func(c *ent.CategoryCreate) { c.SetDescription(*req.Description) })
	}
	if req.ParentId != nil {
		opts = append(opts, func(c *ent.CategoryCreate) { c.SetParentID(*req.ParentId) })
	}
	if req.Icon != nil {
		opts = append(opts, func(c *ent.CategoryCreate) { c.SetIcon(*req.Icon) })
	}
	if req.Metadata != nil {
		opts = append(opts, func(c *ent.CategoryCreate) { c.SetMetadata(req.Metadata.AsMap()) })
	}

	entity, err := s.categoryRepo.Create(ctx, req.GetTenantId(), req.GetName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateCategoryResponse{
		Category: categoryToProto(entity),
	}, nil
}

func (s *CategoryService) GetCategory(ctx context.Context, req *assetV1.GetCategoryRequest) (*assetV1.GetCategoryResponse, error) {
	entity, err := s.categoryRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorCategoryNotFound("category not found")
	}

	return &assetV1.GetCategoryResponse{
		Category: categoryToProto(entity),
	}, nil
}

func (s *CategoryService) ListCategories(ctx context.Context, req *assetV1.ListCategoriesRequest) (*assetV1.ListCategoriesResponse, error) {
	filters := make(map[string]interface{})
	if req.ParentId != nil {
		filters["parent_id"] = *req.ParentId
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

	entities, total, err := s.categoryRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.Category, len(entities))
	for i, e := range entities {
		items[i] = categoryToProto(e)
	}

	return &assetV1.ListCategoriesResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *CategoryService) UpdateCategory(ctx context.Context, req *assetV1.UpdateCategoryRequest) (*assetV1.UpdateCategoryResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.Name != nil {
			updates["name"] = *req.Data.Name
		}
		if req.Data.Description != nil {
			updates["description"] = *req.Data.Description
		}
		if req.Data.ParentId != nil {
			updates["parent_id"] = *req.Data.ParentId
		}
		if req.Data.Icon != nil {
			updates["icon"] = *req.Data.Icon
		}
		if req.Data.Metadata != nil {
			updates["metadata"] = req.Data.Metadata.AsMap()
		}
	}

	entity, err := s.categoryRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateCategoryResponse{
		Category: categoryToProto(entity),
	}, nil
}

func (s *CategoryService) DeleteCategory(ctx context.Context, req *assetV1.DeleteCategoryRequest) (*emptypb.Empty, error) {
	err := s.categoryRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *CategoryService) GetCategoryTree(ctx context.Context, req *assetV1.GetCategoryTreeRequest) (*assetV1.GetCategoryTreeResponse, error) {
	entities, err := s.categoryRepo.GetTree(ctx, req.GetTenantId(), req.GetRootId())
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*assetV1.CategoryTreeNode)
	for _, e := range entities {
		nodeMap[e.ID] = &assetV1.CategoryTreeNode{
			Category: categoryToProto(e),
			Children: []*assetV1.CategoryTreeNode{},
		}
	}

	var roots []*assetV1.CategoryTreeNode
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

	return &assetV1.GetCategoryTreeResponse{
		Nodes: roots,
	}, nil
}

func categoryToProto(e *ent.Category) *assetV1.Category {
	if e == nil {
		return nil
	}

	result := &assetV1.Category{
		Id:          &e.ID,
		TenantId:    e.TenantID,
		Name:        ptrString(e.Name),
		Description: ptrString(e.Description),
		ParentId:    ptrString(e.ParentID),
		Icon:        ptrString(e.Icon),
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
