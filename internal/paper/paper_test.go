package paper

import (
	"context"
	"errors"
	"testing"

	"github.com/go-tangra/go-tangra-paperless/sdk/v4/pkg/paperlessclient"
	"google.golang.org/grpc"

	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
)

type fakeAPI struct {
	err      error
	created  paperlessclient.CreateInput
	hardSeen bool
}

func (f *fakeAPI) EnsureCategory(context.Context, string, []string) (string, error) { return "cat-1", f.err }
func (f *fakeAPI) CreateDocument(_ context.Context, _ string, in paperlessclient.CreateInput) (paperlessclient.Document, error) {
	f.created = in
	return paperlessclient.Document{ID: "d-1"}, f.err
}
func (f *fakeAPI) DownloadDocument(context.Context, string, string) ([]byte, string, string, error) {
	return []byte("pdf"), "a.pdf", "application/pdf", f.err
}
func (f *fakeAPI) DeleteDocument(_ context.Context, _, _ string, hard bool) error {
	f.hardSeen = hard
	return f.err
}
func (f *fakeAPI) Search(context.Context, string, string, int) ([]paperlessclient.SearchHit, error) {
	return []paperlessclient.SearchHit{{ID: "d-1", Snippet: "…x…", Rank: 0.5}}, f.err
}

func TestAdapter(t *testing.T) {
	ctx := context.Background()
	f := &fakeAPI{}
	p := NewWithAPI(f)
	if id, err := p.EnsureCategory(ctx, "t", []string{"Assets"}); err != nil || id != "cat-1" {
		t.Fatal(id, err)
	}
	if id, err := p.Create(ctx, "t", documents.PaperDocument{Name: "a", Tags: map[string]string{"k": "v"}, Content: []byte("x")}); err != nil || id != "d-1" || f.created.Tags["k"] != "v" {
		t.Fatal(id, err, f.created)
	}
	if b, err := p.Download(ctx, "t", "d-1"); err != nil || string(b) != "pdf" {
		t.Fatal(string(b), err)
	}
	if err := p.Delete(ctx, "t", "d-1"); err != nil || !f.hardSeen {
		t.Fatal("delete must be hard", err)
	}
	if hits, err := p.Search(ctx, "t", "x", 5); err != nil || len(hits) != 1 || hits[0].Snippet != "…x…" {
		t.Fatal(hits, err)
	}
	// Error mapping.
	f.err = paperlessclient.ErrNotFound
	if _, err := p.Download(ctx, "t", "x"); !errors.Is(err, documents.ErrPaperNotFound) {
		t.Errorf("not found %v", err)
	}
	f.err = paperlessclient.ErrUnavailable
	if err := p.Delete(ctx, "t", "x"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unavailable %v", err)
	}
	f.err = paperlessclient.ErrForbidden
	if _, err := p.Search(ctx, "t", "x", 5); !errors.Is(err, paperlessclient.ErrForbidden) {
		t.Errorf("forbidden %v", err)
	}
	if _, err := p.Create(ctx, "t", documents.PaperDocument{}); err == nil {
		t.Error("create error swallowed")
	}
	if _, err := p.EnsureCategory(ctx, "t", nil); err == nil {
		t.Error("category error swallowed")
	}
}

func TestLazyDial(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("no route")
	p := New(func(context.Context) (grpc.ClientConnInterface, error) { return nil, boom })
	checks := []func() error{
		func() error { _, err := p.EnsureCategory(ctx, "t", nil); return err },
		func() error { _, err := p.Create(ctx, "t", documents.PaperDocument{}); return err },
		func() error { _, err := p.Download(ctx, "t", "x"); return err },
		func() error { return p.Delete(ctx, "t", "x") },
		func() error { _, err := p.Search(ctx, "t", "x", 1); return err },
	}
	for i, c := range checks {
		if err := c(); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%d: %v", i, err)
		}
	}
	// A successful dial builds the SDK client once.
	dials := 0
	var conn grpc.ClientConnInterface = (*grpc.ClientConn)(nil)
	q := New(func(context.Context) (grpc.ClientConnInterface, error) { dials++; return conn, nil })
	if _, err := q.client(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := q.client(ctx); err != nil || dials != 1 {
		t.Fatalf("dials %d %v", dials, err)
	}
}
