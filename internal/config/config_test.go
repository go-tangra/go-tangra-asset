package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fconfig "github.com/go-freya/freya/config"
)

// valid returns a Config that passes both the framework and module Validate.
func valid() Config {
	c := Default()
	c.ServiceName = "asset"
	c.TrustDomain = "example.org"
	c.Authz.Path = "/etc/asset/policy.yaml"
	c.DB.DSN = "postgres://localhost/asset"
	c.Valkey.Addresses = []string{"valkey:6379"}
	c.KEK = KEK{Source: "file", Path: "/etc/asset/kek"}
	c.ObjectStore.Endpoint = "https://minio.example.org:9000"
	c.ObjectStore.Bucket = "asset"
	c.ObjectStore.UseSSL = true
	c.Gateway.Issuer = "https://gw.example.org"
	return c
}

func TestDefaultSecure(t *testing.T) {
	d := Default()
	if d.KEK.Source != "file" {
		t.Errorf("kek.source default = %q, want file", d.KEK.Source)
	}
	if d.Valkey.AllowPlaintext {
		t.Error("valkey.allow_plaintext must default to false")
	}
	if !d.Events.Enabled {
		t.Error("events.enabled must default to true")
	}
	if d.DB.MaxConns != 16 {
		t.Errorf("db.max_conns default = %d, want 16", d.DB.MaxConns)
	}
	if d.ObjectStore.Region != "us-east-1" || d.ObjectStore.PresignTTL != 300 {
		t.Errorf("unexpected object_store defaults: %+v", d.ObjectStore)
	}
	if d.Scheduler.IntervalSeconds != 3600 || d.Scheduler.WarrantySoonDays != 30 ||
		d.Scheduler.LicenseSoonDays != 30 || d.Scheduler.InsuranceSoonDays != 30 {
		t.Errorf("unexpected scheduler defaults: %+v", d.Scheduler)
	}
	if d.Uploads.MaxSizeBytes != 20<<20 {
		t.Errorf("uploads.max_size_bytes default = %d", d.Uploads.MaxSizeBytes)
	}
	if d.Inventory.Service != "inventory" {
		t.Errorf("inventory.service default = %q, want inventory", d.Inventory.Service)
	}
	if d.Gateway.Service != "gateway" {
		t.Errorf("gateway.service default = %q, want gateway", d.Gateway.Service)
	}
	if d.Limits.MaxRequestBytes != 1<<20 || d.Limits.MaxBackupBytes != 32<<20 {
		t.Errorf("unexpected limits defaults: %+v", d.Limits)
	}
	// A pristine Default() has no service_name/db and must not validate.
	if err := Default().Validate(); err == nil {
		t.Error("Default() must not validate without required fields")
	}
}

func TestValidateOK(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	// env source is also accepted.
	c := valid()
	c.KEK = KEK{Source: "env", Env: "ASSET_KEK"}
	if err := c.Validate(); err != nil {
		t.Fatalf("env kek rejected: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"framework", func(c *Config) { c.ServiceName = "" }, "service_name"},
		{"db missing", func(c *Config) { c.DB.DSN = "" }, "db.dsn"},
		{"db prod tls", func(c *Config) { c.Env = "production" }, "sslmode"},
		{"valkey missing", func(c *Config) { c.Valkey.Addresses = nil }, "valkey.addresses"},
		{"valkey prod plaintext", func(c *Config) {
			c.Env = "production"
			c.DB.DSN = "postgres://h/db?sslmode=verify-full"
			c.Valkey.AllowPlaintext = true
		}, "allow_plaintext"},
		{"kek file no path", func(c *Config) { c.KEK = KEK{Source: "file"} }, "kek.path"},
		{"kek env no env", func(c *Config) { c.KEK = KEK{Source: "env"} }, "kek.env"},
		{"kek bad source", func(c *Config) { c.KEK = KEK{Source: "vault"} }, "kek.source"},
		{"object store missing", func(c *Config) { c.ObjectStore.Bucket = "" }, "object_store.endpoint"},
		{"presign ttl", func(c *Config) { c.ObjectStore.PresignTTL = 1 }, "presign_ttl_seconds"},
		{"prod ssl", func(c *Config) {
			c.Env = "production"
			c.DB.DSN = "postgres://h/db?sslmode=verify-full"
			c.ObjectStore.UseSSL = false
		}, "object_store.use_ssl"},
		{"uploads size", func(c *Config) { c.Uploads.MaxSizeBytes = 1 }, "uploads.max_size_bytes"},
		{"sched interval", func(c *Config) { c.Scheduler.IntervalSeconds = 1 }, "scheduler.interval_seconds"},
		{"sched warranty", func(c *Config) { c.Scheduler.WarrantySoonDays = 0 }, "warranty_soon_days"},
		{"sched license", func(c *Config) { c.Scheduler.LicenseSoonDays = 0 }, "license_soon_days"},
		{"sched insurance", func(c *Config) { c.Scheduler.InsuranceSoonDays = 0 }, "insurance_soon_days"},
		{"gateway service", func(c *Config) { c.Gateway.Service = "" }, "gateway.service"},
		{"gateway issuer", func(c *Config) { c.Gateway.Issuer = "http://gw" }, "gateway.issuer"},
		{"mesh prod insecure", func(c *Config) {
			c.Env = "production"
			c.DB.DSN = "postgres://h/db?sslmode=verify-full"
			c.MeshEnroll.Enabled = true
			c.MeshEnroll.Insecure = true
		}, "mesh_enroll.insecure"},
		{"max request", func(c *Config) { c.Limits.MaxRequestBytes = 1 }, "max_request_bytes"},
		{"max backup", func(c *Config) { c.Limits.MaxBackupBytes = 1 }, "max_backup_bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

func TestValidateProdOK(t *testing.T) {
	c := valid()
	c.Env = "production"
	c.DB.DSN = "postgres://h/db?sslmode=verify-full"
	c.MeshEnroll.Enabled = true // enabled + secure is fine
	if err := c.Validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
}

func TestWarnings(t *testing.T) {
	c := valid()
	if w := c.Warnings(); len(w) != 0 {
		t.Fatalf("secure config produced warnings: %v", w)
	}
	c.Valkey.AllowPlaintext = true
	c.ObjectStore.UseSSL = false
	c.MeshEnroll.Enabled = true
	c.MeshEnroll.Insecure = true
	c.Admin.EnablePprof = true // framework warning path
	w := c.Warnings()
	joined := strings.Join(w, "\n")
	if !strings.Contains(joined, "valkey.allow_plaintext") {
		t.Errorf("missing valkey warning: %v", w)
	}
	if !strings.Contains(joined, "object_store.use_ssl") {
		t.Errorf("missing object_store warning: %v", w)
	}
	if !strings.Contains(joined, "mesh_enroll.insecure") {
		t.Errorf("missing mesh_enroll warning: %v", w)
	}
	if !strings.Contains(joined, "pprof") {
		t.Errorf("framework warnings not surfaced: %v", w)
	}
}

func TestDurationHelpers(t *testing.T) {
	c := valid()
	c.Scheduler.IntervalSeconds = 120
	c.ObjectStore.PresignTTL = 90
	c.Scheduler.WarrantySoonDays = 30
	c.Scheduler.LicenseSoonDays = 15
	c.Scheduler.InsuranceSoonDays = 45
	if c.SchedulerInterval() != 120*time.Second {
		t.Errorf("SchedulerInterval=%v", c.SchedulerInterval())
	}
	if c.PresignTTL() != 90*time.Second {
		t.Errorf("PresignTTL=%v", c.PresignTTL())
	}
	sw := c.SoonWindows()
	if sw.Warranty != 30*24*time.Hour || sw.License != 15*24*time.Hour || sw.Insurance != 45*24*time.Hour {
		t.Errorf("SoonWindows=%+v", sw)
	}
}

func TestAddrHelpers(t *testing.T) {
	c := valid()
	c.Config.Server = fconfig.Server{GRPCAddr: ":1", HTTPAddr: ":2"}
	c.Config.Admin.Addr = ":3"
	if c.GRPCAddr() != ":1" || c.HTTPAddr() != ":2" || c.AdminAddr() != ":3" {
		t.Errorf("addr helpers: grpc=%q http=%q admin=%q", c.GRPCAddr(), c.HTTPAddr(), c.AdminAddr())
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "asset.yaml")
	yaml := `
service_name: asset
trust_domain: example.org
authz:
  source: file
  path: /etc/asset/policy.yaml
db:
  dsn: postgres://localhost/asset
  max_conns: 8
valkey:
  addresses: ["valkey:6379"]
  allow_plaintext: true
kek:
  source: env
  env: ASSET_KEK
object_store:
  endpoint: https://minio.example.org:9000
  bucket: asset
  region: eu-central-1
  use_ssl: false
  access_key: ak
  secret_key: sk
  presign_ttl_seconds: 600
inventory:
  service: inventory
scheduler:
  interval_seconds: 900
  warranty_soon_days: 60
  license_soon_days: 45
  insurance_soon_days: 20
uploads:
  max_size_bytes: 52428800
events:
  enabled: false
gateway:
  service: gateway
  issuer: https://gw.example.org
mesh_enroll:
  enabled: true
  enroll_url: https://lcm.example.org/enroll
  lcm_grpc: lcm.example.org:9443
  tenant_id: t1
  token_file: /var/lib/asset/token
  state_file: /var/lib/asset/state
  insecure: true
limits_asset:
  max_request_bytes: 2097152
  max_backup_bytes: 67108864
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.DB.MaxConns != 8 || c.ObjectStore.Region != "eu-central-1" || c.ObjectStore.PresignTTL != 600 {
		t.Errorf("unexpected loaded values: %+v %+v", c.DB, c.ObjectStore)
	}
	if c.Scheduler.IntervalSeconds != 900 || c.Scheduler.WarrantySoonDays != 60 || c.Uploads.MaxSizeBytes != 52428800 {
		t.Errorf("scheduler/uploads not loaded: %+v %+v", c.Scheduler, c.Uploads)
	}
	if !c.Valkey.AllowPlaintext || !c.MeshEnroll.Insecure || c.Events.Enabled || c.ObjectStore.UseSSL {
		t.Errorf("bool fields not loaded: %+v %+v %+v", c.Valkey, c.MeshEnroll, c.Events)
	}
	if c.MeshEnroll.TenantID != "t1" || c.Limits.MaxRequestBytes != 2097152 {
		t.Errorf("mesh/limits not loaded: %+v %+v", c.MeshEnroll, c.Limits)
	}
	// mesh_enroll.insecure is fine because Env is not production here.
	if err := c.Validate(); err != nil {
		t.Fatalf("loaded config invalid: %v", err)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("Load of missing file must error")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("db:\n  unknown_field: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Error("Load with unknown field must error")
	}
}
