// Package scheduler is the lifecycle worker: on an interval it sweeps every
// tenant (system scope) and evaluates warranty expiry (purchase_date +
// warranty_months), license and insurance validity (valid_to), and consumable
// stock (amount <= min_amount). It publishes asset.warranty.expiring /
// license.expiring / insurance.expiring / consumable.low_stock to the tenant's
// event bus, auto-transitions lapsed licenses and policies to "expired", and
// dedups through asset_notify_state so the same condition is not re-alerted on
// every pass (a condition that clears and recurs alerts again).
package scheduler

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/config"
	"github.com/go-tangra/go-tangra-asset/v4/internal/events"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Notify states.
const (
	StateSoon    = "soon"
	StateExpired = "expired"
	StateLow     = "low"
	StateOK      = "ok"
)

// Report summarizes one tenant sweep.
type Report struct {
	TenantID         string
	WarrantyExpiring int
	LicenseEvents    int
	LicensesExpired  int
	InsuranceEvents  int
	InsuranceExpired int
	LowStock         int
	Errors           int
}

// Service runs the sweeps.
type Service struct {
	st      repo.Store
	pub     events.Publisher
	aud     audit.Recorder
	windows config.SoonWindows
	log     *slog.Logger
	now     func() time.Time
}

// New builds the worker (pub/aud/log may be nil).
func New(st repo.Store, pub events.Publisher, aud audit.Recorder, windows config.SoonWindows, log *slog.Logger) *Service {
	if pub == nil {
		pub = events.HubPublisher{}
	}
	if log == nil {
		log = slog.Default()
	}
	day := 24 * time.Hour
	if windows.Warranty <= 0 {
		windows.Warranty = 30 * day
	}
	if windows.License <= 0 {
		windows.License = 30 * day
	}
	if windows.Insurance <= 0 {
		windows.Insurance = 30 * day
	}
	return &Service{st: st, pub: pub, aud: aud, windows: windows, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock injects the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Run sweeps every tenant now and then on each tick until ctx is done.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	s.SweepAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.SweepAll(ctx)
		}
	}
}

// SweepAll sweeps every tenant that has asset data.
func (s *Service) SweepAll(ctx context.Context) []Report {
	ids, err := s.st.TenantIDs(ctx)
	if err != nil {
		s.log.Warn("scheduler: tenant enumeration", "err", err)
		return nil
	}
	out := make([]Report, 0, len(ids))
	for _, id := range ids {
		if ctx.Err() != nil {
			return out
		}
		r := s.Sweep(ctx, id)
		if r.Errors > 0 {
			s.log.Warn("scheduler: sweep had errors", "tenant", id, "errors", r.Errors)
		}
		out = append(out, r)
	}
	return out
}

// shouldNotify reports whether the condition in state differs from what was
// last notified, and records the new state. It never notifies twice for the
// same (key, state).
func (s *Service) shouldNotify(ctx context.Context, tenantID, key, state string, rep *Report) bool {
	prev, err := s.st.GetNotifyState(ctx, tenantID, key)
	if err == nil && prev.LastState == state {
		return false
	}
	if err := s.st.UpsertNotifyState(ctx, store.NotifyState{TenantID: tenantID, ConditionKey: key, LastNotifiedAt: s.now(), LastState: state}); err != nil {
		rep.Errors++
		return false
	}
	return state != StateOK
}

// Sweep evaluates one tenant.
func (s *Service) Sweep(ctx context.Context, tenantID string) Report {
	rep := Report{TenantID: tenantID}
	now := s.now()
	s.sweepWarranty(ctx, tenantID, now, &rep)
	s.sweepLicenses(ctx, tenantID, now, &rep)
	s.sweepInsurance(ctx, tenantID, now, &rep)
	s.sweepConsumables(ctx, tenantID, &rep)
	return rep
}

func (s *Service) sweepWarranty(ctx context.Context, tenantID string, now time.Time, rep *Report) {
	rows, err := s.st.AllAssets(ctx, tenantID)
	if err != nil {
		rep.Errors++
		return
	}
	for _, a := range rows {
		if a.PurchaseDate == nil || a.WarrantyMonths <= 0 || a.Status == store.AssetArchived {
			continue
		}
		exp := a.PurchaseDate.AddDate(0, a.WarrantyMonths, 0)
		key := "warranty:" + a.ID
		state := StateOK
		switch {
		case exp.Before(now):
			state = StateExpired
		case exp.Sub(now) <= s.windows.Warranty:
			state = StateSoon
		}
		if s.shouldNotify(ctx, tenantID, key, state, rep) {
			s.pub.Publish(ctx, tenantID, events.WarrantyExpiring, events.WarrantyExpiringPayload(a.ID, a.AssetTag, exp))
			rep.WarrantyExpiring++
		}
	}
}

func (s *Service) sweepLicenses(ctx context.Context, tenantID string, now time.Time, rep *Report) {
	rows, err := s.st.AllLicenses(ctx, tenantID)
	if err != nil {
		rep.Errors++
		return
	}
	for _, l := range rows {
		if l.ValidTo == nil || l.Status == store.LicSuspended {
			continue
		}
		key := "license:" + l.ID
		state := StateOK
		switch {
		case l.ValidTo.Before(now):
			state = StateExpired
		case l.ValidTo.Sub(now) <= s.windows.License:
			state = StateSoon
		}
		if state == StateExpired && l.Status != store.LicExpired {
			l.Status, l.UpdatedAt = store.LicExpired, now
			if err := s.st.UpdateLicense(ctx, l); err != nil {
				rep.Errors++
			} else {
				rep.LicensesExpired++
				audit.Emit(ctx, s.aud, audit.Event{TenantID: tenantID, EventType: audit.LicenseExpired, ActorKind: audit.ActorSystem, ActorID: "scheduler",
					SubjectKind: audit.SubjectLicense, SubjectID: l.ID, Outcome: audit.OutcomeOK})
			}
		}
		if s.shouldNotify(ctx, tenantID, key, state, rep) {
			s.pub.Publish(ctx, tenantID, events.LicenseExpiring, events.LicenseExpiringPayload(l.ID, l.Name, *l.ValidTo))
			rep.LicenseEvents++
		}
	}
}

func (s *Service) sweepInsurance(ctx context.Context, tenantID string, now time.Time, rep *Report) {
	rows, err := s.st.AllInsurance(ctx, tenantID)
	if err != nil {
		rep.Errors++
		return
	}
	for _, p := range rows {
		if p.ValidTo == nil || p.Status == store.InsCancelled {
			continue
		}
		key := "insurance:" + p.ID
		state := StateOK
		switch {
		case p.ValidTo.Before(now):
			state = StateExpired
		case p.ValidTo.Sub(now) <= s.windows.Insurance:
			state = StateSoon
		}
		if state == StateExpired && p.Status != store.InsExpired {
			p.Status, p.UpdatedAt = store.InsExpired, now
			if err := s.st.UpdateInsurance(ctx, p); err != nil {
				rep.Errors++
			} else {
				rep.InsuranceExpired++
				audit.Emit(ctx, s.aud, audit.Event{TenantID: tenantID, EventType: audit.InsuranceExpired, ActorKind: audit.ActorSystem, ActorID: "scheduler",
					SubjectKind: audit.SubjectInsurance, SubjectID: p.ID, Outcome: audit.OutcomeOK})
			}
		}
		if s.shouldNotify(ctx, tenantID, key, state, rep) {
			s.pub.Publish(ctx, tenantID, events.InsuranceExpiring, events.InsuranceExpiringPayload(p.ID, p.PolicyNumber, *p.ValidTo))
			rep.InsuranceEvents++
		}
	}
}

func (s *Service) sweepConsumables(ctx context.Context, tenantID string, rep *Report) {
	rows, err := s.st.AllConsumables(ctx, tenantID)
	if err != nil {
		rep.Errors++
		return
	}
	for _, c := range rows {
		if c.MinAmount <= 0 {
			continue
		}
		key := "low_stock:" + c.ID
		state := StateOK
		if c.Amount <= c.MinAmount {
			state = StateLow + ":" + strconv.Itoa(c.Amount)
		}
		if s.shouldNotify(ctx, tenantID, key, state, rep) {
			s.pub.Publish(ctx, tenantID, events.ConsumableLowStock, events.LowStockPayload(c.ID, c.Name, c.Amount, c.MinAmount))
			rep.LowStock++
		}
	}
}
