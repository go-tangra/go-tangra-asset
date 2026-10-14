package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-tangra/go-tangra-asset/v4/internal/config"
)

var placeholderRE = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)

// joinValues are values of the shape the gateway fills in: core values,
// generated secrets and the host inputs.
var joinValues = map[string]string{
	"TRUST_DOMAIN": "infra.example.org", "GATEWAY_ISSUER": "https://portal.example.org:8443",
	"LCM_ENROLL_URL": "https://portal.example.org:8443/api/lcm/v1/enroll", "LCM_GRPC": "portal.example.org:9945",
	"GATEWAY_GRPC": "portal.example.org:9643", "AUTH_GRPC": "portal.example.org:9543", "MESH_TENANT_ID": "00000000-0000-0000-0000-000000000001",
	"MODULE_IMAGE": "ghcr.io/go-tangra/go-tangra-asset", "MODULE_VERSION": "4.4.0",
	"GEN_PASSWORD_1": strings.Repeat("a", 48), "GEN_PASSWORD_2": strings.Repeat("b", 48), "GEN_PASSWORD_3": strings.Repeat("d", 48),
	"GEN_KEY_1":             "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
	"MODULE_ADVERTISE_HOST": "asset.example.org", "MODULE_BIND_IP": "10.0.0.5",
	"VALKEY_ADDRESS": "valkey.example.org:6380", "VALKEY_PASSWORD": strings.Repeat("c", 32), "VALKEY_CA_FILE": "/etc/tangra/valkey-ca.pem",
}

func render(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "bundle", name))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	rendered := placeholderRE.ReplaceAllStringFunc(string(raw), func(m string) string {
		k := placeholderRE.FindStringSubmatch(m)[1]
		v, ok := joinValues[k]
		if !ok {
			missing = append(missing, k)
		}
		return v
	})
	if len(missing) > 0 {
		t.Fatalf("%s uses placeholders the gateway does not fill: %v", name, missing)
	}
	return rendered
}

// The descriptor's host inputs accept values of the shape operators give,
// and every bundle host it asks TLS material for is a service of the bundle.
func TestCatalogueDescriptorHostInputs(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tangra-module.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Module string `yaml:"module"`
		Bundle struct {
			TLSHosts []string `yaml:"tls_hosts"`
		} `yaml:"bundle"`
		HostInputs []struct {
			Key     string `yaml:"key"`
			Pattern string `yaml:"pattern"`
		} `yaml:"host_inputs"`
	}
	if err := yaml.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Module != "asset" {
		t.Fatal(d.Module)
	}
	keys := map[string]bool{}
	for _, in := range d.HostInputs {
		keys[in.Key] = true
		if !regexp.MustCompile(in.Pattern).MatchString(joinValues[in.Key]) {
			t.Fatalf("host input %s refuses %q", in.Key, joinValues[in.Key])
		}
	}
	for _, k := range []string{"VALKEY_ADDRESS", "VALKEY_PASSWORD", "VALKEY_CA_FILE"} {
		if !keys[k] {
			t.Fatalf("host input %s missing: asset must reach the core's Valkey", k)
		}
	}
	compose := render(t, "compose.yaml")
	for _, h := range d.Bundle.TLSHosts {
		if !strings.Contains(compose, "\n  "+h+":\n") {
			t.Fatalf("tls_hosts names %s, which is not a bundle service", h)
		}
	}
}

// The join bundle's templates, rendered with values of the shape the gateway
// fills in, are a valid production configuration and policy.
func TestJoinBundleRendersToValidProductionConfig(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"config.yaml", "policy.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(render(t, name)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	compose := render(t, "compose.yaml")
	if strings.Contains(compose, "valkey/valkey") || !strings.Contains(compose, "/etc/tangra/valkey-ca.pem:/tls/valkey-ca.pem:ro") {
		t.Fatal("the bundle must use the core's Valkey (mounting its CA), not run its own")
	}
	cfg, err := config.Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("rendered config invalid: %v", err)
	}
	if !cfg.IsProduction() || cfg.Gateway.Issuer != "https://portal.example.org:8443" || !cfg.ObjectStore.UseSSL || !cfg.Events.Enabled {
		t.Fatalf("%+v %+v %+v", cfg.Gateway, cfg.ObjectStore.UseSSL, cfg.Events)
	}
	// Events go to the core's Valkey, where the gateway's SSE hub reads them.
	if cfg.Valkey.AllowPlaintext || len(cfg.Valkey.Addresses) != 1 || cfg.Valkey.Addresses[0] != "valkey.example.org:6380" ||
		cfg.Valkey.Username != "asset" || cfg.Valkey.Password != joinValues["VALKEY_PASSWORD"] || cfg.Valkey.CAFile != "/tls/valkey-ca.pem" {
		t.Fatalf("%+v", cfg.Valkey.Addresses)
	}
	// The only accepted warning is the raised request bound uploads need.
	for _, w := range cfg.Warnings() {
		if !strings.HasPrefix(w, "limits.max_request_bytes") {
			t.Fatalf("insecure opt-out in the bundle: %s", w)
		}
	}
	pol, _ := os.ReadFile(filepath.Join(dir, "policy.yaml"))
	if !strings.Contains(string(pol), "spiffe://infra.example.org/svc/gateway") {
		t.Fatal(string(pol))
	}
}
