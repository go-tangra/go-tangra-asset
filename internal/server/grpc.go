package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-asset/internal/cert"
	customLogging "github.com/go-tangra/go-tangra-asset/internal/middleware/logging"
	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/service"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"

	appViewer "github.com/go-tangra/go-tangra-common/viewer"
	"github.com/go-tangra/go-tangra-common/middleware/audit"
	"github.com/go-tangra/go-tangra-common/middleware/mtls"
)

// NewGrpcWhiteListMatcher defines public endpoints that don't require authentication
func NewGrpcWhiteListMatcher() selector.MatchFunc {
	whiteList := make(map[string]bool)
	whiteList["/asset.service.v1.SystemService/HealthCheck"] = true

	return func(ctx context.Context, operation string) bool {
		if _, ok := whiteList[operation]; ok {
			return false
		}
		return true
	}
}

// systemViewerMiddleware injects system viewer context for all requests
func systemViewerMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			ctx = appViewer.NewSystemViewerContext(ctx)
			return handler(ctx, req)
		}
	}
}

// NewGRPCServer creates a gRPC server with mTLS and audit logging
func NewGRPCServer(
	ctx *bootstrap.Context,
	certManager *cert.CertManager,
	auditLogRepo *data.AuditLogRepo,
	systemSvc *service.SystemService,
	supplierSvc *service.SupplierService,
	employeeSvc *service.EmployeeService,
	locationSvc *service.LocationService,
	categorySvc *service.CategoryService,
	assetSvc *service.AssetService,
	consumableSvc *service.ConsumableService,
	licenseSvc *service.LicenseService,
) *grpc.Server {
	cfg := ctx.GetConfig()
	logger := ctx.GetLogger()
	l := ctx.NewLoggerHelper("asset/grpc")

	var opts []grpc.ServerOption

	// Get gRPC server configuration
	if cfg.Server != nil && cfg.Server.Grpc != nil {
		opts = append(opts, grpc.Address(cfg.Server.Grpc.Addr))
		opts = append(opts, grpc.Timeout(cfg.Server.Grpc.Timeout.AsDuration()))
	}

	// Configure TLS if certificates are available
	tlsEnabled := false
	if certManager != nil && certManager.IsTLSEnabled() {
		tlsConfig, err := certManager.GetServerTLSConfig()
		if err != nil {
			l.Warnf("Failed to get TLS config, running without TLS: %v", err)
		} else {
			opts = append(opts, grpc.TLSConfig(tlsConfig))
			l.Info("gRPC server configured with mTLS")
			tlsEnabled = true
		}
	} else {
		l.Warn("TLS not enabled, running without mTLS")
	}

	// Build middleware stack
	var ms []middleware.Middleware
	ms = append(ms, recovery.Recovery())
	ms = append(ms, systemViewerMiddleware())
	ms = append(ms, customLogging.RedactedServer(logger))

	// Add mTLS middleware only when TLS is enabled
	if tlsEnabled {
		ms = append(ms, mtls.MTLSMiddleware(
			logger,
			mtls.WithPublicEndpoints(
				"/grpc.health.v1.Health/Check",
				"/grpc.health.v1.Health/Watch",
				"/asset.service.v1.SystemService/HealthCheck",
			),
		))
	} else {
		l.Warn("mTLS middleware disabled (TLS not configured)")
	}

	// Add audit logging middleware
	ms = append(ms, audit.Server(
		logger,
		audit.WithServiceName("asset-service"),
		audit.WithWriteAuditLogFunc(func(ctx context.Context, log *audit.AuditLog) error {
			return auditLogRepo.CreateFromEntry(ctx, log.ToEntry())
		}),
		audit.WithSkipOperations(
			"/grpc.health.v1.Health/Check",
			"/grpc.health.v1.Health/Watch",
			"/asset.service.v1.SystemService/HealthCheck",
		),
	))

	ms = append(ms, validate.Validator())

	opts = append(opts, grpc.Middleware(ms...))

	srv := grpc.NewServer(opts...)

	// Register services with redacted wrappers
	assetV1.RegisterRedactedSystemServiceServer(srv, systemSvc, nil)
	assetV1.RegisterRedactedSupplierServiceServer(srv, supplierSvc, nil)
	assetV1.RegisterRedactedEmployeeServiceServer(srv, employeeSvc, nil)
	assetV1.RegisterRedactedLocationServiceServer(srv, locationSvc, nil)
	assetV1.RegisterRedactedCategoryServiceServer(srv, categorySvc, nil)
	assetV1.RegisterRedactedAssetServiceServer(srv, assetSvc, nil)
	assetV1.RegisterRedactedConsumableServiceServer(srv, consumableSvc, nil)
	assetV1.RegisterRedactedLicenseServiceServer(srv, licenseSvc, nil)

	return srv
}
