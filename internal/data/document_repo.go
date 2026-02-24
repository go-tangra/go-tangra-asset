package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/assetdocument"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type DocumentRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewDocumentRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *DocumentRepo {
	return &DocumentRepo{
		log:       ctx.NewLoggerHelper("asset/document/repo"),
		entClient: entClient,
	}
}

func (r *DocumentRepo) Create(ctx context.Context, entityType, entityID, fileName string, fileSize int64, mimeType, storageKey, checksum string, description string, uploadedBy *uint32) (*ent.AssetDocument, error) {
	id := uuid.New().String()

	create := r.entClient.Client().AssetDocument.Create().
		SetID(id).
		SetEntityType(entityType).
		SetEntityID(entityID).
		SetFileName(fileName).
		SetFileSize(fileSize).
		SetMimeType(mimeType).
		SetStorageKey(storageKey).
		SetChecksum(checksum)

	if description != "" {
		create = create.SetDescription(description)
	}
	if uploadedBy != nil {
		create = create.SetUploadedBy(*uploadedBy)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		r.log.Errorf("create document failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create document failed")
	}
	return entity, nil
}

func (r *DocumentRepo) GetByID(ctx context.Context, id string) (*ent.AssetDocument, error) {
	entity, err := r.entClient.Client().AssetDocument.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get document failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get document failed")
	}
	return entity, nil
}

func (r *DocumentRepo) ListByEntity(ctx context.Context, entityType, entityID string) ([]*ent.AssetDocument, error) {
	entities, err := r.entClient.Client().AssetDocument.Query().
		Where(
			assetdocument.EntityType(entityType),
			assetdocument.EntityID(entityID),
		).
		Order(ent.Desc(assetdocument.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list documents failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("list documents failed")
	}
	return entities, nil
}

func (r *DocumentRepo) DeleteByEntityID(ctx context.Context, entityType, entityID string) (int, error) {
	n, err := r.entClient.Client().AssetDocument.Delete().
		Where(
			assetdocument.EntityType(entityType),
			assetdocument.EntityID(entityID),
		).
		Exec(ctx)
	if err != nil {
		r.log.Errorf("delete documents by entity failed: %s", err.Error())
		return 0, assetV1.ErrorInternalServerError("delete documents failed")
	}
	return n, nil
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().AssetDocument.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorDocumentNotFound("document not found")
		}
		r.log.Errorf("delete document failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete document failed")
	}
	return nil
}
