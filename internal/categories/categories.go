// Package categories manages the per-tenant asset category tree: CRUD, the
// nested tree with computed asset/child counts, the (tenant,name,parent) unique
// guard, and the delete guard (a category with children or assets cannot be
// removed). The concrete store enforces per-tenant RLS.
package categories

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Errors.
var (
	ErrNotFound = errors.New("categories: not found")
	ErrConflict = errors.New("categories: duplicate name under parent")
	ErrInUse    = errors.New("categories: category has children or assets")
)

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "categories: " + e.Msg }

// Input is the create/update body.
type Input struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	ParentID    string            `json:"parent_id"`
	Icon        string            `json:"icon"`
	Tags        map[string]string `json:"tags"`
}

// Node is a category plus its nested children.
type Node struct {
	store.Category
	Children []*Node `json:"children"`
}

// Service manages categories.
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
	if len(in.Tags) > 64 {
		return ValidationError{"too many tags"}
	}
	return nil
}

func (s *Service) emit(ctx context.Context, subj authz.Subjects, typ audit.EventType, id string) {
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: typ, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectCategory, SubjectID: id, Outcome: audit.OutcomeOK})
}

// Create inserts a category; a duplicate name under the same parent is ErrConflict.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (store.Category, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Category{}, err
	}
	if err := validate(in); err != nil {
		return store.Category{}, err
	}
	if in.ParentID != "" {
		if _, err := s.st.GetCategory(ctx, subj.TenantID, in.ParentID); err != nil {
			return store.Category{}, ValidationError{"parent_id does not exist"}
		}
	}
	now := s.now()
	c := store.Category{ID: store.NewID(), TenantID: subj.TenantID, Name: strings.TrimSpace(in.Name), Description: in.Description,
		ParentID: in.ParentID, Icon: in.Icon, Tags: in.Tags, CreatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	if err := s.st.CreateCategory(ctx, c); err != nil {
		return store.Category{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.CategoryCreated, c.ID)
	return s.Get(ctx, subj, c.ID)
}

// Get returns one category with its counts.
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (store.Category, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Category{}, err
	}
	c, err := s.st.GetCategory(ctx, subj.TenantID, id)
	if err != nil {
		return store.Category{}, mapErr(err)
	}
	return c, nil
}

// List returns the tenant's categories (flat, by name).
func (s *Service) List(ctx context.Context, subj authz.Subjects) ([]store.Category, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListCategories(ctx, subj.TenantID)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name) })
	if rows == nil {
		rows = []store.Category{}
	}
	return rows, nil
}

// Tree returns the nested category tree (roots sorted by name).
func (s *Service) Tree(ctx context.Context, subj authz.Subjects) ([]*Node, error) {
	rows, err := s.List(ctx, subj)
	if err != nil {
		return nil, err
	}
	return BuildTree(rows), nil
}

// BuildTree nests a flat category list. A node whose parent is missing (or
// would form a cycle) is treated as a root so the tree is always complete.
func BuildTree(rows []store.Category) []*Node {
	nodes := make(map[string]*Node, len(rows))
	for _, c := range rows {
		nodes[c.ID] = &Node{Category: c, Children: []*Node{}}
	}
	roots := []*Node{}
	for _, c := range rows {
		n := nodes[c.ID]
		if p, ok := nodes[c.ParentID]; ok && c.ParentID != c.ID && !isAncestor(nodes, c.ID, c.ParentID) {
			p.Children = append(p.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots
}

// isAncestor reports whether candidate has id among its ancestors (cycle guard).
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

// Update replaces the editable fields; a category cannot become its own parent
// or a descendant's child.
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (store.Category, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.Category{}, err
	}
	if err := validate(in); err != nil {
		return store.Category{}, err
	}
	c, err := s.st.GetCategory(ctx, subj.TenantID, id)
	if err != nil {
		return store.Category{}, mapErr(err)
	}
	if in.ParentID != "" {
		if in.ParentID == id {
			return store.Category{}, ValidationError{"a category cannot be its own parent"}
		}
		if _, err := s.st.GetCategory(ctx, subj.TenantID, in.ParentID); err != nil {
			return store.Category{}, ValidationError{"parent_id does not exist"}
		}
		rows, lerr := s.st.ListCategories(ctx, subj.TenantID)
		if lerr != nil {
			return store.Category{}, lerr
		}
		nodes := map[string]*Node{}
		for _, r := range rows {
			nodes[r.ID] = &Node{Category: r}
		}
		if isAncestor(nodes, id, in.ParentID) {
			return store.Category{}, ValidationError{"parent_id would create a cycle"}
		}
	}
	c.Name, c.Description, c.ParentID, c.Icon, c.Tags = strings.TrimSpace(in.Name), in.Description, in.ParentID, in.Icon, in.Tags
	c.UpdatedAt = s.now()
	if err := s.st.UpdateCategory(ctx, c); err != nil {
		return store.Category{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.CategoryUpdated, id)
	return s.Get(ctx, subj, id)
}

// Delete removes a category without children or assets.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if _, err := s.st.GetCategory(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	if n, err := s.st.CountChildCategories(ctx, subj.TenantID, id); err != nil {
		return err
	} else if n > 0 {
		return ErrInUse
	}
	if n, err := s.st.CountAssetsByCategory(ctx, subj.TenantID, id); err != nil {
		return err
	} else if n > 0 {
		return ErrInUse
	}
	if err := s.st.DeleteCategory(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.CategoryDeleted, id)
	return nil
}
