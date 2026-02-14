package service

import (
	"context"
	"strconv"

	"github.com/go-kratos/kratos/v2/transport"
)

// getTenantID extracts tenant_id from gRPC metadata
func getTenantID(ctx context.Context) uint32 {
	if tr, ok := transport.FromServerContext(ctx); ok {
		if v := tr.RequestHeader().Get("x-md-global-tenant-id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				return uint32(id)
			}
		}
	}
	return 0
}

// getUserID extracts user_id from gRPC metadata
func getUserID(ctx context.Context) uint32 {
	if tr, ok := transport.FromServerContext(ctx); ok {
		if v := tr.RequestHeader().Get("x-md-global-user-id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				return uint32(id)
			}
		}
	}
	return 0
}

// getUsername extracts username from gRPC metadata
func getUsername(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.RequestHeader().Get("x-md-global-username")
	}
	return ""
}
