// Package paper adapts the paperless module's SDK client to the documents
// service (feature 030). The connection is dialled lazily so the asset
// module starts, and serves everything else, while paperless is down.
package paper

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-tangra/go-tangra-paperless/sdk/v4/pkg/paperlessclient"
	"google.golang.org/grpc"

	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
)

// ErrUnavailable wraps paperless transport failures.
var ErrUnavailable = errors.New("paper: paperless unavailable")

// API is the part of the SDK client the adapter uses.
type API interface {
	EnsureCategory(ctx context.Context, tenantID string, path []string) (string, error)
	CreateDocument(ctx context.Context, tenantID string, in paperlessclient.CreateInput) (paperlessclient.Document, error)
	DownloadDocument(ctx context.Context, tenantID, id string) ([]byte, string, string, error)
	DeleteDocument(ctx context.Context, tenantID, id string, hard bool) error
	Search(ctx context.Context, tenantID, query string, limit int) ([]paperlessclient.SearchHit, error)
}

// Paper implements documents.Paper.
type Paper struct {
	dial func(ctx context.Context) (grpc.ClientConnInterface, error)

	mu  sync.Mutex
	api API
}

// New returns an adapter dialling paperless with dial on first use.
func New(dial func(ctx context.Context) (grpc.ClientConnInterface, error)) *Paper {
	return &Paper{dial: dial}
}

// NewWithAPI returns an adapter over an existing client (tests).
func NewWithAPI(api API) *Paper { return &Paper{api: api} }

func (p *Paper) client(ctx context.Context) (API, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.api == nil {
		conn, err := p.dial(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		p.api = paperlessclient.New(conn)
	}
	return p.api, nil
}

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, paperlessclient.ErrNotFound):
		return documents.ErrPaperNotFound
	case errors.Is(err, paperlessclient.ErrUnavailable):
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return err
}

// EnsureCategory implements documents.Paper.
func (p *Paper) EnsureCategory(ctx context.Context, tenantID string, path []string) (string, error) {
	c, err := p.client(ctx)
	if err != nil {
		return "", err
	}
	id, err := c.EnsureCategory(ctx, tenantID, path)
	return id, mapErr(err)
}

// Create implements documents.Paper.
func (p *Paper) Create(ctx context.Context, tenantID string, d documents.PaperDocument) (string, error) {
	c, err := p.client(ctx)
	if err != nil {
		return "", err
	}
	doc, err := c.CreateDocument(ctx, tenantID, paperlessclient.CreateInput{CategoryID: d.CategoryID, Name: d.Name, Description: d.Description,
		FileName: d.FileName, MimeType: d.MimeType, Tags: d.Tags, Content: d.Content})
	if err != nil {
		return "", mapErr(err)
	}
	return doc.ID, nil
}

// Download implements documents.Paper.
func (p *Paper) Download(ctx context.Context, tenantID, id string) ([]byte, error) {
	c, err := p.client(ctx)
	if err != nil {
		return nil, err
	}
	b, _, _, err := c.DownloadDocument(ctx, tenantID, id)
	return b, mapErr(err)
}

// Delete implements documents.Paper (hard delete: the stored file goes too).
func (p *Paper) Delete(ctx context.Context, tenantID, id string) error {
	c, err := p.client(ctx)
	if err != nil {
		return err
	}
	return mapErr(c.DeleteDocument(ctx, tenantID, id, true))
}

// Search implements documents.Paper.
func (p *Paper) Search(ctx context.Context, tenantID, query string, limit int) ([]documents.PaperHit, error) {
	c, err := p.client(ctx)
	if err != nil {
		return nil, err
	}
	hits, err := c.Search(ctx, tenantID, query, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]documents.PaperHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, documents.PaperHit{ID: h.ID, Snippet: h.Snippet, Rank: h.Rank})
	}
	return out, nil
}

var _ documents.Paper = (*Paper)(nil)
