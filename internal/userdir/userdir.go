// Package userdir resolves platform users (the assignees of assets) through the
// auth service's Profiles API over the Freya SPIFFE channel. Assets carry only
// the platform user id; display names are resolved here for views and the
// assignment history, and ListUsers feeds the assignee picker. The phone number
// and e-mail are never returned by auth and never stored by the asset module.
package userdir

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	authv1 "github.com/go-tangra/go-tangra-auth/sdk/v4/api/proto/auth/v1"
	"google.golang.org/grpc"
)

// ErrUnknownUser is returned when the id does not resolve within the tenant.
var ErrUnknownUser = errors.New("userdir: unknown user")

// User is a platform member as the asset module sees it.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// Directory answers "who is this user id" and lists a tenant's active members.
type Directory interface {
	Resolve(ctx context.Context, tenantID, userID string) (User, error)
	ListUsers(ctx context.Context, tenantID string) ([]User, error)
}

// --- auth-backed implementation

// Auth is the Directory over the auth Profiles gRPC service.
type Auth struct{ c authv1.ProfilesClient }

// New builds the auth-backed directory on a Freya mesh connection to "auth".
func New(conn grpc.ClientConnInterface) *Auth { return &Auth{c: authv1.NewProfilesClient(conn)} }

// Resolve looks one user up; an id auth does not know is ErrUnknownUser.
func (a *Auth) Resolve(ctx context.Context, tenantID, userID string) (User, error) {
	res, err := a.c.Lookup(ctx, &authv1.LookupProfilesRequest{TenantId: tenantID, UserIds: []string{userID}})
	if err != nil {
		return User{}, err
	}
	for _, p := range res.GetProfiles() {
		if p.GetUserId() == userID {
			return User{ID: p.GetUserId(), DisplayName: p.GetDisplayName(), AvatarURL: p.GetAvatarUrl()}, nil
		}
	}
	return User{}, ErrUnknownUser
}

// ListUsers pages every active member of the tenant and resolves their names
// (100 per lookup, the auth cap).
func (a *Auth) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	var ids []string
	cursor := ""
	for {
		res, err := a.c.ListMembers(ctx, &authv1.ListMembersRequest{TenantId: tenantID, Cursor: cursor, Limit: 1000})
		if err != nil {
			return nil, err
		}
		ids = append(ids, res.GetUserIds()...)
		if res.GetNextCursor() == "" || len(res.GetUserIds()) == 0 {
			break
		}
		cursor = res.GetNextCursor()
	}
	out := make([]User, 0, len(ids))
	for i := 0; i < len(ids); i += 100 {
		end := min(i+100, len(ids))
		res, err := a.c.Lookup(ctx, &authv1.LookupProfilesRequest{TenantId: tenantID, UserIds: ids[i:end]})
		if err != nil {
			return nil, err
		}
		for _, p := range res.GetProfiles() {
			out = append(out, User{ID: p.GetUserId(), DisplayName: p.GetDisplayName(), AvatarURL: p.GetAvatarUrl()})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out, nil
}

// --- fake

// Fake is an in-memory Directory for tests. Users are keyed by tenant.
type Fake struct {
	mu    sync.Mutex
	users map[string]map[string]User // tenant → id → user
	Err   error                      // when set, every call fails with it
}

// NewFake builds an empty directory.
func NewFake() *Fake { return &Fake{users: map[string]map[string]User{}} }

// Add registers a user in the tenant.
func (f *Fake) Add(tenantID string, u User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.users[tenantID] == nil {
		f.users[tenantID] = map[string]User{}
	}
	f.users[tenantID][u.ID] = u
}

// Resolve implements Directory.
func (f *Fake) Resolve(_ context.Context, tenantID, userID string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return User{}, f.Err
	}
	if u, ok := f.users[tenantID][userID]; ok {
		return u, nil
	}
	return User{}, ErrUnknownUser
}

// ListUsers implements Directory.
func (f *Fake) ListUsers(_ context.Context, tenantID string) ([]User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return nil, f.Err
	}
	out := make([]User, 0, len(f.users[tenantID]))
	for _, u := range f.users[tenantID] {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}

var _ Directory = (*Auth)(nil)
var _ Directory = (*Fake)(nil)
