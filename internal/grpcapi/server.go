// Package grpcapi serves asset.v1 for other platform services on the Freya
// SPIFFE mTLS channel: the caller is an authenticated service acting for the
// tenant named in the request. Nothing here is proxied by the gateway. The
// tenant comes from the request and the actor identity from the verified SPIFFE
// peer. Responses never carry object-store credentials; sealed contact fields
// are blank for service callers (they are not tenant admins).
package grpcapi

import (
	"context"
	"errors"
	"regexp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/go-freya/freya/authn"
	assetv1 "github.com/go-freya/freya/services/asset/api/proto/asset/v1"
	"github.com/go-freya/freya/services/asset/internal/assets"
	"github.com/go-freya/freya/services/asset/internal/authz"
	"github.com/go-freya/freya/services/asset/internal/categories"
	"github.com/go-freya/freya/services/asset/internal/consumables"
	"github.com/go-freya/freya/services/asset/internal/documents"
	"github.com/go-freya/freya/services/asset/internal/insurance"
	"github.com/go-freya/freya/services/asset/internal/invsync"
	"github.com/go-freya/freya/services/asset/internal/licenses"
	"github.com/go-freya/freya/services/asset/internal/locations"
	"github.com/go-freya/freya/services/asset/internal/repo"
	"github.com/go-freya/freya/services/asset/internal/stats"
	"github.com/go-freya/freya/services/asset/internal/suppliers"
	"github.com/go-freya/freya/services/asset/internal/userdir"
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// callerFunc resolves the SPIFFE identity of a call (overridable in tests).
var callerFunc = func(ctx context.Context) (string, bool) {
	p, ok := authn.FromContext(ctx)
	if !ok {
		return "", false
	}
	return p.ID.String(), true
}

// caller returns the service subjects for the tenant named in the request; the
// tenant must be a uuid and the peer must present a SPIFFE identity.
func caller(ctx context.Context, tenantID string) (authz.Subjects, error) {
	id, ok := callerFunc(ctx)
	if !ok {
		return authz.Subjects{}, status.Error(codes.Unauthenticated, "service identity required")
	}
	if !uuidRE.MatchString(tenantID) {
		return authz.Subjects{}, status.Error(codes.InvalidArgument, "tenant_id must be a uuid")
	}
	return authz.Subjects{TenantID: tenantID, UserID: id, ActorKind: authz.ActorService}, nil
}

// grpcError maps a service/domain error to a gRPC status. Detail is never
// surfaced beyond the stable reason string.
func grpcError(err error) error {
	var (
		av assets.ValidationError
		cv categories.ValidationError
		sv suppliers.ValidationError
		lv locations.ValidationError
		kv consumables.ValidationError
		xv licenses.ValidationError
		iv insurance.ValidationError
		dv documents.ValidationError
	)
	switch {
	case errors.Is(err, assets.ErrNotFound), errors.Is(err, categories.ErrNotFound), errors.Is(err, suppliers.ErrNotFound),
		errors.Is(err, locations.ErrNotFound), errors.Is(err, consumables.ErrNotFound), errors.Is(err, licenses.ErrNotFound),
		errors.Is(err, insurance.ErrNotFound), errors.Is(err, documents.ErrNotFound), errors.Is(err, repo.ErrNotFound):
		return status.Error(codes.NotFound, "not_found")
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, documents.ErrCrossTenant):
		return status.Error(codes.PermissionDenied, "forbidden")
	case errors.As(err, &av), errors.As(err, &cv), errors.As(err, &sv), errors.As(err, &lv), errors.As(err, &kv),
		errors.As(err, &xv), errors.As(err, &iv), errors.As(err, &dv), errors.Is(err, documents.ErrEntityType), errors.Is(err, documents.ErrMediaType):
		return status.Error(codes.InvalidArgument, "validation_failed")
	case errors.Is(err, assets.ErrConflict), errors.Is(err, assets.ErrState), errors.Is(err, categories.ErrConflict), errors.Is(err, categories.ErrInUse),
		errors.Is(err, suppliers.ErrConflict), errors.Is(err, suppliers.ErrInUse), errors.Is(err, locations.ErrConflict), errors.Is(err, locations.ErrInUse),
		errors.Is(err, insurance.ErrConflict), errors.Is(err, repo.ErrConflict), errors.Is(err, repo.ErrNotEmpty):
		return status.Error(codes.FailedPrecondition, "conflict")
	case errors.Is(err, documents.ErrTooLarge):
		return status.Error(codes.ResourceExhausted, "body_too_large")
	case errors.Is(err, invsync.ErrUnavailable):
		return status.Error(codes.Unavailable, "inventory_unavailable")
	}
	return status.Error(codes.Unavailable, "temporarily_unavailable")
}

// Deps carries the services the asset.v1 servers use.
type Deps struct {
	Assets      *assets.Service
	Categories  *categories.Service
	Suppliers   *suppliers.Service
	Locations   *locations.Service
	Consumables *consumables.Service
	Licenses    *licenses.Service
	Insurance   *insurance.Service
	Documents   *documents.Service
	Sync        *invsync.Service
	Stats       *stats.Service
	Users       userdir.Directory
	Health      func() map[string]string
}

// Register registers the asset.v1 mesh servers on the gRPC server. Callers
// are authenticated services; nothing here is gateway-proxied.
func Register(gs grpc.ServiceRegistrar, d Deps) {
	if d.Assets != nil {
		assetv1.RegisterAssetServiceServer(gs, &AssetServer{d: d})
	}
	if d.Categories != nil {
		assetv1.RegisterCategoryServiceServer(gs, &CategoryServer{d: d})
	}
	if d.Suppliers != nil {
		assetv1.RegisterSupplierServiceServer(gs, &SupplierServer{d: d})
	}
	if d.Locations != nil {
		assetv1.RegisterLocationServiceServer(gs, &LocationServer{d: d})
	}
	if d.Consumables != nil {
		assetv1.RegisterConsumableServiceServer(gs, &ConsumableServer{d: d})
	}
	if d.Licenses != nil {
		assetv1.RegisterLicenseServiceServer(gs, &LicenseServer{d: d})
	}
	if d.Insurance != nil {
		assetv1.RegisterInsurancePolicyServiceServer(gs, &InsuranceServer{d: d})
	}
	assetv1.RegisterUserServiceServer(gs, &UserServer{d: d})
	assetv1.RegisterSystemServiceServer(gs, &SystemServer{d: d})
}

// errUnavailableSync is returned for RPCs whose optional service is not wired.
var errUnavailableSync = errors.New("grpcapi: service not wired")
