package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	assetv1 "github.com/go-tangra/go-tangra-asset/v4/api/proto/asset/v1"
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
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/sealed"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stats"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
	"github.com/go-tangra/go-tangra-asset/v4/internal/suppliers"
	"github.com/go-tangra/go-tangra-asset/v4/internal/userdir"
)

const (
	tenant = "11111111-1111-7111-8111-111111111111"
	user1  = "55555555-5555-7555-8555-555555555555"
)

type kit struct {
	d    Deps
	mem  *memstore.Mem
	inv  *invclient.Fake
	blob *blob.Fake
}

func newKit(t *testing.T) kit {
	t.Helper()
	mem := memstore.New()
	env, err := sealed.NewEnvelope(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	b := blob.NewFake()
	inv := invclient.NewFake()
	dir := userdir.NewFake()
	dir.Add(tenant, userdir.User{ID: user1, DisplayName: "Ann"})
	a := assets.New(mem, dir, nil, nil)
	a.SetBlobStore(b)
	d := Deps{
		Assets: a, Categories: categories.New(mem, nil), Suppliers: suppliers.New(mem, env, nil), Locations: locations.New(mem, env, nil),
		Consumables: consumables.New(mem, nil), Licenses: licenses.New(mem, nil), Insurance: insurance.New(mem, nil),
		Documents: documents.New(mem, b, nil, 1024, time.Minute), Sync: invsync.New(mem, inv, nil), Stats: stats.New(mem, config.SoonWindows{}),
		Users: dir, Health: func() map[string]string { return map[string]string{"store": "ok"} },
	}
	return kit{d: d, mem: mem, inv: inv, blob: b}
}

func withFakeCaller(t *testing.T, id string, ok bool) {
	t.Helper()
	prev := callerFunc
	callerFunc = func(context.Context) (string, bool) { return id, ok }
	t.Cleanup(func() { callerFunc = prev })
}

func code(err error) codes.Code { return status.Code(err) }

type fakeRegistrar struct{ n int }

func (f *fakeRegistrar) RegisterService(*grpc.ServiceDesc, any) { f.n++ }

func TestRegisterAndCaller(t *testing.T) {
	k := newKit(t)
	r := &fakeRegistrar{}
	Register(r, k.d)
	if r.n != 9 {
		t.Fatalf("services registered: %d", r.n)
	}
	r2 := &fakeRegistrar{}
	Register(r2, Deps{})
	if r2.n != 2 {
		t.Fatalf("user+system always registered: %d", r2.n)
	}
	withFakeCaller(t, "", false)
	if _, err := (&AssetServer{d: k.d}).GetAsset(context.Background(), &assetv1.GetAssetRequest{TenantId: tenant}); code(err) != codes.Unauthenticated {
		t.Fatal(err)
	}
	withFakeCaller(t, "spiffe://example.org/svc/x", true)
	if _, err := (&AssetServer{d: k.d}).GetAsset(context.Background(), &assetv1.GetAssetRequest{TenantId: "nope"}); code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := map[codes.Code][]error{
		codes.NotFound:           {assets.ErrNotFound, categories.ErrNotFound, suppliers.ErrNotFound, locations.ErrNotFound, consumables.ErrNotFound, licenses.ErrNotFound, insurance.ErrNotFound, documents.ErrNotFound, repo.ErrNotFound},
		codes.PermissionDenied:   {documents.ErrCrossTenant},
		codes.InvalidArgument:    {assets.ValidationError{}, categories.ValidationError{}, suppliers.ValidationError{}, locations.ValidationError{}, consumables.ValidationError{}, licenses.ValidationError{}, insurance.ValidationError{}, documents.ValidationError{}, documents.ErrEntityType, documents.ErrMediaType},
		codes.FailedPrecondition: {assets.ErrConflict, assets.ErrState, categories.ErrConflict, categories.ErrInUse, suppliers.ErrConflict, suppliers.ErrInUse, locations.ErrConflict, locations.ErrInUse, insurance.ErrConflict, repo.ErrConflict, repo.ErrNotEmpty},
		codes.ResourceExhausted:  {documents.ErrTooLarge},
		codes.Unavailable:        {invsync.ErrUnavailable, errors.New("other"), backup.ErrBadSchema},
	}
	for want, errs := range cases {
		for _, e := range errs {
			if got := code(grpcError(e)); got != want {
				t.Fatalf("%v: got %v want %v", e, got, want)
			}
		}
	}
}

func TestAssetServiceFlow(t *testing.T) {
	k := newKit(t)
	withFakeCaller(t, "spiffe://example.org/svc/x", true)
	ctx := context.Background()
	s := &AssetServer{d: k.d}
	a, err := s.CreateAsset(ctx, &assetv1.CreateAssetRequest{TenantId: tenant, Asset: &assetv1.AssetInput{Name: "Laptop", PurchaseCost: 100, PurchaseDate: &assetv1.Timestamp{Unix: 1700000000}, UsefulLifeYears: 3}})
	if err != nil || a.GetAssetTag() == "" || a.GetBookValue() <= 0 {
		t.Fatal(a, err)
	}
	if _, err := s.CreateAsset(ctx, &assetv1.CreateAssetRequest{TenantId: tenant}); code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	if _, err := s.CreateAsset(ctx, &assetv1.CreateAssetRequest{TenantId: tenant, Asset: &assetv1.AssetInput{Name: "x", AssetTag: a.GetAssetTag()}}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if g, err := s.GetAsset(ctx, &assetv1.GetAssetRequest{TenantId: tenant, Id: a.GetId()}); err != nil || g.GetName() != "Laptop" {
		t.Fatal(g, err)
	}
	if _, err := s.GetAsset(ctx, &assetv1.GetAssetRequest{TenantId: tenant, Id: "missing"}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	if l, err := s.ListAssets(ctx, &assetv1.ListAssetsRequest{TenantId: tenant, Query: "lap"}); err != nil || len(l.GetAssets()) != 1 {
		t.Fatal(l, err)
	}
	if u, err := s.UpdateAsset(ctx, &assetv1.UpdateAssetRequest{TenantId: tenant, Id: a.GetId(), Asset: &assetv1.AssetInput{Name: "L2"}}); err != nil || u.GetName() != "L2" {
		t.Fatal(u, err)
	}
	if _, err := s.UpdateAsset(ctx, &assetv1.UpdateAssetRequest{TenantId: tenant, Id: "missing", Asset: &assetv1.AssetInput{Name: "L2"}}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	as, err := s.AssignAsset(ctx, &assetv1.AssignAssetRequest{TenantId: tenant, Id: a.GetId(), UserId: user1})
	if err != nil || as.GetStatus() != "assigned" || as.GetAssigneeName() != "Ann" {
		t.Fatal(as, err)
	}
	if _, err := s.AssignAsset(ctx, &assetv1.AssignAssetRequest{TenantId: tenant, Id: a.GetId(), UserId: user1}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if un, err := s.UnassignAsset(ctx, &assetv1.UnassignAssetRequest{TenantId: tenant, Id: a.GetId()}); err != nil || un.GetStatus() != "deployable" {
		t.Fatal(un, err)
	}
	if _, err := s.UnassignAsset(ctx, &assetv1.UnassignAssetRequest{TenantId: tenant, Id: a.GetId()}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	h, err := s.GetAssignmentHistory(ctx, &assetv1.GetAssignmentHistoryRequest{TenantId: tenant, Id: a.GetId()})
	if err != nil || len(h.GetAssignments()) != 2 || h.GetAssignments()[1].GetReturnedAt() == nil {
		t.Fatal(h, err)
	}
	if _, err := s.GetAssignmentHistory(ctx, &assetv1.GetAssignmentHistoryRequest{TenantId: tenant, Id: "missing"}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	// Photo + documents.
	ph, err := s.UploadPhoto(ctx, &assetv1.UploadPhotoRequest{TenantId: tenant, Id: a.GetId(), MimeType: "image/png", Content: []byte("PNG")})
	if err != nil || !ph.GetHasPhoto() {
		t.Fatal(ph, err)
	}
	if _, err := s.UploadPhoto(ctx, &assetv1.UploadPhotoRequest{TenantId: tenant, Id: a.GetId(), MimeType: "text/plain", Content: []byte("x")}); code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	if _, err := s.UploadPhoto(ctx, &assetv1.UploadPhotoRequest{TenantId: tenant, Id: "missing", MimeType: "image/png", Content: []byte("x")}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	if _, err := s.DeletePhoto(ctx, &assetv1.DeletePhotoRequest{TenantId: tenant, Id: a.GetId()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeletePhoto(ctx, &assetv1.DeletePhotoRequest{TenantId: tenant, Id: a.GetId()}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	d, err := s.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant, EntityId: a.GetId(), FileName: "inv.pdf", MimeType: "application/pdf", Content: []byte("%PDF")})
	if err != nil || d.GetChecksum() == "" {
		t.Fatal(d, err)
	}
	if _, err := s.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant, EntityId: a.GetId(), FileName: "big", Content: make([]byte, 5000)}); code(err) != codes.ResourceExhausted {
		t.Fatal(err)
	}
	if l, err := s.ListDocuments(ctx, &assetv1.ListDocumentsRequest{TenantId: tenant, EntityId: a.GetId()}); err != nil || len(l.GetDocuments()) != 1 {
		t.Fatal(l, err)
	}
	if _, err := s.ListDocuments(ctx, &assetv1.ListDocumentsRequest{TenantId: tenant, EntityId: "missing"}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	dl, err := s.DownloadDocument(ctx, &assetv1.DownloadDocumentRequest{TenantId: tenant, EntityId: a.GetId(), DocumentId: d.GetId()})
	if err != nil || dl.GetUrl() == "" || dl.GetDocument().GetFileName() != "inv.pdf" {
		t.Fatal(dl, err)
	}
	if _, err := s.DownloadDocument(ctx, &assetv1.DownloadDocumentRequest{TenantId: tenant, EntityId: a.GetId(), DocumentId: "missing"}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	_ = k.blob.Delete(ctx, "tenants/"+tenant+"/documents/"+d.GetId())
	if _, err := s.DownloadDocument(ctx, &assetv1.DownloadDocumentRequest{TenantId: tenant, EntityId: a.GetId(), DocumentId: d.GetId()}); code(err) != codes.NotFound {
		t.Fatal("missing object:", err)
	}
	k.blob.FailDelete(errors.New("down"))
	if _, err := s.DeleteDocument(ctx, &assetv1.DeleteDocumentRequest{TenantId: tenant, EntityId: a.GetId(), DocumentId: d.GetId()}); err == nil {
		t.Fatal("blob failure")
	}
	if _, err := s.DeleteDocument(ctx, &assetv1.DeleteDocumentRequest{TenantId: tenant, EntityId: a.GetId(), DocumentId: d.GetId()}); err != nil {
		t.Fatal(err)
	}
	// Sync.
	k.inv.Set(tenant, []invclient.Host{{ID: "h1", Hostname: "pc-1", SystemSerial: "S1"}})
	pv, err := s.InventorySyncPreview(ctx, &assetv1.InventorySyncPreviewRequest{TenantId: tenant})
	if err != nil || pv.GetCreate() != 1 {
		t.Fatal(pv, err)
	}
	ex, err := s.InventorySyncExecute(ctx, &assetv1.InventorySyncExecuteRequest{TenantId: tenant})
	if err != nil || ex.GetCreated() != 1 {
		t.Fatal(ex, err)
	}
	k.inv.Set(tenant, []invclient.Host{{ID: "h1", Hostname: "pc-1", SystemSerial: "S1", Model: "M"}})
	pv, _ = s.InventorySyncPreview(ctx, &assetv1.InventorySyncPreviewRequest{TenantId: tenant})
	if pv.GetUpdate() != 1 || len(pv.GetChanges()[0].GetChanges()) == 0 {
		t.Fatal(pv)
	}
	k.inv.Down = true
	if _, err := s.InventorySyncPreview(ctx, &assetv1.InventorySyncPreviewRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	if _, err := s.InventorySyncExecute(ctx, &assetv1.InventorySyncExecuteRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	ns := &AssetServer{d: Deps{Assets: k.d.Assets}}
	if _, err := ns.InventorySyncPreview(ctx, &assetv1.InventorySyncPreviewRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	if _, err := ns.InventorySyncExecute(ctx, &assetv1.InventorySyncExecuteRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	if _, err := s.DeleteAsset(ctx, &assetv1.DeleteAssetRequest{TenantId: tenant, Id: a.GetId()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteAsset(ctx, &assetv1.DeleteAssetRequest{TenantId: tenant, Id: a.GetId()}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	// Every RPC refuses an unauthenticated peer.
	withFakeCaller(t, "", false)
	bad := []error{}
	_, e := s.CreateAsset(ctx, &assetv1.CreateAssetRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.ListAssets(ctx, &assetv1.ListAssetsRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.UpdateAsset(ctx, &assetv1.UpdateAssetRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.DeleteAsset(ctx, &assetv1.DeleteAssetRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.AssignAsset(ctx, &assetv1.AssignAssetRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.UnassignAsset(ctx, &assetv1.UnassignAssetRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.GetAssignmentHistory(ctx, &assetv1.GetAssignmentHistoryRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.UploadPhoto(ctx, &assetv1.UploadPhotoRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.DeletePhoto(ctx, &assetv1.DeletePhotoRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.ListDocuments(ctx, &assetv1.ListDocumentsRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.DeleteDocument(ctx, &assetv1.DeleteDocumentRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.DownloadDocument(ctx, &assetv1.DownloadDocumentRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.InventorySyncPreview(ctx, &assetv1.InventorySyncPreviewRequest{TenantId: tenant})
	bad = append(bad, e)
	_, e = s.InventorySyncExecute(ctx, &assetv1.InventorySyncExecuteRequest{TenantId: tenant})
	bad = append(bad, e)
	for i, e := range bad {
		if code(e) != codes.Unauthenticated {
			t.Fatalf("rpc %d: %v", i, e)
		}
	}
}

func TestOrgServices(t *testing.T) {
	k := newKit(t)
	withFakeCaller(t, "spiffe://example.org/svc/x", true)
	ctx := context.Background()
	cs := &CategoryServer{d: k.d}
	root, err := cs.CreateCategory(ctx, &assetv1.CreateCategoryRequest{TenantId: tenant, Category: &assetv1.CategoryInput{Name: "HW"}})
	if err != nil {
		t.Fatal(err)
	}
	child, _ := cs.CreateCategory(ctx, &assetv1.CreateCategoryRequest{TenantId: tenant, Category: &assetv1.CategoryInput{Name: "Laptops", ParentId: root.GetId()}})
	if _, err := cs.CreateCategory(ctx, &assetv1.CreateCategoryRequest{TenantId: tenant, Category: &assetv1.CategoryInput{Name: "Laptops", ParentId: root.GetId()}}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if g, err := cs.GetCategory(ctx, &assetv1.GetCategoryRequest{TenantId: tenant, Id: root.GetId()}); err != nil || g.GetChildCount() != 1 {
		t.Fatal(g, err)
	}
	if l, err := cs.ListCategories(ctx, &assetv1.ListCategoriesRequest{TenantId: tenant}); err != nil || len(l.GetCategories()) != 2 {
		t.Fatal(l, err)
	}
	tr, err := cs.GetCategoryTree(ctx, &assetv1.GetCategoryTreeRequest{TenantId: tenant})
	if err != nil || len(tr.GetRoots()) != 1 || len(tr.GetRoots()[0].GetChildren()) != 1 {
		t.Fatal(tr, err)
	}
	if _, err := cs.UpdateCategory(ctx, &assetv1.UpdateCategoryRequest{TenantId: tenant, Id: child.GetId(), Category: &assetv1.CategoryInput{Name: "Notebooks", ParentId: root.GetId()}}); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.DeleteCategory(ctx, &assetv1.DeleteCategoryRequest{TenantId: tenant, Id: root.GetId()}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if _, err := cs.DeleteCategory(ctx, &assetv1.DeleteCategoryRequest{TenantId: tenant, Id: child.GetId()}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []error{
		first(cs.CreateCategory(ctx, &assetv1.CreateCategoryRequest{TenantId: "x"})), first(cs.GetCategory(ctx, &assetv1.GetCategoryRequest{TenantId: "x"})),
		first(cs.ListCategories(ctx, &assetv1.ListCategoriesRequest{TenantId: "x"})), first(cs.UpdateCategory(ctx, &assetv1.UpdateCategoryRequest{TenantId: "x"})),
		first(cs.DeleteCategory(ctx, &assetv1.DeleteCategoryRequest{TenantId: "x"})), first(cs.GetCategoryTree(ctx, &assetv1.GetCategoryTreeRequest{TenantId: "x"})),
	} {
		if code(e) != codes.InvalidArgument {
			t.Fatal(e)
		}
	}

	ss := &SupplierServer{d: k.d}
	sp, err := ss.CreateSupplier(ctx, &assetv1.CreateSupplierRequest{TenantId: tenant, Supplier: &assetv1.SupplierInput{Name: "Acme", Email: "a@x.example"}})
	if err != nil || sp.GetEmail() != "" { // service callers are not admins → redacted
		t.Fatal(sp, err)
	}
	if _, err := ss.CreateSupplier(ctx, &assetv1.CreateSupplierRequest{TenantId: tenant, Supplier: &assetv1.SupplierInput{Name: "Acme"}}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if g, err := ss.GetSupplier(ctx, &assetv1.GetSupplierRequest{TenantId: tenant, Id: sp.GetId()}); err != nil || g.GetName() != "Acme" {
		t.Fatal(g, err)
	}
	if l, err := ss.ListSuppliers(ctx, &assetv1.ListSuppliersRequest{TenantId: tenant}); err != nil || len(l.GetSuppliers()) != 1 {
		t.Fatal(l, err)
	}
	if _, err := ss.UpdateSupplier(ctx, &assetv1.UpdateSupplierRequest{TenantId: tenant, Id: sp.GetId(), Supplier: &assetv1.SupplierInput{Name: "Acme2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.DeleteSupplier(ctx, &assetv1.DeleteSupplierRequest{TenantId: tenant, Id: sp.GetId()}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []error{
		first(ss.CreateSupplier(ctx, &assetv1.CreateSupplierRequest{TenantId: "x"})), first(ss.GetSupplier(ctx, &assetv1.GetSupplierRequest{TenantId: "x"})),
		first(ss.ListSuppliers(ctx, &assetv1.ListSuppliersRequest{TenantId: "x"})), first(ss.UpdateSupplier(ctx, &assetv1.UpdateSupplierRequest{TenantId: "x"})),
		first(ss.DeleteSupplier(ctx, &assetv1.DeleteSupplierRequest{TenantId: "x"})),
	} {
		if code(e) != codes.InvalidArgument {
			t.Fatal(e)
		}
	}

	ls := &LocationServer{d: k.d}
	hq, err := ls.CreateLocation(ctx, &assetv1.CreateLocationRequest{TenantId: tenant, Location: &assetv1.LocationInput{Name: "HQ", Phone: "1"}})
	if err != nil || hq.GetPhone() != "" {
		t.Fatal(hq, err)
	}
	fl, _ := ls.CreateLocation(ctx, &assetv1.CreateLocationRequest{TenantId: tenant, Location: &assetv1.LocationInput{Name: "Floor", ParentId: hq.GetId()}})
	if fl.GetPath() != "HQ / Floor" {
		t.Fatal(fl)
	}
	if g, err := ls.GetLocation(ctx, &assetv1.GetLocationRequest{TenantId: tenant, Id: hq.GetId()}); err != nil || g.GetChildCount() != 1 {
		t.Fatal(g, err)
	}
	if l, err := ls.ListLocations(ctx, &assetv1.ListLocationsRequest{TenantId: tenant}); err != nil || len(l.GetLocations()) != 2 {
		t.Fatal(l, err)
	}
	if tr, err := ls.GetLocationTree(ctx, &assetv1.GetLocationTreeRequest{TenantId: tenant}); err != nil || len(tr.GetRoots()) != 1 || len(tr.GetRoots()[0].GetChildren()) != 1 {
		t.Fatal(tr, err)
	}
	if _, err := ls.UpdateLocation(ctx, &assetv1.UpdateLocationRequest{TenantId: tenant, Id: fl.GetId(), Location: &assetv1.LocationInput{Name: "F1", ParentId: hq.GetId()}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ls.DeleteLocation(ctx, &assetv1.DeleteLocationRequest{TenantId: tenant, Id: hq.GetId()}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if _, err := ls.DeleteLocation(ctx, &assetv1.DeleteLocationRequest{TenantId: tenant, Id: fl.GetId()}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []error{
		first(ls.CreateLocation(ctx, &assetv1.CreateLocationRequest{TenantId: "x"})), first(ls.GetLocation(ctx, &assetv1.GetLocationRequest{TenantId: "x"})),
		first(ls.ListLocations(ctx, &assetv1.ListLocationsRequest{TenantId: "x"})), first(ls.UpdateLocation(ctx, &assetv1.UpdateLocationRequest{TenantId: "x"})),
		first(ls.DeleteLocation(ctx, &assetv1.DeleteLocationRequest{TenantId: "x"})), first(ls.GetLocationTree(ctx, &assetv1.GetLocationTreeRequest{TenantId: "x"})),
	} {
		if code(e) != codes.InvalidArgument {
			t.Fatal(e)
		}
	}
}

func first[T any](_ T, err error) error { return err }

func TestInventoryServices(t *testing.T) {
	k := newKit(t)
	withFakeCaller(t, "spiffe://example.org/svc/x", true)
	ctx := context.Background()
	cs := &ConsumableServer{d: k.d}
	c, err := cs.CreateConsumable(ctx, &assetv1.CreateConsumableRequest{TenantId: tenant, Consumable: &assetv1.ConsumableInput{Name: "Toner", Amount: 1, MinAmount: 2}})
	if err != nil || !c.GetLowStock() {
		t.Fatal(c, err)
	}
	if g, err := cs.GetConsumable(ctx, &assetv1.GetConsumableRequest{TenantId: tenant, Id: c.GetId()}); err != nil || g.GetName() != "Toner" {
		t.Fatal(g, err)
	}
	if l, err := cs.ListConsumables(ctx, &assetv1.ListConsumablesRequest{TenantId: tenant}); err != nil || len(l.GetConsumables()) != 1 {
		t.Fatal(l, err)
	}
	if u, err := cs.UpdateConsumable(ctx, &assetv1.UpdateConsumableRequest{TenantId: tenant, Id: c.GetId(), Consumable: &assetv1.ConsumableInput{Name: "Toner", Amount: 9, MinAmount: 2}}); err != nil || u.GetLowStock() {
		t.Fatal(u, err)
	}
	d, err := cs.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant, EntityId: c.GetId(), FileName: "sds.txt", Content: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := cs.ListDocuments(ctx, &assetv1.ListDocumentsRequest{TenantId: tenant, EntityId: c.GetId()}); len(l.GetDocuments()) != 1 {
		t.Fatal(l)
	}
	if dl, err := cs.DownloadDocument(ctx, &assetv1.DownloadDocumentRequest{TenantId: tenant, EntityId: c.GetId(), DocumentId: d.GetId()}); err != nil || dl.GetUrl() == "" {
		t.Fatal(dl, err)
	}
	if _, err := cs.DeleteDocument(ctx, &assetv1.DeleteDocumentRequest{TenantId: tenant, EntityId: c.GetId(), DocumentId: d.GetId()}); err != nil {
		t.Fatal(err)
	}
	_, _ = cs.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant, EntityId: c.GetId(), FileName: "a", Content: []byte("x")})
	if _, err := cs.DeleteConsumable(ctx, &assetv1.DeleteConsumableRequest{TenantId: tenant, Id: c.GetId()}); err != nil {
		t.Fatal(err)
	}
	if k.blob.Len() != 0 {
		t.Fatal("documents not purged")
	}
	if _, err := cs.DeleteConsumable(ctx, &assetv1.DeleteConsumableRequest{TenantId: tenant, Id: c.GetId()}); code(err) != codes.NotFound {
		t.Fatal(err)
	}
	for _, e := range []error{
		first(cs.CreateConsumable(ctx, &assetv1.CreateConsumableRequest{TenantId: "x"})), first(cs.GetConsumable(ctx, &assetv1.GetConsumableRequest{TenantId: "x"})),
		first(cs.ListConsumables(ctx, &assetv1.ListConsumablesRequest{TenantId: "x"})), first(cs.UpdateConsumable(ctx, &assetv1.UpdateConsumableRequest{TenantId: "x"})),
		first(cs.DeleteConsumable(ctx, &assetv1.DeleteConsumableRequest{TenantId: "x"})),
	} {
		if code(e) != codes.InvalidArgument {
			t.Fatal(e)
		}
	}

	ls := &LicenseServer{d: k.d}
	l, err := ls.CreateLicense(ctx, &assetv1.CreateLicenseRequest{TenantId: tenant, License: &assetv1.LicenseInput{Name: "Office", ValidTo: &assetv1.Timestamp{Unix: 1900000000}}})
	if err != nil || l.GetValidTo().GetUnix() != 1900000000 {
		t.Fatal(l, err)
	}
	if g, err := ls.GetLicense(ctx, &assetv1.GetLicenseRequest{TenantId: tenant, Id: l.GetId()}); err != nil || g.GetName() != "Office" {
		t.Fatal(g, err)
	}
	if ll, err := ls.ListLicenses(ctx, &assetv1.ListLicensesRequest{TenantId: tenant}); err != nil || len(ll.GetLicenses()) != 1 {
		t.Fatal(ll, err)
	}
	if _, err := ls.UpdateLicense(ctx, &assetv1.UpdateLicenseRequest{TenantId: tenant, Id: l.GetId(), License: &assetv1.LicenseInput{Name: "O365"}}); err != nil {
		t.Fatal(err)
	}
	ld, _ := ls.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant, EntityId: l.GetId(), FileName: "k", Content: []byte("x")})
	if lst, _ := ls.ListDocuments(ctx, &assetv1.ListDocumentsRequest{TenantId: tenant, EntityId: l.GetId()}); len(lst.GetDocuments()) != 1 {
		t.Fatal(lst)
	}
	if _, err := ls.DownloadDocument(ctx, &assetv1.DownloadDocumentRequest{TenantId: tenant, EntityId: l.GetId(), DocumentId: ld.GetId()}); err != nil {
		t.Fatal(err)
	}
	if _, err := ls.DeleteDocument(ctx, &assetv1.DeleteDocumentRequest{TenantId: tenant, EntityId: l.GetId(), DocumentId: ld.GetId()}); err != nil {
		t.Fatal(err)
	}
	_, _ = ls.UploadDocument(ctx, &assetv1.UploadDocumentRequest{TenantId: tenant, EntityId: l.GetId(), FileName: "k2", Content: []byte("x")})
	if _, err := ls.DeleteLicense(ctx, &assetv1.DeleteLicenseRequest{TenantId: tenant, Id: l.GetId()}); err != nil {
		t.Fatal(err)
	}
	if k.blob.Len() != 0 {
		t.Fatal("license documents not purged")
	}
	for _, e := range []error{
		first(ls.CreateLicense(ctx, &assetv1.CreateLicenseRequest{TenantId: "x"})), first(ls.GetLicense(ctx, &assetv1.GetLicenseRequest{TenantId: "x"})),
		first(ls.ListLicenses(ctx, &assetv1.ListLicensesRequest{TenantId: "x"})), first(ls.UpdateLicense(ctx, &assetv1.UpdateLicenseRequest{TenantId: "x"})),
		first(ls.DeleteLicense(ctx, &assetv1.DeleteLicenseRequest{TenantId: "x"})),
	} {
		if code(e) != codes.InvalidArgument {
			t.Fatal(e)
		}
	}

	is := &InsuranceServer{d: k.d}
	pol, err := is.CreateInsurancePolicy(ctx, &assetv1.CreateInsurancePolicyRequest{TenantId: tenant, Policy: &assetv1.InsurancePolicyInput{Name: "Fleet", PolicyNumber: "P1"}})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := (&AssetServer{d: k.d}).CreateAsset(ctx, &assetv1.CreateAssetRequest{TenantId: tenant, Asset: &assetv1.AssetInput{Name: "L"}})
	pa, err := is.AddAssetToPolicy(ctx, &assetv1.AddAssetToPolicyRequest{TenantId: tenant, PolicyId: pol.GetId(), AssetId: a.GetId(), CoveredValue: 5})
	if err != nil || pa.GetAssetTag() != a.GetAssetTag() {
		t.Fatal(pa, err)
	}
	if _, err := is.AddAssetToPolicy(ctx, &assetv1.AddAssetToPolicyRequest{TenantId: tenant, PolicyId: pol.GetId(), AssetId: a.GetId()}); code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	if g, err := is.GetInsurancePolicy(ctx, &assetv1.GetInsurancePolicyRequest{TenantId: tenant, Id: pol.GetId()}); err != nil || g.GetAssetCount() != 1 {
		t.Fatal(g, err)
	}
	if l, err := is.ListPolicyAssets(ctx, &assetv1.ListPolicyAssetsRequest{TenantId: tenant, PolicyId: pol.GetId()}); err != nil || len(l.GetAssets()) != 1 {
		t.Fatal(l, err)
	}
	if l, err := is.ListInsurancePolicies(ctx, &assetv1.ListInsurancePoliciesRequest{TenantId: tenant}); err != nil || len(l.GetPolicies()) != 1 {
		t.Fatal(l, err)
	}
	if _, err := is.UpdateInsurancePolicy(ctx, &assetv1.UpdateInsurancePolicyRequest{TenantId: tenant, Id: pol.GetId(), Policy: &assetv1.InsurancePolicyInput{Name: "F2", PolicyNumber: "P1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := is.RemoveAssetFromPolicy(ctx, &assetv1.RemoveAssetFromPolicyRequest{TenantId: tenant, PolicyId: pol.GetId(), AssetId: a.GetId()}); err != nil {
		t.Fatal(err)
	}
	if _, err := is.DeleteInsurancePolicy(ctx, &assetv1.DeleteInsurancePolicyRequest{TenantId: tenant, Id: pol.GetId()}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []error{
		first(is.CreateInsurancePolicy(ctx, &assetv1.CreateInsurancePolicyRequest{TenantId: "x"})), first(is.GetInsurancePolicy(ctx, &assetv1.GetInsurancePolicyRequest{TenantId: "x"})),
		first(is.ListInsurancePolicies(ctx, &assetv1.ListInsurancePoliciesRequest{TenantId: "x"})), first(is.UpdateInsurancePolicy(ctx, &assetv1.UpdateInsurancePolicyRequest{TenantId: "x"})),
		first(is.DeleteInsurancePolicy(ctx, &assetv1.DeleteInsurancePolicyRequest{TenantId: "x"})), first(is.ListPolicyAssets(ctx, &assetv1.ListPolicyAssetsRequest{TenantId: "x"})),
		first(is.AddAssetToPolicy(ctx, &assetv1.AddAssetToPolicyRequest{TenantId: "x"})), first(is.RemoveAssetFromPolicy(ctx, &assetv1.RemoveAssetFromPolicyRequest{TenantId: "x"})),
	} {
		if code(e) != codes.InvalidArgument {
			t.Fatal(e)
		}
	}
}

func TestUserAndSystem(t *testing.T) {
	k := newKit(t)
	withFakeCaller(t, "spiffe://example.org/svc/x", true)
	ctx := context.Background()
	us := &UserServer{d: k.d}
	if l, err := us.ListUsers(ctx, &assetv1.ListUsersRequest{TenantId: tenant, Query: "an"}); err != nil || len(l.GetUsers()) != 1 {
		t.Fatal(l, err)
	}
	if l, err := us.ListUsers(ctx, &assetv1.ListUsersRequest{TenantId: tenant, Query: "zzz"}); err != nil || len(l.GetUsers()) != 0 {
		t.Fatal(l, err)
	}
	if l, err := (&UserServer{d: Deps{}}).ListUsers(ctx, &assetv1.ListUsersRequest{TenantId: tenant}); err != nil || len(l.GetUsers()) != 0 {
		t.Fatal(l, err)
	}
	if _, err := us.ListUsers(ctx, &assetv1.ListUsersRequest{TenantId: "x"}); code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	dir := userdir.NewFake()
	dir.Err = errors.New("down")
	if _, err := (&UserServer{d: Deps{Users: dir}}).ListUsers(ctx, &assetv1.ListUsersRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	sys := &SystemServer{d: k.d}
	if h, err := sys.Health(ctx, &assetv1.HealthRequest{}); err != nil || h.GetStatus() != "ok" {
		t.Fatal(h, err)
	}
	if h, _ := (&SystemServer{d: Deps{Health: func() map[string]string { return map[string]string{"s": "down"} }}}).Health(ctx, &assetv1.HealthRequest{}); h.GetStatus() != "degraded" {
		t.Fatal(h)
	}
	_ = k.mem.CreateAsset(ctx, store.Asset{ID: "a", TenantID: tenant, AssetTag: "T", Name: "n", PurchaseCost: 10})
	if d, err := sys.GetDashboardStats(ctx, &assetv1.GetDashboardStatsRequest{TenantId: tenant}); err != nil || d.GetTotalAssets() != 1 || d.GetTotalCost() != 10 {
		t.Fatal(d, err)
	}
	if _, err := sys.GetDashboardStats(ctx, &assetv1.GetDashboardStatsRequest{TenantId: "x"}); code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	if _, err := (&SystemServer{d: Deps{}}).GetDashboardStats(ctx, &assetv1.GetDashboardStatsRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	k.mem.FailNext("TenantStats")
	if _, err := sys.GetDashboardStats(ctx, &assetv1.GetDashboardStatsRequest{TenantId: tenant}); code(err) != codes.Unavailable {
		t.Fatal(err)
	}
	if fromTS(nil) != nil || fromTS(&assetv1.Timestamp{}) != nil || ts(time.Time{}) != nil {
		t.Fatal("timestamp helpers")
	}
}
