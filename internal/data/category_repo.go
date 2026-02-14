package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/category"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type CategoryRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewCategoryRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *CategoryRepo {
	return &CategoryRepo{
		log:       ctx.NewLoggerHelper("asset/category/repo"),
		entClient: entClient,
	}
}

func (r *CategoryRepo) Create(ctx context.Context, tenantID uint32, name string, opts ...func(*ent.CategoryCreate)) (*ent.Category, error) {
	id := uuid.New().String()

	create := r.entClient.Client().Category.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetName(name).
		SetCreateTime(time.Now())

	for _, opt := range opts {
		opt(create)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, assetV1.ErrorBadRequest("category '%s' already exists", name)
		}
		r.log.Errorf("create category failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create category failed")
	}
	return entity, nil
}

func (r *CategoryRepo) GetByID(ctx context.Context, id string) (*ent.Category, error) {
	entity, err := r.entClient.Client().Category.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get category failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get category failed")
	}
	return entity, nil
}

func (r *CategoryRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.Category, int, error) {
	query := r.entClient.Client().Category.Query().Where(category.TenantID(tenantID))

	if parentID, ok := filters["parent_id"].(string); ok && parentID != "" {
		query = query.Where(category.ParentID(parentID))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(category.NameContains(q))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count categories failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list categories failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(category.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("list categories failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list categories failed")
	}

	return entities, total, nil
}

func (r *CategoryRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.Category, error) {
	update := r.entClient.Client().Category.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if description, ok := updates["description"].(string); ok {
		update = update.SetDescription(description)
	}
	if parentID, ok := updates["parent_id"].(string); ok {
		update = update.SetParentID(parentID)
	}
	if icon, ok := updates["icon"].(string); ok {
		update = update.SetIcon(icon)
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorCategoryNotFound("category not found")
		}
		r.log.Errorf("update category failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update category failed")
	}
	return entity, nil
}

func (r *CategoryRepo) Delete(ctx context.Context, id string) error {
	count, err := r.entClient.Client().Category.Query().
		Where(category.ID(id)).
		QueryChildren().
		Count(ctx)
	if err != nil {
		r.log.Errorf("check category children failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete category failed")
	}
	if count > 0 {
		return assetV1.ErrorCategoryHasChildren("category has %d children", count)
	}

	assetCount, err := r.entClient.Client().Category.Query().
		Where(category.ID(id)).
		QueryAssets().
		Count(ctx)
	if err != nil {
		r.log.Errorf("check category assets failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete category failed")
	}
	if assetCount > 0 {
		return assetV1.ErrorCategoryHasAssets("category has %d assets", assetCount)
	}

	err = r.entClient.Client().Category.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorCategoryNotFound("category not found")
		}
		r.log.Errorf("delete category failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete category failed")
	}
	return nil
}

func (r *CategoryRepo) GetTree(ctx context.Context, tenantID uint32, rootID string) ([]*ent.Category, error) {
	query := r.entClient.Client().Category.Query().Where(category.TenantID(tenantID))

	entities, err := query.Order(ent.Asc(category.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("get category tree failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get category tree failed")
	}

	if rootID != "" {
		subtreeIDs := make(map[string]bool)
		subtreeIDs[rootID] = true

		for {
			foundNew := false
			for _, e := range entities {
				if e.ParentID != "" && subtreeIDs[e.ParentID] && !subtreeIDs[e.ID] {
					subtreeIDs[e.ID] = true
					foundNew = true
				}
			}
			if !foundNew {
				break
			}
		}

		var filtered []*ent.Category
		for _, e := range entities {
			if subtreeIDs[e.ID] {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	return entities, nil
}
