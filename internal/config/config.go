// Package config loads and validates the asset (ITAM) service configuration: the
// Freya framework config plus the module's own sections. Every value is explicit;
// insecure opt-outs are named and surfaced at start (Constitution I/VII). The
// asset/consumable/license store, the Valkey event bus, the S3-compatible object
// store (photos + documents), the inventory-sync client, the lifecycle scheduler
// and the module's own mesh enrollment all read from here.
//
// The module's yaml keys are chosen to NOT collide with the framework sections
// the embedded config already owns (server, admin, discovery, limits, identity,
// authz): the module's request/backup bounds live under "limits_asset", never
// the framework "limits".
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	fconfig "github.com/go-freya/freya/config"
	"gopkg.in/yaml.v3"
)

// Config is the asset service configuration. The embedded framework config
// (inline) already carries service_name, trust_domain, env, identity, authz,
// limits, admin, discovery and server (grpc_addr/http_addr); the fields below
// are the module's own.
type Config struct {
	fconfig.Config `yaml:",inline"`

	DB          DB          `yaml:"db"`
	Valkey      Valkey      `yaml:"valkey"`
	KEK         KEK         `yaml:"kek"`
	ObjectStore ObjectStore `yaml:"object_store"`
	Inventory   Inventory   `yaml:"inventory"`
	Scheduler   Scheduler   `yaml:"scheduler"`
	Uploads     Uploads     `yaml:"uploads"`
	Events      Events      `yaml:"events"`
	Gateway     Gateway     `yaml:"gateway"`
	MeshEnroll  MeshEnroll  `yaml:"mesh_enroll"`
	Limits      Limits      `yaml:"limits_asset"`
}

// DB configures the PostgreSQL/TimescaleDB store.
type DB struct {
	DSN        string `yaml:"dsn"`
	MigrateDSN string `yaml:"migrate_dsn"`
	MaxConns   int32  `yaml:"max_conns"`
}

// Valkey configures the platform event bus and any shared cache.
type Valkey struct {
	Addresses      []string `yaml:"addresses"`
	Username       string   `yaml:"username"`
	Password       string   `yaml:"password"`
	AllowPlaintext bool     `yaml:"allow_plaintext"`
	CAFile         string   `yaml:"ca_file"`
}

// KEK names where the 32-byte key-encryption key (sealed supplier/location
// contact fields, object-store credentials) comes from.
type KEK struct {
	Source string `yaml:"source"` // file | env
	Path   string `yaml:"path"`
	Env    string `yaml:"env"`
}

// ObjectStore configures S3-compatible blob storage for asset photos and
// polymorphic documents. Access/secret keys are sealed at rest or read from env;
// never returned to callers.
type ObjectStore struct {
	Endpoint   string `yaml:"endpoint"`
	Bucket     string `yaml:"bucket"`
	Region     string `yaml:"region"`
	UseSSL     bool   `yaml:"use_ssl"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
	PresignTTL int    `yaml:"presign_ttl_seconds"`
}

// Inventory names the upstream inventory service the module previews/executes
// asset syncs against.
type Inventory struct {
	Service string `yaml:"service"`
}

// Scheduler bounds the lifecycle sweeper: how often it runs and how far ahead it
// warns of warranty/license/insurance expiry.
type Scheduler struct {
	IntervalSeconds   int `yaml:"interval_seconds"`
	WarrantySoonDays  int `yaml:"warranty_soon_days"`
	LicenseSoonDays   int `yaml:"license_soon_days"`
	InsuranceSoonDays int `yaml:"insurance_soon_days"`
}

// Uploads bounds inbound photo/document uploads.
type Uploads struct {
	MaxSizeBytes int64 `yaml:"max_size_bytes"`
}

// Events toggles the realtime publisher.
type Events struct {
	Enabled bool `yaml:"enabled"`
}

// Gateway names the application gateway and the platform token issuer.
type Gateway struct {
	Service string `yaml:"service"`
	Issuer  string `yaml:"issuer"`
}

// MeshEnroll configures how the asset SERVER obtains its own mesh SPIFFE SVID by
// enrolling with lcm over the network (identity.provider=provided). This is the
// module's own SVID enrollment.
type MeshEnroll struct {
	Enabled       bool   `yaml:"enabled"`
	EnrollURL     string `yaml:"enroll_url"`
	LCMGRPCTarget string `yaml:"lcm_grpc"`
	TenantID      string `yaml:"tenant_id"`
	TokenFile     string `yaml:"token_file"`
	StateFile     string `yaml:"state_file"`
	Insecure      bool   `yaml:"insecure"`
}

// Limits bound the module's request shapes. They live under "limits_asset" so
// they never collide with the framework's own "limits" section.
type Limits struct {
	MaxRequestBytes int64 `yaml:"max_request_bytes"`
	MaxBackupBytes  int64 `yaml:"max_backup_bytes"`
}

// SoonWindows are the look-ahead windows the lifecycle scheduler warns within.
type SoonWindows struct {
	Warranty  time.Duration
	License   time.Duration
	Insurance time.Duration
}

// Default returns secure defaults on top of the Freya defaults.
func Default() Config {
	return Config{
		Config:      fconfig.Default(),
		DB:          DB{MaxConns: 16},
		KEK:         KEK{Source: "file"},
		ObjectStore: ObjectStore{Region: "us-east-1", PresignTTL: 300},
		Inventory:   Inventory{Service: "inventory"},
		Scheduler:   Scheduler{IntervalSeconds: 3600, WarrantySoonDays: 30, LicenseSoonDays: 30, InsuranceSoonDays: 30},
		Uploads:     Uploads{MaxSizeBytes: 20 << 20},
		Events:      Events{Enabled: true},
		Gateway:     Gateway{Service: "gateway"},
		Limits:      Limits{MaxRequestBytes: 1 << 20, MaxBackupBytes: 32 << 20},
	}
}

// Load reads YAML over Default(); unknown fields are rejected.
func Load(path string) (Config, error) {
	cfg := Default()
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-supplied config path
	if err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("config: %s: %w", path, err)
	}
	return cfg, nil
}

// Validate checks the Freya config and every module section. It refuses a
// missing db, kek or object_store outright and enforces the production TLS guards.
func (c Config) Validate() error {
	if err := c.Config.Validate(); err != nil {
		return err
	}
	prod := c.IsProduction()
	if c.DB.DSN == "" {
		return errors.New("config: db.dsn is required")
	}
	if prod && !strings.Contains(c.DB.DSN, "sslmode=verify-full") && !strings.Contains(c.DB.DSN, "sslmode=verify-ca") {
		return errors.New("config: db.dsn must use sslmode=verify-full (or verify-ca) in production")
	}
	if len(c.Valkey.Addresses) == 0 {
		return errors.New("config: valkey.addresses is required")
	}
	if prod && c.Valkey.AllowPlaintext {
		return errors.New("config: valkey.allow_plaintext is not permitted in production")
	}
	switch c.KEK.Source {
	case "file":
		if c.KEK.Path == "" {
			return errors.New("config: kek.path is required for kek.source file")
		}
	case "env":
		if c.KEK.Env == "" {
			return errors.New("config: kek.env is required for kek.source env")
		}
	default:
		return errors.New("config: kek.source must be file or env")
	}
	if c.ObjectStore.Endpoint == "" || c.ObjectStore.Bucket == "" {
		return errors.New("config: object_store.endpoint and object_store.bucket are required")
	}
	if c.ObjectStore.PresignTTL < 30 || c.ObjectStore.PresignTTL > 3600 {
		return errors.New("config: object_store.presign_ttl_seconds must be within [30, 3600]")
	}
	if prod && !c.ObjectStore.UseSSL {
		return errors.New("config: object_store.use_ssl is required in production")
	}
	if c.Uploads.MaxSizeBytes < 1<<10 || c.Uploads.MaxSizeBytes > 1<<30 {
		return errors.New("config: uploads.max_size_bytes must be within [1 KiB, 1 GiB]")
	}
	if c.Scheduler.IntervalSeconds < 10 || c.Scheduler.IntervalSeconds > 86400 {
		return errors.New("config: scheduler.interval_seconds must be within [10, 86400]")
	}
	if c.Scheduler.WarrantySoonDays < 1 || c.Scheduler.WarrantySoonDays > 3650 {
		return errors.New("config: scheduler.warranty_soon_days must be within [1, 3650]")
	}
	if c.Scheduler.LicenseSoonDays < 1 || c.Scheduler.LicenseSoonDays > 3650 {
		return errors.New("config: scheduler.license_soon_days must be within [1, 3650]")
	}
	if c.Scheduler.InsuranceSoonDays < 1 || c.Scheduler.InsuranceSoonDays > 3650 {
		return errors.New("config: scheduler.insurance_soon_days must be within [1, 3650]")
	}
	if c.Gateway.Service == "" {
		return errors.New("config: gateway.service is required")
	}
	if iu, err := url.Parse(c.Gateway.Issuer); err != nil || iu.Scheme != "https" || iu.Host == "" {
		return errors.New("config: gateway.issuer must be an https origin")
	}
	if prod && c.MeshEnroll.Enabled && c.MeshEnroll.Insecure {
		return errors.New("config: mesh_enroll.insecure is not permitted in production")
	}
	if c.Limits.MaxRequestBytes < 1<<10 || c.Limits.MaxRequestBytes > 64<<20 {
		return errors.New("config: limits_asset.max_request_bytes must be within [1 KiB, 64 MiB]")
	}
	if c.Limits.MaxBackupBytes < 1<<10 || c.Limits.MaxBackupBytes > 256<<20 {
		return errors.New("config: limits_asset.max_backup_bytes must be within [1 KiB, 256 MiB]")
	}
	return nil
}

// Warnings lists accepted insecure opt-outs (surfaced at start).
func (c Config) Warnings() []string {
	w := c.Config.Warnings()
	if c.Valkey.AllowPlaintext {
		w = append(w, "valkey.allow_plaintext: event-bus traffic without TLS (development only)")
	}
	if !c.ObjectStore.UseSSL {
		w = append(w, "object_store.use_ssl=false: blob traffic without TLS (development only)")
	}
	if c.MeshEnroll.Enabled && c.MeshEnroll.Insecure {
		w = append(w, "mesh_enroll.insecure: SVID enrollment without TLS (development only)")
	}
	return w
}

// GRPCAddr is the mesh gRPC listener (framework server section).
func (c Config) GRPCAddr() string { return c.Config.Server.GRPCAddr }

// HTTPAddr is the mesh HTTP listener (framework server section).
func (c Config) HTTPAddr() string { return c.Config.Server.HTTPAddr }

// AdminAddr is the framework admin/operations listener.
func (c Config) AdminAddr() string { return c.Config.Admin.Addr }

// SchedulerInterval is the lifecycle sweeper tick.
func (c Config) SchedulerInterval() time.Duration {
	return time.Duration(c.Scheduler.IntervalSeconds) * time.Second
}

// PresignTTL is the lifetime of a presigned photo/document URL.
func (c Config) PresignTTL() time.Duration {
	return time.Duration(c.ObjectStore.PresignTTL) * time.Second
}

// SoonWindows returns the warranty/license/insurance expiry look-ahead windows.
func (c Config) SoonWindows() SoonWindows {
	day := 24 * time.Hour
	return SoonWindows{
		Warranty:  time.Duration(c.Scheduler.WarrantySoonDays) * day,
		License:   time.Duration(c.Scheduler.LicenseSoonDays) * day,
		Insurance: time.Duration(c.Scheduler.InsuranceSoonDays) * day,
	}
}
