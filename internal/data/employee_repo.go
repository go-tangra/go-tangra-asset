package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/employee"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type EmployeeRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

func NewEmployeeRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *EmployeeRepo {
	return &EmployeeRepo{
		log:       ctx.NewLoggerHelper("asset/employee/repo"),
		entClient: entClient,
	}
}

func (r *EmployeeRepo) Create(ctx context.Context, tenantID uint32, firstName, lastName string, opts ...func(*ent.EmployeeCreate)) (*ent.Employee, error) {
	id := uuid.New().String()

	create := r.entClient.Client().Employee.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetFirstName(firstName).
		SetLastName(lastName).
		SetCreateTime(time.Now())

	for _, opt := range opts {
		opt(create)
	}

	entity, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, assetV1.ErrorBadRequest("employee already exists with this email or number")
		}
		r.log.Errorf("create employee failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("create employee failed")
	}
	return entity, nil
}

func (r *EmployeeRepo) GetByID(ctx context.Context, id string) (*ent.Employee, error) {
	entity, err := r.entClient.Client().Employee.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get employee failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("get employee failed")
	}
	return entity, nil
}

func (r *EmployeeRepo) List(ctx context.Context, tenantID uint32, page, pageSize int, filters map[string]interface{}) ([]*ent.Employee, int, error) {
	query := r.entClient.Client().Employee.Query().Where(employee.TenantID(tenantID))

	if department, ok := filters["department"].(string); ok && department != "" {
		query = query.Where(employee.Department(department))
	}
	if q, ok := filters["query"].(string); ok && q != "" {
		query = query.Where(
			employee.Or(
				employee.FirstNameContains(q),
				employee.LastNameContains(q),
				employee.EmailContains(q),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count employees failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list employees failed")
	}

	if page > 0 && pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	entities, err := query.Order(ent.Asc(employee.FieldLastName), ent.Asc(employee.FieldFirstName)).All(ctx)
	if err != nil {
		r.log.Errorf("list employees failed: %s", err.Error())
		return nil, 0, assetV1.ErrorInternalServerError("list employees failed")
	}

	return entities, total, nil
}

func (r *EmployeeRepo) Update(ctx context.Context, id string, updates map[string]interface{}) (*ent.Employee, error) {
	update := r.entClient.Client().Employee.UpdateOneID(id)

	if firstName, ok := updates["first_name"].(string); ok {
		update = update.SetFirstName(firstName)
	}
	if lastName, ok := updates["last_name"].(string); ok {
		update = update.SetLastName(lastName)
	}
	if email, ok := updates["email"].(string); ok {
		update = update.SetEmail(email)
	}
	if phone, ok := updates["phone"].(string); ok {
		update = update.SetPhone(phone)
	}
	if department, ok := updates["department"].(string); ok {
		update = update.SetDepartment(department)
	}
	if jobTitle, ok := updates["job_title"].(string); ok {
		update = update.SetJobTitle(jobTitle)
	}
	if employeeNumber, ok := updates["employee_number"].(string); ok {
		update = update.SetEmployeeNumber(employeeNumber)
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

	update = update.SetUpdateTime(time.Now())

	entity, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, assetV1.ErrorEmployeeNotFound("employee not found")
		}
		r.log.Errorf("update employee failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("update employee failed")
	}
	return entity, nil
}

func (r *EmployeeRepo) ListAll(ctx context.Context, tenantID uint32) ([]*ent.Employee, error) {
	entities, err := r.entClient.Client().Employee.Query().
		Where(employee.TenantID(tenantID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list all employees failed: %s", err.Error())
		return nil, assetV1.ErrorInternalServerError("list all employees failed")
	}
	return entities, nil
}

func (r *EmployeeRepo) Delete(ctx context.Context, id string) error {
	// Check for active asset assignments
	count, err := r.entClient.Client().Employee.Query().
		Where(employee.ID(id)).
		QueryAssets().
		Count(ctx)
	if err != nil {
		r.log.Errorf("check employee assets failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete employee failed")
	}
	if count > 0 {
		return assetV1.ErrorEmployeeHasAssets("employee has %d assigned assets", count)
	}

	err = r.entClient.Client().Employee.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return assetV1.ErrorEmployeeNotFound("employee not found")
		}
		r.log.Errorf("delete employee failed: %s", err.Error())
		return assetV1.ErrorInternalServerError("delete employee failed")
	}
	return nil
}
