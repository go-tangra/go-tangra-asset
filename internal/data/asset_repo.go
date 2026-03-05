package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/asset"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type AssetRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewAssetRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AssetRepo {
	return &AssetRepo{
		log:       ctx.NewLoggerHelper("asset/asset/repo"),
		entClient: entClient,
	}
}

func (r *AssetRepo) Create(ctx context.Context, tenantID uint32, assetTag, name string, opts ...func(*ent.AssetCreate)) (*ent.Asset, error) {
	id := uuid.New().String()

	create := r.entClient.Client().Asset.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetAssetTag(assetTag).
		SetName(name).
		SetStatus(1). // Deployable
		SetCreateTime(time.Now())

	for _, opt := range opts {
		opt(create)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, assetV1.ErrorBadRequest("asset with tag '%s' already exists", assetTag)
		}
		r.log.Errorf("create asset failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create asset failed")
	}
	return entity, nil
}

func (r *AssetRepo) GetByID(ctx context.Context, id string) (*ent.Asset, error) {
	entity, err := r.entClient.Client().Asset.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get asset failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get asset failed")
	}
	return entity, nil
}

func (r *AssetRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.Asset, int, error) {
	query := r.entClient.Client().Asset.Query().Where(asset.TenantID(tenantID))

	if status, ok := filters["status"].(int32); ok && status > 0 {
		query = query.Where(asset.Status(status))
	}
	if categoryID, ok := filters["category_id"].(string); ok && categoryID != "" {
		query = query.Where(asset.CategoryID(categoryID))
	}
	if supplierID, ok := filters["supplier_id"].(string); ok && supplierID != "" {
		query = query.Where(asset.SupplierID(supplierID))
	}
	if locationID, ok := filters["location_id"].(string); ok && locationID != "" {
		query = query.Where(asset.LocationID(locationID))
	}
	if userID, ok := filters["user_id"].(uint32); ok && userID > 0 {
		query = query.Where(asset.UserID(userID))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(
			asset.Or(
				asset.NameContains(q),
				asset.AssetTagContains(q),
				asset.SerialContains(q),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count assets failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list assets failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(asset.FieldAssetTag)).All(ctx)
	if err != nil {
		r.log.Errorf("list assets failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list assets failed")
	}

	return entities, total, nil
}

func (r *AssetRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.Asset, error) {
	update := r.entClient.Client().Asset.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if assetTag, ok := updates["asset_tag"].(string); ok {
		update = update.SetAssetTag(assetTag)
	}
	if serial, ok := updates["serial"].(string); ok {
		update = update.SetSerial(serial)
	}
	if modelName, ok := updates["model_name"].(string); ok {
		update = update.SetModelName(modelName)
	}
	if modelNumber, ok := updates["model_number"].(string); ok {
		update = update.SetModelNumber(modelNumber)
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
	if userID, ok := updates["user_id"].(uint32); ok {
		update = update.SetUserID(userID)
	}
	if status, ok := updates["status"].(int32); ok {
		update = update.SetStatus(status)
	}
	if photoKey, ok := updates["photo_key"].(string); ok {
		update = update.SetPhotoKey(photoKey)
	}
	if warrantyMonths, ok := updates["warranty_months"].(int32); ok {
		update = update.SetWarrantyMonths(warrantyMonths)
	}
	if orderNumber, ok := updates["order_number"].(string); ok {
		update = update.SetOrderNumber(orderNumber)
	}
	if purchaseCost, ok := updates["purchase_cost"].(float64); ok {
		update = update.SetPurchaseCost(purchaseCost)
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
	if salvageValue, ok := updates["salvage_value"].(float64); ok {
		update = update.SetSalvageValue(salvageValue)
	}
	if usefulLifeYears, ok := updates["useful_life_years"].(int32); ok {
		update = update.SetUsefulLifeYears(usefulLifeYears)
	}
	if depreciationRate, ok := updates["depreciation_rate"].(float64); ok {
		update = update.SetDepreciationRate(depreciationRate)
	}

	// Handle clearing nullable fields
	if _, ok := updates["clear_user_id"]; ok {
		update = update.ClearUserID()
	}
	if _, ok := updates["clear_location_id"]; ok {
		update = update.ClearLocationID()
	}
	if _, ok := updates["clear_photo_key"]; ok {
		update = update.ClearPhotoKey()
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorAssetNotFound("asset not found")
		}
		r.log.Errorf("update asset failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update asset failed")
	}
	return entity, nil
}

func (r *AssetRepo) ListAll(ctx context.Context, tenantID uint32) ([]*ent.Asset, error) {
	entities, err := r.entClient.Client().Asset.Query().
		Where(asset.TenantID(tenantID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list all assets failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("list all assets failed")
	}
	return entities, nil
}

func (r *AssetRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().Asset.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorAssetNotFound("asset not found")
		}
		r.log.Errorf("delete asset failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete asset failed")
	}
	return nil
}
