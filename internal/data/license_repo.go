package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/license"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type LicenseRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewLicenseRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *LicenseRepo {
	return &LicenseRepo{
		log:       ctx.NewLoggerHelper("asset/license/repo"),
		entClient: entClient,
	}
}

func (r *LicenseRepo) Create(ctx context.Context, tenantID uint32, name string, opts ...func(*ent.LicenseCreate)) (*ent.License, error) {
	id := uuid.New().String()

	create := r.entClient.Client().License.Create().
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
			return nil, assetV1.ErrorBadRequest("license '%s' already exists", name)
		}
		r.log.Errorf("create license failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create license failed")
	}
	return entity, nil
}

func (r *LicenseRepo) GetByID(ctx context.Context, id string) (*ent.License, error) {
	entity, err := r.entClient.Client().License.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get license failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get license failed")
	}
	return entity, nil
}

func (r *LicenseRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.License, int, error) {
	query := r.entClient.Client().License.Query().Where(license.TenantID(tenantID))

	if supplierID, ok := filters["supplier_id"].(string); ok && supplierID != "" {
		query = query.Where(license.SupplierID(supplierID))
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where(license.Status(status))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(license.NameContains(q))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count licenses failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list licenses failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(license.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("list licenses failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list licenses failed")
	}

	return entities, total, nil
}

func (r *LicenseRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.License, error) {
	update := r.entClient.Client().License.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if supplierID, ok := updates["supplier_id"].(string); ok {
		update = update.SetSupplierID(supplierID)
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
	if status, ok := updates["status"].(string); ok {
		update = update.SetStatus(status)
	}
	if purchaseDate, ok := updates["purchase_date"].(time.Time); ok {
		update = update.SetPurchaseDate(purchaseDate)
	}
	if validFrom, ok := updates["valid_from"].(time.Time); ok {
		update = update.SetValidFrom(validFrom)
	}
	if validTo, ok := updates["valid_to"].(time.Time); ok {
		update = update.SetValidTo(validTo)
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorLicenseNotFound("license not found")
		}
		r.log.Errorf("update license failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update license failed")
	}
	return entity, nil
}

func (r *LicenseRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().License.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorLicenseNotFound("license not found")
		}
		r.log.Errorf("delete license failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete license failed")
	}
	return nil
}
