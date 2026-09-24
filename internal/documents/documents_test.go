package documents

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/blob"
	"github.com/go-tangra/go-tangra-asset/v4/internal/memstore"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

const (
	tenant = "11111111-1111-7111-8111-111111111111"
	other  = "22222222-2222-7222-8222-222222222222"
)

var (
	subj  = authz.Subjects{TenantID: tenant, UserID: "u", ActorKind: authz.ActorUser}
	osubj = authz.Subjects{TenantID: other, UserID: "o", ActorKind: authz.ActorUser}
)

func ctx() context.Context { return context.Background() }

type fx struct {
	svc  *Service
	mem  *memstore.Mem
	blob *blob.Fake
}

func newFx(t *testing.T) *fx {
	t.Helper()
	mem := memstore.New()
	b := blob.NewFake()
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a1", TenantID: tenant, AssetTag: "T1", Name: "Laptop"})
	_ = mem.CreateConsumable(ctx(), store.Consumable{ID: "c1", TenantID: tenant, Name: "Toner"})
	_ = mem.CreateLicense(ctx(), store.License{ID: "l1", TenantID: tenant, Name: "Office"})
	_ = mem.CreateAsset(ctx(), store.Asset{ID: "a2", TenantID: other, AssetTag: "T2", Name: "Other"})
	return &fx{svc: New(mem, b, nil, 1024, 0), mem: mem, blob: b}
}

func up(name, mime, body string) UploadInput {
	return UploadInput{FileName: name, MimeType: mime, Size: int64(len(body)), Reader: strings.NewReader(body)}
}

func TestPhotoLifecycle(t *testing.T) {
	f := newFx(t)
	a, err := f.svc.UploadPhoto(ctx(), subj, "a1", up("p.png", "image/png", "PNGDATA"))
	if err != nil || a.PhotoKey != PhotoKey(tenant, "a1", ".png") {
		t.Fatalf("%+v %v", a, err)
	}
	if !strings.HasPrefix(a.PhotoKey, "tenants/"+tenant+"/") {
		t.Fatal("photo key must be tenant-prefixed")
	}
	rc, mime, err := f.svc.GetPhoto(ctx(), subj, "a1")
	if err != nil || mime != "image/png" {
		t.Fatal(mime, err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "PNGDATA" {
		t.Fatalf("bytes: %q", b)
	}
	// Replace with a jpeg: the old object goes away.
	a, err = f.svc.UploadPhoto(ctx(), subj, "a1", up("p.jpg", "image/jpeg; charset=binary", "JPG"))
	if err != nil || !strings.HasSuffix(a.PhotoKey, ".jpg") {
		t.Fatal(a, err)
	}
	if _, err := f.blob.Get(ctx(), PhotoKey(tenant, "a1", ".png")); err == nil {
		t.Fatal("old photo object should be deleted")
	}
	if f.blob.Len() != 1 {
		t.Fatalf("objects: %d", f.blob.Len())
	}
	// Refusals.
	if _, err := f.svc.UploadPhoto(ctx(), subj, "a1", up("x.exe", "application/x-msdownload", "MZ")); !errors.Is(err, ErrMediaType) {
		t.Fatalf("media type: %v", err)
	}
	if _, err := f.svc.UploadPhoto(ctx(), subj, "a1", up("p.png", "image/png", strings.Repeat("x", 2000))); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("too large: %v", err)
	}
	if _, err := f.svc.UploadPhoto(ctx(), subj, "missing", up("p.png", "image/png", "x")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.UploadPhoto(ctx(), osubj, "a1", up("p.png", "image/png", "x")); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant upload")
	}
	if _, err := f.svc.UploadPhoto(ctx(), authz.Subjects{}, "a1", up("p.png", "image/png", "x")); err == nil {
		t.Fatal("forbidden")
	}
	f.blob.FailPut(errors.New("s3 down"))
	if _, err := f.svc.UploadPhoto(ctx(), subj, "a1", up("p.png", "image/png", "x")); err == nil {
		t.Fatal("blob failure")
	}
	f.mem.FailNext("UpdateAsset")
	if _, err := f.svc.UploadPhoto(ctx(), subj, "a1", up("p.png", "image/png", "x")); err == nil {
		t.Fatal("store failure")
	}
	if _, _, err := f.svc.GetPhoto(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := f.svc.GetPhoto(ctx(), osubj, "a2"); !errors.Is(err, ErrNotFound) {
		t.Fatal("asset without photo")
	}
	if _, _, err := f.svc.GetPhoto(ctx(), authz.Subjects{}, "a1"); err == nil {
		t.Fatal("forbidden")
	}
	// A key outside the tenant prefix is refused even if the row says so.
	raw, _ := f.mem.GetAsset(ctx(), tenant, "a1")
	good := raw.PhotoKey
	raw.PhotoKey = "tenants/" + other + "/assets/a2/photo.png"
	_ = f.mem.UpdateAsset(ctx(), raw)
	if _, _, err := f.svc.GetPhoto(ctx(), subj, "a1"); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("cross tenant key: %v", err)
	}
	if err := f.svc.DeletePhoto(ctx(), subj, "a1"); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("cross tenant key delete: %v", err)
	}
	raw.PhotoKey = good
	_ = f.mem.UpdateAsset(ctx(), raw)
	// Object missing → not found.
	_ = f.blob.Delete(ctx(), good)
	if _, _, err := f.svc.GetPhoto(ctx(), subj, "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	_, _ = f.blob.Put(ctx(), good, strings.NewReader("J"), 1, "image/jpeg")
	f.blob.FailDelete(errors.New("s3 down"))
	if err := f.svc.DeletePhoto(ctx(), subj, "a1"); err == nil {
		t.Fatal("delete failure")
	}
	f.mem.FailNext("UpdateAsset")
	if err := f.svc.DeletePhoto(ctx(), subj, "a1"); err == nil {
		t.Fatal("store failure")
	}
	_, _ = f.blob.Put(ctx(), good, strings.NewReader("J"), 1, "image/jpeg")
	if err := f.svc.DeletePhoto(ctx(), subj, "a1"); err != nil {
		t.Fatal(err)
	}
	if a, _ := f.mem.GetAsset(ctx(), tenant, "a1"); a.PhotoKey != "" {
		t.Fatal("photo key not cleared")
	}
	if err := f.svc.DeletePhoto(ctx(), subj, "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := f.svc.DeletePhoto(ctx(), subj, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := f.svc.DeletePhoto(ctx(), authz.Subjects{}, "a1"); err == nil {
		t.Fatal("forbidden")
	}
	if PhotoMime("x.bin") != "application/octet-stream" || PhotoMime("x.gif") != "image/gif" || PhotoMime("x.webp") != "image/webp" {
		t.Fatal("PhotoMime")
	}
}

func TestDocumentsPolymorphic(t *testing.T) {
	f := newFx(t)
	d, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", UploadInput{FileName: "../invoice.pdf", MimeType: "application/pdf", Size: 5, Reader: strings.NewReader("%PDF-"), Description: "inv"})
	if err != nil {
		t.Fatal(err)
	}
	if d.FileName != "invoice.pdf" || d.Checksum == "" || d.StorageKey != DocumentKey(tenant, d.ID) || d.UploadedBy != "u" {
		t.Fatalf("%+v", d)
	}
	c, err := f.svc.Upload(ctx(), subj, store.EntityConsumable, "c1", up("sds.txt", "", "safety"))
	if err != nil || c.MimeType != "application/octet-stream" {
		t.Fatal(c, err)
	}
	l, err := f.svc.Upload(ctx(), subj, store.EntityLicense, "l1", up("key.txt", "text/plain", "ABC"))
	if err != nil {
		t.Fatal(err)
	}
	// Each entity lists only its own.
	for _, tc := range []struct{ typ, id, want string }{{store.EntityAsset, "a1", d.ID}, {store.EntityConsumable, "c1", c.ID}, {store.EntityLicense, "l1", l.ID}} {
		rows, err := f.svc.List(ctx(), subj, tc.typ, tc.id)
		if err != nil || len(rows) != 1 || rows[0].ID != tc.want {
			t.Fatalf("%s: %v %v", tc.typ, rows, err)
		}
	}
	if _, err := f.svc.List(ctx(), subj, "widget", "a1"); !errors.Is(err, ErrEntityType) {
		t.Fatal(err)
	}
	if _, err := f.svc.List(ctx(), subj, store.EntityAsset, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.List(ctx(), authz.Subjects{}, store.EntityAsset, "a1"); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := f.svc.List(ctx(), osubj, store.EntityAsset, "a1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant list")
	}
	// Download exact bytes; a document is bound to its entity.
	rc, meta, err := f.svc.Download(ctx(), subj, store.EntityAsset, "a1", d.ID)
	if err != nil || meta.FileName != "invoice.pdf" {
		t.Fatal(meta, err)
	}
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(b) != "%PDF-" {
		t.Fatalf("bytes %q", b)
	}
	if _, _, err := f.svc.Download(ctx(), subj, store.EntityConsumable, "c1", d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("document bound to entity")
	}
	if _, _, err := f.svc.Download(ctx(), osubj, store.EntityAsset, "a1", d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant download")
	}
	if _, _, err := f.svc.Download(ctx(), authz.Subjects{}, store.EntityAsset, "a1", d.ID); err == nil {
		t.Fatal("forbidden")
	}
	url, err := f.svc.DownloadURL(ctx(), subj, store.EntityAsset, "a1", d.ID)
	if err != nil || !strings.Contains(url, d.StorageKey) || !strings.Contains(url, "ttl=300") {
		t.Fatal(url, err)
	}
	if _, err := f.svc.DownloadURL(ctx(), subj, store.EntityAsset, "a1", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.DownloadURL(ctx(), authz.Subjects{}, store.EntityAsset, "a1", d.ID); err == nil {
		t.Fatal("forbidden")
	}
	// Refusals on upload.
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("", "x", "y")); err == nil {
		t.Fatal("file name required")
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("big", "x", strings.Repeat("y", 2000))); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "missing", up("f", "x", "y")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityConsumable, "missing", up("f", "x", "y")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Upload(ctx(), subj, store.EntityLicense, "missing", up("f", "x", "y")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Upload(ctx(), subj, "widget", "a1", up("f", "x", "y")); !errors.Is(err, ErrEntityType) {
		t.Fatal(err)
	}
	if _, err := f.svc.Upload(ctx(), authz.Subjects{}, store.EntityAsset, "a1", up("f", "x", "y")); err == nil {
		t.Fatal("forbidden")
	}
	f.blob.FailPut(errors.New("down"))
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("f", "x", "y")); err == nil {
		t.Fatal("blob failure")
	}
	before := f.blob.Len()
	f.mem.FailNext("InsertDocument")
	if _, err := f.svc.Upload(ctx(), subj, store.EntityAsset, "a1", up("f", "x", "y")); err == nil {
		t.Fatal("store failure")
	}
	if f.blob.Len() != before {
		t.Fatal("orphan object left behind after a failed insert")
	}
	// Cross-tenant key in a row is refused.
	raw, _ := f.mem.GetDocument(ctx(), tenant, l.ID)
	raw.StorageKey = "tenants/" + other + "/documents/x"
	_ = f.mem.DeleteDocument(ctx(), tenant, l.ID)
	_ = f.mem.InsertDocument(ctx(), raw)
	if _, _, err := f.svc.Download(ctx(), subj, store.EntityLicense, "l1", l.ID); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("cross tenant key: %v", err)
	}
	// Missing object → not found.
	_ = f.blob.Delete(ctx(), c.StorageKey)
	if _, _, err := f.svc.Download(ctx(), subj, store.EntityConsumable, "c1", c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// Delete.
	f.blob.FailDelete(errors.New("down"))
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d.ID); err == nil {
		t.Fatal("blob delete failure")
	}
	f.mem.FailNext("DeleteDocument")
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d.ID); err == nil {
		t.Fatal("store delete failure")
	}
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.blob.Get(ctx(), d.StorageKey); err == nil {
		t.Fatal("object should be removed")
	}
	if err := f.svc.Delete(ctx(), subj, store.EntityAsset, "a1", d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx(), authz.Subjects{}, store.EntityAsset, "a1", d.ID); err == nil {
		t.Fatal("forbidden")
	}
	// Purge removes everything for an entity (tolerating a bad key).
	_, _ = f.svc.Upload(ctx(), subj, store.EntityConsumable, "c1", up("a", "x", "1"))
	_, _ = f.svc.Upload(ctx(), subj, store.EntityConsumable, "c1", up("b", "x", "2"))
	f.svc.Purge(ctx(), tenant, store.EntityConsumable, "c1")
	if rows, _ := f.mem.ListDocuments(ctx(), tenant, store.EntityConsumable, "c1"); len(rows) != 0 {
		t.Fatalf("purge left %d rows", len(rows))
	}
	f.svc.Purge(ctx(), tenant, store.EntityLicense, "l1") // bad key row: tolerated
	f.svc.PurgeRows(ctx(), tenant, []store.Document{{TenantID: other, StorageKey: "tenants/" + other + "/documents/z"}})
	f.mem.FailNext("ListDocuments")
	f.svc.Purge(ctx(), tenant, store.EntityLicense, "l1")
	f.mem.FailNext("ListDocuments")
	if _, err := f.svc.List(ctx(), subj, store.EntityAsset, "a1"); err == nil {
		t.Fatal("list failure")
	}
	if f.svc.MaxSize() != 1024 {
		t.Fatal("max size")
	}
	if s := New(f.mem, f.blob, nil, 0, 0); s.MaxSize() != 20<<20 || s.presignTTL != 5*time.Minute {
		t.Fatal("defaults")
	}
}
