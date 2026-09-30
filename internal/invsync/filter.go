package invsync

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invclient"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// ActionExcluded marks a host the tenant's sync filter skips (feature 030).
const ActionExcluded = "excluded"

// Exclusion reasons.
const (
	ReasonStale           = "stale"
	ReasonRetired         = "retired"
	ReasonVM              = "virtual_machine"
	ReasonContainer       = "container"
	ReasonHostnameExclude = "hostname_excluded"
	ReasonHostnameInclude = "hostname_not_included"
	ReasonOSExclude       = "os_excluded"
	ReasonOSInclude       = "os_not_included"
)

// Limits of the filter settings.
const (
	MaxPatterns   = 50
	MaxPatternLen = 200
)

// SettingsValidationError reports an invalid filter setting.
type SettingsValidationError struct{ Field, Msg string }

func (e SettingsValidationError) Error() string { return "invsync: " + e.Field + ": " + e.Msg }

// Settings returns the tenant's filter settings (no filter when never set).
func (s *Service) Settings(ctx context.Context, subj authz.Subjects) (store.InvSyncSettings, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.InvSyncSettings{}, err
	}
	st, _, err := s.st.GetInvSyncSettings(ctx, subj.TenantID)
	return normalizedCopy(st), err
}

// SaveSettings validates and stores the tenant's filter settings.
func (s *Service) SaveSettings(ctx context.Context, subj authz.Subjects, in store.InvSyncSettings) (store.InvSyncSettings, error) {
	if err := authz.RequireTenant(subj, subj.TenantID); err != nil {
		return store.InvSyncSettings{}, err
	}
	var err error
	for _, f := range []struct {
		name string
		v    *[]string
	}{{"hostname_include", &in.HostnameInclude}, {"hostname_exclude", &in.HostnameExclude}, {"os_include", &in.OSInclude}, {"os_exclude", &in.OSExclude}} {
		if *f.v, err = normalizePatterns(f.name, *f.v); err != nil {
			return store.InvSyncSettings{}, err
		}
	}
	in.TenantID, in.UpdatedBy, in.UpdatedAt = subj.TenantID, subj.ActorID(), s.now()
	if err := s.st.PutInvSyncSettings(ctx, in); err != nil {
		return store.InvSyncSettings{}, err
	}
	audit.Emit(ctx, s.aud, audit.Event{TenantID: subj.TenantID, EventType: audit.InventorySyncSettingsUpdated, ActorKind: subj.ActorKind, ActorID: subj.ActorID(),
		SubjectKind: audit.SubjectSync, SubjectID: "settings", Outcome: audit.OutcomeOK, Details: map[string]any{
			"exclude_vms": in.ExcludeVMs, "exclude_containers": in.ExcludeContainers, "skip_stale": in.SkipStale, "skip_retired": in.SkipRetired,
			"hostname_include": len(in.HostnameInclude), "hostname_exclude": len(in.HostnameExclude), "os_include": len(in.OSInclude), "os_exclude": len(in.OSExclude)}})
	return normalizedCopy(in), nil
}

// normalizePatterns trims, lower-cases, drops empties/duplicates and checks
// the glob syntax.
func normalizePatterns(field string, in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range in {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		if len(p) > MaxPatternLen {
			return nil, SettingsValidationError{field, fmt.Sprintf("patterns are at most %d characters", MaxPatternLen)}
		}
		if _, err := path.Match(p, ""); err != nil {
			return nil, SettingsValidationError{field, fmt.Sprintf("%q is not a valid pattern (use * and ?)", p)}
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) > MaxPatterns {
		return nil, SettingsValidationError{field, fmt.Sprintf("at most %d patterns", MaxPatterns)}
	}
	return out, nil
}

func normalizedCopy(s store.InvSyncSettings) store.InvSyncSettings {
	for _, v := range []*[]string{&s.HostnameInclude, &s.HostnameExclude, &s.OSInclude, &s.OSExclude} {
		if *v == nil {
			*v = []string{}
		}
	}
	return s
}

// needsRoles reports whether the filter uses the virtualization role.
func needsRoles(f store.InvSyncSettings) bool { return f.ExcludeVMs || f.ExcludeContainers }

func matchAny(patterns []string, value string) bool {
	v := strings.ToLower(value)
	for _, p := range patterns {
		if ok, _ := path.Match(p, v); ok {
			return true
		}
	}
	return false
}

// exclusion returns why the filter skips a host ("" = synced). Hosts whose
// virtualization role is unknown (older agents) are not excluded by the
// VM/container toggles.
func exclusion(f store.InvSyncSettings, h invclient.Host, role string) string {
	switch {
	case f.SkipRetired && h.Status == "retired":
		return ReasonRetired
	case f.SkipStale && h.Status == "stale":
		return ReasonStale
	case f.ExcludeVMs && role == "vm":
		return ReasonVM
	case f.ExcludeContainers && role == "container":
		return ReasonContainer
	case matchAny(f.HostnameExclude, h.Hostname):
		return ReasonHostnameExclude
	case len(f.HostnameInclude) > 0 && !matchAny(f.HostnameInclude, h.Hostname):
		return ReasonHostnameInclude
	case matchAny(f.OSExclude, h.OSName):
		return ReasonOSExclude
	case len(f.OSInclude) > 0 && !matchAny(f.OSInclude, h.OSName):
		return ReasonOSInclude
	}
	return ""
}

// applyFilter marks the changes of excluded hosts (their assets are never
// touched, existing ones are not deleted).
func applyFilter(changes []Change, byHost map[string]invclient.Host, f store.InvSyncSettings, roles map[string]string) []Change {
	for i := range changes {
		h := byHost[changes[i].HostID]
		if r := exclusion(f, h, roles[h.ID]); r != "" {
			changes[i].Action, changes[i].Reason, changes[i].Changes = ActionExcluded, r, nil
		}
	}
	return changes
}
