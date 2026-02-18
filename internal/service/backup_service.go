package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-common/grpcx"

	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/asset"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/assetassignment"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/category"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/consumable"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/employee"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/insurancepolicy"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/insurancepolicyasset"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/license"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/location"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent/supplier"
)

const (
	backupModule  = "asset"
	backupVersion = "1.0"
)

type BackupService struct {
	assetV1.UnimplementedBackupServiceServer

	log       *log.Helper
	entClient *entCrud.EntClient[*ent.Client]
}

func NewBackupService(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BackupService {
	return &BackupService{
		log:       ctx.NewLoggerHelper("asset/service/backup"),
		entClient: entClient,
	}
}

type backupData struct {
	Module     string         `json:"module"`
	Version    string         `json:"version"`
	ExportedAt time.Time      `json:"exportedAt"`
	TenantID   uint32         `json:"tenantId"`
	FullBackup bool           `json:"fullBackup"`
	Data       backupEntities `json:"data"`
}

type backupEntities struct {
	Categories            []json.RawMessage `json:"categories,omitempty"`
	Suppliers             []json.RawMessage `json:"suppliers,omitempty"`
	Locations             []json.RawMessage `json:"locations,omitempty"`
	Employees             []json.RawMessage `json:"employees,omitempty"`
	Assets                []json.RawMessage `json:"assets,omitempty"`
	Consumables           []json.RawMessage `json:"consumables,omitempty"`
	Licenses              []json.RawMessage `json:"licenses,omitempty"`
	AssetAssignments      []json.RawMessage `json:"assetAssignments,omitempty"`
	AssetDocuments        []json.RawMessage `json:"assetDocuments,omitempty"`
	InsurancePolicies     []json.RawMessage `json:"insurancePolicies,omitempty"`
	InsurancePolicyAssets []json.RawMessage `json:"insurancePolicyAssets,omitempty"`
}

// topologicalSortByParentID sorts items so parents come before children.
func topologicalSortByParentID[T any](items []T, getID func(T) string, getParentID func(T) string) []T {
	idSet := make(map[string]bool, len(items))
	for _, item := range items {
		idSet[getID(item)] = true
	}

	childMap := make(map[string][]T)
	var roots []T
	for _, item := range items {
		pid := getParentID(item)
		if pid == "" || !idSet[pid] {
			roots = append(roots, item)
		} else {
			childMap[pid] = append(childMap[pid], item)
		}
	}

	result := make([]T, 0, len(items))
	var walk func([]T)
	walk = func(nodes []T) {
		for _, n := range nodes {
			result = append(result, n)
			if children, ok := childMap[getID(n)]; ok {
				walk(children)
			}
		}
	}
	walk(roots)
	return result
}

func marshalEntities[T any](entities []*T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(entities))
	for _, e := range entities {
		b, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

func (s *BackupService) ExportBackup(ctx context.Context, req *assetV1.ExportBackupRequest) (*assetV1.ExportBackupResponse, error) {
	tenantID := grpcx.GetTenantIDFromContext(ctx)
	full := false

	if grpcx.IsPlatformAdmin(ctx) && req.TenantId != nil && *req.TenantId == 0 {
		full = true
		tenantID = 0
	} else if req.TenantId != nil && *req.TenantId != 0 {
		if grpcx.IsPlatformAdmin(ctx) {
			tenantID = *req.TenantId
		}
	}

	client := s.entClient.Client()
	now := time.Now()

	categories, err := s.exportCategories(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export categories: %w", err)
	}
	suppliers, err := s.exportSuppliers(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export suppliers: %w", err)
	}
	locations, err := s.exportLocations(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export locations: %w", err)
	}
	employees, err := s.exportEmployees(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export employees: %w", err)
	}
	assets, err := s.exportAssets(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export assets: %w", err)
	}
	consumables, err := s.exportConsumables(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export consumables: %w", err)
	}
	licenses, err := s.exportLicenses(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export licenses: %w", err)
	}
	assetAssignments, err := s.exportAssetAssignments(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export asset assignments: %w", err)
	}
	assetDocuments, err := s.exportAssetDocuments(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export asset documents: %w", err)
	}
	insurancePolicies, err := s.exportInsurancePolicies(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export insurance policies: %w", err)
	}
	insurancePolicyAssets, err := s.exportInsurancePolicyAssets(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export insurance policy assets: %w", err)
	}

	backup := backupData{
		Module:     backupModule,
		Version:    backupVersion,
		ExportedAt: now,
		TenantID:   tenantID,
		FullBackup: full,
		Data: backupEntities{
			Categories:            categories,
			Suppliers:             suppliers,
			Locations:             locations,
			Employees:             employees,
			Assets:                assets,
			Consumables:           consumables,
			Licenses:              licenses,
			AssetAssignments:      assetAssignments,
			AssetDocuments:        assetDocuments,
			InsurancePolicies:     insurancePolicies,
			InsurancePolicyAssets: insurancePolicyAssets,
		},
	}

	data, err := json.Marshal(backup)
	if err != nil {
		return nil, fmt.Errorf("marshal backup: %w", err)
	}

	entityCounts := map[string]int64{
		"categories":            int64(len(categories)),
		"suppliers":             int64(len(suppliers)),
		"locations":             int64(len(locations)),
		"employees":             int64(len(employees)),
		"assets":                int64(len(assets)),
		"consumables":           int64(len(consumables)),
		"licenses":              int64(len(licenses)),
		"assetAssignments":      int64(len(assetAssignments)),
		"assetDocuments":        int64(len(assetDocuments)),
		"insurancePolicies":     int64(len(insurancePolicies)),
		"insurancePolicyAssets": int64(len(insurancePolicyAssets)),
	}

	s.log.Infof("exported backup: module=%s tenant=%d full=%v entities=%v", backupModule, tenantID, full, entityCounts)

	return &assetV1.ExportBackupResponse{
		Data:         data,
		Module:       backupModule,
		Version:      backupVersion,
		ExportedAt:   timestamppb.New(now),
		TenantId:     tenantID,
		EntityCounts: entityCounts,
	}, nil
}

func (s *BackupService) ImportBackup(ctx context.Context, req *assetV1.ImportBackupRequest) (*assetV1.ImportBackupResponse, error) {
	tenantID := grpcx.GetTenantIDFromContext(ctx)
	isPlatformAdmin := grpcx.IsPlatformAdmin(ctx)
	mode := req.GetMode()

	var backup backupData
	if err := json.Unmarshal(req.GetData(), &backup); err != nil {
		return nil, fmt.Errorf("invalid backup data: %w", err)
	}

	if backup.Module != backupModule {
		return nil, fmt.Errorf("backup module mismatch: expected %s, got %s", backupModule, backup.Module)
	}
	if backup.Version != backupVersion {
		return nil, fmt.Errorf("backup version mismatch: expected %s, got %s", backupVersion, backup.Version)
	}

	// For full backups, only platform admins can restore
	if backup.FullBackup && !isPlatformAdmin {
		return nil, fmt.Errorf("only platform admins can restore full backups")
	}

	// Non-platform admins always restore to their own tenant
	if !isPlatformAdmin || !backup.FullBackup {
		tenantID = grpcx.GetTenantIDFromContext(ctx)
	} else {
		tenantID = 0 // Signal for full backup restore — each entity carries its own tenant_id
	}

	client := s.entClient.Client()
	var results []*assetV1.EntityImportResult
	var warnings []string

	// Import in FK dependency order
	importFuncs := []struct {
		name  string
		items []json.RawMessage
		fn    func(context.Context, *ent.Client, []json.RawMessage, uint32, bool, assetV1.RestoreMode) (*assetV1.EntityImportResult, []string)
	}{
		{"categories", backup.Data.Categories, s.importCategories},
		{"suppliers", backup.Data.Suppliers, s.importSuppliers},
		{"locations", backup.Data.Locations, s.importLocations},
		{"employees", backup.Data.Employees, s.importEmployees},
		{"assets", backup.Data.Assets, s.importAssets},
		{"consumables", backup.Data.Consumables, s.importConsumables},
		{"licenses", backup.Data.Licenses, s.importLicenses},
		{"assetAssignments", backup.Data.AssetAssignments, s.importAssetAssignments},
		{"assetDocuments", backup.Data.AssetDocuments, s.importAssetDocuments},
		{"insurancePolicies", backup.Data.InsurancePolicies, s.importInsurancePolicies},
		{"insurancePolicyAssets", backup.Data.InsurancePolicyAssets, s.importInsurancePolicyAssets},
	}

	for _, imp := range importFuncs {
		if len(imp.items) == 0 {
			continue
		}
		result, w := imp.fn(ctx, client, imp.items, tenantID, backup.FullBackup, mode)
		if result != nil {
			results = append(results, result)
		}
		warnings = append(warnings, w...)
	}

	s.log.Infof("imported backup: module=%s tenant=%d mode=%v results=%d warnings=%d", backupModule, tenantID, mode, len(results), len(warnings))

	return &assetV1.ImportBackupResponse{
		Success:  true,
		Results:  results,
		Warnings: warnings,
	}, nil
}

// --- Export helpers ---

func (s *BackupService) exportCategories(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Category.Query()
	if !full {
		query = query.Where(category.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportSuppliers(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Supplier.Query()
	if !full {
		query = query.Where(supplier.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportLocations(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Location.Query()
	if !full {
		query = query.Where(location.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportEmployees(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Employee.Query()
	if !full {
		query = query.Where(employee.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportAssets(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Asset.Query()
	if !full {
		query = query.Where(asset.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportConsumables(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.Consumable.Query()
	if !full {
		query = query.Where(consumable.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportLicenses(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.License.Query()
	if !full {
		query = query.Where(license.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportAssetAssignments(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.AssetAssignment.Query()
	if !full {
		// AssetAssignment doesn't have tenant_id — filter via parent asset
		query = query.Where(assetassignment.HasAssetWith(asset.TenantID(tenantID)))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportAssetDocuments(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.AssetDocument.Query()
	// AssetDocument has no tenant_id and no easy FK filter — export all
	_ = tenantID
	_ = full
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportInsurancePolicies(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.InsurancePolicy.Query()
	if !full {
		query = query.Where(insurancepolicy.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportInsurancePolicyAssets(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.InsurancePolicyAsset.Query()
	if !full {
		query = query.Where(insurancepolicyasset.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

// --- Import helpers ---

func (s *BackupService) importCategories(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "categories", Total: int64(len(items))}
	var warnings []string

	var entities []*ent.Category
	for _, raw := range items {
		var e ent.Category
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("categories: unmarshal error: %v", err))
			result.Failed++
			continue
		}
		entities = append(entities, &e)
	}

	// Topological sort for self-referential parent_id (string, not pointer)
	sorted := topologicalSortByParentID(entities,
		func(e *ent.Category) string { return e.ID },
		func(e *ent.Category) string { return e.ParentID },
	)

	for _, e := range sorted {
		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Category.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Category.UpdateOneID(e.ID).
				SetName(e.Name).
				SetDescription(e.Description).
				SetParentID(e.ParentID).
				SetIcon(e.Icon).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("categories: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Category.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetDescription(e.Description).
				SetParentID(e.ParentID).
				SetIcon(e.Icon).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("categories: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importSuppliers(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "suppliers", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.Supplier
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("suppliers: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Supplier.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Supplier.UpdateOneID(e.ID).
				SetName(e.Name).
				SetCode(e.Code).
				SetAddress(e.Address).
				SetCity(e.City).
				SetState(e.State).
				SetCountry(e.Country).
				SetPostalCode(e.PostalCode).
				SetContactPerson(e.ContactPerson).
				SetTelephone(e.Telephone).
				SetEmail(e.Email).
				SetWebsite(e.Website).
				SetNotes(e.Notes).
				SetTags(e.Tags).
				SetMetadata(e.Metadata).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("suppliers: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Supplier.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetCode(e.Code).
				SetAddress(e.Address).
				SetCity(e.City).
				SetState(e.State).
				SetCountry(e.Country).
				SetPostalCode(e.PostalCode).
				SetContactPerson(e.ContactPerson).
				SetTelephone(e.Telephone).
				SetEmail(e.Email).
				SetWebsite(e.Website).
				SetNotes(e.Notes).
				SetTags(e.Tags).
				SetMetadata(e.Metadata).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("suppliers: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importLocations(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "locations", Total: int64(len(items))}
	var warnings []string

	var entities []*ent.Location
	for _, raw := range items {
		var e ent.Location
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("locations: unmarshal error: %v", err))
			result.Failed++
			continue
		}
		entities = append(entities, &e)
	}

	// Topological sort for self-referential parent_id (string, not pointer)
	sorted := topologicalSortByParentID(entities,
		func(e *ent.Location) string { return e.ID },
		func(e *ent.Location) string { return e.ParentID },
	)

	for _, e := range sorted {
		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Location.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Location.UpdateOneID(e.ID).
				SetName(e.Name).
				SetCode(e.Code).
				SetDescription(e.Description).
				SetParentID(e.ParentID).
				SetPath(e.Path).
				SetAddress(e.Address).
				SetCity(e.City).
				SetState(e.State).
				SetCountry(e.Country).
				SetPostalCode(e.PostalCode).
				SetContact(e.Contact).
				SetPhone(e.Phone).
				SetEmail(e.Email).
				SetStatus(e.Status).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("locations: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Location.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetCode(e.Code).
				SetDescription(e.Description).
				SetParentID(e.ParentID).
				SetPath(e.Path).
				SetAddress(e.Address).
				SetCity(e.City).
				SetState(e.State).
				SetCountry(e.Country).
				SetPostalCode(e.PostalCode).
				SetContact(e.Contact).
				SetPhone(e.Phone).
				SetEmail(e.Email).
				SetStatus(e.Status).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("locations: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importEmployees(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "employees", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.Employee
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("employees: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Employee.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Employee.UpdateOneID(e.ID).
				SetFirstName(e.FirstName).
				SetLastName(e.LastName).
				SetEmail(e.Email).
				SetPhone(e.Phone).
				SetDepartment(e.Department).
				SetJobTitle(e.JobTitle).
				SetEmployeeNumber(e.EmployeeNumber).
				SetNotes(e.Notes).
				SetTags(e.Tags).
				SetMetadata(e.Metadata).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("employees: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Employee.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetFirstName(e.FirstName).
				SetLastName(e.LastName).
				SetEmail(e.Email).
				SetPhone(e.Phone).
				SetDepartment(e.Department).
				SetJobTitle(e.JobTitle).
				SetEmployeeNumber(e.EmployeeNumber).
				SetNotes(e.Notes).
				SetTags(e.Tags).
				SetMetadata(e.Metadata).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("employees: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importAssets(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "assets", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.Asset
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("assets: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Asset.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Asset.UpdateOneID(e.ID).
				SetAssetTag(e.AssetTag).
				SetName(e.Name).
				SetSerial(e.Serial).
				SetModelName(e.ModelName).
				SetModelNumber(e.ModelNumber).
				SetCategoryID(e.CategoryID).
				SetSupplierID(e.SupplierID).
				SetLocationID(e.LocationID).
				SetEmployeeID(e.EmployeeID).
				SetStatus(e.Status).
				SetPhotoKey(e.PhotoKey).
				SetNillableWarrantyMonths(e.WarrantyMonths).
				SetNillablePurchaseDate(e.PurchaseDate).
				SetOrderNumber(e.OrderNumber).
				SetNillablePurchaseCost(e.PurchaseCost).
				SetNotes(e.Notes).
				SetNillableSalvageValue(e.SalvageValue).
				SetNillableUsefulLifeYears(e.UsefulLifeYears).
				SetNillableDepreciationRate(e.DepreciationRate).
				SetTags(e.Tags).
				SetMetadata(e.Metadata).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assets: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Asset.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetAssetTag(e.AssetTag).
				SetName(e.Name).
				SetSerial(e.Serial).
				SetModelName(e.ModelName).
				SetModelNumber(e.ModelNumber).
				SetCategoryID(e.CategoryID).
				SetSupplierID(e.SupplierID).
				SetLocationID(e.LocationID).
				SetEmployeeID(e.EmployeeID).
				SetStatus(e.Status).
				SetPhotoKey(e.PhotoKey).
				SetNillableWarrantyMonths(e.WarrantyMonths).
				SetNillablePurchaseDate(e.PurchaseDate).
				SetOrderNumber(e.OrderNumber).
				SetNillablePurchaseCost(e.PurchaseCost).
				SetNotes(e.Notes).
				SetNillableSalvageValue(e.SalvageValue).
				SetNillableUsefulLifeYears(e.UsefulLifeYears).
				SetNillableDepreciationRate(e.DepreciationRate).
				SetTags(e.Tags).
				SetMetadata(e.Metadata).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assets: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importConsumables(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "consumables", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.Consumable
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("consumables: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.Consumable.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.Consumable.UpdateOneID(e.ID).
				SetName(e.Name).
				SetDescription(e.Description).
				SetCategoryID(e.CategoryID).
				SetSupplierID(e.SupplierID).
				SetLocationID(e.LocationID).
				SetModelName(e.ModelName).
				SetModelNumber(e.ModelNumber).
				SetAmount(e.Amount).
				SetMinAmount(e.MinAmount).
				SetNillablePurchaseDate(e.PurchaseDate).
				SetNillablePurchaseCost(e.PurchaseCost).
				SetOrderNumber(e.OrderNumber).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("consumables: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.Consumable.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetDescription(e.Description).
				SetCategoryID(e.CategoryID).
				SetSupplierID(e.SupplierID).
				SetLocationID(e.LocationID).
				SetModelName(e.ModelName).
				SetModelNumber(e.ModelNumber).
				SetAmount(e.Amount).
				SetMinAmount(e.MinAmount).
				SetNillablePurchaseDate(e.PurchaseDate).
				SetNillablePurchaseCost(e.PurchaseCost).
				SetOrderNumber(e.OrderNumber).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("consumables: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importLicenses(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "licenses", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.License
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("licenses: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.License.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.License.UpdateOneID(e.ID).
				SetName(e.Name).
				SetSupplierID(e.SupplierID).
				SetNillablePurchaseDate(e.PurchaseDate).
				SetNillablePurchaseCost(e.PurchaseCost).
				SetOrderNumber(e.OrderNumber).
				SetNillableValidFrom(e.ValidFrom).
				SetNillableValidTo(e.ValidTo).
				SetNotes(e.Notes).
				SetStatus(e.Status).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("licenses: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.License.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetSupplierID(e.SupplierID).
				SetNillablePurchaseDate(e.PurchaseDate).
				SetNillablePurchaseCost(e.PurchaseCost).
				SetOrderNumber(e.OrderNumber).
				SetNillableValidFrom(e.ValidFrom).
				SetNillableValidTo(e.ValidTo).
				SetNotes(e.Notes).
				SetStatus(e.Status).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("licenses: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importAssetAssignments(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "assetAssignments", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.AssetAssignment
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("assetAssignments: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		existing, _ := client.AssetAssignment.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.AssetAssignment.UpdateOneID(e.ID).
				SetAssetID(e.AssetID).
				SetAssetName(e.AssetName).
				SetEmployeeID(e.EmployeeID).
				SetEmployeeName(e.EmployeeName).
				SetAction(e.Action).
				SetAssignedAt(e.AssignedAt).
				SetNillableReturnedAt(e.ReturnedAt).
				SetNillableAssignedBy(e.AssignedBy).
				SetNotes(e.Notes).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assetAssignments: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.AssetAssignment.Create().
				SetID(e.ID).
				SetAssetID(e.AssetID).
				SetAssetName(e.AssetName).
				SetEmployeeID(e.EmployeeID).
				SetEmployeeName(e.EmployeeName).
				SetAction(e.Action).
				SetAssignedAt(e.AssignedAt).
				SetNillableReturnedAt(e.ReturnedAt).
				SetNillableAssignedBy(e.AssignedBy).
				SetNotes(e.Notes).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assetAssignments: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importAssetDocuments(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "assetDocuments", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.AssetDocument
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("assetDocuments: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		existing, _ := client.AssetDocument.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.AssetDocument.UpdateOneID(e.ID).
				SetEntityType(e.EntityType).
				SetEntityID(e.EntityID).
				SetFileName(e.FileName).
				SetFileSize(e.FileSize).
				SetMimeType(e.MimeType).
				SetStorageKey(e.StorageKey).
				SetChecksum(e.Checksum).
				SetDescription(e.Description).
				SetNillableUploadedBy(e.UploadedBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assetDocuments: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.AssetDocument.Create().
				SetID(e.ID).
				SetEntityType(e.EntityType).
				SetEntityID(e.EntityID).
				SetFileName(e.FileName).
				SetFileSize(e.FileSize).
				SetMimeType(e.MimeType).
				SetStorageKey(e.StorageKey).
				SetChecksum(e.Checksum).
				SetDescription(e.Description).
				SetNillableUploadedBy(e.UploadedBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("assetDocuments: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importInsurancePolicies(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "insurancePolicies", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.InsurancePolicy
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("insurancePolicies: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.InsurancePolicy.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.InsurancePolicy.UpdateOneID(e.ID).
				SetName(e.Name).
				SetPolicyNumber(e.PolicyNumber).
				SetProvider(e.Provider).
				SetCoverageType(e.CoverageType).
				SetNillablePremiumAmount(e.PremiumAmount).
				SetNillableDeductible(e.Deductible).
				SetNillableCoverageLimit(e.CoverageLimit).
				SetNillableValidFrom(e.ValidFrom).
				SetNillableValidTo(e.ValidTo).
				SetStatus(e.Status).
				SetNotes(e.Notes).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("insurancePolicies: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.InsurancePolicy.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetPolicyNumber(e.PolicyNumber).
				SetProvider(e.Provider).
				SetCoverageType(e.CoverageType).
				SetNillablePremiumAmount(e.PremiumAmount).
				SetNillableDeductible(e.Deductible).
				SetNillableCoverageLimit(e.CoverageLimit).
				SetNillableValidFrom(e.ValidFrom).
				SetNillableValidTo(e.ValidTo).
				SetStatus(e.Status).
				SetNotes(e.Notes).
				SetNillableCreateBy(e.CreateBy).
				SetNillableUpdateBy(e.UpdateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("insurancePolicies: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importInsurancePolicyAssets(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode assetV1.RestoreMode) (*assetV1.EntityImportResult, []string) {
	result := &assetV1.EntityImportResult{EntityType: "insurancePolicyAssets", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.InsurancePolicyAsset
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("insurancePolicyAssets: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full {
			tid = e.TenantID
		}

		existing, _ := client.InsurancePolicyAsset.Get(ctx, e.ID)
		if existing != nil {
			if mode == assetV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.InsurancePolicyAsset.UpdateOneID(e.ID).
				SetPolicyID(e.PolicyID).
				SetAssetID(e.AssetID).
				SetNillableCoveredValue(e.CoveredValue).
				SetNotes(e.Notes).
				SetAssetTag(e.AssetTag).
				SetAssetName(e.AssetName).
				SetModelName(e.ModelName).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("insurancePolicyAssets: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.InsurancePolicyAsset.Create().
				SetID(e.ID).
				SetTenantID(tid).
				SetPolicyID(e.PolicyID).
				SetAssetID(e.AssetID).
				SetNillableCoveredValue(e.CoveredValue).
				SetNotes(e.Notes).
				SetAssetTag(e.AssetTag).
				SetAssetName(e.AssetName).
				SetModelName(e.ModelName).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("insurancePolicyAssets: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}
