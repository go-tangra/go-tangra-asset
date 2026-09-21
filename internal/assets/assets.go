// Package assets is the asset service: CRUD with auto asset-tag generation, the
// assign/unassign lifecycle to platform users (status gate + assignment history +
// realtime events), assignment history reads, and the computed depreciated book
// value on every read. Authorization is coarse (the caller is scoped to its
// tenant); the concrete store enforces per-tenant RLS. Store sentinels are
// masked into the package's own ErrNotFound/ErrConflict; a lifecycle refusal
// (assigning an asset that is not deployable, unassigning one that is not
// assigned) surfaces as ErrState.
package assets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-freya/freya/services/asset/internal/audit"
	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/blob"
	"github.com/go-freya/freya/services/asset/internal/deprec"
	"github.com/go-freya/freya/services/asset/internal/events"
	"github.com/go-freya/freya/services/asset/internal/repo"
	"github.com/go-freya/freya/services/asset/internal/store"
	"github.com/go-freya/freya/services/asset/internal/userdir"
)

// Errors.
var (
	ErrNotFound = errors.New("assets: not found")
	ErrConflict = errors.New("assets: duplicate asset tag")
	ErrState    = errors.New("assets: lifecycle state does not permit the operation")
)

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "assets: " + e.Msg }

// Service manages assets.
type Service struct {
	st    repo.Store
	users userdir.Directory
	pub   events.Publisher
	aud   audit.Recorder
	blobs blob.Store // optional: photo/document objects removed on delete
	now   func() time.Time
}

// New builds the service. users resolves assignee names (required for Assign);
// pub and aud may be nil.
func New(st repo.Store, users userdir.Directory, pub events.Publisher, aud audit.Recorder) *Service {
	if pub == nil {
		pub = events.HubPublisher{}
	}
	return &Service{st: st, users: users, pub: pub, aud: aud, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// SetBlobStore attaches the object store so deleting an asset removes its photo
// and document objects.
func (s *Service) SetBlobStore(b blob.Store) { s.blobs = b }

// View is the JSON projection of an asset with the computed fields.
type View struct {
	store.Asset
	AssigneeName string `json:"assignee_name,omitempty"`
	HasPhoto     bool   `json:"has_photo"`
}

// Input is the create/update body.
type Input struct {
	AssetTag         string            `json:"asset_tag"`
	Name             string            `json:"name"`
	Serial           string            `json:"serial"`
	ModelName        string            `json:"model_name"`
	ModelNumber      string            `json:"model_number"`
	CategoryID       string            `json:"category_id"`
	SupplierID       string            `json:"supplier_id"`
	LocationID       string            `json:"location_id"`
	Status           string            `json:"status"`
	WarrantyMonths   int               `json:"warranty_months"`
	PurchaseDate     *time.Time        `json:"purchase_date"`
	OrderNumber      string            `json:"order_number"`
	PurchaseCost     float64           `json:"purchase_cost"`
	Notes            string            `json:"notes"`
	SalvageValue     float64           `json:"salvage_value"`
	UsefulLifeYears  int               `json:"useful_life_years"`
	DepreciationRate float64           `json:"depreciation_rate"`
	Tags             map[string]string `json:"tags"`
}

// AutoTag generates an AST-xxxxxx tag (6 upper-hex characters).
func AutoTag() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return "AST-" + strings.ToUpper(hex.EncodeToString(b[:]))
}

// ValidTag reports whether an explicit tag is acceptable: 1..64 visible
// characters, no whitespace.
func ValidTag(tag string) bool {
	if tag == "" || len(tag) > 64 {
		return false
	}
	for _, r := range tag {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	return true
}

func validStatus(st string) bool {
	switch st {
	case store.AssetDeployable, store.AssetAssigned, store.AssetBroken, store.AssetArchived:
		return true
	}
	return false
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, repo.ErrConflict):
		return ErrConflict
	}
	return err
}

func (s *Service) view(ctx context.Context, a store.Asset) View {
	a.BookValue = s.bookValue(a)
	v := View{Asset: a, HasPhoto: a.PhotoKey != ""}
	if a.UserID != "" && s.users != nil {
		if u, err := s.users.Resolve(ctx, a.TenantID, a.UserID); err == nil {
			v.AssigneeName = u.DisplayName
		}
	}
	return v
}

func (s *Service) bookValue(a store.Asset) float64 {
	pd := time.Time{}
	if a.PurchaseDate != nil {
		pd = *a.PurchaseDate
	}
	return deprec.BookValue(a.PurchaseCost, a.SalvageValue, a.UsefulLifeYears, a.DepreciationRate, pd, s.now())
}

// BookValue exposes the depreciation of a stored asset at the service clock
// (used by the statistics rollup).
func (s *Service) BookValue(a store.Asset) float64 { return s.bookValue(a) }

// checkRefs verifies the referenced category/supplier/location exist in the tenant.
func (s *Service) checkRefs(ctx context.Context, tenantID string, in Input) error {
	if in.CategoryID != "" {
		if _, err := s.st.GetCategory(ctx, tenantID, in.CategoryID); err != nil {
			return ValidationError{"category_id does not exist"}
		}
	}
	if in.SupplierID != "" {
		if _, err := s.st.GetSupplier(ctx, tenantID, in.SupplierID); err != nil {
			return ValidationError{"supplier_id does not exist"}
		}
	}
	if in.LocationID != "" {
		if _, err := s.st.GetLocation(ctx, tenantID, in.LocationID); err != nil {
			return ValidationError{"location_id does not exist"}
		}
	}
	return nil
}

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return ValidationError{"name is required (max 200)"}
	}
	if in.AssetTag != "" && !ValidTag(in.AssetTag) {
		return ValidationError{"asset_tag is invalid"}
	}
	if in.Status != "" && !validStatus(in.Status) {
		return ValidationError{"status is invalid"}
	}
	if in.PurchaseCost < 0 || in.SalvageValue < 0 || in.WarrantyMonths < 0 || in.UsefulLifeYears < 0 || in.DepreciationRate < 0 || in.DepreciationRate > 1 {
		return ValidationError{"numeric fields out of range"}
	}
	if len(in.Tags) > 64 {
		return ValidationError{"too many tags"}
	}
	return nil
}

// Create inserts an asset in the caller's tenant. A blank asset_tag is auto-
// generated (retrying on the rare collision); an explicit duplicate is ErrConflict.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	if err := validate(in); err != nil {
		return View{}, err
	}
	if in.Status == store.AssetAssigned {
		return View{}, ValidationError{"status assigned is set through assign"}
	}
	if err := s.checkRefs(ctx, subj.TenantID, in); err != nil {
		return View{}, err
	}
	now := s.now()
	a := store.Asset{ID: store.NewID(), TenantID: subj.TenantID, CreatedBy: subj.ActorID(), UpdatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	apply(&a, in)
	if a.Status == "" {
		a.Status = store.AssetDeployable
	}
	if a.DepreciationRate <= 0 {
		a.DepreciationRate = deprec.DefaultRate
	}
	auto := a.AssetTag == ""
	for attempt := 0; ; attempt++ {
		if auto {
			a.AssetTag = AutoTag()
		}
		err := s.st.CreateAsset(ctx, a)
		if err == nil {
			break
		}
		if auto && errors.Is(err, repo.ErrConflict) && attempt < 5 {
			continue
		}
		return View{}, mapErr(err)
	}
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetCreated, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectAsset, SubjectID: a.ID, Outcome: audit.OutcomeOK, Details: map[string]any{"asset_tag": a.AssetTag}})
	return s.view(ctx, a), nil
}

func apply(a *store.Asset, in Input) {
	a.AssetTag = strings.TrimSpace(in.AssetTag)
	a.Name = strings.TrimSpace(in.Name)
	a.Serial = strings.TrimSpace(in.Serial)
	a.ModelName, a.ModelNumber = in.ModelName, in.ModelNumber
	a.CategoryID, a.SupplierID, a.LocationID = in.CategoryID, in.SupplierID, in.LocationID
	a.Status = in.Status
	a.WarrantyMonths, a.PurchaseDate, a.OrderNumber, a.PurchaseCost, a.Notes = in.WarrantyMonths, in.PurchaseDate, in.OrderNumber, in.PurchaseCost, in.Notes
	a.SalvageValue, a.UsefulLifeYears, a.DepreciationRate = in.SalvageValue, in.UsefulLifeYears, in.DepreciationRate
	a.Tags = in.Tags
}

// Get returns one asset with its computed book value and assignee name.
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, id)
	if err != nil {
		return View{}, mapErr(err)
	}
	return s.view(ctx, a), nil
}

// List returns the caller's assets matching f.
func (s *Service) List(ctx context.Context, subj authz.Subjects, f store.AssetFilter) ([]View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListAssets(ctx, subj.TenantID, f)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, a := range rows {
		out = append(out, s.view(ctx, a))
	}
	return out, nil
}

// Update replaces the editable fields (a blank asset_tag keeps the current
// one). The assignee and the "assigned" status are owned by the lifecycle: an
// assigned asset keeps its status/user through an update, and status cannot be
// set to assigned here.
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	if err := validate(in); err != nil {
		return View{}, err
	}
	if in.Status == store.AssetAssigned {
		return View{}, ValidationError{"status assigned is set through assign"}
	}
	if err := s.checkRefs(ctx, subj.TenantID, in); err != nil {
		return View{}, err
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, id)
	if err != nil {
		return View{}, mapErr(err)
	}
	wasAssigned := a.Status == store.AssetAssigned
	userID, photo, tag := a.UserID, a.PhotoKey, a.AssetTag
	apply(&a, in)
	a.PhotoKey = photo
	if a.AssetTag == "" {
		a.AssetTag = tag // a blank tag keeps the existing one
	}
	if a.Status == "" {
		a.Status = store.AssetDeployable
	}
	if a.DepreciationRate <= 0 {
		a.DepreciationRate = deprec.DefaultRate
	}
	if wasAssigned {
		a.Status, a.UserID, a.LocationID = store.AssetAssigned, userID, ""
	} else {
		a.UserID = ""
	}
	a.UpdatedBy, a.UpdatedAt = subj.ActorID(), s.now()
	if err := s.st.UpdateAsset(ctx, a); err != nil {
		return View{}, mapErr(err)
	}
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetUpdated, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectAsset, SubjectID: a.ID, Outcome: audit.OutcomeOK})
	return s.view(ctx, a), nil
}

// Delete removes an asset, its assignment history, documents and policy links,
// and (when an object store is attached) its photo and document objects.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, id)
	if err != nil {
		return mapErr(err)
	}
	if s.blobs != nil {
		if a.PhotoKey != "" {
			_ = s.blobs.Delete(ctx, a.PhotoKey)
		}
		if docs, derr := s.st.ListDocuments(ctx, subj.TenantID, store.EntityAsset, id); derr == nil {
			for _, d := range docs {
				_ = s.blobs.Delete(ctx, d.StorageKey)
			}
		}
	}
	if err := s.st.DeleteAsset(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetDeleted, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectAsset, SubjectID: id, Outcome: audit.OutcomeOK, Details: map[string]any{"asset_tag": a.AssetTag}})
	return nil
}

// Assign checks the asset out to a platform user: only a deployable asset may
// be assigned; the location is cleared, an assignment row is written with the
// resolved user name and the acting user, and asset.assigned is published.
func (s *Service) Assign(ctx context.Context, subj authz.Subjects, id, userID, notes string) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	if userID == "" {
		return View{}, ValidationError{"user_id is required"}
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, id)
	if err != nil {
		return View{}, mapErr(err)
	}
	if a.Status != store.AssetDeployable {
		audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetAssigned, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
			SubjectKind: audit.SubjectAsset, SubjectID: id, Outcome: audit.OutcomeRefused, Reason: "status " + a.Status})
		return View{}, fmt.Errorf("%w: asset is %s", ErrState, a.Status)
	}
	userName := ""
	if s.users != nil {
		u, uerr := s.users.Resolve(ctx, subj.TenantID, userID)
		if uerr != nil {
			if errors.Is(uerr, userdir.ErrUnknownUser) {
				return View{}, ValidationError{"user_id does not exist"}
			}
			return View{}, uerr
		}
		userName = u.DisplayName
	}
	now := s.now()
	a.Status, a.UserID, a.LocationID = store.AssetAssigned, userID, ""
	a.UpdatedBy, a.UpdatedAt = subj.ActorID(), now
	if err := s.st.UpdateAsset(ctx, a); err != nil {
		return View{}, mapErr(err)
	}
	row := store.Assignment{ID: store.NewID(), TenantID: subj.TenantID, AssetID: a.ID, AssetName: a.Name, UserID: userID, UserName: userName,
		Action: store.ActionAssigned, AssignedAt: now, AssignedBy: subj.ActorID(), Notes: notes}
	if err := s.st.InsertAssignment(ctx, row); err != nil {
		return View{}, err
	}
	s.pub.Publish(ctx, subj.TenantID, events.AssetAssigned, events.AssetAssignedPayload(a.ID, a.AssetTag, userID))
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetAssigned, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectAssignment, SubjectID: row.ID, Outcome: audit.OutcomeOK, Details: map[string]any{"asset_id": a.ID, "user_id": userID}})
	return s.view(ctx, a), nil
}

// Unassign checks an assigned asset back in: it becomes deployable at the given
// location (if any), the active assignment row is closed, an unassigned history
// row is written and asset.unassigned is published.
func (s *Service) Unassign(ctx context.Context, subj authz.Subjects, id, locationID, notes string) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, id)
	if err != nil {
		return View{}, mapErr(err)
	}
	if a.Status != store.AssetAssigned {
		audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetUnassigned, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
			SubjectKind: audit.SubjectAsset, SubjectID: id, Outcome: audit.OutcomeRefused, Reason: "status " + a.Status})
		return View{}, fmt.Errorf("%w: asset is %s", ErrState, a.Status)
	}
	if locationID != "" {
		if _, lerr := s.st.GetLocation(ctx, subj.TenantID, locationID); lerr != nil {
			return View{}, ValidationError{"location_id does not exist"}
		}
	}
	now := s.now()
	prevUser := a.UserID
	userName := ""
	if s.users != nil && prevUser != "" {
		if u, uerr := s.users.Resolve(ctx, subj.TenantID, prevUser); uerr == nil {
			userName = u.DisplayName
		}
	}
	a.Status, a.UserID, a.LocationID = store.AssetDeployable, "", locationID
	a.UpdatedBy, a.UpdatedAt = subj.ActorID(), now
	if err := s.st.UpdateAsset(ctx, a); err != nil {
		return View{}, mapErr(err)
	}
	if err := s.st.CloseActiveAssignment(ctx, subj.TenantID, a.ID, now); err != nil && !errors.Is(err, repo.ErrNotFound) {
		return View{}, err
	}
	ret := now
	row := store.Assignment{ID: store.NewID(), TenantID: subj.TenantID, AssetID: a.ID, AssetName: a.Name, UserID: prevUser, UserName: userName,
		Action: store.ActionUnassigned, AssignedAt: now, ReturnedAt: &ret, AssignedBy: subj.ActorID(), Notes: notes}
	if err := s.st.InsertAssignment(ctx, row); err != nil {
		return View{}, err
	}
	s.pub.Publish(ctx, subj.TenantID, events.AssetUnassigned, events.AssetUnassignedPayload(a.ID, a.AssetTag, prevUser))
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.AssetUnassigned, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectAssignment, SubjectID: row.ID, Outcome: audit.OutcomeOK, Details: map[string]any{"asset_id": a.ID, "user_id": prevUser}})
	return s.view(ctx, a), nil
}

// Assignments returns the asset's assignment history, newest first.
func (s *Service) Assignments(ctx context.Context, subj authz.Subjects, id string, limit int) ([]store.Assignment, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	if _, err := s.st.GetAsset(ctx, subj.TenantID, id); err != nil {
		return nil, mapErr(err)
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.st.ListAssignments(ctx, subj.TenantID, id, limit)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []store.Assignment{}
	}
	return rows, nil
}
