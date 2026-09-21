// Package consumables manages stock items (toner, cables, ...): CRUD with the
// stock amount and reorder threshold; a consumable at or below its minimum is
// flagged low-stock on read (the scheduler emits the event). The concrete store
// enforces per-tenant RLS.
package consumables

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

// ErrNotFound is returned when a consumable does not exist within the caller's tenant.
var ErrNotFound = errors.New("consumables: not found")

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "consumables: " + e.Msg }

// Input is the create/update body.
type Input struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	CategoryID   string            `json:"category_id"`
	SupplierID   string            `json:"supplier_id"`
	LocationID   string            `json:"location_id"`
	ModelName    string            `json:"model_name"`
	ModelNumber  string            `json:"model_number"`
	Amount       int               `json:"amount"`
	MinAmount    int               `json:"min_amount"`
	PurchaseDate *time.Time        `json:"purchase_date"`
	PurchaseCost float64           `json:"purchase_cost"`
	OrderNumber  string            `json:"order_number"`
	Notes        string            `json:"notes"`
	Tags         map[string]string `json:"tags"`
}

// View is a consumable with its computed low-stock flag.
type View struct {
	store.Consumable
	LowStock bool `json:"low_stock"`
}

// IsLowStock reports amount <= min_amount (with a configured threshold).
func IsLowStock(c store.Consumable) bool { return c.MinAmount > 0 && c.Amount <= c.MinAmount }

// Service manages consumables.
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

func view(c store.Consumable) View { return View{Consumable: c, LowStock: IsLowStock(c)} }

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return ValidationError{"name is required (max 200)"}
	}
	if in.Amount < 0 || in.MinAmount < 0 || in.PurchaseCost < 0 {
		return ValidationError{"numeric fields out of range"}
	}
	if len(in.Tags) > 64 {
		return ValidationError{"too many tags"}
	}
	return nil
}

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

func apply(c *store.Consumable, in Input) {
	c.Name, c.Description = strings.TrimSpace(in.Name), in.Description
	c.CategoryID, c.SupplierID, c.LocationID = in.CategoryID, in.SupplierID, in.LocationID
	c.ModelName, c.ModelNumber, c.Amount, c.MinAmount = in.ModelName, in.ModelNumber, in.Amount, in.MinAmount
	c.PurchaseDate, c.PurchaseCost, c.OrderNumber, c.Notes, c.Tags = in.PurchaseDate, in.PurchaseCost, in.OrderNumber, in.Notes, in.Tags
}

func (s *Service) emit(ctx context.Context, subj authz.Subjects, typ audit.EventType, id string) {
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: typ, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectConsumable, SubjectID: id, Outcome: audit.OutcomeOK})
}

// Create inserts a consumable.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	if err := validate(in); err != nil {
		return View{}, err
	}
	if err := s.checkRefs(ctx, subj.TenantID, in); err != nil {
		return View{}, err
	}
	now := s.now()
	c := store.Consumable{ID: store.NewID(), TenantID: subj.TenantID, CreatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	apply(&c, in)
	if err := s.st.CreateConsumable(ctx, c); err != nil {
		return View{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.ConsumableCreated, c.ID)
	return view(c), nil
}

// Get returns one consumable.
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	c, err := s.st.GetConsumable(ctx, subj.TenantID, id)
	if err != nil {
		return View{}, mapErr(err)
	}
	return view(c), nil
}

// List returns the tenant's consumables matching f.
func (s *Service) List(ctx context.Context, subj authz.Subjects, f store.ListOpts) ([]View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListConsumables(ctx, subj.TenantID, f)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, c := range rows {
		out = append(out, view(c))
	}
	return out, nil
}

// Update replaces the editable fields.
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (View, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return View{}, err
	}
	if err := validate(in); err != nil {
		return View{}, err
	}
	if err := s.checkRefs(ctx, subj.TenantID, in); err != nil {
		return View{}, err
	}
	c, err := s.st.GetConsumable(ctx, subj.TenantID, id)
	if err != nil {
		return View{}, mapErr(err)
	}
	apply(&c, in)
	c.UpdatedAt = s.now()
	if err := s.st.UpdateConsumable(ctx, c); err != nil {
		return View{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.ConsumableUpdated, id)
	return view(c), nil
}

// Delete removes a consumable (its documents are removed by the documents service).
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if err := s.st.DeleteConsumable(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.ConsumableDeleted, id)
	return nil
}
