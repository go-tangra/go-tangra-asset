// Package assetmanifest declares what the asset (ITAM) module registers with
// the application gateway: routes derived from the embedded OpenAPI document,
// the API permissions, the CASL abilities and the navigation entries. The asset
// gRPC surface is service-to-service and is not proxied by the gateway.
package assetmanifest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/grpc"

	"github.com/go-freya/freya/services/asset/api/openapi"
	authv1 "github.com/go-freya/freya/services/auth/api/proto/auth/v1"
	"github.com/go-freya/freya/services/gateway/pkg/gatewayclient"
)

// Module identity.
const (
	Module       = "asset"
	DisplayName  = "Assets"
	Version      = "1.0.0"
	RemotePrefix = "/ui"
)

// OpenAPI operation extensions.
const (
	PermissionExtension = "x-freya-permission"
	PublicExtension     = "x-freya-public"
	BodyLimitExtension  = "x-freya-max-body-bytes"
	TimeoutExtension    = "x-freya-timeout-seconds"
)

// Permissions the module registers.
var Permissions = []gatewayclient.Permission{
	{Resource: "assets", Action: "read", Description: "List and read assets, org records, documents, users and the live stream"},
	{Resource: "assets", Action: "manage", Description: "Create, update and delete assets"},
	{Resource: "assets", Action: "assign", Description: "Assign and unassign assets to users"},
	{Resource: "categories", Action: "manage", Description: "Manage the category tree"},
	{Resource: "suppliers", Action: "manage", Description: "Manage suppliers"},
	{Resource: "locations", Action: "manage", Description: "Manage the location tree"},
	{Resource: "consumables", Action: "manage", Description: "Manage consumables (stock)"},
	{Resource: "licenses", Action: "manage", Description: "Manage software licenses"},
	{Resource: "insurance", Action: "manage", Description: "Manage insurance policies and asset coverage"},
	{Resource: "documents", Action: "manage", Description: "Upload and delete photos and documents"},
	{Resource: "inventory", Action: "sync", Description: "Preview and execute inventory synchronisation"},
	{Resource: "stats", Action: "read", Description: "Read the asset dashboard statistics"},
	{Resource: "backup", Action: "manage", Description: "Export and import tenant asset data"},
}

// Grants maps built-in role slugs to the permissions they hold.
var Grants = map[string][]string{
	"owner":    PermissionRefs(),
	"admin":    PermissionRefs(),
	"member":   {"assets:read", "stats:read"},
	"auditor":  {"assets:read", "stats:read"},
	"operator": {"assets:read", "assets:manage", "assets:assign", "categories:manage", "suppliers:manage", "locations:manage", "consumables:manage", "licenses:manage", "insurance:manage", "documents:manage", "inventory:sync", "stats:read"},
}

// Methods proxied by the gateway: none (asset gRPC is service to service).
var Methods []gatewayclient.Method

// Abilities are the CASL rules bound to the permissions.
var Abilities = []gatewayclient.Ability{
	{Action: []string{"read"}, Subject: []string{"Asset", "AssetCategory", "AssetSupplier", "AssetLocation", "AssetConsumable", "AssetLicense", "AssetInsurance"}, Requires: "assets:read"},
	{Action: []string{"create", "update", "delete"}, Subject: []string{"Asset"}, Requires: "assets:manage"},
	{Action: []string{"assign"}, Subject: []string{"Asset"}, Requires: "assets:assign"},
	{Action: []string{"manage"}, Subject: []string{"AssetCategory"}, Requires: "categories:manage"},
	{Action: []string{"manage"}, Subject: []string{"AssetSupplier"}, Requires: "suppliers:manage"},
	{Action: []string{"manage"}, Subject: []string{"AssetLocation"}, Requires: "locations:manage"},
	{Action: []string{"manage"}, Subject: []string{"AssetConsumable"}, Requires: "consumables:manage"},
	{Action: []string{"manage"}, Subject: []string{"AssetLicense"}, Requires: "licenses:manage"},
	{Action: []string{"manage"}, Subject: []string{"AssetInsurance"}, Requires: "insurance:manage"},
	{Action: []string{"manage"}, Subject: []string{"AssetDocument"}, Requires: "documents:manage"},
	{Action: []string{"sync"}, Subject: []string{"AssetInventory"}, Requires: "inventory:sync"},
	{Action: []string{"read"}, Subject: []string{"AssetStats"}, Requires: "stats:read"},
	{Action: []string{"manage"}, Subject: []string{"AssetBackup"}, Requires: "backup:manage"},
}

// Nav lists the navigation contributions.
var Nav = []gatewayclient.NavEntry{
	{Title: "Assets", Path: "/asset", Icon: "mdi-laptop", Order: 700, Requires: "assets:read"},
	{Title: "Categories", Path: "/asset/categories", Icon: "mdi-shape-outline", Order: 710, Requires: "assets:read"},
	{Title: "Suppliers", Path: "/asset/suppliers", Icon: "mdi-truck-outline", Order: 720, Requires: "assets:read"},
	{Title: "Locations", Path: "/asset/locations", Icon: "mdi-map-marker-outline", Order: 730, Requires: "assets:read"},
	{Title: "Consumables", Path: "/asset/consumables", Icon: "mdi-package-variant", Order: 740, Requires: "assets:read"},
	{Title: "Licenses", Path: "/asset/licenses", Icon: "mdi-license", Order: 750, Requires: "assets:read"},
	{Title: "Insurance", Path: "/asset/insurance", Icon: "mdi-shield-check-outline", Order: 760, Requires: "assets:read"},
	{Title: "Inventory Sync", Path: "/asset/inventory-sync", Icon: "mdi-sync", Order: 770, Requires: "inventory:sync"},
	{Title: "Dashboard", Path: "/asset/dashboard", Icon: "mdi-view-dashboard-outline", Order: 780, Requires: "stats:read"},
}

// PermissionRefs lists "resource:action" for every declared permission.
func PermissionRefs() []string {
	out := make([]string, 0, len(Permissions))
	for _, p := range Permissions {
		out = append(out, p.Resource+":"+p.Action)
	}
	return out
}

// Routes derives the gateway routes from the OpenAPI document.
func Routes(doc *openapi3.T) ([]gatewayclient.Route, error) {
	known := map[string]bool{}
	for _, p := range PermissionRefs() {
		known[p] = true
	}
	var routes []gatewayclient.Route
	for p, item := range doc.Paths.Map() {
		for m, op := range item.Operations() {
			r := gatewayclient.Route{Method: strings.ToUpper(m), Path: p}
			perm, _ := op.Extensions[PermissionExtension].(string)
			public, _ := op.Extensions[PublicExtension].(bool)
			switch {
			case public:
				r.Public = true
			case perm == "":
				return nil, fmt.Errorf("assetmanifest: %s %s declares no permission", r.Method, p)
			case !known[perm]:
				return nil, fmt.Errorf("assetmanifest: %s %s uses undeclared permission %q", r.Method, p, perm)
			default:
				r.Permission = perm
			}
			if v, ok := op.Extensions[BodyLimitExtension]; ok {
				n, ok := v.(float64)
				if !ok || n <= 0 {
					return nil, fmt.Errorf("assetmanifest: %s %s has a bad body limit", r.Method, p)
				}
				r.MaxBodyBytes = uint64(n)
			}
			if v, ok := op.Extensions[TimeoutExtension]; ok {
				n, ok := v.(float64)
				if !ok || n <= 0 || n > 600 {
					return nil, fmt.Errorf("assetmanifest: %s %s has a bad timeout", r.Method, p)
				}
				r.Timeout = time.Duration(n) * time.Second
			}
			routes = append(routes, r)
		}
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Method+" "+routes[i].Path < routes[j].Method+" "+routes[j].Path })
	return routes, nil
}

// Load parses the embedded document.
func Load() (*openapi3.T, error) {
	return openapi3.NewLoader().LoadFromData(openapi.Asset)
}

// Manifest builds the gateway manifest from the embedded OpenAPI document.
func Manifest() (gatewayclient.Manifest, error) {
	doc, err := Load()
	if err != nil {
		return gatewayclient.Manifest{}, err
	}
	routes, err := Routes(doc)
	if err != nil {
		return gatewayclient.Manifest{}, err
	}
	return gatewayclient.Manifest{
		Module: Module, DisplayName: DisplayName, Version: Version,
		Prefixes:    []string{"/api/asset"},
		Routes:      routes,
		Methods:     Methods,
		Permissions: Permissions,
		Abilities:   Abilities,
		Exposes:     []string{"./routes", "./nav"},
		Nav:         Nav,
	}, nil
}

// SeedRequest builds the auth registration request: every module permission plus
// the built-in role grants.
func SeedRequest() *authv1.RegisterPermissionsRequest {
	req := &authv1.RegisterPermissionsRequest{}
	for _, p := range Permissions {
		req.Permissions = append(req.Permissions, &authv1.PermissionDef{Resource: p.Resource, Action: p.Action, Description: p.Description})
	}
	for _, slug := range []string{"owner", "admin", "member", "auditor", "operator"} {
		req.BuiltinGrants = append(req.BuiltinGrants, &authv1.BuiltinGrant{Role: slug, Permissions: Grants[slug]})
	}
	return req
}

// SeedPermissions registers the module's permissions with the auth service and
// grants them to the built-in roles (idempotent). The gateway registers the
// permissions from the manifest for routing; only the asset module knows the
// role grants, so it pushes them to auth here.
func SeedPermissions(ctx context.Context, cc grpc.ClientConnInterface) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := authv1.NewAuthorizationClient(cc).RegisterPermissions(ctx, SeedRequest())
	return err
}
