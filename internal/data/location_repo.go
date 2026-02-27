package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/location"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type LocationRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewLocationRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *LocationRepo {
	return &LocationRepo{
		log:       ctx.NewLoggerHelper("asset/location/repo"),
		entClient: entClient,
	}
}

func (r *LocationRepo) Create(ctx context.Context, tenantID uint32, name string, opts ...func(*ent.LocationCreate)) (*ent.Location, error) {
	id := uuid.New().String()

	create := r.entClient.Client().Location.Create().
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
			return nil, assetV1.ErrorBadRequest("location '%s' already exists", name)
		}
		r.log.Errorf("create location failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create location failed")
	}
	return entity, nil
}

func (r *LocationRepo) GetByID(ctx context.Context, id string) (*ent.Location, error) {
	entity, err := r.entClient.Client().Location.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get location failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get location failed")
	}
	return entity, nil
}

func (r *LocationRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.Location, int, error) {
	query := r.entClient.Client().Location.Query().Where(location.TenantID(tenantID))

	if parentID, ok := filters["parent_id"].(string); ok && parentID != "" {
		query = query.Where(location.ParentID(parentID))
	}
	if status, ok := filters["status"].(int32); ok && status > 0 {
		query = query.Where(location.Status(status))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(location.NameContains(q))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count locations failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list locations failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(location.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("list locations failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list locations failed")
	}

	return entities, total, nil
}

func (r *LocationRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.Location, error) {
	update := r.entClient.Client().Location.UpdateOneID(id)

	if name, ok := updates["name"].(string); ok {
		update = update.SetName(name)
	}
	if code, ok := updates["code"].(string); ok {
		update = update.SetCode(code)
	}
	if description, ok := updates["description"].(string); ok {
		update = update.SetDescription(description)
	}
	if parentID, ok := updates["parent_id"].(string); ok {
		update = update.SetParentID(parentID)
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
	if contact, ok := updates["contact"].(string); ok {
		update = update.SetContact(contact)
	}
	if phone, ok := updates["phone"].(string); ok {
		update = update.SetPhone(phone)
	}
	if email, ok := updates["email"].(string); ok {
		update = update.SetEmail(email)
	}
	if status, ok := updates["status"].(int32); ok {
		update = update.SetStatus(status)
	}
	if tags, ok := updates["tags"].(string); ok {
		update = update.SetTags(tags)
	}
	if metadata, ok := updates["metadata"].(map[string]interface{}); ok {
		update = update.SetMetadata(metadata)
	}

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorLocationNotFound("location not found")
		}
		r.log.Errorf("update location failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update location failed")
	}
	return entity, nil
}

func (r *LocationRepo) Delete(ctx context.Context, id string) error {
	// Check for children
	count, err := r.entClient.Client().Location.Query().
		Where(location.ID(id)).
		QueryChildren().
		Count(ctx)
	if err != nil {
		r.log.Errorf("check location children failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete location failed")
	}
	if count > 0 {
		return assetV1.ErrorLocationHasChildren("location has %d children", count)
	}

	// Check for assets
	assetCount, err := r.entClient.Client().Location.Query().
		Where(location.ID(id)).
		QueryAssets().
		Count(ctx)
	if err != nil {
		r.log.Errorf("check location assets failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete location failed")
	}
	if assetCount > 0 {
		return assetV1.ErrorLocationHasAssets("location has %d assets", assetCount)
	}

	err = r.entClient.Client().Location.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorLocationNotFound("location not found")
		}
		r.log.Errorf("delete location failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete location failed")
	}
	return nil
}

func (r *LocationRepo) GetTree(ctx context.Context, tenantID uint32, rootID string) ([]*ent.Location, error) {
	query := r.entClient.Client().Location.Query().Where(location.TenantID(tenantID))

	entities, err := query.Order(ent.Asc(location.FieldName)).All(ctx)
	if err != nil {
		r.log.Errorf("get location tree failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get location tree failed")
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

		var filtered []*ent.Location
		for _, e := range entities {
			if subtreeIDs[e.ID] {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}

	return entities, nil
}
