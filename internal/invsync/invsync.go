// Package invsync reconciles the tenant's assets against the platform inventory
// module: Preview lists the inventory hosts (over the SPIFFE channel) and diffs
// them against the assets (match by inventory host id, then serial, then
// hostname) into create/update/unchanged changes; Execute applies the selected
// hosts, creating auto-tagged assets from the host hardware/OS and updating the
// sync-owned fields of matched ones. When the inventory module is unavailable
// the request fails with ErrUnavailable and nothing is written.
package invsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/assets"
	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/deprec"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invclient"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// ErrUnavailable is returned when the inventory module cannot be reached.
var ErrUnavailable = invclient.ErrUnavailable

// Preview is the result of a dry run.
type Preview struct {
	Hosts     int      `json:"hosts"`
	Create    int      `json:"create"`
	Update    int      `json:"update"`
	Unchanged int      `json:"unchanged"`
	Changes   []Change `json:"changes"`
}

// Result is the outcome of an execute.
type Result struct {
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
	Changes  []Change `json:"changes"`
	Selected int      `json:"selected"`
}

// Service runs the sync.
type Service struct {
	st  repo.Store
	inv invclient.Client
	aud audit.Recorder
	now func() time.Time
}

// New builds the service (aud may be nil).
func New(st repo.Store, inv invclient.Client, aud audit.Recorder) *Service {
	return &Service{st: st, inv: inv, aud: aud, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

func (s *Service) load(ctx context.Context, tenantID string) ([]invclient.Host, []store.Asset, error) {
	if s.inv == nil {
		return nil, nil, ErrUnavailable
	}
	hosts, err := s.inv.ListHosts(ctx, tenantID)
	if err != nil {
		if !errors.Is(err, invclient.ErrUnavailable) {
			err = fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		return nil, nil, err
	}
	all, err := s.st.AllAssets(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	return hosts, all, nil
}

func summarize(changes []Change) Preview {
	p := Preview{Hosts: len(changes), Changes: changes}
	for _, c := range changes {
		switch c.Action {
		case ActionCreate:
			p.Create++
		case ActionUpdate:
			p.Update++
		default:
			p.Unchanged++
		}
	}
	return p
}

// Preview diffs the tenant's inventory hosts against its assets (no writes).
func (s *Service) Preview(ctx context.Context, subj authz.Subjects) (Preview, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return Preview{}, err
	}
	hosts, all, err := s.load(ctx, subj.TenantID)
	if err != nil {
		return Preview{}, err
	}
	p := summarize(Diff(hosts, all))
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.InventorySyncPreviewed, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectSync, SubjectID: "preview", Outcome: audit.OutcomeOK, Details: map[string]any{"hosts": p.Hosts, "create": p.Create, "update": p.Update}})
	return p, nil
}

// Execute applies the sync for the selected hostnames (all hosts when the
// selection is empty): creates auto-tagged assets for unmatched hosts and
// updates the sync-owned fields of matched ones.
func (s *Service) Execute(ctx context.Context, subj authz.Subjects, hostnames []string) (Result, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return Result{}, err
	}
	hosts, all, err := s.load(ctx, subj.TenantID)
	if err != nil {
		return Result{}, err
	}
	selected := map[string]bool{}
	for _, h := range hostnames {
		selected[h] = true
	}
	byHost := map[string]invclient.Host{}
	for _, h := range hosts {
		byHost[h.Hostname] = h
	}
	changes := Diff(hosts, all)
	res := Result{Errors: []string{}, Changes: []Change{}}
	now := s.now()
	for _, c := range changes {
		if len(selected) > 0 && !selected[c.Hostname] {
			continue
		}
		res.Selected++
		h := byHost[c.Hostname]
		d := desired(h)
		switch c.Action {
		case ActionCreate:
			a := store.Asset{ID: store.NewID(), TenantID: subj.TenantID, Status: store.AssetDeployable, DepreciationRate: deprec.DefaultRate,
				CreatedBy: subj.ActorID(), UpdatedBy: subj.ActorID(), CreatedAt: now, UpdatedAt: now}
			applyDesired(&a, d)
			var cerr error
			for attempt := 0; attempt < 5; attempt++ {
				a.AssetTag = assets.AutoTag()
				if cerr = s.st.CreateAsset(ctx, a); !errors.Is(cerr, repo.ErrConflict) {
					break
				}
			}
			if cerr != nil {
				res.Errors = append(res.Errors, c.Hostname+": "+cerr.Error())
				continue
			}
			c.AssetID, c.AssetTag = a.ID, a.AssetTag
			res.Created++
		case ActionUpdate:
			a, gerr := s.st.GetAsset(ctx, subj.TenantID, c.AssetID)
			if gerr != nil {
				res.Errors = append(res.Errors, c.Hostname+": "+gerr.Error())
				continue
			}
			applyDesired(&a, d)
			a.UpdatedBy, a.UpdatedAt = subj.ActorID(), now
			if uerr := s.st.UpdateAsset(ctx, a); uerr != nil {
				res.Errors = append(res.Errors, c.Hostname+": "+uerr.Error())
				continue
			}
			res.Updated++
		default:
			res.Skipped++
		}
		res.Changes = append(res.Changes, c)
	}
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.InventorySyncExecuted, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectSync, SubjectID: "execute", Outcome: audit.OutcomeOK,
		Details: map[string]any{"selected": res.Selected, "created": res.Created, "updated": res.Updated, "errors": len(res.Errors)}})
	return res, nil
}
