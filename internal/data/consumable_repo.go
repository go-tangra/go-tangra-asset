package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/consumable"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type ConsumableRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewConsumableRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *ConsumableRepo {
	return &ConsumableRepo{
		log:       ctx.NewLoggerHelper("asset/consumable/repo"),
		entClient: entClient,
	}
}

func (r *ConsumableRepo) Create(ctx context.Context, tenantID uint32, name string, opts ...func(*ent.ConsumableCreate)) (*ent.Consumable, error) {
	id := uuid.New().String()

	create := r.entClient.Client().Consumable.Create().
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
			return nil, assetV1.ErrorBadRequest("consumable '%s' already exists", name)
		}
		r.log.Errorf("create consumable failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create consumable failed")
	}
	return entity, nil
}

func (r *ConsumableRepo) GetByID(ctx context.Context, id string) (*ent.Consumable, error) {
	entity, err := r.entClient.Client().Consumable.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get consumable failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get consumable failed")
	}
	return entity, nil
}

func (r *ConsumableRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.Consumable, int, error) {
	query := r.entClient.Client().Consumable.Query().Where(consumable.TenantID(tenantID))

	if categoryID, ok := filters["category_id"].(string); ok && categoryID != "" {
		query = query.Where(consumable.CategoryID(categoryID))
	}
	if supplierID, ok := filters["supplier_id"].(string); ok && supplierID != "" {
		query = query.Where(consumable.SupplierID(supplierID))
	}
	if locationID, ok := filters["location_id"].(string); ok && locationID != "" {
		query = query.Where(consumable.LocationID(locationID))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(consumable.NameContains(q))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count consumables failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list consumables failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(consumable.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("list consumables failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list consumables failed")
	}

	return entities, total, nil
}

func (r *ConsumableRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.Consumable, error) {
	update := r.entClient.Client().Consumable.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if description, ok := updates["description"].(string); ok {
		update = update.SetDescription(description)
	}
	if categoryID, ok := updates["category_id"].(string); ok {
		update = update.SetCategoryID(categoryID)
	}
	if supplierID, ok := updates["supplier_id"].(string); ok {
		update = update.SetSupplierID(supplierID)
	}
	if locationID, ok := updates["location_id"].(string); ok {
		update = update.SetLocationID(locationID)
	}
	if modelName, ok := updates["model_name"].(string); ok {
		update = update.SetModelName(modelName)
	}
	if modelNumber, ok := updates["model_number"].(string); ok {
		update = update.SetModelNumber(modelNumber)
	}
	if amount, ok := updates["amount"].(int32); ok {
		update = update.SetAmount(amount)
	}
	if minAmount, ok := updates["min_amount"].(int32); ok {
		update = update.SetMinAmount(minAmount)
	}
	if purchaseCost, ok := updates["purchase_cost"].(float64); ok {
		update = update.SetPurchaseCost(purchaseCost)
	}
	if orderNumber, ok := updates["order_number"].(string); ok {
		update = update.SetOrderNumber(orderNumber)
	}
	if notes, ok := updates["notes"].(string); ok {
		update = update.SetNotes(notes)
	}
	if tags, ok := updates["tags"].(string); ok {
		update = update.SetTags(tags)
	}
	if metadata, ok := updates["metadata"].(map[string]interface{}); ok {
		update = update.SetMetadata(metadata)
	}
	if purchaseDate, ok := updates["purchase_date"].(time.Time); ok {
		update = update.SetPurchaseDate(purchaseDate)
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorConsumableNotFound("consumable not found")
		}
		r.log.Errorf("update consumable failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update consumable failed")
	}
	return entity, nil
}

func (r *ConsumableRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().Consumable.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorConsumableNotFound("consumable not found")
		}
		r.log.Errorf("delete consumable failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete consumable failed")
	}
	return nil
}
