package memstore

import (
	"context"

	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

func copySettings(s store.InvSyncSettings) store.InvSyncSettings {
	s.HostnameInclude = append([]string(nil), s.HostnameInclude...)
	s.HostnameExclude = append([]string(nil), s.HostnameExclude...)
	s.OSInclude = append([]string(nil), s.OSInclude...)
	s.OSExclude = append([]string(nil), s.OSExclude...)
	return s
}

// GetInvSyncSettings implements repo.Store.
func (m *Mem) GetInvSyncSettings(_ context.Context, tenantID string) (store.InvSyncSettings, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("GetInvSyncSettings"); err != nil {
		return store.InvSyncSettings{}, false, err
	}
	s, ok := m.invsync[tenantID]
	if !ok {
		return store.InvSyncSettings{TenantID: tenantID}, false, nil
	}
	return copySettings(s), true, nil
}

// PutInvSyncSettings implements repo.Store.
func (m *Mem) PutInvSyncSettings(_ context.Context, s store.InvSyncSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.fail("PutInvSyncSettings"); err != nil {
		return err
	}
	if m.invsync == nil {
		m.invsync = map[string]store.InvSyncSettings{}
	}
	m.invsync[s.TenantID] = copySettings(s)
	return nil
}
