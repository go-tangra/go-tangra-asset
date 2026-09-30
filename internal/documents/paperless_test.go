package documents

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// fakePaper is an in-memory paperless.
type fakePaper struct {
	mu       sync.Mutex
	cats     map[string]string // joined path -> id
	docs     map[string]PaperDocument
	tenant   map[string]string // doc id -> tenant
	n        int
	fail     map[string]error
	searches []string
}

func newPaper() *fakePaper {
	return &fakePaper{cats: map[string]string{}, docs: map[string]PaperDocument{}, tenant: map[string]string{}, fail: map[string]error{}}
}

func (p *fakePaper) err(op string) error {
	e := p.fail[op]
	delete(p.fail, op)
	return e
}

func (p *fakePaper) EnsureCategory(_ context.Context, tenantID string, path []string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.err("category"); e != nil {
		return "", e
	}
	k := tenantID + "|" + strings.Join(path, "/")
	if id, ok := p.cats[k]; ok {
		return id, nil
	}
	p.n++
	p.cats[k] = "cat-" + strconv.Itoa(p.n)
	return p.cats[k], nil
}

func (p *fakePaper) Create(_ context.Context, tenantID string, d PaperDocument) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.err("create"); e != nil {
		return "", e
	}
	p.n++
	id := "pd-" + strconv.Itoa(p.n)
	p.docs[id], p.tenant[id] = d, tenantID
	return id, nil
}

func (p *fakePaper) Download(_ context.Context, tenantID, id string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.err("download"); e != nil {
		return nil, e
	}
	d, ok := p.docs[id]
	if !ok || p.tenant[id] != tenantID {
		return nil, ErrPaperNotFound
	}
	return d.Content, nil
}

func (p *fakePaper) Delete(_ context.Context, tenantID, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.err("delete"); e != nil {
		return e
	}
	if _, ok := p.docs[id]; !ok || p.tenant[id] != tenantID {
		return ErrPaperNotFound
	}
	delete(p.docs, id)
	return nil
}

func (p *fakePaper) Search(_ context.Context, tenantID, query string, limit int) ([]PaperHit, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.err("search"); e != nil {
		return nil, e
	}
	p.searches = append(p.searches, query)
	var out []PaperHit
	for id, d := range p.docs {
		if p.tenant[id] == tenantID && strings.Contains(strings.ToLower(string(d.Content)), strings.ToLower(query)) {
			out = append(out, PaperHit{ID: id, Snippet: "…" + query + "…", Rank: 1})
		}
	}
	return out, nil
}

func (p *fakePaper) catFor(tenantID string, path ...string) string {
	return p.cats[tenantID+"|"+strings.Join(path, "/")]
}

func TestUploadToPaperless(t *testing.T) {
	f := newFx(t)
	p := newPaper()
	f.svc.SetPaperless(p)
	d, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("invoice.pdf", "application/pdf", "Invoice for laptop T1"))
	if err != nil {
		t.Fatal(err)
	}
	if d.PaperlessDocumentID == "" || d.StorageKey != store.PaperlessKeyPrefix+d.PaperlessDocumentID || d.Checksum == "" || d.FileSize != 21 || f.blob.Len() != 0 {
		t.Fatalf("document %+v (blobs %d)", d, f.blob.Len())
	}
	pd := p.docs[d.PaperlessDocumentID]
	if pd.CategoryID != p.catFor(tenant, "Assets", "T1") || pd.Tags["asset_entity_id"] != "a1" || pd.Tags["asset_label"] != "T1" {
		t.Fatalf("paperless doc %+v cats %v", pd, p.cats)
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityConsumable, "c1", up("sds.pdf", "", "toner sheet")); err != nil || p.catFor(tenant, "Assets", "Consumables", "Toner") == "" {
		t.Fatalf("consumable %v %v", err, p.cats)
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityLicense, "l1", up("lic.txt", "text/plain", "license key terms")); err != nil || p.catFor(tenant, "Assets", "Licenses", "Office") == "" {
		t.Fatalf("license %v %v", err, p.cats)
	}
	rc, got, err := f.svc.Download(ctx(), subj, store.EntityAsset, "a1", d.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	if string(b) != "Invoice for laptop T1" || got.ID != d.ID {
		t.Fatalf("download %q", b)
	}
	if _, err := f.svc.DownloadURL(ctx(), subj, store.EntityAsset, "a1", d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("presign %v", err)
	}
	// Search finds asset documents by content, maps them back, drops others.
	p.docs["foreign"], p.tenant["foreign"] = PaperDocument{Content: []byte("laptop manual")}, tenant
	hits, err := f.svc.Search(ctx(), subj, "LAPTOP", 10)
	if err != nil || len(hits) != 1 || hits[0].Document.ID != d.ID || hits[0].Snippet == "" {
		t.Fatalf("search %+v %v", hits, err)
	}
	if hits, _ := f.svc.Search(ctx(), osubj, "laptop", 10); len(hits) != 0 {
		t.Fatal("search crossed tenants")
	}
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.docs[d.PaperlessDocumentID]; ok {
		t.Fatal("paperless document not deleted")
	}
	// Purge removes paperless documents too (consumable/license delete).
	docs, _ := f.svc.List(ctx(), subj, store.EntityConsumable, "c1")
	f.svc.PurgeRows(ctx(), tenant, docs)
	if left, _ := f.mem.ListDocuments(ctx(), tenant, store.EntityConsumable, "c1"); len(left) != 0 {
		t.Fatal("purge left rows")
	}
}

func TestPaperlessFailures(t *testing.T) {
	f := newFx(t)
	p := newPaper()
	f.svc.SetPaperless(p)
	boom := errors.New("paperless down")
	for _, op := range []string{"category", "create"} {
		p.fail[op] = boom
		if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("x.pdf", "application/pdf", "x")); !errors.Is(err, boom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", UploadInput{FileName: "big", Reader: strings.NewReader(strings.Repeat("x", 2000))}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("size %v", err)
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "missing", up("x.pdf", "", "x")); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing entity %v", err)
	}
	// A failed row insert removes the paperless copy.
	f.mem.FailNext("InsertDocument")
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("x.pdf", "", "x")); err == nil || len(p.docs) != 0 {
		t.Errorf("insert failure: %v docs %d", err, len(p.docs))
	}
	d, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("y.pdf", "", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	p.fail["download"] = boom
	if _, _, err := f.svc.Download(ctx(), subj, store.EntityAsset, "a1", d.ID); !errors.Is(err, boom) {
		t.Errorf("download %v", err)
	}
	delete(p.docs, d.PaperlessDocumentID)
	if _, _, err := f.svc.Download(ctx(), subj, store.EntityAsset, "a1", d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("gone %v", err)
	}
	// Deleting a document already gone from paperless still removes the row.
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d.ID); err != nil {
		t.Errorf("delete gone %v", err)
	}
	d2, _ := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("z.pdf", "", "z"))
	p.fail["delete"] = boom
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d2.ID); !errors.Is(err, boom) {
		t.Errorf("delete failure %v", err)
	}
	// Search validation and failures.
	for _, q := range []string{" ", strings.Repeat("q", 201)} {
		var ve ValidationError
		if _, err := f.svc.Search(ctx(), subj, q, 10); !errors.As(err, &ve) {
			t.Errorf("query %q: %v", q, err)
		}
	}
	p.fail["search"] = boom
	if _, err := f.svc.Search(ctx(), subj, "x", 0); !errors.Is(err, boom) {
		t.Errorf("search failure %v", err)
	}
	f.mem.FailNext("ListDocumentsByPaperlessIDs")
	if _, err := f.svc.Search(ctx(), subj, "z", 500); err == nil {
		t.Error("lookup failure swallowed")
	}
	if _, err := f.svc.Search(ctx(), authz.Subjects{UserID: "u", ActorKind: authz.ActorUser}, "z", 10); err == nil {
		t.Error("no tenant accepted")
	}
	// Without paperless: search unavailable, paperless-backed rows unreadable.
	plain := New(f.mem, f.blob, nil, 1024, 0)
	if _, err := plain.Search(ctx(), subj, "z", 10); !errors.Is(err, ErrSearchUnavailable) {
		t.Errorf("no paperless search %v", err)
	}
	if _, _, err := plain.Download(ctx(), subj, store.EntityAsset, "a1", d2.ID); !errors.Is(err, ErrSearchUnavailable) {
		t.Errorf("no paperless download %v", err)
	}
	if _, err := plain.Migrate(ctx(), 10); !errors.Is(err, ErrSearchUnavailable) {
		t.Errorf("no paperless migrate %v", err)
	}
}

func TestMigrateToPaperless(t *testing.T) {
	f := newFx(t)
	// Three documents in the object store (before paperless was attached).
	var ids []string
	for i, e := range []struct{ typ, id string }{{store.EntityAsset, "a1"}, {store.EntityConsumable, "c1"}, {store.EntityLicense, "l1"}} {
		d, err := f.svc.Upload(ctx(), subj, e.typ, e.id, up("doc"+strconv.Itoa(i)+".txt", "text/plain", "content "+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.ID)
	}
	od, _ := f.svc.Upload(ctx(), osubj, store.EntityAsset, "a2", up("other.txt", "", "other tenant"))
	if f.blob.Len() != 4 {
		t.Fatalf("blobs %d", f.blob.Len())
	}
	p := newPaper()
	f.svc.SetPaperless(p)
	// A document whose checksum does not match stays put.
	bad, _ := f.mem.GetDocument(ctx(), tenant, ids[2])
	_ = f.mem.DeleteDocument(ctx(), tenant, bad.ID)
	bad.Checksum = "deadbeef"
	_ = f.mem.InsertDocument(ctx(), bad)

	res, err := f.svc.Migrate(ctx(), 100)
	if err != nil || res.Migrated != 3 || res.Failed != 1 {
		t.Fatalf("migrate %+v %v", res, err)
	}
	for _, id := range ids[:2] {
		d, _ := f.mem.GetDocument(ctx(), tenant, id)
		if d.PaperlessDocumentID == "" || !strings.HasPrefix(d.StorageKey, store.PaperlessKeyPrefix) {
			t.Fatalf("not migrated %+v", d)
		}
		rc, _, err := f.svc.Download(ctx(), subj, d.EntityType, d.EntityID, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := io.ReadAll(rc); !strings.HasPrefix(string(b), "content ") {
			t.Fatalf("migrated bytes %q", b)
		}
	}
	if o, _ := f.mem.GetDocument(ctx(), other, od.ID); o.PaperlessDocumentID == "" || p.tenant[o.PaperlessDocumentID] != other {
		t.Fatalf("other tenant %+v", o)
	}
	if f.blob.Len() != 1 { // only the checksum-mismatch object remains
		t.Fatalf("old objects left: %d", f.blob.Len())
	}
	// A second pass retries only the failed one.
	if res, err := f.svc.Migrate(ctx(), 100); err != nil || res.Migrated != 0 || res.Failed != 1 {
		t.Fatalf("second pass %+v %v", res, err)
	}
	f.mem.FailNext("ListUnmigratedDocuments")
	if _, err := f.svc.Migrate(ctx(), 0); err == nil {
		t.Error("list failure swallowed")
	}
}

func TestMigrateFailures(t *testing.T) {
	boom := errors.New("down")
	cases := map[string]func(f *fx, p *fakePaper, d store.Document){
		"blob missing": func(f *fx, p *fakePaper, d store.Document) { _ = f.blob.Delete(ctx(), d.StorageKey) },
		"create":       func(f *fx, p *fakePaper, d store.Document) { p.fail["create"] = boom },
		"record":       func(f *fx, p *fakePaper, d store.Document) { f.mem.FailNext("MoveDocumentToPaperless") },
		"cross tenant": func(f *fx, p *fakePaper, d store.Document) {
			_ = f.mem.DeleteDocument(ctx(), tenant, d.ID)
			d.StorageKey = "tenants/" + other + "/documents/x"
			_ = f.mem.InsertDocument(ctx(), d)
		},
	}
	for name, setup := range cases {
		f := newFx(t)
		d, _ := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("a.txt", "", "abc"))
		p := newPaper()
		f.svc.SetPaperless(p)
		setup(f, p, d)
		if res, err := f.svc.Migrate(ctx(), 10); err != nil || res.Failed != 1 || res.Migrated != 0 {
			t.Errorf("%s: %+v %v", name, res, err)
		}
		if name == "record" && len(p.docs) != 0 {
			t.Errorf("record failure left the paperless copy")
		}
	}
}

func TestCategoryPath(t *testing.T) {
	if got := strings.Join(categoryPath(store.EntityAsset, " a/b "), "|"); got != "Assets|a-b" {
		t.Errorf("asset %s", got)
	}
	if got := strings.Join(categoryPath(store.EntityLicense, ""), "|"); got != "Assets|Licenses|unnamed" {
		t.Errorf("license %s", got)
	}
	f := newFx(t)
	_ = f.mem.CreateAsset(ctx(), store.Asset{ID: "a9", TenantID: tenant, Name: "No tag"})
	if l, err := f.svc.entityLabel(ctx(), tenant, store.EntityAsset, "a9"); err != nil || l == "" {
		t.Errorf("label %q %v", l, err)
	}
	if _, err := f.svc.entityLabel(ctx(), tenant, "bogus", "x"); !errors.Is(err, ErrEntityType) {
		t.Errorf("entity type %v", err)
	}
	_ = memstore.New()
}
