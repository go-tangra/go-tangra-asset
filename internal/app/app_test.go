//go:build integration

// Package app end-to-end test: brings up the whole service (Build + Run) against
// a real TimescaleDB and Valkey (testcontainers), a self-issued local-dev
// identity, an all-allow policy, an injected token verifier, a fake object
// store, user directory and inventory client, so the wiring in app.go — store/
// migrations, envelope, audit writer, event bus, services, the scheduler worker,
// the HTTP and gRPC surfaces and the register/seed loops — is exercised without
// the auth/lcm/gateway/inventory peers being reachable. Run with:
//
//	go test -tags integration ./internal/app/
//
// It skips cleanly when Docker/testcontainers is unavailable.
package app_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/services/auth/pkg/authclient"

	"github.com/go-freya/freya/services/asset/internal/app"
	"github.com/go-freya/freya/services/asset/internal/blob"
	"github.com/go-freya/freya/services/asset/internal/config"
	"github.com/go-freya/freya/services/asset/internal/invclient"
	"github.com/go-freya/freya/services/asset/internal/userdir"
)

const appTenant = "11111111-1111-7111-8111-111111111111"

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (authclient.Identity, error) {
	if token == "admin" {
		return authclient.Identity{UserID: "u1", TenantID: appTenant, Roles: []string{"admin"}}, nil
	}
	return authclient.Identity{}, errors.New("unauthenticated")
}

func startTimescale(t *testing.T) (adminDSN, appDSN string) {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "timescale/timescaledb:latest-pg16", ExposedPorts: []string{"5432/tcp"},
			Env:        map[string]string{"POSTGRES_PASSWORD": "test", "POSTGRES_DB": "asset"},
			WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2 * time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432/tcp")
	adminDSN = "postgres://postgres:test@" + host + ":" + port.Port() + "/asset?sslmode=disable"
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Exec(ctx, "CREATE ROLE asset_app LOGIN PASSWORD 'app' NOBYPASSRLS")
	_ = conn.Close(ctx)
	appDSN = "postgres://asset_app:app@" + host + ":" + port.Port() + "/asset?sslmode=disable"
	return
}

func startValkey(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "valkey/valkey:8-alpine", ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Skipf("valkey container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379/tcp")
	return host + ":" + port.Port()
}

func testConfig(t *testing.T) config.Config {
	adminDSN, appDSN := startTimescale(t)
	valkeyAddr := startValkey(t)
	c := config.Default()
	c.ServiceName, c.TrustDomain, c.Env = "asset", "example.org", "dev"
	c.DB.DSN, c.DB.MigrateDSN, c.DB.MaxConns = appDSN, adminDSN, 4
	c.Valkey.Addresses, c.Valkey.AllowPlaintext = []string{valkeyAddr}, true
	c.Server.GRPCAddr, c.Server.HTTPAddr, c.Admin.Addr = "127.0.0.1:0", "127.0.0.1:0", "127.0.0.1:0"
	c.Discovery.Static = map[string][]string{
		"lcm": {"127.0.0.1:1"}, "auth": {"127.0.0.1:1"}, "gateway": {"127.0.0.1:1"}, "inventory": {"127.0.0.1:1"},
	}
	c.ObjectStore.Endpoint, c.ObjectStore.Bucket = "127.0.0.1:1", "asset"
	c.Gateway.Service, c.Gateway.Issuer = "gateway", "https://localhost:8443"
	c.Scheduler.IntervalSeconds = 10
	return c
}

func options() (app.Options, *invclient.Fake) {
	inv := invclient.NewFake()
	dir := userdir.NewFake()
	dir.Add(appTenant, userdir.User{ID: "u2", DisplayName: "Ann"})
	return app.Options{
		Migrate: true, KEK: make([]byte, 32), Verifier: fakeVerifier{}, Blob: blob.NewFake(), Users: dir, Inv: inv,
		Freya: []freya.Option{freya.WithInsecureLocalDev(), freya.WithAllowAllPolicy()},
	}, inv
}

func TestBuildWiresTheService(t *testing.T) {
	ctx := context.Background()
	o, inv := options()
	a, err := app.Build(ctx, testConfig(t), o)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer a.Close()
	if a.Freya == nil || a.Store == nil || a.Repo == nil || a.Env == nil || a.HTTP == nil || a.Hub == nil || a.Sched == nil || a.Audit == nil || a.Blob == nil {
		t.Fatalf("app not fully wired: %+v", a)
	}
	do := func(method, path, tok, body string) *httptest.ResponseRecorder {
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
		a.HTTP.Handler().ServeHTTP(w, r)
		return w
	}
	if w := do("GET", "/api/asset/v1/assets", "", ""); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := do("GET", "/api/asset/v1/health", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"store":"ok"`) {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	// End to end through the real store: create, assign (fake directory), unassign, sync (fake inventory).
	w := do("POST", "/api/asset/v1/assets", "admin", `{"name":"Laptop","purchase_cost":1000,"purchase_date":"2024-01-01T00:00:00Z","useful_life_years":5}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	id := strings.Split(strings.Split(w.Body.String(), `"id":"`)[1], `"`)[0]
	if w := do("POST", "/api/asset/v1/assets/"+id+"/assign", "admin", `{"user_id":"u2"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"assignee_name":"Ann"`) {
		t.Fatalf("assign: %d %s", w.Code, w.Body.String())
	}
	if w := do("POST", "/api/asset/v1/assets/"+id+"/unassign", "admin", `{}`); w.Code != 200 {
		t.Fatalf("unassign: %d %s", w.Code, w.Body.String())
	}
	inv.Set(appTenant, []invclient.Host{{ID: "h1", Hostname: "pc-1", SystemSerial: "S1"}})
	if w := do("POST", "/api/asset/v1/assets/inventory-sync/preview", "admin", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"create":1`) {
		t.Fatalf("sync preview: %d %s", w.Code, w.Body.String())
	}
	if w := do("GET", "/api/asset/v1/stats", "admin", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"total_assets":1`) {
		t.Fatalf("stats: %d %s", w.Code, w.Body.String())
	}
	// The scheduler sweeps the real store (system scope) without error.
	if reps := a.Sched.SweepAll(ctx); len(reps) != 1 || reps[0].Errors != 0 {
		t.Fatalf("sweep: %+v", reps)
	}
}

func TestRunStartsAndStops(t *testing.T) {
	o, _ := options()
	a, err := app.Build(context.Background(), testConfig(t), o)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not stop after cancel")
	}
}
