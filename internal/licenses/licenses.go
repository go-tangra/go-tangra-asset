// Package licenses manages software licenses: CRUD with validity dates and a
// status (active|expired|suspended). A license past valid_to is auto-expired by
// the lifecycle scheduler. The concrete store enforces per-tenant RLS.
package licenses

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-freya/freya/services/asset/internal/audit"
	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/repo"
	"github.com/go-freya/freya/services/asset/internal/store"
)

// ErrNotFound is returned when a license does not exist within the caller's tenant.
var ErrNotFound = errors.New("licenses: not found")

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "licenses: " + e.Msg }

// Input is the create/update body.
type Input struct {
	Name         string            `json:"name"`
	SupplierID   string            `json:"supplier_id"`
	PurchaseDate *time.Time        `json:"purchase_date"`
	PurchaseCost float64           `json:"purchase_cost"`
	OrderNumber  string            `json:"order_number"`
	ValidFrom    *time.Time        `json:"valid_from"`
	ValidTo      *time.Time        `json:"valid_to"`
	Notes        string            `json:"notes"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata"`
}

// Service manages licenses.
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

func mapErr(err error) error {
	if errors.Is(err, repo.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return ValidationError{"name is required (max 200)"}
	}
	switch in.Status {
	case "", store.LicActive, store.LicExpired, store.LicSuspended:
	default:
		return ValidationError{"status is invalid"}
	}
	if in.PurchaseCost < 0 {
		return ValidationError{"purchase_cost out of range"}
	}
	if in.ValidFrom != nil && in.ValidTo != nil && in.ValidTo.Before(*in.ValidFrom) {
		return ValidationError{"valid_to precedes valid_from"}
	}
	if len(in.Metadata) > 64 {
		return ValidationError{"too many metadata entries"}
	}
	return nil
}

func apply(l *store.License, in Input) {
	l.Name, l.SupplierID = strings.TrimSpace(in.Name), in.SupplierID
	l.PurchaseDate, l.PurchaseCost, l.OrderNumber = in.PurchaseDate, in.PurchaseCost, in.OrderNumber
	l.ValidFrom, l.ValidTo, l.Notes, l.Metadata = in.ValidFrom, in.ValidTo, in.Notes, in.Metadata
	l.Status = in.Status
	if l.Status == "" {
		l.Status = store.LicActive
	}
}

func (s *Service) emit(ctx context.Context, subj authz.Subjects, typ audit.EventType, id string) {
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: typ, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectLicense, SubjectID: id, Outcome: audit.OutcomeOK})
}

func (s *Service) checkSupplier(ctx context.Context, tenantID, id string) error {
	if id == "" {
		return nil
	}
	if _, err := s.st.GetSupplier(ctx, tenantID, id); err != nil {
		return ValidationError{"supplier_id does not exist"}
	}
	return nil
}

// Create inserts a license.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (store.License, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.License{}, err
	}
	if err := validate(in); err != nil {
		return store.License{}, err
	}
	if err := s.checkSupplier(ctx, subj.TenantID, in.SupplierID); err != nil {
		return store.License{}, err
	}
	now := s.now()
	l := store.License{ID: store.NewID(), TenantID: subj.TenantID, CreatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	apply(&l, in)
	if err := s.st.CreateLicense(ctx, l); err != nil {
		return store.License{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.LicenseCreated, l.ID)
	return l, nil
}

// Get returns one license.
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (store.License, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.License{}, err
	}
	l, err := s.st.GetLicense(ctx, subj.TenantID, id)
	if err != nil {
		return store.License{}, mapErr(err)
	}
	return l, nil
}

// List returns the tenant's licenses matching f.
func (s *Service) List(ctx context.Context, subj authz.Subjects, f store.ListOpts) ([]store.License, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListLicenses(ctx, subj.TenantID, f)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []store.License{}
	}
	return rows, nil
}

// Update replaces the editable fields.
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (store.License, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.License{}, err
	}
	if err := validate(in); err != nil {
		return store.License{}, err
	}
	if err := s.checkSupplier(ctx, subj.TenantID, in.SupplierID); err != nil {
		return store.License{}, err
	}
	l, err := s.st.GetLicense(ctx, subj.TenantID, id)
	if err != nil {
		return store.License{}, mapErr(err)
	}
	apply(&l, in)
	l.UpdatedAt = s.now()
	if err := s.st.UpdateLicense(ctx, l); err != nil {
		return store.License{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.LicenseUpdated, id)
	return l, nil
}

// Delete removes a license.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if err := s.st.DeleteLicense(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.LicenseDeleted, id)
	return nil
}
