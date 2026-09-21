// Package backup exports and imports a tenant's asset data. The export is FK-
// ordered (suppliers, locations, categories, assets, assignments, consumables,
// licenses, insurance policies, policy-asset links, document metadata) with ids
// preserved so a restore round-trips. It carries NO secrets and NO contact PII:
// the sealed supplier/location contact fields are dropped, and document bytes
// are never exported (only their tenant-prefixed object keys, which are restored
// only into the same tenant). A cross-tenant restore (target tenant other than
// the caller's) or a full restore (wipe then load) requires a platform admin.
package backup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-freya/freya/services/asset/internal/audit"
	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/repo"
	"github.com/go-freya/freya/services/asset/internal/store"
)

// SchemaVersion is the backup document version this service reads and writes.
const SchemaVersion = 1

// Restore modes.
const (
	ModeSkip      = "skip"
	ModeOverwrite = "overwrite"
)

// Errors.
var (
	ErrBadSchema = errors.New("backup: unsupported schema version")
	ErrTooLarge  = errors.New("backup: document exceeds the size limit")
)

// MaxRows bounds any single collection in an import (a parser guard).
const MaxRows = 200000

// Backup is the export document.
type Backup struct {
	SchemaVersion int                     `json:"schema_version"`
	ExportedAt    time.Time               `json:"exported_at"`
	TenantID      string                  `json:"tenant_id"`
	Suppliers     []store.Supplier        `json:"suppliers"`
	Locations     []store.Location        `json:"locations"`
	Categories    []store.Category        `json:"categories"`
	Assets        []store.Asset           `json:"assets"`
	Assignments   []store.Assignment      `json:"assignments"`
	Consumables   []store.Consumable      `json:"consumables"`
	Licenses      []store.License         `json:"licenses"`
	Insurance     []store.InsurancePolicy `json:"insurance_policies"`
	PolicyAssets  []store.PolicyAsset     `json:"policy_assets"`
	Documents     []store.Document        `json:"documents"`
}

// Options control an import.
type Options struct {
	Mode     string `json:"mode"`      // skip | overwrite
	TenantID string `json:"tenant_id"` // target tenant (platform-admin when not the caller's)
	Full     bool   `json:"full"`      // wipe the target tenant before loading (platform-admin)
}

// Result reports what an import did.
type Result struct {
	Imported map[string]int `json:"imported"`
	Skipped  map[string]int `json:"skipped"`
	Deleted  int            `json:"deleted"`
}

// Service exports and imports tenant data.
type Service struct {
	st  repo.Store
	aud audit.Recorder
	now func() time.Time
}

// New builds the service (aud may be nil).
func New(st repo.Store, aud audit.Recorder) *Service {
	return &Service{st: st, aud: aud, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// scrubSupplier drops the sealed contact fields.
func scrubSupplier(sp store.Supplier) store.Supplier {
	sp.ContactPerson, sp.Telephone, sp.Email = "", "", ""
	return sp
}

func scrubLocation(l store.Location) store.Location {
	l.Contact, l.Phone, l.Email = "", "", ""
	l.ChildCount, l.AssetCount = 0, 0
	return l
}

// Export builds a backup of the caller's tenant.
func (s *Service) Export(ctx context.Context, subj authz.Subjects) (Backup, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return Backup{}, err
	}
	t := subj.TenantID
	b := Backup{SchemaVersion: SchemaVersion, ExportedAt: s.now(), TenantID: t,
		Suppliers: []store.Supplier{}, Locations: []store.Location{}, Categories: []store.Category{}, Assets: []store.Asset{},
		Assignments: []store.Assignment{}, Consumables: []store.Consumable{}, Licenses: []store.License{},
		Insurance: []store.InsurancePolicy{}, PolicyAssets: []store.PolicyAsset{}, Documents: []store.Document{}}
	sups, err := s.st.ListSuppliers(ctx, t, store.ListOpts{})
	if err != nil {
		return Backup{}, err
	}
	for _, sp := range sups {
		b.Suppliers = append(b.Suppliers, scrubSupplier(sp))
	}
	locs, err := s.st.ListLocations(ctx, t)
	if err != nil {
		return Backup{}, err
	}
	for _, l := range locs {
		b.Locations = append(b.Locations, scrubLocation(l))
	}
	if b.Categories, err = s.st.ListCategories(ctx, t); err != nil {
		return Backup{}, err
	}
	for i := range b.Categories {
		b.Categories[i].AssetCount, b.Categories[i].ChildCount = 0, 0
	}
	if b.Assets, err = s.st.AllAssets(ctx, t); err != nil {
		return Backup{}, err
	}
	for i := range b.Assets {
		b.Assets[i].BookValue = 0
		asg, aerr := s.st.ListAssignments(ctx, t, b.Assets[i].ID, 0)
		if aerr != nil {
			return Backup{}, aerr
		}
		b.Assignments = append(b.Assignments, asg...)
		docs, derr := s.st.ListDocuments(ctx, t, store.EntityAsset, b.Assets[i].ID)
		if derr != nil {
			return Backup{}, derr
		}
		b.Documents = append(b.Documents, docs...)
	}
	if b.Consumables, err = s.st.AllConsumables(ctx, t); err != nil {
		return Backup{}, err
	}
	for _, c := range b.Consumables {
		docs, derr := s.st.ListDocuments(ctx, t, store.EntityConsumable, c.ID)
		if derr != nil {
			return Backup{}, derr
		}
		b.Documents = append(b.Documents, docs...)
	}
	if b.Licenses, err = s.st.AllLicenses(ctx, t); err != nil {
		return Backup{}, err
	}
	for _, l := range b.Licenses {
		docs, derr := s.st.ListDocuments(ctx, t, store.EntityLicense, l.ID)
		if derr != nil {
			return Backup{}, derr
		}
		b.Documents = append(b.Documents, docs...)
	}
	if b.Insurance, err = s.st.AllInsurance(ctx, t); err != nil {
		return Backup{}, err
	}
	for i := range b.Insurance {
		b.Insurance[i].AssetCount = 0
		pas, perr := s.st.ListPolicyAssets(ctx, t, b.Insurance[i].ID)
		if perr != nil {
			return Backup{}, perr
		}
		b.PolicyAssets = append(b.PolicyAssets, pas...)
	}
	nilToEmpty(&b)
	audit.Emit(ctx, s.aud, audit.Event{TenantID: t, EventType: audit.BackupExported, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectBackup, SubjectID: "export", Outcome: audit.OutcomeOK, Details: map[string]any{"assets": len(b.Assets)}})
	return b, nil
}

func nilToEmpty(b *Backup) {
	if b.Assignments == nil {
		b.Assignments = []store.Assignment{}
	}
	if b.Documents == nil {
		b.Documents = []store.Document{}
	}
	if b.PolicyAssets == nil {
		b.PolicyAssets = []store.PolicyAsset{}
	}
	if b.Categories == nil {
		b.Categories = []store.Category{}
	}
	if b.Assets == nil {
		b.Assets = []store.Asset{}
	}
	if b.Consumables == nil {
		b.Consumables = []store.Consumable{}
	}
	if b.Licenses == nil {
		b.Licenses = []store.License{}
	}
	if b.Insurance == nil {
		b.Insurance = []store.InsurancePolicy{}
	}
}

// Validate checks a parsed backup document before anything is written.
func Validate(b Backup) error {
	if b.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: %d", ErrBadSchema, b.SchemaVersion)
	}
	for name, n := range map[string]int{"suppliers": len(b.Suppliers), "locations": len(b.Locations), "categories": len(b.Categories),
		"assets": len(b.Assets), "assignments": len(b.Assignments), "consumables": len(b.Consumables), "licenses": len(b.Licenses),
		"insurance_policies": len(b.Insurance), "policy_assets": len(b.PolicyAssets), "documents": len(b.Documents)} {
		if n > MaxRows {
			return fmt.Errorf("%w: %s has %d rows", ErrTooLarge, name, n)
		}
	}
	for _, a := range b.Assets {
		if a.ID == "" || a.AssetTag == "" {
			return fmt.Errorf("%w: asset without id/tag", ErrBadSchema)
		}
	}
	for _, sp := range b.Suppliers {
		if sp.ID == "" || sp.Name == "" {
			return fmt.Errorf("%w: supplier without id/name", ErrBadSchema)
		}
	}
	for _, l := range b.Locations {
		if l.ID == "" || l.Name == "" {
			return fmt.Errorf("%w: location without id/name", ErrBadSchema)
		}
	}
	for _, c := range b.Categories {
		if c.ID == "" || c.Name == "" {
			return fmt.Errorf("%w: category without id/name", ErrBadSchema)
		}
	}
	for _, p := range b.Insurance {
		if p.ID == "" || p.PolicyNumber == "" {
			return fmt.Errorf("%w: policy without id/number", ErrBadSchema)
		}
	}
	return nil
}

// Import loads a backup into the target tenant (the caller's by default).
func (s *Service) Import(ctx context.Context, subj authz.Subjects, b Backup, opts Options) (Result, error) {
	res := Result{Imported: map[string]int{}, Skipped: map[string]int{}}
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return res, err
	}
	target := opts.TenantID
	if target == "" {
		target = subj.TenantID
	}
	if target != subj.TenantID || opts.Full {
		if err := authz.RequirePlatformAdmin(subj); err != nil {
			audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.BackupImported, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
				SubjectKind: audit.SubjectBackup, SubjectID: "import", Outcome: audit.OutcomeRefused, Reason: "platform-admin required"})
			return res, err
		}
	}
	if err := Validate(b); err != nil {
		return res, err
	}
	mode := ModeSkip
	if opts.Mode == ModeOverwrite {
		mode = ModeOverwrite
	}
	if opts.Full {
		n, err := s.wipe(ctx, target)
		res.Deleted = n
		if err != nil {
			return res, err
		}
	}
	// A restore into a tenant other than the backup's origin gets fresh ids so
	// primary keys never collide with the origin's rows; same-tenant restores
	// keep their ids so skip/overwrite can match them.
	if b.TenantID != "" && b.TenantID != target {
		b = remap(b)
	}
	now := s.now()

	// Suppliers.
	for _, sp := range b.Suppliers {
		sp = scrubSupplier(sp)
		sp.TenantID = target
		if _, err := s.st.GetSupplier(ctx, target, sp.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["suppliers"]++
				continue
			}
			if err := s.st.UpdateSupplier(ctx, sp); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateSupplier(ctx, sp); err != nil {
			return res, err
		}
		res.Imported["suppliers"]++
	}
	// Locations (parents before children).
	for _, l := range orderLocations(b.Locations) {
		l = scrubLocation(l)
		l.TenantID = target
		if _, err := s.st.GetLocation(ctx, target, l.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["locations"]++
				continue
			}
			if err := s.st.UpdateLocation(ctx, l); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateLocation(ctx, l); err != nil {
			return res, err
		}
		res.Imported["locations"]++
	}
	// Categories (parents before children).
	for _, c := range orderCategories(b.Categories) {
		c.TenantID = target
		c.AssetCount, c.ChildCount = 0, 0
		if _, err := s.st.GetCategory(ctx, target, c.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["categories"]++
				continue
			}
			if err := s.st.UpdateCategory(ctx, c); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateCategory(ctx, c); err != nil {
			return res, err
		}
		res.Imported["categories"]++
	}
	// Assets. History rows are restored only for assets created by this import:
	// an existing asset (skipped or overwritten) keeps its own history so a
	// repeated import never duplicates it.
	created := map[string]bool{}
	for _, a := range b.Assets {
		a.TenantID = target
		a.BookValue = 0
		if a.Status == "" {
			a.Status = store.AssetDeployable
		}
		if _, err := s.st.GetAsset(ctx, target, a.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["assets"]++
				continue
			}
			if err := s.st.UpdateAsset(ctx, a); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateAsset(ctx, a); err != nil {
			return res, err
		} else {
			created[a.ID] = true
		}
		res.Imported["assets"]++
	}
	for _, g := range b.Assignments {
		if !created[g.AssetID] {
			res.Skipped["assignments"]++
			continue
		}
		g.TenantID = target
		if err := s.st.InsertAssignment(ctx, g); err != nil {
			return res, err
		}
		res.Imported["assignments"]++
	}
	// Consumables.
	for _, c := range b.Consumables {
		c.TenantID = target
		if _, err := s.st.GetConsumable(ctx, target, c.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["consumables"]++
				continue
			}
			if err := s.st.UpdateConsumable(ctx, c); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateConsumable(ctx, c); err != nil {
			return res, err
		}
		res.Imported["consumables"]++
	}
	// Licenses.
	for _, l := range b.Licenses {
		l.TenantID = target
		if l.Status == "" {
			l.Status = store.LicActive
		}
		if _, err := s.st.GetLicense(ctx, target, l.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["licenses"]++
				continue
			}
			if err := s.st.UpdateLicense(ctx, l); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateLicense(ctx, l); err != nil {
			return res, err
		}
		res.Imported["licenses"]++
	}
	// Insurance.
	for _, p := range b.Insurance {
		p.TenantID = target
		p.AssetCount = 0
		if p.Status == "" {
			p.Status = store.InsActive
		}
		if _, err := s.st.GetInsurance(ctx, target, p.ID); err == nil {
			if mode == ModeSkip {
				res.Skipped["insurance_policies"]++
				continue
			}
			if err := s.st.UpdateInsurance(ctx, p); err != nil {
				return res, err
			}
		} else if !errors.Is(err, repo.ErrNotFound) {
			return res, err
		} else if err := s.st.CreateInsurance(ctx, p); err != nil {
			return res, err
		}
		res.Imported["insurance_policies"]++
	}
	// Policy assets (unique per pair: a duplicate is skipped).
	for _, pa := range b.PolicyAssets {
		pa.TenantID = target
		if pa.CreatedAt.IsZero() {
			pa.CreatedAt = now
		}
		err := s.st.AddPolicyAsset(ctx, pa)
		switch {
		case err == nil:
			res.Imported["policy_assets"]++
		case errors.Is(err, repo.ErrConflict), errors.Is(err, repo.ErrNotFound):
			res.Skipped["policy_assets"]++
		default:
			return res, err
		}
	}
	// Documents: metadata only, and only when the object key already belongs to
	// the target tenant (bytes are never part of a backup).
	prefix := "tenants/" + target + "/"
	for _, d := range b.Documents {
		if !strings.HasPrefix(d.StorageKey, prefix) {
			res.Skipped["documents"]++
			continue
		}
		d.TenantID = target
		if _, err := s.st.GetDocument(ctx, target, d.ID); err == nil {
			res.Skipped["documents"]++
			continue
		}
		err := s.st.InsertDocument(ctx, d)
		switch {
		case err == nil:
			res.Imported["documents"]++
		case errors.Is(err, repo.ErrConflict):
			res.Skipped["documents"]++
		default:
			return res, err
		}
	}
	audit.Emit(ctx, s.aud, audit.Event{TenantID: target, EventType: audit.BackupImported, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectBackup, SubjectID: "import", Outcome: audit.OutcomeOK,
		Details: map[string]any{"mode": mode, "full": opts.Full, "cross_tenant": target != subj.TenantID, "assets": res.Imported["assets"]}})
	return res, nil
}

// wipe deletes every record of the target tenant (full restore).
func (s *Service) wipe(ctx context.Context, target string) (int, error) {
	n := 0
	pols, err := s.st.AllInsurance(ctx, target)
	if err != nil {
		return n, err
	}
	for _, p := range pols {
		if err := s.st.DeleteInsurance(ctx, target, p.ID); err != nil {
			return n, err
		}
		n++
	}
	lics, err := s.st.AllLicenses(ctx, target)
	if err != nil {
		return n, err
	}
	for _, l := range lics {
		if err := s.st.DeleteLicense(ctx, target, l.ID); err != nil {
			return n, err
		}
		n++
	}
	cons, err := s.st.AllConsumables(ctx, target)
	if err != nil {
		return n, err
	}
	for _, c := range cons {
		if err := s.st.DeleteConsumable(ctx, target, c.ID); err != nil {
			return n, err
		}
		n++
	}
	assets, err := s.st.AllAssets(ctx, target)
	if err != nil {
		return n, err
	}
	for _, a := range assets {
		if err := s.st.DeleteAsset(ctx, target, a.ID); err != nil {
			return n, err
		}
		n++
	}
	cats, err := s.st.ListCategories(ctx, target)
	if err != nil {
		return n, err
	}
	for _, c := range reverse(orderCategories(cats)) {
		if err := s.st.DeleteCategory(ctx, target, c.ID); err != nil {
			return n, err
		}
		n++
	}
	locs, err := s.st.ListLocations(ctx, target)
	if err != nil {
		return n, err
	}
	for _, l := range reverse(orderLocations(locs)) {
		if err := s.st.DeleteLocation(ctx, target, l.ID); err != nil {
			return n, err
		}
		n++
	}
	sups, err := s.st.ListSuppliers(ctx, target, store.ListOpts{})
	if err != nil {
		return n, err
	}
	for _, sp := range sups {
		if err := s.st.DeleteSupplier(ctx, target, sp.ID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// remap rewrites every id and foreign key in the backup with fresh ids.
func remap(b Backup) Backup {
	b.Suppliers = append([]store.Supplier(nil), b.Suppliers...)
	b.Locations = append([]store.Location(nil), b.Locations...)
	b.Categories = append([]store.Category(nil), b.Categories...)
	b.Assets = append([]store.Asset(nil), b.Assets...)
	b.Assignments = append([]store.Assignment(nil), b.Assignments...)
	b.Consumables = append([]store.Consumable(nil), b.Consumables...)
	b.Licenses = append([]store.License(nil), b.Licenses...)
	b.Insurance = append([]store.InsurancePolicy(nil), b.Insurance...)
	b.PolicyAssets = append([]store.PolicyAsset(nil), b.PolicyAssets...)
	b.Documents = append([]store.Document(nil), b.Documents...)
	ids := map[string]string{}
	fresh := func(old string) string {
		if old == "" {
			return ""
		}
		if n, ok := ids[old]; ok {
			return n
		}
		n := store.NewID()
		ids[old] = n
		return n
	}
	for i := range b.Suppliers {
		b.Suppliers[i].ID = fresh(b.Suppliers[i].ID)
	}
	for i := range b.Locations {
		b.Locations[i].ID = fresh(b.Locations[i].ID)
	}
	for i := range b.Locations {
		b.Locations[i].ParentID = fresh(b.Locations[i].ParentID)
	}
	for i := range b.Categories {
		b.Categories[i].ID = fresh(b.Categories[i].ID)
	}
	for i := range b.Categories {
		b.Categories[i].ParentID = fresh(b.Categories[i].ParentID)
	}
	for i := range b.Assets {
		a := &b.Assets[i]
		a.ID, a.CategoryID, a.SupplierID, a.LocationID = fresh(a.ID), fresh(a.CategoryID), fresh(a.SupplierID), fresh(a.LocationID)
	}
	for i := range b.Assignments {
		b.Assignments[i].ID, b.Assignments[i].AssetID = fresh(b.Assignments[i].ID), fresh(b.Assignments[i].AssetID)
	}
	for i := range b.Consumables {
		c := &b.Consumables[i]
		c.ID, c.CategoryID, c.SupplierID, c.LocationID = fresh(c.ID), fresh(c.CategoryID), fresh(c.SupplierID), fresh(c.LocationID)
	}
	for i := range b.Licenses {
		b.Licenses[i].ID, b.Licenses[i].SupplierID = fresh(b.Licenses[i].ID), fresh(b.Licenses[i].SupplierID)
	}
	for i := range b.Insurance {
		b.Insurance[i].ID = fresh(b.Insurance[i].ID)
	}
	for i := range b.PolicyAssets {
		pa := &b.PolicyAssets[i]
		pa.ID, pa.PolicyID, pa.AssetID = fresh(pa.ID), fresh(pa.PolicyID), fresh(pa.AssetID)
	}
	for i := range b.Documents {
		b.Documents[i].ID, b.Documents[i].EntityID = fresh(b.Documents[i].ID), fresh(b.Documents[i].EntityID)
	}
	return b
}

func reverse[T any](in []T) []T {
	out := make([]T, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

// orderCategories returns parents before children (orphans/cycles appended).
func orderCategories(rows []store.Category) []store.Category {
	byID := map[string]store.Category{}
	for _, c := range rows {
		byID[c.ID] = c
	}
	var out []store.Category
	done := map[string]bool{}
	var visit func(c store.Category, depth int)
	visit = func(c store.Category, depth int) {
		if done[c.ID] || depth > 64 {
			return
		}
		if p, ok := byID[c.ParentID]; ok && c.ParentID != c.ID {
			visit(p, depth+1)
		}
		if !done[c.ID] {
			done[c.ID] = true
			out = append(out, c)
		}
	}
	for _, c := range rows {
		visit(c, 0)
	}
	return out
}

func orderLocations(rows []store.Location) []store.Location {
	byID := map[string]store.Location{}
	for _, l := range rows {
		byID[l.ID] = l
	}
	var out []store.Location
	done := map[string]bool{}
	var visit func(l store.Location, depth int)
	visit = func(l store.Location, depth int) {
		if done[l.ID] || depth > 64 {
			return
		}
		if p, ok := byID[l.ParentID]; ok && l.ParentID != l.ID {
			visit(p, depth+1)
		}
		if !done[l.ID] {
			done[l.ID] = true
			out = append(out, l)
		}
	}
	for _, l := range rows {
		visit(l, 0)
	}
	return out
}
