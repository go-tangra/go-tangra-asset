package authz

import (
	"errors"
	"testing"
)

func TestIsAdmin(t *testing.T) {
	cases := []struct {
		name string
		s    Subjects
		want bool
	}{
		{"system actor", Subjects{ActorKind: ActorSystem}, true},
		{"admin role", Subjects{ActorKind: ActorUser, Roles: []string{"admin"}}, true},
		{"owner role", Subjects{ActorKind: ActorUser, Roles: []string{"owner"}}, true},
		{"system role", Subjects{ActorKind: ActorUser, Roles: []string{"system"}}, true},
		{"plain user", Subjects{ActorKind: ActorUser, Roles: []string{"member"}}, false},
		{"no roles", Subjects{ActorKind: ActorUser}, false},
		{"service no role", Subjects{ActorKind: ActorService}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.IsAdmin(); got != tc.want {
				t.Errorf("IsAdmin()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestIsPlatformAdmin(t *testing.T) {
	cases := []struct {
		name string
		s    Subjects
		want bool
	}{
		{"system actor", Subjects{ActorKind: ActorSystem}, true},
		{"platform-admin role", Subjects{ActorKind: ActorUser, Roles: []string{"platform-admin"}}, true},
		{"admin role", Subjects{ActorKind: ActorUser, Roles: []string{"admin"}}, true},
		{"owner role", Subjects{ActorKind: ActorUser, Roles: []string{"owner"}}, true},
		{"system role", Subjects{ActorKind: ActorUser, Roles: []string{"system"}}, true},
		{"plain user", Subjects{ActorKind: ActorUser, Roles: []string{"member"}}, false},
		{"service no role", Subjects{ActorKind: ActorService}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.IsPlatformAdmin(); got != tc.want {
				t.Errorf("IsPlatformAdmin()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestActorID(t *testing.T) {
	if got := (Subjects{UserID: "u1", ActorKind: ActorUser}).ActorID(); got != "u1" {
		t.Errorf("ActorID()=%q want u1", got)
	}
	if got := (Subjects{ActorKind: ActorService}).ActorID(); got != ActorService {
		t.Errorf("ActorID()=%q want %q", got, ActorService)
	}
	if got := (Subjects{ActorKind: ActorSystem}).ActorID(); got != ActorSystem {
		t.Errorf("ActorID()=%q want %q", got, ActorSystem)
	}
}

func TestRequireAdmin(t *testing.T) {
	if err := RequireAdmin(Subjects{ActorKind: ActorUser, Roles: []string{"admin"}}); err != nil {
		t.Errorf("admin rejected: %v", err)
	}
	err := RequireAdmin(Subjects{ActorKind: ActorUser, Roles: []string{"member"}})
	if err == nil {
		t.Fatal("expected forbidden for member")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error is not ErrForbidden: %v", err)
	}
}

func TestRequirePlatformAdmin(t *testing.T) {
	if err := RequirePlatformAdmin(Subjects{ActorKind: ActorSystem}); err != nil {
		t.Errorf("system rejected: %v", err)
	}
	if err := RequirePlatformAdmin(Subjects{ActorKind: ActorUser, Roles: []string{"platform-admin"}}); err != nil {
		t.Errorf("platform-admin rejected: %v", err)
	}
	err := RequirePlatformAdmin(Subjects{ActorKind: ActorUser, Roles: []string{"admin-lite"}})
	if err == nil {
		t.Fatal("expected forbidden")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("error is not ErrForbidden: %v", err)
	}
}

func TestRequireTenant(t *testing.T) {
	cases := []struct {
		name     string
		s        Subjects
		tenantID string
		wantErr  bool
	}{
		{"same tenant", Subjects{TenantID: "t1", ActorKind: ActorUser}, "t1", false},
		{"system cross tenant", Subjects{TenantID: "t1", ActorKind: ActorSystem}, "t2", false},
		{"mismatch", Subjects{TenantID: "t1", ActorKind: ActorUser}, "t2", true},
		{"empty tenant", Subjects{TenantID: "t1", ActorKind: ActorUser}, "", true},
		{"empty tenant system", Subjects{TenantID: "t1", ActorKind: ActorSystem}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireTenant(tc.s, tc.tenantID)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, ErrForbidden) {
					t.Errorf("error is not ErrForbidden: %v", err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
