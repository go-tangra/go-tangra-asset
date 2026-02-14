package service

import (
	"context"
	"strings"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

// computeChangedFields compares an existing employee with LDAP data and returns the list of changed fields
func computeChangedFields(existing *ent.Employee, ldap *data.LdapEmployee) []string {
	var changed []string

	if ldap.FirstName != "" && ldap.FirstName != existing.FirstName {
		changed = append(changed, "first_name")
	}
	if ldap.LastName != "" && ldap.LastName != existing.LastName {
		changed = append(changed, "last_name")
	}
	if ldap.Email != "" && ldap.Email != existing.Email {
		changed = append(changed, "email")
	}
	if ldap.Phone != "" && ldap.Phone != existing.Phone {
		changed = append(changed, "phone")
	}
	if ldap.Department != "" && ldap.Department != existing.Department {
		changed = append(changed, "department")
	}
	if ldap.JobTitle != "" && ldap.JobTitle != existing.JobTitle {
		changed = append(changed, "job_title")
	}
	if ldap.EmployeeNumber != "" && ldap.EmployeeNumber != existing.EmployeeNumber {
		changed = append(changed, "employee_number")
	}

	return changed
}

// buildCreateOpts creates functional options for EmployeeCreate from LDAP data
func buildCreateOpts(ldap *data.LdapEmployee) []func(*ent.EmployeeCreate) {
	var opts []func(*ent.EmployeeCreate)

	if ldap.Email != "" {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetEmail(ldap.Email) })
	}
	if ldap.Phone != "" {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetPhone(ldap.Phone) })
	}
	if ldap.Department != "" {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetDepartment(ldap.Department) })
	}
	if ldap.JobTitle != "" {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetJobTitle(ldap.JobTitle) })
	}
	if ldap.EmployeeNumber != "" {
		opts = append(opts, func(c *ent.EmployeeCreate) { c.SetEmployeeNumber(ldap.EmployeeNumber) })
	}

	return opts
}

// buildUpdateMap creates an update map from LDAP data for changed fields
func buildUpdateMap(ldap *data.LdapEmployee, changedFields []string) map[string]interface{} {
	updates := make(map[string]interface{})

	for _, field := range changedFields {
		switch field {
		case "first_name":
			updates["first_name"] = ldap.FirstName
		case "last_name":
			updates["last_name"] = ldap.LastName
		case "email":
			updates["email"] = ldap.Email
		case "phone":
			updates["phone"] = ldap.Phone
		case "department":
			updates["department"] = ldap.Department
		case "job_title":
			updates["job_title"] = ldap.JobTitle
		case "employee_number":
			updates["employee_number"] = ldap.EmployeeNumber
		}
	}

	return updates
}

// ldapEmployeeToProto converts LDAP employee data to a proto Employee message
func ldapEmployeeToProto(ldap *data.LdapEmployee) *assetV1.Employee {
	return &assetV1.Employee{
		FirstName:      ptrString(ldap.FirstName),
		LastName:       ptrString(ldap.LastName),
		Email:          ptrString(ldap.Email),
		Phone:          ptrString(ldap.Phone),
		Department:     ptrString(ldap.Department),
		JobTitle:       ptrString(ldap.JobTitle),
		EmployeeNumber: ptrString(ldap.EmployeeNumber),
	}
}

// buildPreview generates a sync preview by comparing LDAP entries against existing employees
func (s *EmployeeService) buildPreview(ctx context.Context, tenantID uint32) (*assetV1.LdapSyncPreviewResponse, []data.LdapEmployee, error) {
	ldapEmployees, err := s.ldapClient.FetchEmployees(ctx)
	if err != nil {
		return nil, nil, assetV1.ErrorLdapSyncFailed("failed to fetch LDAP employees: %v", err)
	}

	existingEmployees, err := s.employeeRepo.ListAll(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}

	// Build lookup maps
	byEmployeeNumber := make(map[string]*ent.Employee)
	byEmail := make(map[string]*ent.Employee)
	for _, e := range existingEmployees {
		if e.EmployeeNumber != "" {
			byEmployeeNumber[strings.ToLower(e.EmployeeNumber)] = e
		}
		if e.Email != "" {
			byEmail[strings.ToLower(e.Email)] = e
		}
	}

	var changes []*assetV1.LdapSyncChange
	var warnings []string
	unchangedCount := int32(0)

	for i := range ldapEmployees {
		ldapEmp := &ldapEmployees[i]
		var existing *ent.Employee

		// Match by employee number first, then by email
		if ldapEmp.EmployeeNumber != "" {
			existing = byEmployeeNumber[strings.ToLower(ldapEmp.EmployeeNumber)]
		}
		if existing == nil && ldapEmp.Email != "" {
			existing = byEmail[strings.ToLower(ldapEmp.Email)]
		}

		if existing == nil {
			// New employee
			changes = append(changes, &assetV1.LdapSyncChange{
				Action:   assetV1.LdapSyncChange_ACTION_CREATE,
				Employee: ldapEmployeeToProto(ldapEmp),
				LdapDn:   ptrString(ldapEmp.DN),
			})
		} else {
			changedFields := computeChangedFields(existing, ldapEmp)
			if len(changedFields) > 0 {
				changes = append(changes, &assetV1.LdapSyncChange{
					Action:        assetV1.LdapSyncChange_ACTION_UPDATE,
					Employee:      ldapEmployeeToProto(ldapEmp),
					ChangedFields: changedFields,
					ExistingId:    ptrString(existing.ID),
					LdapDn:        ptrString(ldapEmp.DN),
				})
			} else {
				unchangedCount++
			}
		}
	}

	newCount := int32(0)
	updateCount := int32(0)
	for _, c := range changes {
		switch c.Action {
		case assetV1.LdapSyncChange_ACTION_CREATE:
			newCount++
		case assetV1.LdapSyncChange_ACTION_UPDATE:
			updateCount++
		}
	}

	return &assetV1.LdapSyncPreviewResponse{
		TotalLdapEntries: int32(len(ldapEmployees)),
		NewCount:         newCount,
		UpdateCount:      updateCount,
		UnchangedCount:   unchangedCount,
		Changes:          changes,
		Warnings:         warnings,
	}, ldapEmployees, nil
}
