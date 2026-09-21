// Package insurance manages insurance policies and their per-asset coverage:
// CRUD with the (tenant,policy_number) and (tenant,name) unique guards, the
// policy↔asset M2M (unique per pair, with a covered value and the denormalized
// asset tag/name) and the computed asset_count. A policy past valid_to is
// auto-expired by the lifecycle scheduler. The concrete store enforces per-
// tenant RLS.
package insurance

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

// Errors.
var (
	ErrNotFound = errors.New("insurance: not found")
	ErrConflict = errors.New("insurance: duplicate policy number/name or asset already covered")
)

// ValidationError reports a bad input field.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return "insurance: " + e.Msg }

// Input is the create/update body.
type Input struct {
	Name          string            `json:"name"`
	PolicyNumber  string            `json:"policy_number"`
	Provider      string            `json:"provider"`
	CoverageType  string            `json:"coverage_type"`
	PremiumAmount float64           `json:"premium_amount"`
	Deductible    float64           `json:"deductible"`
	CoverageLimit float64           `json:"coverage_limit"`
	ValidFrom     *time.Time        `json:"valid_from"`
	ValidTo       *time.Time        `json:"valid_to"`
	Status        string            `json:"status"`
	Notes         string            `json:"notes"`
	Metadata      map[string]string `json:"metadata"`
}

// Service manages insurance policies.
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
	}
	return err
}

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 200 {
		return ValidationError{"name is required (max 200)"}
	}
	if strings.TrimSpace(in.PolicyNumber) == "" || len(in.PolicyNumber) > 100 {
		return ValidationError{"policy_number is required (max 100)"}
	}
	switch in.CoverageType {
	case "", store.CovAllRisk, store.CovFireTheft, store.CovLiability, store.CovEquipment, store.CovCyber:
	default:
		return ValidationError{"coverage_type is invalid"}
	}
	switch in.Status {
	case "", store.InsActive, store.InsExpired, store.InsCancelled:
	default:
		return ValidationError{"status is invalid"}
	}
	if in.PremiumAmount < 0 || in.Deductible < 0 || in.CoverageLimit < 0 {
		return ValidationError{"amounts out of range"}
	}
	if in.ValidFrom != nil && in.ValidTo != nil && in.ValidTo.Before(*in.ValidFrom) {
		return ValidationError{"valid_to precedes valid_from"}
	}
	if len(in.Metadata) > 64 {
		return ValidationError{"too many metadata entries"}
	}
	return nil
}

func apply(p *store.InsurancePolicy, in Input) {
	p.Name, p.PolicyNumber, p.Provider, p.CoverageType = strings.TrimSpace(in.Name), strings.TrimSpace(in.PolicyNumber), in.Provider, in.CoverageType
	p.PremiumAmount, p.Deductible, p.CoverageLimit = in.PremiumAmount, in.Deductible, in.CoverageLimit
	p.ValidFrom, p.ValidTo, p.Notes, p.Metadata = in.ValidFrom, in.ValidTo, in.Notes, in.Metadata
	p.Status = in.Status
	if p.Status == "" {
		p.Status = store.InsActive
	}
}

func (s *Service) emit(ctx context.Context, subj authz.Subjects, typ audit.EventType, kind, id string, details map[string]any) {
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: typ, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: kind, SubjectID: id, Outcome: audit.OutcomeOK, Details: details})
}

// Create inserts a policy; a duplicate policy number or name is ErrConflict.
func (s *Service) Create(ctx context.Context, subj authz.Subjects, in Input) (store.InsurancePolicy, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.InsurancePolicy{}, err
	}
	if err := validate(in); err != nil {
		return store.InsurancePolicy{}, err
	}
	now := s.now()
	p := store.InsurancePolicy{ID: store.NewID(), TenantID: subj.TenantID, CreatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
	apply(&p, in)
	if err := s.st.CreateInsurance(ctx, p); err != nil {
		return store.InsurancePolicy{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.InsuranceCreated, audit.SubjectInsurance, p.ID, nil)
	return s.Get(ctx, subj, p.ID)
}

// Get returns one policy with its asset count.
func (s *Service) Get(ctx context.Context, subj authz.Subjects, id string) (store.InsurancePolicy, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.InsurancePolicy{}, err
	}
	p, err := s.st.GetInsurance(ctx, subj.TenantID, id)
	if err != nil {
		return store.InsurancePolicy{}, mapErr(err)
	}
	return p, nil
}

// List returns the tenant's policies matching f.
func (s *Service) List(ctx context.Context, subj authz.Subjects, f store.ListOpts) ([]store.InsurancePolicy, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	rows, err := s.st.ListInsurance(ctx, subj.TenantID, f)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []store.InsurancePolicy{}
	}
	return rows, nil
}

// Update replaces the editable fields.
func (s *Service) Update(ctx context.Context, subj authz.Subjects, id string, in Input) (store.InsurancePolicy, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.InsurancePolicy{}, err
	}
	if err := validate(in); err != nil {
		return store.InsurancePolicy{}, err
	}
	p, err := s.st.GetInsurance(ctx, subj.TenantID, id)
	if err != nil {
		return store.InsurancePolicy{}, mapErr(err)
	}
	apply(&p, in)
	p.UpdatedAt = s.now()
	if err := s.st.UpdateInsurance(ctx, p); err != nil {
		return store.InsurancePolicy{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.InsuranceUpdated, audit.SubjectInsurance, id, nil)
	return s.Get(ctx, subj, id)
}

// Delete removes a policy and its coverage links.
func (s *Service) Delete(ctx context.Context, subj authz.Subjects, id string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if err := s.st.DeleteInsurance(ctx, subj.TenantID, id); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.InsuranceDeleted, audit.SubjectInsurance, id, nil)
	return nil
}

// ListPolicyAssets returns the assets a policy covers.
func (s *Service) ListPolicyAssets(ctx context.Context, subj authz.Subjects, policyID string) ([]store.PolicyAsset, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return nil, err
	}
	if _, err := s.st.GetInsurance(ctx, subj.TenantID, policyID); err != nil {
		return nil, mapErr(err)
	}
	rows, err := s.st.ListPolicyAssets(ctx, subj.TenantID, policyID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []store.PolicyAsset{}
	}
	return rows, nil
}

// AddAssetToPolicy covers an asset under a policy; an asset already covered by
// the policy is ErrConflict.
func (s *Service) AddAssetToPolicy(ctx context.Context, subj authz.Subjects, policyID, assetID string, coveredValue float64, notes string) (store.PolicyAsset, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.PolicyAsset{}, err
	}
	if assetID == "" {
		return store.PolicyAsset{}, ValidationError{"asset_id is required"}
	}
	if coveredValue < 0 {
		return store.PolicyAsset{}, ValidationError{"covered_value out of range"}
	}
	if _, err := s.st.GetInsurance(ctx, subj.TenantID, policyID); err != nil {
		return store.PolicyAsset{}, mapErr(err)
	}
	a, err := s.st.GetAsset(ctx, subj.TenantID, assetID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return store.PolicyAsset{}, ValidationError{"asset_id does not exist"}
		}
		return store.PolicyAsset{}, err
	}
	pa := store.PolicyAsset{ID: store.NewID(), TenantID: subj.TenantID, PolicyID: policyID, AssetID: assetID, CoveredValue: coveredValue,
		Notes: notes, AssetTag: a.AssetTag, AssetName: a.Name, ModelName: a.ModelName, CreatedBy: subj.ActorID(), CreatedAt: s.now()}
	if err := s.st.AddPolicyAsset(ctx, pa); err != nil {
		return store.PolicyAsset{}, mapErr(err)
	}
	s.emit(ctx, subj, audit.PolicyAssetAdded, audit.SubjectPolicyAsset, pa.ID, map[string]any{"policy_id": policyID, "asset_id": assetID})
	return pa, nil
}

// RemoveAssetFromPolicy drops an asset's coverage.
func (s *Service) RemoveAssetFromPolicy(ctx context.Context, subj authz.Subjects, policyID, assetID string) error {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return err
	}
	if err := s.st.RemovePolicyAsset(ctx, subj.TenantID, policyID, assetID); err != nil {
		return mapErr(err)
	}
	s.emit(ctx, subj, audit.PolicyAssetRemoved, audit.SubjectPolicyAsset, policyID+"/"+assetID, map[string]any{"policy_id": policyID, "asset_id": assetID})
	return nil
}
