// Package invclient is the asset module's view of the platform inventory module:
// the hosts of a tenant, as the inventory service reports them over the Freya
// SPIFFE channel. It adapts services/inventory/pkg/inventoryclient behind a small
// interface so inventory-sync is unit-tested with a Fake and degrades cleanly
// (ErrUnavailable, no writes) when the inventory module cannot be reached.
package invclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-freya/freya/services/inventory/pkg/inventoryclient"
	"google.golang.org/grpc"
)

// ErrUnavailable wraps any transport/inventory failure so callers can report
// "inventory unavailable" without leaking detail.
var ErrUnavailable = errors.New("invclient: inventory unavailable")

// Host is the subset of an inventory host the asset sync consumes.
type Host struct {
	ID           string
	Hostname     string
	SystemSerial string
	Manufacturer string
	Model        string
	OSName       string
	OSVersion    string
	OSArch       string
	Status       string
	LastSeen     time.Time
}

// Client lists a tenant's inventory hosts.
type Client interface {
	ListHosts(ctx context.Context, tenantID string) ([]Host, error)
}

// Mesh is the Client over the inventory gRPC surface.
type Mesh struct{ c *inventoryclient.Client }

// New builds the mesh client on a Freya connection to the inventory service.
func New(conn grpc.ClientConnInterface) *Mesh { return &Mesh{c: inventoryclient.New(conn)} }

// ListHosts pages every host of the tenant.
func (m *Mesh) ListHosts(ctx context.Context, tenantID string) ([]Host, error) {
	var out []Host
	cursor := ""
	for {
		hosts, err := m.c.ListHosts(ctx, tenantID, inventoryclient.HostFilter{Limit: 500, CursorID: cursor})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		for _, h := range hosts {
			out = append(out, Host{
				ID: h.ID, Hostname: h.Hostname, SystemSerial: h.SystemSerial, Manufacturer: h.Manufacturer,
				Model: h.Model, OSName: h.OSName, OSVersion: h.OSVersion, OSArch: h.OSArch, Status: h.Status, LastSeen: h.LastSeen,
			})
		}
		if len(hosts) < 500 {
			return out, nil
		}
		cursor = hosts[len(hosts)-1].ID
	}
}

// Fake is an in-memory Client for tests.
type Fake struct {
	mu    sync.Mutex
	hosts map[string][]Host
	Down  bool // when set, ListHosts returns ErrUnavailable
}

// NewFake builds an empty fake.
func NewFake() *Fake { return &Fake{hosts: map[string][]Host{}} }

// Set replaces the hosts of a tenant.
func (f *Fake) Set(tenantID string, hosts []Host) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hosts[tenantID] = append([]Host(nil), hosts...)
}

// ListHosts implements Client.
func (f *Fake) ListHosts(_ context.Context, tenantID string) ([]Host, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Down {
		return nil, ErrUnavailable
	}
	return append([]Host(nil), f.hosts[tenantID]...), nil
}

var _ Client = (*Mesh)(nil)
var _ Client = (*Fake)(nil)
