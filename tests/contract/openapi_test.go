// Package contract verifies the asset OpenAPI document against the mounted
// HTTP surface and the gateway manifest: the document parses and validates,
// every declared route carries a permission (or is public), and every declared
// route is implemented once the handlers are registered.
package contract

import (
	"testing"

	"github.com/go-freya/freya/internal/testrt"
	"github.com/go-freya/freya/internal/testutil"

	"github.com/go-freya/freya/services/asset/internal/assets"
	"github.com/go-freya/freya/services/asset/internal/backup"
	"github.com/go-freya/freya/services/asset/internal/blob"
	"github.com/go-freya/freya/services/asset/internal/categories"
	"github.com/go-freya/freya/services/asset/internal/config"
	"github.com/go-freya/freya/services/asset/internal/consumables"
	"github.com/go-freya/freya/services/asset/internal/documents"
	"github.com/go-freya/freya/services/asset/internal/httpapi"
	"github.com/go-freya/freya/services/asset/internal/insurance"
	"github.com/go-freya/freya/services/asset/internal/licenses"
	"github.com/go-freya/freya/services/asset/internal/locations"
	"github.com/go-freya/freya/services/asset/internal/memstore"
	"github.com/go-freya/freya/services/asset/internal/sealed"
	"github.com/go-freya/freya/services/asset/internal/stats"
	"github.com/go-freya/freya/services/asset/internal/stream"
	"github.com/go-freya/freya/services/asset/internal/suppliers"
	"github.com/go-freya/freya/services/asset/pkg/assetmanifest"
)

func TestOpenAPIDocumentAndRoutes(t *testing.T) {
	doc, err := httpapi.LoadDocument()
	if err != nil {
		t.Fatalf("document: %v", err)
	}
	routes, err := assetmanifest.Routes(doc)
	if err != nil {
		t.Fatalf("manifest routes: %v", err)
	}
	declared := httpapi.DeclaredRoutes(doc)
	if len(routes) != len(declared) {
		t.Fatalf("manifest routes %d != declared %d", len(routes), len(declared))
	}
	public := httpapi.PublicRoutes(doc)
	for _, r := range routes {
		if !r.Public && r.Permission == "" {
			t.Fatalf("%s %s: no permission", r.Method, r.Path)
		}
		if r.Public != public[httpapi.Route{Method: r.Method, Path: r.Path}] {
			t.Fatalf("%s %s: public flag mismatch", r.Method, r.Path)
		}
		if r.Path == "/api/asset/v1/stream" && r.Timeout != 0 {
			t.Fatal("/stream must not declare an oversized timeout")
		}
	}
	if len(public) != 1 {
		t.Fatalf("exactly /health is public: %v", public)
	}
	m, err := assetmanifest.Manifest()
	if err != nil || m.Module != "asset" || len(m.Nav) != 9 {
		t.Fatalf("%+v %v", m, err)
	}

	// Every declared route is implemented after Register.
	mem := memstore.New()
	rt := testrt.New(t, testutil.MustCA("example.org"), "asset")
	env, _ := sealed.NewEnvelope(make([]byte, 32))
	s, err := httpapi.NewHandler(rt)
	if err != nil {
		t.Fatal(err)
	}
	hub := stream.NewHub(stream.NewMemory(), stream.Config{}, nil)
	defer hub.Close()
	b := blob.NewFake()
	s.Register(httpapi.Deps{
		Assets: assets.New(mem, nil, nil, nil), Categories: categories.New(mem, nil), Suppliers: suppliers.New(mem, env, nil), Locations: locations.New(mem, env, nil),
		Consumables: consumables.New(mem, nil), Licenses: licenses.New(mem, nil), Insurance: insurance.New(mem, nil), Documents: documents.New(mem, b, nil, 0, 0),
		Stats: stats.New(mem, config.SoonWindows{}), Backup: backup.New(mem, nil), Hub: hub,
	})
	if missing := s.Missing(); len(missing) != 0 {
		t.Fatalf("declared but unimplemented: %v", missing)
	}
}
