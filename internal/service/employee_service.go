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

type EmployeeService struct {
	assetV1.UnimplementedEmployeeServiceServer

	log          *log.Helper
	employeeRepo *data.EmployeeRepo
	ldapClient   *data.LdapClient
}

func NewEmployeeService(ctx *bootstrap.Context, employeeRepo *data.EmployeeRepo, ldapClient *data.LdapClient) *EmployeeService {
	return &EmployeeService{
		log:          ctx.NewLoggerHelper("asset/service/employee"),
		employeeRepo: employeeRepo,
		ldapClient:   ldapClient,
	}
}

func (s *EmployeeService) CreateEmployee(ctx context.Context, req *assetV1.CreateEmployeeRequest) (*assetV1.CreateEmployeeResponse, error) {
	opts := []func(*ent.EmployeeCreate){}

	if req.Email != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetEmail(*req.Email) })
	}
	if req.Phone != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetPhone(*req.Phone) })
	}
	if req.Department != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetDepartment(*req.Department) })
	}
	if req.JobTitle != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetJobTitle(*req.JobTitle) })
	}
	if req.EmployeeNumber != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetEmployeeNumber(*req.EmployeeNumber) })
	}
	if req.Notes != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetNotes(*req.Notes) })
	}
	if req.Tags != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetTags(*req.Tags) })
	}
	if req.Metadata != nil {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetMetadata(*req.Metadata) })
	}

	entity, err := s.employeeRepo.Create(ctx, req.GetTenantId(), req.GetFirstName(), req.GetLastName(), opts...)
	if err != nil {
		return nil, err
	}

	return &assetV1.CreateEmployeeResponse{
		Employee: employeeToProto(entity),
	}, nil
}

func (s *EmployeeService) GetEmployee(ctx context.Context, req *assetV1.GetEmployeeRequest) (*assetV1.GetEmployeeResponse, error) {
	entity, err := s.employeeRepo.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, assetV1.ErrorEmployeeNotFound("employee not found")
	}

	return &assetV1.GetEmployeeResponse{
		Employee: employeeToProto(entity),
	}, nil
}

func (s *EmployeeService) ListEmployees(ctx context.Context, req *assetV1.ListEmployeesRequest) (*assetV1.ListEmployeesResponse, error) {
	filters := make(map[string]interface{})
	if req.Department != nil {
		filters["department"] = *req.Department
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

	entities, total, err := s.employeeRepo.List(ctx, req.GetTenantId(), page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*assetV1.Employee, len(entities))
	for i, e := range entities {
		items[i] = employeeToProto(e)
	}

	return &assetV1.ListEmployeesResponse{
		Items: items,
		Total: ptrInt32(int32(total)),
	}, nil
}

func (s *EmployeeService) UpdateEmployee(ctx context.Context, req *assetV1.UpdateEmployeeRequest) (*assetV1.UpdateEmployeeResponse, error) {
	updates := make(map[string]interface{})

	if req.Data != nil {
		if req.Data.FirstName != nil {
			updates["first_name"] = *req.Data.FirstName
		}
		if req.Data.LastName != nil {
			updates["last_name"] = *req.Data.LastName
		}
		if req.Data.Email != nil {
			updates["email"] = *req.Data.Email
		}
		if req.Data.Phone != nil {
			updates["phone"] = *req.Data.Phone
		}
		if req.Data.Department != nil {
			updates["department"] = *req.Data.Department
		}
		if req.Data.JobTitle != nil {
			updates["job_title"] = *req.Data.JobTitle
		}
		if req.Data.EmployeeNumber != nil {
			updates["employee_number"] = *req.Data.EmployeeNumber
		}
		if req.Data.Notes != nil {
			updates["notes"] = *req.Data.Notes
		}
		if req.Data.Tags != nil {
			updates["tags"] = *req.Data.Tags
		}
		if req.Data.Metadata != nil {
			updates["metadata"] = *req.Data.Metadata
		}
	}

	entity, err := s.employeeRepo.Update(ctx, req.GetId(), updates)
	if err != nil {
		return nil, err
	}

	return &assetV1.UpdateEmployeeResponse{
		Employee: employeeToProto(entity),
	}, nil
}

func (s *EmployeeService) DeleteEmployee(ctx context.Context, req *assetV1.DeleteEmployeeRequest) (*emptypb.Empty, error) {
	err := s.employeeRepo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *EmployeeService) LdapSyncPreview(ctx context.Context, req *assetV1.LdapSyncPreviewRequest) (*assetV1.LdapSyncPreviewResponse, error) {
	if !s.ldapClient.IsConfigured() {
		return nil, assetV1.ErrorLdapNotConfigured("LDAP is not configured")
	}

	preview, _, err := s.buildPreview(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}

	return preview, nil
}

func (s *EmployeeService) LdapSyncExecute(ctx context.Context, req *assetV1.LdapSyncExecuteRequest) (*assetV1.LdapSyncExecuteResponse, error) {
	if !s.ldapClient.IsConfigured() {
		return nil, assetV1.ErrorLdapNotConfigured("LDAP is not configured")
	}

	preview, ldapEmployees, err := s.buildPreview(ctx, req.GetTenantId())
	if err != nil {
		return nil, err
	}

	// Build a set of selected DNs for filtering
	selectedDNs := make(map[string]bool)
	if len(req.GetSelectedDns()) > 0 {
		for _, dn := range req.GetSelectedDns() {
			selectedDNs[dn] = true
		}
	}

	// Build a DN -> LdapEmployee lookup
	ldapByDN := make(map[string]*data.LdapEmployee)
	for i := range ldapEmployees {
		ldapByDN[ldapEmployees[i].DN] = &ldapEmployees[i]
	}

	var (
		createdCount int32
		updatedCount int32
		skippedCount int32
		errorCount   int32
		syncErrors   []string
	)

	for _, change := range preview.Changes {
		dn := change.GetLdapDn()

		// Filter by selected DNs if specified
		if len(selectedDNs) > 0 && !selectedDNs[dn] {
			skippedCount++
			continue
		}

		ldapEmp := ldapByDN[dn]
		if ldapEmp == nil {
			skippedCount++
			continue
		}

		switch change.Action {
		case assetV1.LdapSyncChange_ACTION_CREATE:
			firstName := ldapEmp.FirstName
			lastName := ldapEmp.LastName
			if firstName == "" {
				firstName = "Unknown"
			}
			if lastName == "" {
				lastName = "Unknown"
			}

			opts := buildCreateOpts(ldapEmp)
			_, createErr := s.employeeRepo.Create(ctx, req.GetTenantId(), firstName, lastName, opts...)
			if createErr != nil {
				errorCount++
				syncErrors = append(syncErrors, "create "+ldapEmp.Email+": "+createErr.Error())
			} else {
				createdCount++
			}

		case assetV1.LdapSyncChange_ACTION_UPDATE:
			updates := buildUpdateMap(ldapEmp, change.ChangedFields)
			_, updateErr := s.employeeRepo.Update(ctx, change.GetExistingId(), updates)
			if updateErr != nil {
				errorCount++
				syncErrors = append(syncErrors, "update "+ldapEmp.Email+": "+updateErr.Error())
			} else {
				updatedCount++
			}
		}
	}

	return &assetV1.LdapSyncExecuteResponse{
		CreatedCount: createdCount,
		UpdatedCount: updatedCount,
		SkippedCount: skippedCount,
		ErrorCount:   errorCount,
		Errors:       syncErrors,
	}, nil
}

func employeeToProto(e *ent.Employee) *assetV1.Employee {
	if e == nil {
		return nil
	}

	result := &assetV1.Employee{
		Id:             &e.ID,
		TenantId:       e.TenantID,
		FirstName:      ptrString(e.FirstName),
		LastName:       ptrString(e.LastName),
		Email:          ptrString(e.Email),
		Phone:          ptrString(e.Phone),
		Department:     ptrString(e.Department),
		JobTitle:       ptrString(e.JobTitle),
		EmployeeNumber: ptrString(e.EmployeeNumber),
		Notes:          ptrString(e.Notes),
		Tags:           ptrString(e.Tags),
		Metadata:       ptrString(e.Metadata),
		CreatedBy:      e.CreateBy,
		UpdatedBy:      e.UpdateBy,
	}

	if e.CreateTime != nil {
		result.CreatedAt = timestamppb.New(*e.CreateTime)
	}
	if e.UpdateTime != nil {
		result.UpdatedAt = timestamppb.New(*e.UpdateTime)
	}

	return result
}
