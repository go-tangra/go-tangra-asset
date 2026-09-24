// Package app wires the asset service: configuration -> Freya runtime ->
// store/sealing/object store -> domain services -> the mesh HTTP+gRPC surfaces
// (via the gateway), plus gateway registration, permission seeding and the
// lifecycle scheduler worker. It refuses to start without a KEK, a store and an
// object store (Constitution I).
package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra-lcm/sdk/v4/pkg/lcmidentity"
	"github.com/go-tangra/go-tangra/v4"

	authv1 "github.com/go-tangra/go-tangra-auth/sdk/v4/api/proto/auth/v1"
	"github.com/go-tangra/go-tangra-auth/sdk/v4/pkg/authclient"
	"github.com/go-tangra/go-tangra-portal/sdk/v4/pkg/gatewayclient"

	"github.com/go-tangra/go-tangra-asset/v4/internal/assets"
	"github.com/go-tangra/go-tangra-asset/v4/internal/audit"
	"github.com/go-tangra/go-tangra-asset/v4/internal/backup"
	"github.com/go-tangra/go-tangra-asset/v4/internal/blob"
	"github.com/go-tangra/go-tangra-asset/v4/internal/categories"
	"github.com/go-tangra/go-tangra-asset/v4/internal/config"
	"github.com/go-tangra/go-tangra-asset/v4/internal/consumables"
	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
	"github.com/go-tangra/go-tangra-asset/v4/internal/events"
	"github.com/go-tangra/go-tangra-asset/v4/internal/grpcapi"
	"github.com/go-tangra/go-tangra-asset/v4/internal/httpapi"
	"github.com/go-tangra/go-tangra-asset/v4/internal/insurance"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invclient"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invsync"
	"github.com/go-tangra/go-tangra-asset/v4/internal/licenses"
	"github.com/go-tangra/go-tangra-asset/v4/internal/locations"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo/repodb"
	"github.com/go-tangra/go-tangra-asset/v4/internal/scheduler"
	"github.com/go-tangra/go-tangra-asset/v4/internal/sealed"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stats"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stream"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stream/valkeykv"
	"github.com/go-tangra/go-tangra-asset/v4/internal/suppliers"
	"github.com/go-tangra/go-tangra-asset/v4/internal/userdir"
	"github.com/go-tangra/go-tangra-asset/v4/pkg/assetmanifest"
)

// Options override infrastructure (tests) and attach optional parts.
type Options struct {
	Logger   slog.Handler
	KEK      []byte
	Verifier httpapi.Verifier
	Blob     blob.Store        // object store override (tests: blob.NewFake())
	Repo     repo.Store        // store override (tests: memstore); skips the DB
	Users    userdir.Directory // directory override (tests)
	Inv      invclient.Client  // inventory client override (tests)
	Freya    []freya.Option
	Migrate  bool
	Remote   fs.FS // built federated UI remote (nil serves no remote)
}

// App is the wired service.
type App struct {
	Cfg      config.Config
	Log      *slog.Logger
	Freya    *freya.App
	Store    *store.Store
	Repo     repo.Store
	Env      *sealed.Envelope
	Blob     blob.Store
	Audit    *audit.Writer
	Verifier httpapi.Verifier
	HTTP     *httpapi.Server
	Hub      *stream.Hub
	Sched    *scheduler.Service

	closers []func()
	workers []func(context.Context)
}

// Build wires the service.
func Build(ctx context.Context, cfg config.Config, o Options) (a *App, err error) {
	a = &App{Cfg: cfg}
	handler := o.Logger
	if handler == nil {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	}
	a.Log = slog.New(handler)

	// Mesh identity: enroll for the module's own SPIFFE SVID over lcm.
	fopts := append([]freya.Option{freya.WithLogger(handler)}, o.Freya...)
	if cfg.MeshEnroll.Enabled {
		raw, rerr := os.ReadFile(cfg.MeshEnroll.TokenFile)
		if rerr != nil {
			return nil, fmt.Errorf("asset: mesh enroll token: %w", rerr)
		}
		prov, perr := lcmidentity.NewNet(ctx, lcmidentity.NetConfig{
			EnrollURL: cfg.MeshEnroll.EnrollURL, LCMGRPCTarget: cfg.MeshEnroll.LCMGRPCTarget,
			TenantID: cfg.MeshEnroll.TenantID, TrustDomain: cfg.Config.TrustDomain, ServiceName: cfg.Config.ServiceName,
			EnrollmentToken: strings.TrimSpace(string(raw)), Insecure: cfg.MeshEnroll.Insecure, StateFile: cfg.MeshEnroll.StateFile,
		})
		if perr != nil {
			return nil, fmt.Errorf("asset: mesh enroll: %w", perr)
		}
		a.closers = append(a.closers, func() { _ = prov.Close() })
		fopts = append(fopts, freya.WithIdentityProvider(prov))
	}
	if a.Freya, err = freya.New(cfg.Config, fopts...); err != nil {
		return nil, err
	}
	a.closers = append(a.closers, a.Freya.Close)

	// KEK + envelope (seals supplier/location contact PII).
	kek := o.KEK
	if len(kek) == 0 {
		if kek, err = sealed.LoadKEK(cfg.KEK.Source, cfg.KEK.Path, cfg.KEK.Env); err != nil {
			return nil, fmt.Errorf("kek: %w", err)
		}
	}
	if a.Env, err = sealed.NewEnvelope(kek); err != nil {
		return nil, err
	}

	// Store (migrate then open the app pool) unless a repo is injected.
	a.Repo = o.Repo
	if a.Repo == nil {
		if o.Migrate {
			mdsn := cfg.DB.MigrateDSN
			if mdsn == "" {
				mdsn = cfg.DB.DSN
			}
			if err = store.Migrate(ctx, mdsn); err != nil {
				return nil, err
			}
		}
		if a.Store, err = store.Open(ctx, cfg.DB.DSN, cfg.DB.MaxConns); err != nil {
			return nil, err
		}
		a.closers = append(a.closers, a.Store.Close)
		a.Repo = repodb.New(a.Store)
	}
	a.Audit = audit.NewWriter(a.Repo, func(err error) { a.Log.Error("audit write failed", "err", err) })
	a.closers = append(a.closers, a.Audit.Close)

	// Object store (photos + documents). Bucket self-provisioned (non-fatal).
	a.Blob = o.Blob
	if a.Blob == nil {
		if a.Blob, err = blob.New(blob.Config{
			Endpoint: cfg.ObjectStore.Endpoint, Bucket: cfg.ObjectStore.Bucket, Region: cfg.ObjectStore.Region,
			AccessKey: cfg.ObjectStore.AccessKey, SecretKey: cfg.ObjectStore.SecretKey, UseSSL: cfg.ObjectStore.UseSSL,
		}); err != nil {
			return nil, fmt.Errorf("object store: %w", err)
		}
	}
	if berr := a.Blob.EnsureBucket(ctx); berr != nil {
		a.Log.Warn("object store: ensure bucket", "bucket", cfg.ObjectStore.Bucket, "err", berr)
	}

	// Verifier (platform token) + user directory from auth.
	a.Verifier = o.Verifier
	users := o.Users
	if a.Verifier == nil || users == nil {
		conn, cerr := a.Freya.Client(ctx, "auth")
		if cerr != nil {
			return nil, fmt.Errorf("auth client: %w", cerr)
		}
		if a.Verifier == nil {
			a.Verifier = authclient.New(authclient.Config{Issuer: cfg.Gateway.Issuer},
				authclient.GRPCKeys{Client: authv1.NewKeysClient(conn)},
				authclient.GRPCRevocations{Client: authv1.NewSessionsClient(conn)})
		}
		if users == nil {
			users = userdir.New(conn)
		}
	}

	// Inventory client (sync). Lazily connected: the module starts without inventory.
	inv := o.Inv
	if inv == nil {
		inv = &lazyInventory{app: a}
	}

	// Event bus (Valkey Streams) for realtime.
	sc := valkeykv.Config{Addresses: cfg.Valkey.Addresses, Username: cfg.Valkey.Username, Password: cfg.Valkey.Password, AllowPlaintext: cfg.Valkey.AllowPlaintext}
	if cfg.Valkey.CAFile != "" {
		if sc.CAPEM, err = os.ReadFile(cfg.Valkey.CAFile); err != nil {
			return nil, fmt.Errorf("valkey ca: %w", err)
		}
	}
	streamClient, serr := valkeykv.New(sc)
	if serr != nil {
		return nil, fmt.Errorf("event bus: %w", serr)
	}
	a.Hub = stream.NewHub(streamClient, stream.Config{}, a.Log)
	a.closers = append(a.closers, a.Hub.Close)
	var pub events.Publisher = events.HubPublisher{}
	if cfg.Events.Enabled {
		pub = events.HubPublisher{Hub: a.Hub}
	}

	// Domain services.
	assetsSvc := assets.New(a.Repo, users, pub, a.Audit)
	assetsSvc.SetBlobStore(a.Blob)
	catSvc := categories.New(a.Repo, a.Audit)
	supSvc := suppliers.New(a.Repo, a.Env, a.Audit)
	locSvc := locations.New(a.Repo, a.Env, a.Audit)
	conSvc := consumables.New(a.Repo, a.Audit)
	licSvc := licenses.New(a.Repo, a.Audit)
	insSvc := insurance.New(a.Repo, a.Audit)
	docSvc := documents.New(a.Repo, a.Blob, a.Audit, cfg.Uploads.MaxSizeBytes, cfg.PresignTTL())
	syncSvc := invsync.New(a.Repo, inv, a.Audit)
	statsSvc := stats.New(a.Repo, cfg.SoonWindows())
	backupSvc := backup.New(a.Repo, a.Audit)
	a.Sched = scheduler.New(a.Repo, pub, a.Audit, cfg.SoonWindows(), a.Log)

	// Mesh HTTP surface (reached only through the gateway).
	hopts := []httpapi.Option{httpapi.WithVerifier(a.Verifier)}
	if o.Remote != nil {
		hopts = append(hopts, httpapi.WithRemote(o.Remote))
	}
	if a.HTTP, err = httpapi.NewHandler(a.Freya, hopts...); err != nil {
		return nil, err
	}
	a.HTTP.Register(httpapi.Deps{
		Assets: assetsSvc, Categories: catSvc, Suppliers: supSvc, Locations: locSvc, Consumables: conSvc, Licenses: licSvc, Insurance: insSvc,
		Documents: docSvc, Sync: syncSvc, Stats: statsSvc, Backup: backupSvc, Users: users, Hub: a.Hub, Health: a.health,
	})
	a.Freya.HTTP().HandlePrefix("/", a.HTTP.Handler())

	// Service-to-service gRPC surface (asset.v1), SPIFFE mTLS, not gateway-proxied.
	grpcapi.Register(a.Freya.GRPC(), grpcapi.Deps{
		Assets: assetsSvc, Categories: catSvc, Suppliers: supSvc, Locations: locSvc, Consumables: conSvc, Licenses: licSvc, Insurance: insSvc,
		Documents: docSvc, Sync: syncSvc, Stats: statsSvc, Users: users, Health: a.health,
	})

	// Lifecycle scheduler worker.
	a.workers = append(a.workers, func(c context.Context) { a.Sched.Run(c, cfg.SchedulerInterval()) })
	return a, nil
}

// health reports the reachability of the store for /health.
func (a *App) health() map[string]string {
	out := map[string]string{"store": "ok", "object_store": "ok"}
	if a.Store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := a.Store.Ping(ctx); err != nil {
			out["store"] = "unreachable"
		}
	}
	return out
}

// lazyInventory dials the inventory service on first use so the module starts
// (and serves everything else) while inventory is down.
type lazyInventory struct {
	app *App
	c   invclient.Client
}

func (l *lazyInventory) ListHosts(ctx context.Context, tenantID string) ([]invclient.Host, error) {
	if l.c == nil {
		conn, err := l.app.Freya.Client(ctx, l.app.Cfg.Inventory.Service)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", invclient.ErrUnavailable, err)
		}
		l.c = invclient.New(conn)
	}
	return l.c.ListHosts(ctx, tenantID)
}

// Run starts the verifier, gateway registration, workers, and the Freya runtime.
func (a *App) Run(ctx context.Context) error {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if v, ok := a.Verifier.(*authclient.Verifier); ok {
		go func() {
			for wctx.Err() == nil {
				if err := v.Start(wctx, func(err error) { a.Log.Warn("verifier", "err", err) }); err == nil {
					return
				}
				select {
				case <-wctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
			}
		}()
	}
	go a.register(wctx)
	for _, w := range a.workers {
		go w(wctx)
	}
	go func() {
		for wctx.Err() == nil && !a.Freya.Ready() {
			time.Sleep(100 * time.Millisecond)
		}
		a.seedLoop(wctx)
	}()
	return a.Freya.Run(ctx)
}

// Close releases resources.
func (a *App) Close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}
	a.closers = nil
}

// register keeps the gateway lease for the manifest.
func (a *App) register(ctx context.Context) {
	for ctx.Err() == nil && !a.Freya.Ready() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	man, err := assetmanifest.Manifest()
	if err != nil {
		a.Log.Error("gateway manifest", "err", err)
		return
	}
	httpEP, err := a.Freya.HTTP().Endpoint()
	if err != nil {
		a.Log.Error("gateway registration: http endpoint", "err", err)
		return
	}
	grpcEP, err := a.Freya.GRPC().Endpoint()
	if err != nil {
		a.Log.Error("gateway registration: grpc endpoint", "err", err)
		return
	}
	var client *gatewayclient.Client
	for ctx.Err() == nil && client == nil {
		conn, cerr := a.Freya.Client(ctx, a.Cfg.Gateway.Service)
		if cerr != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		client, err = gatewayclient.New(conn, gatewayclient.Options{Manifest: man, HTTPURL: "https://" + httpEP.Host, GRPCTarget: grpcEP.Host, Logger: a.Log,
			OnState: func(s gatewayclient.State) {
				a.Log.Info("gateway lease", "registered", s.Registered, "lease", s.LeaseID, "err", s.Err)
			}})
		if err != nil {
			a.Log.Error("gateway client", "err", err)
			return
		}
	}
	if client != nil {
		if err := client.Run(ctx); err != nil {
			a.Log.Error("gateway registration", "err", err)
		}
	}
}
