package assetclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	assetv1 "github.com/go-freya/freya/services/asset/api/proto/asset/v1"
	"github.com/go-freya/freya/services/asset/pkg/assetclient"
)

type stubAssets struct {
	assetv1.UnimplementedAssetServiceServer
	last any
}

func (s *stubAssets) CreateAsset(_ context.Context, req *assetv1.CreateAssetRequest) (*assetv1.Asset, error) {
	s.last = req
	return &assetv1.Asset{Id: "a1", AssetTag: "AST-1", Name: req.GetAsset().GetName(), PurchaseDate: req.GetAsset().GetPurchaseDate(), BookValue: 5, CreatedAt: &assetv1.Timestamp{Unix: 100}}, nil
}
func (s *stubAssets) GetAsset(_ context.Context, req *assetv1.GetAssetRequest) (*assetv1.Asset, error) {
	if req.GetId() == "missing" {
		return nil, status.Error(codes.NotFound, "not_found")
	}
	return &assetv1.Asset{Id: req.GetId(), Tags: map[string]string{"k": "v"}}, nil
}
func (s *stubAssets) ListAssets(_ context.Context, req *assetv1.ListAssetsRequest) (*assetv1.ListAssetsResponse, error) {
	s.last = req
	return &assetv1.ListAssetsResponse{Assets: []*assetv1.Asset{{Id: "a1"}, {Id: "a2"}}}, nil
}
func (s *stubAssets) UpdateAsset(_ context.Context, req *assetv1.UpdateAssetRequest) (*assetv1.Asset, error) {
	return &assetv1.Asset{Id: req.GetId(), Name: req.GetAsset().GetName()}, nil
}
func (s *stubAssets) DeleteAsset(context.Context, *assetv1.DeleteAssetRequest) (*assetv1.Empty, error) {
	return &assetv1.Empty{}, nil
}
func (s *stubAssets) AssignAsset(_ context.Context, req *assetv1.AssignAssetRequest) (*assetv1.Asset, error) {
	return &assetv1.Asset{Id: req.GetId(), UserId: req.GetUserId(), Status: "assigned"}, nil
}
func (s *stubAssets) UnassignAsset(_ context.Context, req *assetv1.UnassignAssetRequest) (*assetv1.Asset, error) {
	return &assetv1.Asset{Id: req.GetId(), LocationId: req.GetLocationId(), Status: "deployable"}, nil
}
func (s *stubAssets) GetAssignmentHistory(context.Context, *assetv1.GetAssignmentHistoryRequest) (*assetv1.GetAssignmentHistoryResponse, error) {
	return &assetv1.GetAssignmentHistoryResponse{Assignments: []*assetv1.Assignment{{Id: "g1", Action: "assigned", AssignedAt: &assetv1.Timestamp{Unix: 10}, ReturnedAt: &assetv1.Timestamp{Unix: 20}}, {Id: "g2"}}}, nil
}
func (s *stubAssets) InventorySyncPreview(context.Context, *assetv1.InventorySyncPreviewRequest) (*assetv1.InventorySyncPreviewResponse, error) {
	return &assetv1.InventorySyncPreviewResponse{Hosts: 1, Create: 1, Changes: []*assetv1.SyncChange{{Hostname: "pc", Action: "create"}}}, nil
}
func (s *stubAssets) InventorySyncExecute(_ context.Context, req *assetv1.InventorySyncExecuteRequest) (*assetv1.InventorySyncExecuteResponse, error) {
	return &assetv1.InventorySyncExecuteResponse{Created: int32(len(req.GetHostnames())), Errors: []string{}}, nil
}

type stubCategories struct {
	assetv1.UnimplementedCategoryServiceServer
}

func (stubCategories) GetCategoryTree(context.Context, *assetv1.GetCategoryTreeRequest) (*assetv1.GetCategoryTreeResponse, error) {
	return &assetv1.GetCategoryTreeResponse{Roots: []*assetv1.Category{{Id: "c1", Name: "HW", Children: []*assetv1.Category{{Id: "c2", Name: "Laptops"}}}}}, nil
}

type stubSuppliers struct {
	assetv1.UnimplementedSupplierServiceServer
}

func (stubSuppliers) ListSuppliers(context.Context, *assetv1.ListSuppliersRequest) (*assetv1.ListSuppliersResponse, error) {
	return &assetv1.ListSuppliersResponse{Suppliers: []*assetv1.Supplier{{Id: "s1", Name: "Acme"}}}, nil
}

type stubLocations struct {
	assetv1.UnimplementedLocationServiceServer
}

func (stubLocations) GetLocationTree(context.Context, *assetv1.GetLocationTreeRequest) (*assetv1.GetLocationTreeResponse, error) {
	return &assetv1.GetLocationTreeResponse{Roots: []*assetv1.Location{{Id: "l1", Name: "HQ", Children: []*assetv1.Location{{Id: "l2"}}}}}, nil
}

type stubUsers struct {
	assetv1.UnimplementedUserServiceServer
}

func (stubUsers) ListUsers(context.Context, *assetv1.ListUsersRequest) (*assetv1.ListUsersResponse, error) {
	return &assetv1.ListUsersResponse{Users: []*assetv1.User{{Id: "u1", DisplayName: "Ann"}}}, nil
}

type stubSystem struct {
	assetv1.UnimplementedSystemServiceServer
}

func (stubSystem) Health(context.Context, *assetv1.HealthRequest) (*assetv1.HealthResponse, error) {
	return &assetv1.HealthResponse{Status: "ok", Components: map[string]string{"store": "ok"}}, nil
}
func (stubSystem) GetDashboardStats(context.Context, *assetv1.GetDashboardStatsRequest) (*assetv1.DashboardStats, error) {
	return &assetv1.DashboardStats{TotalAssets: 3, AssetsByStatus: map[string]int64{"deployable": 3}, TotalCost: 9}, nil
}

func dial(t *testing.T) *assetclient.Client {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	assetv1.RegisterAssetServiceServer(srv, &stubAssets{})
	assetv1.RegisterCategoryServiceServer(srv, stubCategories{})
	assetv1.RegisterSupplierServiceServer(srv, stubSuppliers{})
	assetv1.RegisterLocationServiceServer(srv, stubLocations{})
	assetv1.RegisterUserServiceServer(srv, stubUsers{})
	assetv1.RegisterSystemServiceServer(srv, stubSystem{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return assetclient.New(conn)
}

func TestClient(t *testing.T) {
	c := dial(t)
	ctx := context.Background()
	pd := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	a, err := c.CreateAsset(ctx, "t", assetclient.AssetInput{Name: "L", PurchaseDate: &pd, Tags: map[string]string{"a": "b"}})
	if err != nil || a.AssetTag != "AST-1" || a.PurchaseDate == nil || !a.PurchaseDate.Equal(pd) || a.BookValue != 5 || a.CreatedAt.Unix() != 100 {
		t.Fatalf("%+v %v", a, err)
	}
	if g, err := c.GetAsset(ctx, "t", "a1"); err != nil || g.Tags["k"] != "v" || g.PurchaseDate != nil {
		t.Fatal(g, err)
	}
	if _, err := c.GetAsset(ctx, "t", "missing"); status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
	if l, err := c.ListAssets(ctx, "t", assetclient.AssetFilter{Query: "x", Limit: 2}); err != nil || len(l) != 2 {
		t.Fatal(l, err)
	}
	if u, err := c.UpdateAsset(ctx, "t", "a1", assetclient.AssetInput{Name: "N"}); err != nil || u.Name != "N" {
		t.Fatal(u, err)
	}
	if err := c.DeleteAsset(ctx, "t", "a1"); err != nil {
		t.Fatal(err)
	}
	if as, err := c.AssignAsset(ctx, "t", "a1", "u1", ""); err != nil || as.UserID != "u1" {
		t.Fatal(as, err)
	}
	if un, err := c.UnassignAsset(ctx, "t", "a1", "l1", ""); err != nil || un.LocationID != "l1" {
		t.Fatal(un, err)
	}
	h, err := c.GetAssignmentHistory(ctx, "t", "a1", 5)
	if err != nil || len(h) != 2 || h[0].ReturnedAt == nil || h[1].ReturnedAt != nil || h[0].AssignedAt.Unix() != 10 {
		t.Fatal(h, err)
	}
	if p, err := c.InventorySyncPreview(ctx, "t"); err != nil || p.Create != 1 || len(p.Changes) != 1 {
		t.Fatal(p, err)
	}
	if r, err := c.InventorySyncExecute(ctx, "t", []string{"pc"}); err != nil || r.Created != 1 {
		t.Fatal(r, err)
	}
	if tr, err := c.GetCategoryTree(ctx, "t"); err != nil || len(tr) != 1 || len(tr[0].Children) != 1 {
		t.Fatal(tr, err)
	}
	if s, err := c.ListSuppliers(ctx, "t", ""); err != nil || len(s) != 1 || s[0].Name != "Acme" {
		t.Fatal(s, err)
	}
	if lt, err := c.GetLocationTree(ctx, "t"); err != nil || len(lt) != 1 || len(lt[0].Children) != 1 {
		t.Fatal(lt, err)
	}
	if us, err := c.ListUsers(ctx, "t", ""); err != nil || len(us) != 1 {
		t.Fatal(us, err)
	}
	if st, err := c.GetDashboardStats(ctx, "t"); err != nil || st.TotalAssets != 3 || st.AssetsByStatus["deployable"] != 3 || st.TotalCost != 9 {
		t.Fatal(st, err)
	}
	if s, comps, err := c.Health(ctx); err != nil || s != "ok" || comps["store"] != "ok" {
		t.Fatal(s, comps, err)
	}
}

func TestClientErrors(t *testing.T) {
	// A connection with no server: every call surfaces the transport error.
	conn, err := grpc.NewClient("passthrough:///nowhere", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return nil, context.DeadlineExceeded }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	c := assetclient.New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	errs := []error{}
	_, e := c.CreateAsset(ctx, "t", assetclient.AssetInput{})
	errs = append(errs, e)
	_, e = c.GetAsset(ctx, "t", "a")
	errs = append(errs, e)
	_, e = c.ListAssets(ctx, "t", assetclient.AssetFilter{})
	errs = append(errs, e)
	_, e = c.UpdateAsset(ctx, "t", "a", assetclient.AssetInput{})
	errs = append(errs, e)
	errs = append(errs, c.DeleteAsset(ctx, "t", "a"))
	_, e = c.AssignAsset(ctx, "t", "a", "u", "")
	errs = append(errs, e)
	_, e = c.UnassignAsset(ctx, "t", "a", "", "")
	errs = append(errs, e)
	_, e = c.GetAssignmentHistory(ctx, "t", "a", 0)
	errs = append(errs, e)
	_, e = c.InventorySyncPreview(ctx, "t")
	errs = append(errs, e)
	_, e = c.InventorySyncExecute(ctx, "t", nil)
	errs = append(errs, e)
	_, e = c.GetCategoryTree(ctx, "t")
	errs = append(errs, e)
	_, e = c.ListSuppliers(ctx, "t", "")
	errs = append(errs, e)
	_, e = c.GetLocationTree(ctx, "t")
	errs = append(errs, e)
	_, e = c.ListUsers(ctx, "t", "")
	errs = append(errs, e)
	_, e = c.GetDashboardStats(ctx, "t")
	errs = append(errs, e)
	_, _, e = c.Health(ctx)
	errs = append(errs, e)
	for i, e := range errs {
		if e == nil {
			t.Fatalf("call %d: want error", i)
		}
	}
}
