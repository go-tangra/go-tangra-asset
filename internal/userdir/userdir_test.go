package userdir

import (
	"context"
	"errors"
	"testing"

	authv1 "github.com/go-tangra/go-tangra-auth/sdk/v4/api/proto/auth/v1"
	"google.golang.org/grpc"
)

// stubProfiles is an in-process authv1.ProfilesClient.
type stubProfiles struct {
	members  []string
	profiles map[string]*authv1.PublicProfile
	fail     bool
	pages    int
}

func (s *stubProfiles) Lookup(_ context.Context, in *authv1.LookupProfilesRequest, _ ...grpc.CallOption) (*authv1.LookupProfilesResponse, error) {
	if s.fail {
		return nil, errors.New("boom")
	}
	out := &authv1.LookupProfilesResponse{}
	for _, id := range in.GetUserIds() {
		if p, ok := s.profiles[id]; ok {
			out.Profiles = append(out.Profiles, p)
		}
	}
	return out, nil
}

func (s *stubProfiles) ListMembers(_ context.Context, in *authv1.ListMembersRequest, _ ...grpc.CallOption) (*authv1.ListMembersResponse, error) {
	if s.fail {
		return nil, errors.New("boom")
	}
	s.pages++
	// two pages of one id each when there are two members
	if in.GetCursor() == "" {
		if len(s.members) > 1 {
			return &authv1.ListMembersResponse{UserIds: s.members[:1], NextCursor: "c1"}, nil
		}
		return &authv1.ListMembersResponse{UserIds: s.members}, nil
	}
	return &authv1.ListMembersResponse{UserIds: s.members[1:]}, nil
}

func newAuth(s *stubProfiles) *Auth { return &Auth{c: s} }

func TestAuthResolve(t *testing.T) {
	s := &stubProfiles{profiles: map[string]*authv1.PublicProfile{"u1": {UserId: "u1", DisplayName: "Ann", AvatarUrl: "a"}}}
	d := newAuth(s)
	u, err := d.Resolve(context.Background(), "t", "u1")
	if err != nil || u.DisplayName != "Ann" || u.AvatarURL != "a" {
		t.Fatalf("%+v %v", u, err)
	}
	if _, err := d.Resolve(context.Background(), "t", "nope"); !errors.Is(err, ErrUnknownUser) {
		t.Fatalf("want unknown: %v", err)
	}
	s.fail = true
	if _, err := d.Resolve(context.Background(), "t", "u1"); err == nil {
		t.Fatal("want error")
	}
}

func TestAuthListUsers(t *testing.T) {
	s := &stubProfiles{members: []string{"u2", "u1"}, profiles: map[string]*authv1.PublicProfile{
		"u1": {UserId: "u1", DisplayName: "ann"}, "u2": {UserId: "u2", DisplayName: "Bob"},
	}}
	d := newAuth(s)
	users, err := d.ListUsers(context.Background(), "t")
	if err != nil || len(users) != 2 {
		t.Fatalf("%v %v", users, err)
	}
	if users[0].DisplayName != "ann" || users[1].DisplayName != "Bob" {
		t.Fatalf("sorted case-insensitively: %v", users)
	}
	if s.pages != 2 {
		t.Fatalf("expected two pages, got %d", s.pages)
	}
	s.fail = true
	if _, err := d.ListUsers(context.Background(), "t"); err == nil {
		t.Fatal("want error")
	}
	// lookup failure after a successful member page
	s2 := &stubProfiles{members: []string{"u1"}, profiles: map[string]*authv1.PublicProfile{}}
	d2 := newAuth(s2)
	if users, err := d2.ListUsers(context.Background(), "t"); err != nil || len(users) != 0 {
		t.Fatalf("unknown profiles omitted: %v %v", users, err)
	}
}

func TestFake(t *testing.T) {
	f := NewFake()
	f.Add("t", User{ID: "u1", DisplayName: "Zed"})
	f.Add("t", User{ID: "u2", DisplayName: "Amy"})
	u, err := f.Resolve(context.Background(), "t", "u1")
	if err != nil || u.DisplayName != "Zed" {
		t.Fatal(u, err)
	}
	if _, err := f.Resolve(context.Background(), "t", "x"); !errors.Is(err, ErrUnknownUser) {
		t.Fatal(err)
	}
	if _, err := f.Resolve(context.Background(), "other", "u1"); !errors.Is(err, ErrUnknownUser) {
		t.Fatal("tenant scoped")
	}
	l, _ := f.ListUsers(context.Background(), "t")
	if len(l) != 2 || l[0].DisplayName != "Amy" {
		t.Fatal(l)
	}
	f.Err = errors.New("down")
	if _, err := f.Resolve(context.Background(), "t", "u1"); err == nil {
		t.Fatal("want err")
	}
	if _, err := f.ListUsers(context.Background(), "t"); err == nil {
		t.Fatal("want err")
	}
}
