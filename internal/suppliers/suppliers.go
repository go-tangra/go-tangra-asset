// Package suppliers manages the tenant's suppliers: CRUD with the (tenant,name)
// unique guard, the delete guard (a supplier referenced by assets cannot be
// removed) and sealing of the contact PII (contact person, telephone, e-mail)
// with the KEK envelope: the fields are stored only as ciphertext bound to the
// supplier row and are returned in clear only to tenant admins; everyone else
// sees them redacted (omitted). The concrete store enforces per-tenant RLS.
package suppliers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/sealed"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Errors.
var (
	ErrNotFound = errors.New("suppliers: not found")
	ErrConflict = errors.New("suppliers: duplicate name")
	ErrInUse    = errors.New("suppliers: supplier is referenced by assets")
)

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "suppliers: " + e.Msg }

// Kind is the sealed-contact owner kind.
const Kind = "supplier"

// Input is the create/update body.
type Input struct {
	Name          string            `json:"name"`
	Code          string            `json:"code"`
	Address       string            `json:"address"`
	City          string            `json:"city"`
	State         string            `json:"state"`
	Country       string            `json:"country"`
	PostalCode    string            `json:"postal_code"`
	ContactPerson string            `json:"contact_person"`
	Telephone     string            `json:"telephone"`
	Email         string            `json:"email"`
	Website       string            `json:"website"`
	Notes         string            `json:"notes"`
	Status        string            `json:"status"`
	Tags          map[string]string `json:"tags"`
}

// Service manages suppliers.
type Service struct {
	st  repo.Store
	env *sealed.Envelope
	aud audit.Recorder
	now func() time.Time
}

// New builds the service. env seals the contact fields (required).
func New(st repo.Store, env *sealed.Envelope, aud audit.Recorder) *Service {
	return &Service{st: st, env: env, aud: aud, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

func mapErr(err error) error {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, repo.ErrConflict):
		return ErrConflict
	case errors.Is(err, repo.ErrNotEmpty):
		return ErrInUse
	}
	return err
}

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return ValidationError{"name is required (max 200)"}
	}
	switch in.Status {
	case "", store.SupActive, store.SupInactive:
	default:
		return ValidationError{"status is invalid"}
	}
	for _, f := range []string{in.ContactPerson, in.Telephone, in.Email} {
		if len(f) > 512 {
			return ValidationError{"contact field too long"}
		}
	}
	if len(in.Tags) > 64 {
		return ValidationError{"too many tags"}
	}
	return nil
}

func (s *Service) emit(ctx context.Context, subj authz.Subjects, typ audit.EventType, id string) {
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: typ, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectSupplier, SubjectID: id, Outcome: audit.OutcomeOK})
}

// seal encrypts the contact fields in place (bound to the row id + field).
func (s *Service) seal(sp *store.Supplier) error {
	var err error
	if sp.ContactPerson, err = s.env.SealString(sp.ContactPerson, sealed.ADContact(Kind, sp.ID, "contact_person")); err != nil {
		return err
	}
	if sp.Telephone, err = s.env.SealString(sp.Telephone, sealed.ADContact(Kind, sp.ID, "telephone")); err != nil {
		return err
	}
	sp.Email, err = s.env.SealString(sp.Email, sealed.ADContact(Kind, sp.ID, "email"))
	return err
}

// present opens the contact fields for admins and redacts them for everyone else.
func (s *Service) present(sp store.Supplier, subj authz.Subjects) store.Supplier {
	if !subj.IsAdmin() {
		sp.ContactPerson, sp.Telephone, sp.Email = "", "", ""
		return sp
	}
	open := func(v, field string) string {
		pt, err := s.env.OpenString(v, sealed.ADContact(Kind, sp.ID, field))
		if err != nil {
			return "" // undecryptable → redacted rather than leaking ciphertext
		}
		return pt
	}
	sp.ContactPerson = open(sp.ContactPerson, "contact_person")
	sp.Telephone = open(sp.Telephone, "telephone")
	sp.Email = open(sp.Email, "email")
	return sp
}

func apply(sp *store.Supplier, in Input) {
	sp.Name, sp.Code, sp.Address, sp.City, sp.State, sp.Country, sp.PostalCode = strings.TrimSpace(in.Name), in.Code, in.Address, in.City, in.State, in.Country, in.PostalCode
	sp.ContactPerson, sp.Telephone, sp.Email = strings.TrimSpace(in.ContactPerson), strings.TrimSpace(in.Telephone), strings.TrimSpace(in.Email)
	sp.Website, sp.Notes, sp.Tags = in.Website, in.Notes, in.Tags
	sp.Status = in.Status
	if sp.Status == "" {
		sp.Status = store.SupActive
	}
}

// Create inserts a supplier; a duplicate name is ErrConflict.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (store.Supplier, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Supplier{}, err
	}
	if err := validate(in); err != nil {
		return store.Supplier{}, err
	}
	now := s.now()
	sp := store.Supplier{ID: store.NewID(), TenantID: subj.TenantID, CreatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	apply(&sp, in)
	if err := s.seal(&sp); err != nil {
		return store.Supplier{}, err
	}
	if err := s.st.CreateSupplier(ctx, sp); err != nil {
		return store.Supplier{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.SupplierCreated, sp.ID)
	return s.present(sp, subj), nil
}

// Get returns one supplier (contact fields per authorization).
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (store.Supplier, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Supplier{}, err
	}
	sp, err := s.st.GetSupplier(ctx, subj.TenantID, id)
	if err != nil {
		return store.Supplier{}, mapErr(err)
	}
	return s.present(sp, subj), nil
}

// List returns the tenant's suppliers matching f.
func (s *Service) List(ctx context.Context, subj authz.Subjects, f store.ListOpts) ([]store.Supplier, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListSuppliers(ctx, subj.TenantID, f)
	if err != nil {
		return nil, err
	}
	out := make([]store.Supplier, 0, len(rows))
	for _, sp := range rows {
		out = append(out, s.present(sp, subj))
	}
	return out, nil
}

// Update replaces the editable fields (contact fields are re-sealed).
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (store.Supplier, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Supplier{}, err
	}
	if err := validate(in); err != nil {
		return store.Supplier{}, err
	}
	sp, err := s.st.GetSupplier(ctx, subj.TenantID, id)
	if err != nil {
		return store.Supplier{}, mapErr(err)
	}
	apply(&sp, in)
	sp.UpdatedAt = s.now()
	if err := s.seal(&sp); err != nil {
		return store.Supplier{}, err
	}
	if err := s.st.UpdateSupplier(ctx, sp); err != nil {
		return store.Supplier{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.SupplierUpdated, id)
	return s.present(sp, subj), nil
}

// Delete removes a supplier not referenced by any asset.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if _, err := s.st.GetSupplier(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	if n, err := s.st.CountAssetsBySupplier(ctx, subj.TenantID, id); err != nil {
		return err
	} else if n > 0 {
		return ErrInUse
	}
	if err := s.st.DeleteSupplier(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.SupplierDeleted, id)
	return nil
}
