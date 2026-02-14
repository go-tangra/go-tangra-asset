package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/supplier"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type SupplierRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewSupplierRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *SupplierRepo {
	return &SupplierRepo{
		log:       ctx.NewLoggerHelper("asset/supplier/repo"),
		entClient: entClient,
	}
}

func (r *SupplierRepo) Create(ctx context.Context, tenantID uint32, name string, opts ...func(*ent.SupplierCreate)) (*ent.Supplier, error) {
	id := uuid.New().String()

	create := r.entClient.Client().Supplier.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetName(name).
		SetStatus(1).
		SetCreateTime(time.Now())

	for _, opt := range opts {
		opt(create)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, assetV1.ErrorSupplierAlreadyExists("supplier '%s' already exists", name)
		}
		r.log.Errorf("create supplier failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create supplier failed")
	}
	return entity, nil
}

func (r *SupplierRepo) GetByID(ctx context.Context, id string) (*ent.Supplier, error) {
	entity, err := r.entClient.Client().Supplier.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get supplier failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get supplier failed")
	}
	return entity, nil
}

func (r *SupplierRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.Supplier, int, error) {
	query := r.entClient.Client().Supplier.Query().Where(supplier.TenantID(tenantID))

	if status, ok := filters["status"].(int32); ok && status > 0 {
		query = query.Where(supplier.Status(status))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(supplier.NameContains(q))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count suppliers failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list suppliers failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(supplier.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("list suppliers failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list suppliers failed")
	}

	return entities, total, nil
}

func (r *SupplierRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.Supplier, error) {
	update := r.entClient.Client().Supplier.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if code, ok := updates["code"].(string); ok {
		update = update.SetCode(code)
	}
	if address, ok := updates["address"].(string); ok {
		update = update.SetAddress(address)
	}
	if city, ok := updates["city"].(string); ok {
		update = update.SetCity(city)
	}
	if state, ok := updates["state"].(string); ok {
		update = update.SetState(state)
	}
	if country, ok := updates["country"].(string); ok {
		update = update.SetCountry(country)
	}
	if postalCode, ok := updates["postal_code"].(string); ok {
		update = update.SetPostalCode(postalCode)
	}
	if contactPerson, ok := updates["contact_person"].(string); ok {
		update = update.SetContactPerson(contactPerson)
	}
	if telephone, ok := updates["telephone"].(string); ok {
		update = update.SetTelephone(telephone)
	}
	if email, ok := updates["email"].(string); ok {
		update = update.SetEmail(email)
	}
	if website, ok := updates["website"].(string); ok {
		update = update.SetWebsite(website)
	}
	if notes, ok := updates["notes"].(string); ok {
		update = update.SetNotes(notes)
	}
	if tags, ok := updates["tags"].(string); ok {
		update = update.SetTags(tags)
	}
	if metadata, ok := updates["metadata"].(string); ok {
		update = update.SetMetadata(metadata)
	}
	if status, ok := updates["status"].(int32); ok {
		update = update.SetStatus(status)
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorSupplierNotFound("supplier not found")
		}
		r.log.Errorf("update supplier failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update supplier failed")
	}
	return entity, nil
}

func (r *SupplierRepo) Delete(ctx context.Context, id string) error {
	// Check for associated assets
	count, err := r.entClient.Client().Supplier.Query().
		Where(supplier.ID(id)).
		QueryAssets().
		Count(ctx)
	if err != nil {
		r.log.Errorf("check supplier assets failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete supplier failed")
	}
	if count > 0 {
		return assetV1.ErrorSupplierHasAssets("supplier has %d associated assets", count)
	}

	err = r.entClient.Client().Supplier.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorSupplierNotFound("supplier not found")
		}
		r.log.Errorf("delete supplier failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete supplier failed")
	}
	return nil
}
