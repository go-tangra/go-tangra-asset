package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/assetassignment"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type AssignmentRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewAssignmentRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AssignmentRepo {
	return &AssignmentRepo{
		log:       ctx.NewLoggerHelper("asset/assignment/repo"),
		entClient: entClient,
	}
}

func (r *AssignmentRepo) Create(ctx context.Context, assetID, assetName, employeeID, employeeName string, action int32, assignedBy *uint32, notes string) (*ent.AssetAssignment, error) {
	id := uuid.New().String()

	create := r.entClient.Client().AssetAssignment.Create().
		SetID(id).
		SetAssetID(assetID).
		SetAssetName(assetName).
		SetEmployeeID(employeeID).
		SetEmployeeName(employeeName).
		SetAction(action).
		SetAssignedAt(time.Now())

	if assignedBy != nil {
		create = create.SetAssignedBy(*assignedBy)
	}
	if notes != "" {
		create = create.SetNotes(notes)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		r.log.Errorf("create assignment failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create assignment failed")
	}
	return entity, nil
}

// CloseActiveAssignment sets returned_at on the active assignment for an asset
func (r *AssignmentRepo) CloseActiveAssignment(ctx context.Context, assetID string) error {
	// Find active assignment (returned_at IS NULL)
	assignments, err := r.entClient.Client().AssetAssignment.Query().
		Where(
			assetassignment.AssetID(assetID),
			assetassignment.ReturnedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		r.log.Errorf("find active assignment failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("close assignment failed")
	}

	now := time.Now()
	for _, a := range assignments {
		_, err = r.entClient.Client().AssetAssignment.UpdateOneID(a.ID).
			SetReturnedAt(now).
			Save(ctx)
		if err != nil {
			r.log.Errorf("close assignment failed: %s", err.Error())
			return assetV1.ErrorInternalServerError("close assignment failed")
		}
	}

	return nil
}

func (r *AssignmentRepo) ListByAssetID(ctx context.Context, assetID string, page, pageSize int) ([]*ent.AssetAssignment, int, error) {
	query := r.entClient.Client().AssetAssignment.Query().
		Where(assetassignment.AssetID(assetID))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count assignments failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list assignments failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Desc(assetassignment.FieldAssignedAt)).All(ctx)
	if err != nil {
		r.log.Errorf("list assignments failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list assignments failed")
	}

	return entities, total, nil
}
