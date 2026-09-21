// Package locations manages the tenant's location tree: CRUD with the
// (tenant,name) unique guard, the materialized path, the nested tree with
// computed child/asset counts, the delete guard (children or assets) and the
// sealing of the contact PII (contact, phone, e-mail) with the KEK envelope —
// returned in clear only to tenant admins, redacted otherwise. The concrete
// store enforces per-tenant RLS.
package locations

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/go-freya/freya/services/asset/internal/audit"
	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/repo"
	"github.com/go-freya/freya/services/asset/internal/sealed"
	"github.com/go-freya/freya/services/asset/internal/store"
)

// Errors.
var (
	ErrNotFound = errors.New("locations: not found")
	ErrConflict = errors.New("locations: duplicate name")
	ErrInUse    = errors.New("locations: location has children or assets")
)

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "locations: " + e.Msg }

// Kind is the sealed-contact owner kind.
const Kind = "location"

// Input is the create/update body.
type Input struct {
	Name        string            `json:"name"`
	Code        string            `json:"code"`
	Description string            `json:"description"`
	ParentID    string            `json:"parent_id"`
	Address     string            `json:"address"`
	City        string            `json:"city"`
	State       string            `json:"state"`
	Country     string            `json:"country"`
	PostalCode  string            `json:"postal_code"`
	Contact     string            `json:"contact"`
	Phone       string            `json:"phone"`
	Email       string            `json:"email"`
	Status      string            `json:"status"`
	Tags        map[string]string `json:"tags"`
}

// Node is a location plus its nested children.
type Node struct {
	store.Location
	Children []*Node `json:"children"`
}

// Service manages locations.
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
	case "", store.LocActive, store.LocPlanned, store.LocDecommissioned:
	default:
		return ValidationError{"status is invalid"}
	}
	for _, f := range []string{in.Contact, in.Phone, in.Email} {
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
		SubjectKind: audit.SubjectLocation, SubjectID: id, Outcome: audit.OutcomeOK})
}

func (s *Service) seal(l *store.Location) error {
	var err error
	if l.Contact, err = s.env.SealString(l.Contact, sealed.ADContact(Kind, l.ID, "contact")); err != nil {
		return err
	}
	if l.Phone, err = s.env.SealString(l.Phone, sealed.ADContact(Kind, l.ID, "phone")); err != nil {
		return err
	}
	l.Email, err = s.env.SealString(l.Email, sealed.ADContact(Kind, l.ID, "email"))
	return err
}

func (s *Service) present(l store.Location, subj authz.Subjects) store.Location {
	if !subj.IsAdmin() {
		l.Contact, l.Phone, l.Email = "", "", ""
		return l
	}
	open := func(v, field string) string {
		pt, err := s.env.OpenString(v, sealed.ADContact(Kind, l.ID, field))
		if err != nil {
			return ""
		}
		return pt
	}
	l.Contact, l.Phone, l.Email = open(l.Contact, "contact"), open(l.Phone, "phone"), open(l.Email, "email")
	return l
}

func apply(l *store.Location, in Input) {
	l.Name, l.Code, l.Description, l.ParentID = strings.TrimSpace(in.Name), in.Code, in.Description, in.ParentID
	l.Address, l.City, l.State, l.Country, l.PostalCode = in.Address, in.City, in.State, in.Country, in.PostalCode
	l.Contact, l.Phone, l.Email = strings.TrimSpace(in.Contact), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email)
	l.Status, l.Tags = in.Status, in.Tags
	if l.Status == "" {
		l.Status = store.LocActive
	}
}

// path computes "Parent / Child" from the parent's stored path.
func (s *Service) path(ctx context.Context, tenantID, parentID, name string) (string, error) {
	if parentID == "" {
		return name, nil
	}
	p, err := s.st.GetLocation(ctx, tenantID, parentID)
	if err != nil {
		return "", ValidationError{"parent_id does not exist"}
	}
	base := p.Path
	if base == "" {
		base = p.Name
	}
	return base + " / " + name, nil
}

// Create inserts a location; a duplicate name is ErrConflict.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (store.Location, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Location{}, err
	}
	if err := validate(in); err != nil {
		return store.Location{}, err
	}
	now := s.now()
	l := store.Location{ID: store.NewID(), TenantID: subj.TenantID, CreatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	apply(&l, in)
	var err error
	if l.Path, err = s.path(ctx, subj.TenantID, l.ParentID, l.Name); err != nil {
		return store.Location{}, err
	}
	if err := s.seal(&l); err != nil {
		return store.Location{}, err
	}
	if err := s.st.CreateLocation(ctx, l); err != nil {
		return store.Location{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.LocationCreated, l.ID)
	return s.Get(ctx, subj, l.ID)
}

// Get returns one location with its counts (contact fields per authorization).
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (store.Location, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Location{}, err
	}
	l, err := s.st.GetLocation(ctx, subj.TenantID, id)
	if err != nil {
		return store.Location{}, mapErr(err)
	}
	return s.present(l, subj), nil
}

// List returns the tenant's locations (flat, by path).
func (s *Service) List(ctx context.Context, subj authz.Subjects) ([]store.Location, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListLocations(ctx, subj.TenantID)
	if err != nil {
		return nil, err
	}
	out := make([]store.Location, 0, len(rows))
	for _, l := range rows {
		out = append(out, s.present(l, subj))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path) })
	return out, nil
}

// Tree returns the nested location tree.
func (s *Service) Tree(ctx context.Context, subj authz.Subjects) ([]*Node, error) {
	rows, err := s.List(ctx, subj)
	if err != nil {
		return nil, err
	}
	return BuildTree(rows), nil
}

// BuildTree nests a flat location list; orphans and cycles become roots.
func BuildTree(rows []store.Location) []*Node {
	nodes := make(map[string]*Node, len(rows))
	for _, l := range rows {
		nodes[l.ID] = &Node{Location: l, Children: []*Node{}}
	}
	roots := []*Node{}
	for _, l := range rows {
		n := nodes[l.ID]
		if p, ok := nodes[l.ParentID]; ok && l.ParentID != l.ID && !isAncestor(nodes, l.ID, l.ParentID) {
			p.Children = append(p.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots
}

func isAncestor(nodes map[string]*Node, id, candidate string) bool {
	seen := map[string]bool{}
	for cur := candidate; cur != "" && !seen[cur]; {
		seen[cur] = true
		n, ok := nodes[cur]
		if !ok {
			return false
		}
		if n.ParentID == id {
			return true
		}
		cur = n.ParentID
	}
	return false
}

// Update replaces the editable fields and recomputes the path; a location
// cannot become its own parent or a descendant's child.
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (store.Location, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Location{}, err
	}
	if err := validate(in); err != nil {
		return store.Location{}, err
	}
	l, err := s.st.GetLocation(ctx, subj.TenantID, id)
	if err != nil {
		return store.Location{}, mapErr(err)
	}
	if in.ParentID != "" {
		if in.ParentID == id {
			return store.Location{}, ValidationError{"a location cannot be its own parent"}
		}
		rows, lerr := s.st.ListLocations(ctx, subj.TenantID)
		if lerr != nil {
			return store.Location{}, lerr
		}
		nodes := map[string]*Node{}
		for _, r := range rows {
			nodes[r.ID] = &Node{Location: r}
		}
		if isAncestor(nodes, id, in.ParentID) {
			return store.Location{}, ValidationError{"parent_id would create a cycle"}
		}
	}
	apply(&l, in)
	if l.Path, err = s.path(ctx, subj.TenantID, l.ParentID, l.Name); err != nil {
		return store.Location{}, err
	}
	l.UpdatedAt = s.now()
	if err := s.seal(&l); err != nil {
		return store.Location{}, err
	}
	if err := s.st.UpdateLocation(ctx, l); err != nil {
		return store.Location{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.LocationUpdated, id)
	return s.Get(ctx, subj, id)
}

// Delete removes a location without children or assets.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if _, err := s.st.GetLocation(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	if n, err := s.st.CountChildLocations(ctx, subj.TenantID, id); err != nil {
		return err
	} else if n > 0 {
		return ErrInUse
	}
	if n, err := s.st.CountAssetsByLocation(ctx, subj.TenantID, id); err != nil {
		return err
	} else if n > 0 {
		return ErrInUse
	}
	if err := s.st.DeleteLocation(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.LocationDeleted, id)
	return nil
}
