package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-auth/sdk/v4/pkg/authclient"
	"github.com/go-tangra/go-tangra/v4/freyatest/testrt"
	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"

	"github.com/go-tangra/go-tangra-asset/v4/internal/assets"
	"github.com/go-tangra/go-tangra-asset/v4/internal/backup"
	"github.com/go-tangra/go-tangra-asset/v4/internal/blob"
	"github.com/go-tangra/go-tangra-asset/v4/internal/categories"
	"github.com/go-tangra/go-tangra-asset/v4/internal/config"
	"github.com/go-tangra/go-tangra-asset/v4/internal/consumables"
	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
	"github.com/go-tangra/go-tangra-asset/v4/internal/insurance"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invclient"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invsync"
	"github.com/go-tangra/go-tangra-asset/v4/internal/licenses"
	"github.com/go-tangra/go-tangra-asset/v4/internal/locations"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/sealed"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stats"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stream"
	"github.com/go-tangra/go-tangra-asset/v4/internal/suppliers"
	"github.com/go-tangra/go-tangra-asset/v4/internal/userdir"
)

const (
	apiTenant = "11111111-1111-7111-8111-111111111111"
	apiOther  = "22222222-2222-7222-8222-222222222222"
	apiAdmin  = "33333333-3333-7333-8333-333333333333"
	apiMember = "44444444-4444-7444-8444-444444444444"
	apiUser1  = "55555555-5555-7555-8555-555555555555"
	p         = "/api/asset/v1"
)

type fakeVerifier struct {
	ids map[string]authclient.Identity
}

func (f fakeVerifier) Verify(_ context.Context, token string) (authclient.Identity, error) {
	if id, ok := f.ids[token]; ok {
		return id, nil
	}
	return authclient.Identity{}, ErrUnauthenticated
}

type apiFixture struct {
	s    *Server
	mem  *memstore.Mem
	blob *blob.Fake
	inv  *invclient.Fake
	dir  *userdir.Fake
	hub  *stream.Hub
}

func newAPI(t *testing.T, withHub bool) *apiFixture {
	t.Helper()
	mem := memstore.New()
	rt := testrt.New(t, testutil.MustCA("example.org"), "asset")
	env, err := sealed.NewEnvelope(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	b := blob.NewFake()
	inv := invclient.NewFake()
	dir := userdir.NewFake()
	dir.Add(apiTenant, userdir.User{ID: apiUser1, DisplayName: "Ann Example"})
	dir.Add(apiTenant, userdir.User{ID: apiMember, DisplayName: "Bob Member"})
	v := fakeVerifier{ids: map[string]authclient.Identity{
		"admin":    {UserID: apiAdmin, TenantID: apiTenant, Roles: []string{"admin"}},
		"member":   {UserID: apiMember, TenantID: apiTenant, Roles: []string{"member"}},
		"other":    {UserID: apiAdmin, TenantID: apiOther, Roles: []string{"admin"}},
		"notenant": {UserID: apiAdmin},
	}}
	s, err := NewHandler(rt, WithVerifier(v))
	if err != nil {
		t.Fatal(err)
	}
	var hub *stream.Hub
	if withHub {
		hub = stream.NewHub(stream.NewMemory(), stream.Config{}, nil)
		t.Cleanup(hub.Close)
	}
	assetsSvc := assets.New(mem, dir, nil, nil)
	assetsSvc.SetBlobStore(b)
	docs := documents.New(mem, b, nil, 4096, time.Minute)
	f := &apiFixture{s: s, mem: mem, blob: b, inv: inv, dir: dir, hub: hub}
	s.Register(Deps{
		Assets: assetsSvc, Categories: categories.New(mem, nil), Suppliers: suppliers.New(mem, env, nil), Locations: locations.New(mem, env, nil),
		Consumables: consumables.New(mem, nil), Licenses: licenses.New(mem, nil), Insurance: insurance.New(mem, nil), Documents: docs,
		Sync: invsync.New(mem, inv, nil), Stats: stats.New(mem, config.SoonWindows{}), Backup: backup.New(mem, nil), Users: dir, Hub: hub,
		Health: func() map[string]string { return map[string]string{"store": "ok"} },
	})
	return f
}

func (f *apiFixture) req(t *testing.T, method, path, tok, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://localhost"+path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	if method != "GET" {
		r.Header.Set("X-CSRF-Token", "t")
	}
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, r)
	return w
}

func (f *apiFixture) upload(t *testing.T, path, tok, field, filename, mime, content string, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if field != "" {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, filename)}
		h["Content-Type"] = []string{mime}
		pw, _ := mw.CreatePart(h)
		_, _ = pw.Write([]byte(content))
	}
	for k, v := range extra {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	r := httptest.NewRequest("POST", "https://localhost"+path, &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+tok)
	r.Header.Set("X-CSRF-Token", "t")
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

func want(t *testing.T, w *httptest.ResponseRecorder, code int) map[string]any {
	t.Helper()
	if w.Code != code {
		t.Fatalf("want %d, got %d (%s)", code, w.Code, w.Body.String())
	}
	if w.Code == 204 {
		return nil
	}
	return decodeBody(t, w)
}

func items(m map[string]any) []any {
	v, _ := m["items"].([]any)
	return v
}

func TestEveryRouteImplementedAndHealth(t *testing.T) {
	f := newAPI(t, true)
	if missing := f.s.Missing(); len(missing) != 0 {
		t.Fatalf("routes declared but not implemented: %v", missing)
	}
	w := want(t, f.req(t, "GET", p+"/health", "", ""), 200)
	if w["status"] != "ok" {
		t.Fatal(w)
	}
	if w := f.req(t, "GET", p+"/assets", "", ""); w.Code != 401 {
		t.Fatalf("no token: %d", w.Code)
	}
	if w := f.req(t, "GET", p+"/assets", "bogus", ""); w.Code != 401 {
		t.Fatalf("bad token: %d", w.Code)
	}
	if w := f.req(t, "GET", p+"/assets", "notenant", ""); w.Code != 401 {
		t.Fatalf("token without tenant: %d", w.Code)
	}
	if w := f.req(t, "GET", p+"/nope", "admin", ""); w.Code != 404 {
		t.Fatalf("unknown: %d", w.Code)
	}
	// A hub-less server leaves /stream unimplemented.
	g := newAPI(t, false)
	if w := g.req(t, "GET", p+"/stream", "admin", ""); w.Code != 501 {
		t.Fatalf("stream without hub: %d", w.Code)
	}
}

func TestAssetsLifecycle(t *testing.T) {
	f := newAPI(t, false)
	// Create with auto tag; duplicate explicit tag → 409; validation → 422.
	c := want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"Laptop","purchase_cost":1000,"useful_life_years":5,"purchase_date":"2024-01-01T00:00:00Z","warranty_months":24}`), 201)
	id := c["id"].(string)
	tag := c["asset_tag"].(string)
	if !strings.HasPrefix(tag, "AST-") || c["status"] != "deployable" {
		t.Fatalf("%v", c)
	}
	want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"Dup","asset_tag":"`+tag+`"}`), 409)
	want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":""}`), 422)
	want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"x","unknown":1}`), 400)
	want(t, f.req(t, "POST", p+"/assets", "admin", `not json`), 400)
	// Read includes computed book value.
	g := want(t, f.req(t, "GET", p+"/assets/"+id, "admin", ""), 200)
	if bv, _ := g["book_value"].(float64); bv <= 0 || bv >= 1000 {
		t.Fatalf("book value: %v", g["book_value"])
	}
	want(t, f.req(t, "GET", p+"/assets/missing", "admin", ""), 404)
	want(t, f.req(t, "GET", p+"/assets/"+id, "other", ""), 404) // tenant isolation
	l := want(t, f.req(t, "GET", p+"/assets?query=lap&limit=10", "admin", ""), 200)
	if len(items(l)) != 1 {
		t.Fatal(l)
	}
	if lo := want(t, f.req(t, "GET", p+"/assets", "other", ""), 200); len(items(lo)) != 0 {
		t.Fatal("cross-tenant list")
	}
	want(t, f.req(t, "GET", p+"/assets?limit=9999", "admin", ""), 422) // OpenAPI bound
	// Update.
	u := want(t, f.req(t, "PUT", p+"/assets/"+id, "admin", `{"name":"Laptop 2","status":"broken"}`), 200)
	if u["name"] != "Laptop 2" || u["status"] != "broken" || u["asset_tag"] != tag {
		t.Fatal(u)
	}
	want(t, f.req(t, "PUT", p+"/assets/"+id, "admin", `{"name":"x","status":"assigned"}`), 422)
	want(t, f.req(t, "PUT", p+"/assets/missing", "admin", `{"name":"x"}`), 404)
	want(t, f.req(t, "PUT", p+"/assets/"+id, "admin", `bad`), 400)
	want(t, f.req(t, "PUT", p+"/assets/"+id, "admin", `{"name":"Laptop 2","status":"deployable"}`), 200)
	// Assign: status gate, unknown user, member may assign; history.
	want(t, f.req(t, "POST", p+"/assets/"+id+"/assign", "admin", `{"user_id":"nope"}`), 422)
	want(t, f.req(t, "POST", p+"/assets/"+id+"/assign", "admin", `bad`), 400)
	a := want(t, f.req(t, "POST", p+"/assets/"+id+"/assign", "member", `{"user_id":"`+apiUser1+`","notes":"onboarding"}`), 200)
	if a["status"] != "assigned" || a["assignee_name"] != "Ann Example" || a["user_id"] != apiUser1 {
		t.Fatal(a)
	}
	want(t, f.req(t, "POST", p+"/assets/"+id+"/assign", "admin", `{"user_id":"`+apiUser1+`"}`), 409)
	want(t, f.req(t, "POST", p+"/assets/missing/assign", "admin", `{"user_id":"`+apiUser1+`"}`), 404)
	want(t, f.req(t, "POST", p+"/assets/"+id+"/unassign", "admin", `bad`), 400)
	un := want(t, f.req(t, "POST", p+"/assets/"+id+"/unassign", "admin", `{"notes":"back"}`), 200)
	if un["status"] != "deployable" || un["user_id"] != nil {
		t.Fatal(un)
	}
	want(t, f.req(t, "POST", p+"/assets/"+id+"/unassign", "admin", ""), 409)
	h := want(t, f.req(t, "GET", p+"/assets/"+id+"/assignments?limit=10", "admin", ""), 200)
	rows := items(h)
	if len(rows) != 2 || rows[0].(map[string]any)["action"] != "unassigned" || rows[1].(map[string]any)["assigned_by"] != apiMember {
		t.Fatalf("history: %v", rows)
	}
	want(t, f.req(t, "GET", p+"/assets/missing/assignments", "admin", ""), 404)
	// Users directory.
	us := want(t, f.req(t, "GET", p+"/users?query=ann", "admin", ""), 200)
	if len(items(us)) != 1 {
		t.Fatal(us)
	}
	if us := want(t, f.req(t, "GET", p+"/users", "admin", ""), 200); len(items(us)) != 2 {
		t.Fatal(us)
	}
	f.dir.Err = fmt.Errorf("auth down")
	want(t, f.req(t, "GET", p+"/users", "admin", ""), 503)
	f.dir.Err = nil
	// Delete.
	want(t, f.req(t, "DELETE", p+"/assets/"+id, "other", ""), 404)
	want(t, f.req(t, "DELETE", p+"/assets/"+id, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/assets/"+id, "admin", ""), 404)
	// Store failure → 500 without detail.
	f.mem.FailNext("ListAssets")
	e := want(t, f.req(t, "GET", p+"/assets", "admin", ""), 500)
	if e["reason"] != "internal" || len(e) != 1 {
		t.Fatal(e)
	}
}

func TestOrgRecords(t *testing.T) {
	f := newAPI(t, false)
	// Categories tree + unique + guards.
	root := want(t, f.req(t, "POST", p+"/categories", "admin", `{"name":"Hardware"}`), 201)
	rootID := root["id"].(string)
	child := want(t, f.req(t, "POST", p+"/categories", "admin", `{"name":"Laptops","parent_id":"`+rootID+`"}`), 201)
	childID := child["id"].(string)
	want(t, f.req(t, "POST", p+"/categories", "admin", `{"name":"Laptops","parent_id":"`+rootID+`"}`), 409)
	want(t, f.req(t, "POST", p+"/categories", "admin", `{"name":""}`), 422)
	want(t, f.req(t, "POST", p+"/categories", "admin", `x`), 400)
	tree := want(t, f.req(t, "GET", p+"/categories/tree", "member", ""), 200)
	roots := items(tree)
	if len(roots) != 1 || len(roots[0].(map[string]any)["children"].([]any)) != 1 {
		t.Fatalf("tree: %v", tree)
	}
	if l := want(t, f.req(t, "GET", p+"/categories", "member", ""), 200); len(items(l)) != 2 {
		t.Fatal(l)
	}
	want(t, f.req(t, "GET", p+"/categories/"+childID, "member", ""), 200)
	want(t, f.req(t, "GET", p+"/categories/missing", "member", ""), 404)
	want(t, f.req(t, "PUT", p+"/categories/"+childID, "admin", `{"name":"Notebooks","parent_id":"`+rootID+`"}`), 200)
	want(t, f.req(t, "PUT", p+"/categories/"+childID, "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/categories/"+rootID, "admin", `{"name":"HW","parent_id":"`+childID+`"}`), 422) // cycle
	want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"L","category_id":"`+childID+`"}`), 201)
	want(t, f.req(t, "DELETE", p+"/categories/"+rootID, "admin", ""), 409)  // children
	want(t, f.req(t, "DELETE", p+"/categories/"+childID, "admin", ""), 409) // assets
	want(t, f.req(t, "DELETE", p+"/categories/missing", "admin", ""), 404)

	// Suppliers: contact PII redacted for non-admins.
	sp := want(t, f.req(t, "POST", p+"/suppliers", "admin", `{"name":"Acme","email":"ann@acme.example","telephone":"+1","contact_person":"Ann"}`), 201)
	spID := sp["id"].(string)
	if sp["email"] != "ann@acme.example" {
		t.Fatal(sp)
	}
	want(t, f.req(t, "POST", p+"/suppliers", "admin", `{"name":"Acme"}`), 409)
	want(t, f.req(t, "POST", p+"/suppliers", "admin", `{"name":""}`), 422)
	want(t, f.req(t, "POST", p+"/suppliers", "admin", `x`), 400)
	m := want(t, f.req(t, "GET", p+"/suppliers/"+spID, "member", ""), 200)
	if _, has := m["email"]; has || m["telephone"] != nil || m["contact_person"] != nil {
		t.Fatalf("member must not see contact PII: %v", m)
	}
	body := f.req(t, "GET", p+"/suppliers?query=ac", "member", "").Body.String()
	if strings.Contains(body, "acme.example") || strings.Contains(body, "+1") {
		t.Fatalf("list leaks PII: %s", body)
	}
	want(t, f.req(t, "GET", p+"/suppliers/missing", "admin", ""), 404)
	want(t, f.req(t, "PUT", p+"/suppliers/"+spID, "admin", `{"name":"Acme Inc","status":"inactive"}`), 200)
	want(t, f.req(t, "PUT", p+"/suppliers/"+spID, "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/suppliers/"+spID, "admin", `{"name":""}`), 422)
	want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"S","supplier_id":"`+spID+`"}`), 201)
	want(t, f.req(t, "DELETE", p+"/suppliers/"+spID, "admin", ""), 409)
	want(t, f.req(t, "DELETE", p+"/suppliers/missing", "admin", ""), 404)

	// Locations tree + path + PII.
	hq := want(t, f.req(t, "POST", p+"/locations", "admin", `{"name":"HQ","email":"hq@x.example"}`), 201)
	hqID := hq["id"].(string)
	fl := want(t, f.req(t, "POST", p+"/locations", "admin", `{"name":"Floor 1","parent_id":"`+hqID+`"}`), 201)
	flID := fl["id"].(string)
	if fl["path"] != "HQ / Floor 1" {
		t.Fatal(fl)
	}
	want(t, f.req(t, "POST", p+"/locations", "admin", `{"name":"HQ"}`), 409)
	want(t, f.req(t, "POST", p+"/locations", "admin", `{"name":""}`), 422)
	want(t, f.req(t, "POST", p+"/locations", "admin", `x`), 400)
	lt := want(t, f.req(t, "GET", p+"/locations/tree", "member", ""), 200)
	if len(items(lt)) != 1 || strings.Contains(f.req(t, "GET", p+"/locations/tree", "member", "").Body.String(), "hq@x.example") {
		t.Fatal(lt)
	}
	if l := want(t, f.req(t, "GET", p+"/locations", "member", ""), 200); len(items(l)) != 2 {
		t.Fatal(l)
	}
	want(t, f.req(t, "GET", p+"/locations/"+hqID, "member", ""), 200)
	want(t, f.req(t, "GET", p+"/locations/missing", "member", ""), 404)
	want(t, f.req(t, "PUT", p+"/locations/"+flID, "admin", `{"name":"Floor One","parent_id":"`+hqID+`"}`), 200)
	want(t, f.req(t, "PUT", p+"/locations/"+flID, "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/locations/"+hqID, "admin", `{"name":"HQ","parent_id":"`+flID+`"}`), 422)
	want(t, f.req(t, "DELETE", p+"/locations/"+hqID, "admin", ""), 409)
	want(t, f.req(t, "DELETE", p+"/locations/"+flID, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/locations/missing", "admin", ""), 404)
	// Unassign into a location.
	as := want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"U"}`), 201)
	want(t, f.req(t, "POST", p+"/assets/"+as["id"].(string)+"/assign", "admin", `{"user_id":"`+apiUser1+`"}`), 200)
	un := want(t, f.req(t, "POST", p+"/assets/"+as["id"].(string)+"/unassign", "admin", `{"location_id":"`+hqID+`"}`), 200)
	if un["location_id"] != hqID {
		t.Fatal(un)
	}
	want(t, f.req(t, "POST", p+"/assets/"+as["id"].(string)+"/unassign", "admin", `{"location_id":"nope"}`), 409) // not assigned anymore → 409 first
}

func TestDocumentsAndPhoto(t *testing.T) {
	f := newAPI(t, false)
	a := want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"L"}`), 201)
	id := a["id"].(string)
	c := want(t, f.req(t, "POST", p+"/consumables", "admin", `{"name":"Toner","amount":3,"min_amount":1}`), 201)
	cid := c["id"].(string)
	lic := want(t, f.req(t, "POST", p+"/licenses", "admin", `{"name":"Office"}`), 201)
	lid := lic["id"].(string)

	// Photo.
	want(t, f.req(t, "GET", p+"/assets/"+id+"/photo", "admin", ""), 404)
	ph := want(t, f.upload(t, p+"/assets/"+id+"/photo", "admin", "file", "p.png", "image/png", "PNGBYTES", nil), 200)
	if ph["has_photo"] != nil { // UploadPhoto returns the store asset, not the view
		t.Log("photo view", ph)
	}
	w := f.req(t, "GET", p+"/assets/"+id+"/photo", "member", "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.String() != "PNGBYTES" {
		t.Fatalf("photo: %d %s %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	if g := want(t, f.req(t, "GET", p+"/assets/"+id, "admin", ""), 200); g["has_photo"] != true {
		t.Fatal(g)
	}
	want(t, f.upload(t, p+"/assets/"+id+"/photo", "admin", "file", "x.exe", "application/x-msdownload", "MZ", nil), 415)
	want(t, f.upload(t, p+"/assets/"+id+"/photo", "admin", "", "", "", "", map[string]string{"x": "y"}), 400)
	want(t, f.upload(t, p+"/assets/"+id+"/photo", "admin", "file", "big.png", "image/png", strings.Repeat("x", 5000), nil), 413)
	want(t, f.upload(t, p+"/assets/missing/photo", "admin", "file", "p.png", "image/png", "x", nil), 404)
	want(t, f.req(t, "DELETE", p+"/assets/"+id+"/photo", "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/assets/"+id+"/photo", "admin", ""), 404)
	// Not multipart at all.
	r := httptest.NewRequest("POST", "https://localhost"+p+"/assets/"+id+"/photo", strings.NewReader("raw"))
	r.Header.Set("Authorization", "Bearer admin")
	r.Header.Set("X-CSRF-Token", "t")
	r.Header.Set("Content-Type", "text/plain")
	rw := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(rw, r)
	if rw.Code != 400 {
		t.Fatalf("non-multipart: %d", rw.Code)
	}

	// Documents on each entity type; isolation per entity; download exact bytes.
	d := want(t, f.upload(t, p+"/assets/"+id+"/documents", "admin", "file", "invoice.pdf", "application/pdf", "%PDF-1", map[string]string{"description": "inv"}), 201)
	did := d["id"].(string)
	if !strings.HasPrefix(d["storage_key"].(string), "tenants/"+apiTenant+"/") || d["checksum"] == "" {
		t.Fatal(d)
	}
	cd := want(t, f.upload(t, p+"/consumables/"+cid+"/documents", "admin", "file", "sds.txt", "text/plain", "SDS", nil), 201)
	ld := want(t, f.upload(t, p+"/licenses/"+lid+"/documents", "admin", "file", "key.txt", "text/plain", "KEY", nil), 201)
	if len(items(want(t, f.req(t, "GET", p+"/assets/"+id+"/documents", "member", ""), 200))) != 1 {
		t.Fatal("asset docs")
	}
	if len(items(want(t, f.req(t, "GET", p+"/consumables/"+cid+"/documents", "member", ""), 200))) != 1 {
		t.Fatal("consumable docs")
	}
	if len(items(want(t, f.req(t, "GET", p+"/licenses/"+lid+"/documents", "member", ""), 200))) != 1 {
		t.Fatal("license docs")
	}
	dw := f.req(t, "GET", p+"/assets/"+id+"/documents/"+did+"/download", "member", "")
	if dw.Code != 200 || dw.Body.String() != "%PDF-1" || !strings.Contains(dw.Header().Get("Content-Disposition"), "invoice.pdf") || dw.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("download: %d %q %s", dw.Code, dw.Body.String(), dw.Header().Get("Content-Disposition"))
	}
	if strings.Contains(dw.Header().Get("Content-Disposition"), `"`+"\n") {
		t.Fatal("header injection")
	}
	want(t, f.req(t, "GET", p+"/consumables/"+cid+"/documents/"+did+"/download", "admin", ""), 404) // bound to entity
	want(t, f.req(t, "GET", p+"/assets/"+id+"/documents/"+did+"/download", "other", ""), 404)
	want(t, f.req(t, "GET", p+"/assets/missing/documents", "admin", ""), 404)
	want(t, f.upload(t, p+"/assets/"+id+"/documents", "admin", "", "", "", "", nil), 400)
	want(t, f.upload(t, p+"/assets/"+id+"/documents", "admin", "file", "big.bin", "application/octet-stream", strings.Repeat("x", 5000), nil), 413)
	// No credentials anywhere in the responses.
	for _, path := range []string{p + "/assets/" + id, p + "/assets/" + id + "/documents"} {
		b := f.req(t, "GET", path, "admin", "").Body.String()
		if strings.Contains(strings.ToLower(b), "secret") || strings.Contains(strings.ToLower(b), "access_key") {
			t.Fatalf("credential-looking field in %s: %s", path, b)
		}
	}
	want(t, f.req(t, "DELETE", p+"/assets/"+id+"/documents/"+did, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/assets/"+id+"/documents/"+did, "admin", ""), 404)
	// Deleting the consumable/license purges their documents (rows + objects).
	before := f.blob.Len()
	want(t, f.req(t, "DELETE", p+"/consumables/"+cid, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/licenses/"+lid, "admin", ""), 204)
	if f.blob.Len() != before-2 {
		t.Fatalf("objects not purged: %d → %d", before, f.blob.Len())
	}
	_, _ = cd, ld
	// Deleting the asset removes its photo/document objects.
	want(t, f.upload(t, p+"/assets/"+id+"/photo", "admin", "file", "p.png", "image/png", "P", nil), 200)
	want(t, f.upload(t, p+"/assets/"+id+"/documents", "admin", "file", "a.txt", "text/plain", "A", nil), 201)
	want(t, f.req(t, "DELETE", p+"/assets/"+id, "admin", ""), 204)
	if f.blob.Len() != 0 {
		t.Fatalf("asset objects not removed: %d", f.blob.Len())
	}
}

func TestInventoriesAndInsurance(t *testing.T) {
	f := newAPI(t, false)
	// Consumables.
	c := want(t, f.req(t, "POST", p+"/consumables", "admin", `{"name":"Toner","amount":5,"min_amount":2}`), 201)
	cid := c["id"].(string)
	if c["low_stock"] != false {
		t.Fatal(c)
	}
	want(t, f.req(t, "POST", p+"/consumables", "admin", `{"name":""}`), 422)
	want(t, f.req(t, "POST", p+"/consumables", "admin", `x`), 400)
	u := want(t, f.req(t, "PUT", p+"/consumables/"+cid, "admin", `{"name":"Toner","amount":2,"min_amount":2}`), 200)
	if u["low_stock"] != true {
		t.Fatal(u)
	}
	want(t, f.req(t, "PUT", p+"/consumables/"+cid, "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/consumables/missing", "admin", `{"name":"x"}`), 404)
	want(t, f.req(t, "GET", p+"/consumables/"+cid, "member", ""), 200)
	want(t, f.req(t, "GET", p+"/consumables/missing", "member", ""), 404)
	if l := want(t, f.req(t, "GET", p+"/consumables?query=ton", "member", ""), 200); len(items(l)) != 1 {
		t.Fatal(l)
	}
	// Licenses.
	lic := want(t, f.req(t, "POST", p+"/licenses", "admin", `{"name":"Office","valid_from":"2025-01-01T00:00:00Z","valid_to":"2026-01-01T00:00:00Z"}`), 201)
	lid := lic["id"].(string)
	want(t, f.req(t, "POST", p+"/licenses", "admin", `{"name":"x","valid_from":"2026-01-01T00:00:00Z","valid_to":"2025-01-01T00:00:00Z"}`), 422)
	want(t, f.req(t, "POST", p+"/licenses", "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/licenses/"+lid, "admin", `{"name":"Office 365","status":"suspended"}`), 200)
	want(t, f.req(t, "PUT", p+"/licenses/"+lid, "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/licenses/missing", "admin", `{"name":"x"}`), 404)
	want(t, f.req(t, "GET", p+"/licenses/"+lid, "member", ""), 200)
	want(t, f.req(t, "GET", p+"/licenses/missing", "member", ""), 404)
	if l := want(t, f.req(t, "GET", p+"/licenses", "member", ""), 200); len(items(l)) != 1 {
		t.Fatal(l)
	}
	want(t, f.req(t, "DELETE", p+"/licenses/missing", "admin", ""), 404)
	// Insurance + policy assets.
	pol := want(t, f.req(t, "POST", p+"/insurance-policies", "admin", `{"name":"Fleet","policy_number":"P-1","coverage_type":"all_risk"}`), 201)
	pid := pol["id"].(string)
	want(t, f.req(t, "POST", p+"/insurance-policies", "admin", `{"name":"Other","policy_number":"P-1"}`), 409)
	want(t, f.req(t, "POST", p+"/insurance-policies", "admin", `{"name":"x"}`), 422)
	want(t, f.req(t, "POST", p+"/insurance-policies", "admin", `x`), 400)
	a := want(t, f.req(t, "POST", p+"/assets", "admin", `{"name":"L"}`), 201)
	aid := a["id"].(string)
	pa := want(t, f.req(t, "POST", p+"/insurance-policies/"+pid+"/assets", "admin", `{"asset_id":"`+aid+`","covered_value":900}`), 201)
	if pa["asset_tag"] != a["asset_tag"] {
		t.Fatal(pa)
	}
	want(t, f.req(t, "POST", p+"/insurance-policies/"+pid+"/assets", "admin", `{"asset_id":"`+aid+`"}`), 409)
	want(t, f.req(t, "POST", p+"/insurance-policies/"+pid+"/assets", "admin", `{"asset_id":"nope"}`), 422)
	want(t, f.req(t, "POST", p+"/insurance-policies/"+pid+"/assets", "admin", `x`), 400)
	want(t, f.req(t, "POST", p+"/insurance-policies/missing/assets", "admin", `{"asset_id":"`+aid+`"}`), 404)
	if g := want(t, f.req(t, "GET", p+"/insurance-policies/"+pid, "member", ""), 200); g["asset_count"] != float64(1) {
		t.Fatal(g)
	}
	if l := want(t, f.req(t, "GET", p+"/insurance-policies/"+pid+"/assets", "member", ""), 200); len(items(l)) != 1 {
		t.Fatal(l)
	}
	want(t, f.req(t, "GET", p+"/insurance-policies/missing/assets", "member", ""), 404)
	want(t, f.req(t, "DELETE", p+"/insurance-policies/"+pid+"/assets/"+aid, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/insurance-policies/"+pid+"/assets/"+aid, "admin", ""), 404)
	want(t, f.req(t, "PUT", p+"/insurance-policies/"+pid, "admin", `{"name":"Fleet","policy_number":"P-1","status":"cancelled"}`), 200)
	want(t, f.req(t, "PUT", p+"/insurance-policies/"+pid, "admin", `x`), 400)
	want(t, f.req(t, "PUT", p+"/insurance-policies/missing", "admin", `{"name":"x","policy_number":"n"}`), 404)
	want(t, f.req(t, "GET", p+"/insurance-policies/missing", "member", ""), 404)
	if l := want(t, f.req(t, "GET", p+"/insurance-policies", "member", ""), 200); len(items(l)) != 1 {
		t.Fatal(l)
	}
	want(t, f.req(t, "DELETE", p+"/insurance-policies/"+pid, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/insurance-policies/"+pid, "admin", ""), 404)
	want(t, f.req(t, "DELETE", p+"/consumables/"+cid, "admin", ""), 204)
	want(t, f.req(t, "DELETE", p+"/consumables/"+cid, "admin", ""), 404)
}

func TestSyncStatsBackupStream(t *testing.T) {
	f := newAPI(t, true)
	f.inv.Set(apiTenant, []invclient.Host{{ID: "h1", Hostname: "pc-1", SystemSerial: "SN1", Model: "XPS", OSName: "Linux"}})
	pv := want(t, f.req(t, "POST", p+"/assets/inventory-sync/preview", "admin", ""), 200)
	if pv["create"] != float64(1) || pv["hosts"] != float64(1) {
		t.Fatal(pv)
	}
	if ov := want(t, f.req(t, "POST", p+"/assets/inventory-sync/preview", "other", ""), 200); ov["hosts"] != float64(0) {
		t.Fatal("tenant-scoped preview")
	}
	ex := want(t, f.req(t, "POST", p+"/assets/inventory-sync/execute", "admin", `{"hostnames":["pc-1"]}`), 200)
	if ex["created"] != float64(1) {
		t.Fatal(ex)
	}
	want(t, f.req(t, "POST", p+"/assets/inventory-sync/execute", "admin", `bad`), 400)
	if l := want(t, f.req(t, "GET", p+"/assets?query=pc-1", "admin", ""), 200); len(items(l)) != 1 {
		t.Fatal(l)
	}
	f.inv.Down = true
	want(t, f.req(t, "POST", p+"/assets/inventory-sync/preview", "admin", ""), 503)
	want(t, f.req(t, "POST", p+"/assets/inventory-sync/execute", "admin", ""), 503)
	f.inv.Down = false
	// No sync wired → 503.
	g := newAPI(t, false)
	gs := g.s
	gs.Register(Deps{Assets: assets.New(g.mem, nil, nil, nil), Categories: categories.New(g.mem, nil), Stats: stats.New(g.mem, config.SoonWindows{}), Backup: backup.New(g.mem, nil), Documents: documents.New(g.mem, g.blob, nil, 1, time.Minute), Suppliers: suppliers.New(g.mem, nil, nil), Locations: locations.New(g.mem, nil, nil), Consumables: consumables.New(g.mem, nil), Licenses: licenses.New(g.mem, nil), Insurance: insurance.New(g.mem, nil)})
	want(t, g.req(t, "POST", p+"/assets/inventory-sync/preview", "admin", ""), 503)
	want(t, g.req(t, "POST", p+"/assets/inventory-sync/execute", "admin", ""), 503)
	if us := want(t, g.req(t, "GET", p+"/users", "admin", ""), 200); len(items(us)) != 0 {
		t.Fatal("no directory → empty")
	}
	if h := want(t, g.req(t, "GET", p+"/health", "", ""), 200); h["status"] != "ok" {
		t.Fatal(h)
	}

	// Stats.
	st := want(t, f.req(t, "GET", p+"/stats", "member", ""), 200)
	if st["total_assets"] != float64(1) {
		t.Fatal(st)
	}
	f.mem.FailNext("TenantStats")
	want(t, f.req(t, "GET", p+"/stats", "member", ""), 500)

	// Backup export/import: shapes, non-admin full restore refused.
	exp := want(t, f.req(t, "POST", p+"/backup/export", "admin", ""), 200)
	if exp["schema_version"] != float64(backup.SchemaVersion) || len(exp["assets"].([]any)) != 1 {
		t.Fatal(exp)
	}
	want(t, f.req(t, "POST", p+"/backup/export", "admin", `bad`), 400)
	raw, _ := json.Marshal(exp)
	imp := want(t, f.req(t, "POST", p+"/backup/import", "admin", `{"mode":"skip","backup":`+string(raw)+`}`), 200)
	if imp["skipped"].(map[string]any)["assets"] != float64(1) {
		t.Fatal(imp)
	}
	want(t, f.req(t, "POST", p+"/backup/import", "member", `{"full":true,"backup":`+string(raw)+`}`), 403)
	want(t, f.req(t, "POST", p+"/backup/import", "member", `{"tenant_id":"`+apiOther+`","backup":`+string(raw)+`}`), 403)
	want(t, f.req(t, "POST", p+"/backup/import", "admin", `{"backup":{"schema_version":9}}`), 422)
	want(t, f.req(t, "POST", p+"/backup/import", "admin", `bad`), 400)
	// Health with a degraded component.
	f.s.Register(Deps{Assets: assets.New(f.mem, nil, nil, nil), Categories: categories.New(f.mem, nil), Stats: stats.New(f.mem, config.SoonWindows{}), Backup: backup.New(f.mem, nil), Documents: documents.New(f.mem, f.blob, nil, 1, time.Minute), Suppliers: suppliers.New(f.mem, nil, nil), Locations: locations.New(f.mem, nil, nil), Consumables: consumables.New(f.mem, nil), Licenses: licenses.New(f.mem, nil), Insurance: insurance.New(f.mem, nil),
		Health: func() map[string]string { return map[string]string{"store": "down"} }})
	if h := want(t, f.req(t, "GET", p+"/health", "", ""), 200); h["status"] != "degraded" {
		t.Fatal(h)
	}
}

func TestStreamSSE(t *testing.T) {
	f := newAPI(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("GET", "https://localhost"+p+"/stream", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer admin")
	pr, pw := io.Pipe()
	rec := &streamRecorder{ResponseRecorder: httptest.NewRecorder(), w: pw}
	done := make(chan struct{})
	go func() { f.s.Handler().ServeHTTP(rec, r); _ = pw.Close(); close(done) }()
	// Publish an event and expect it on the stream.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = f.hub.Publish(context.Background(), apiTenant, nil, true, "asset.assigned", map[string]any{"asset_id": "x"})
	}()
	buf := make([]byte, 4096)
	var got string
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(got, "asset.assigned") && time.Now().Before(deadline) {
		n, err := pr.Read(buf)
		if err != nil {
			break
		}
		got += string(buf[:n])
	}
	cancel()
	<-done
	if !strings.Contains(got, "asset.assigned") {
		t.Fatalf("stream did not carry the event: %q", got)
	}
	if w := f.req(t, "GET", p+"/stream", "", ""); w.Code != 401 {
		t.Fatalf("unauthenticated stream: %d", w.Code)
	}
}

// streamRecorder forwards writes to a pipe so the SSE loop can be observed.
type streamRecorder struct {
	*httptest.ResponseRecorder
	w io.Writer
}

func (s *streamRecorder) Write(b []byte) (int, error) { return s.w.Write(b) }
func (s *streamRecorder) Flush()                      {}

var _ http.Flusher = (*streamRecorder)(nil)

func TestHelpers(t *testing.T) {
	if sanitizeFilename("a\"b\\c\nd") != "a_b_c_d" || sanitizeFilename("") != "download" {
		t.Fatal("sanitize")
	}
	if atoiDefault("x", 7) != 7 || atoiDefault("", 3) != 3 || atoiDefault("5", 0) != 5 {
		t.Fatal("atoi")
	}
	if status, _ := Status(fmt.Errorf("x")); status != 503 {
		t.Fatal("status default")
	}
	rr := httptest.NewRecorder()
	Fail(rr, httptest.NewRequest("GET", "/", nil), nil, fmt.Errorf("boom"))
	if rr.Code != 503 {
		t.Fatal(rr.Code)
	}
	rr = httptest.NewRecorder()
	WriteDetail(rr, ErrValidation, map[string]any{"field": "name"})
	if rr.Code != 422 || !strings.Contains(rr.Body.String(), "field") {
		t.Fatal(rr.Body.String())
	}
	if jsonRaw([]byte(`{"a":1}`)) == nil || jsonRaw([]byte(`nope`)) != "nope" {
		t.Fatal("jsonRaw")
	}
}
