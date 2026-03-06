package service

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	grpcMD "google.golang.org/grpc/metadata"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

type UserService struct {
	assetV1.UnimplementedUserServiceServer

	log         *log.Helper
	adminClient *data.AdminClient
}

func NewUserService(ctx *bootstrap.Context, adminClient *data.AdminClient) *UserService {
	return &UserService{
		log:         ctx.NewLoggerHelper("asset/service/user"),
		adminClient: adminClient,
	}
}

func (s *UserService) ListUsers(ctx context.Context, _ *assetV1.ListUsersRequest) (*assetV1.ListUsersResponse, error) {
	// Forward tenant/user metadata to admin-service
	outCtx := forwardMetadata(ctx)

	resp, err := s.adminClient.ListUsers(outCtx)
	if err != nil {
		return nil, assetV1.ErrorInternalServerError("failed to list portal users: %v", err)
	}

	items := make([]*assetV1.PortalUser, 0, len(resp.Items))
	for _, u := range resp.Items {
		items = append(items, &assetV1.PortalUser{
			Id:            u.Id,
			Username:      u.Username,
			Realname:      u.Realname,
			Email:         u.Email,
			OrgUnitNames:  u.OrgUnitNames,
			PositionNames: u.PositionNames,
		})
	}

	return &assetV1.ListUsersResponse{
		Items: items,
		Total: ptrInt32(int32(len(items))),
	}, nil
}

// forwardMetadata propagates auth metadata to outgoing gRPC calls
func forwardMetadata(ctx context.Context) context.Context {
	outMD := grpcMD.MD{}

	if tr, ok := transport.FromServerContext(ctx); ok {
		for _, key := range []string{
			"x-md-global-tenant-id",
			"x-md-global-user-id",
			"x-md-global-username",
			"x-md-global-roles",
		} {
			if v := tr.RequestHeader().Get(key); v != "" {
				outMD.Set(key, v)
			}
		}
	}

	// Ensure tenant_id is set
	tenantID := getTenantID(ctx)
	if tenantID > 0 {
		outMD.Set("x-md-global-tenant-id", fmt.Sprintf("%d", tenantID))
	}

	return grpcMD.NewOutgoingContext(ctx, outMD)
}
