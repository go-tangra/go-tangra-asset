package data

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	grpcMD "google.golang.org/grpc/metadata"

	collectorv1 "github.com/go-tangra/go-tangra-inventory/gen/go/inventory/collector/v1"
)

// InventoryClient wraps a gRPC client for the inventory collector service
type InventoryClient struct {
	conn      *grpc.ClientConn
	log       *log.Helper
	endpoint  string
	apiSecret string
	client    collectorv1.InventoryCollectorServiceClient
}

// NewInventoryClient creates a new inventory collector gRPC client
func NewInventoryClient(ctx *bootstrap.Context) (*InventoryClient, func(), error) {
	l := ctx.NewLoggerHelper("inventory/client/asset-service")

	endpoint := os.Getenv("ASSET_INVENTORY_GRPC_ENDPOINT")
	if endpoint == "" {
		l.Info("ASSET_INVENTORY_GRPC_ENDPOINT not set, inventory sync disabled")
		return &InventoryClient{log: l}, func() {}, nil
	}

	apiSecret := os.Getenv("ASSET_INVENTORY_API_SECRET")

	l.Infof("Connecting to inventory collector at: %s", endpoint)

	connectParams := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.5,
			Jitter:     0.2,
			MaxDelay:   30 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}

	keepaliveParams := keepalive.ClientParameters{
		Time:                5 * time.Minute,
		Timeout:             20 * time.Second,
		PermitWithoutStream: false,
	}

	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(connectParams),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithDefaultServiceConfig(`{
			"loadBalancingConfig": [{"round_robin":{}}],
			"methodConfig": [{
				"name": [{"service": ""}],
				"waitForReady": true,
				"retryPolicy": {
					"MaxAttempts": 3,
					"InitialBackoff": "0.5s",
					"MaxBackoff": "5s",
					"BackoffMultiplier": 2,
					"RetryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
				}
			}]
		}`),
	)
	if err != nil {
		l.Errorf("Failed to connect to inventory collector: %v", err)
		return nil, func() {}, err
	}

	ic := &InventoryClient{
		conn:      conn,
		log:       l,
		endpoint:  endpoint,
		apiSecret: apiSecret,
		client:    collectorv1.NewInventoryCollectorServiceClient(conn),
	}

	cleanup := func() {
		if err := conn.Close(); err != nil {
			l.Errorf("Failed to close inventory collector connection: %v", err)
		}
	}

	l.Info("Inventory collector client initialized successfully")
	return ic, cleanup, nil
}

// IsConfigured returns true if the inventory collector endpoint is set
func (c *InventoryClient) IsConfigured() bool {
	return c != nil && c.conn != nil
}

// withAuth appends x-api-secret metadata to the outgoing context when configured.
func (c *InventoryClient) withAuth(ctx context.Context) context.Context {
	if c.apiSecret != "" {
		ctx = grpcMD.AppendToOutgoingContext(ctx, "x-api-secret", c.apiSecret)
	}
	return ctx
}

// FetchInventories fetches all inventory summaries from the collector
func (c *InventoryClient) FetchInventories(ctx context.Context) ([]*collectorv1.InventorySummary, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("inventory collector is not configured")
	}

	resp, err := c.client.ListInventories(c.withAuth(ctx), &collectorv1.ListInventoriesRequest{
		PageSize: 10000,
	})
	if err != nil {
		c.log.Errorf("failed to list inventories: %v", err)
		return nil, fmt.Errorf("failed to list inventories: %w", err)
	}

	c.log.Infof("fetched %d inventory summaries", len(resp.GetInventories()))
	return resp.GetInventories(), nil
}

// FetchLatestByHostname fetches the latest full inventory for a specific hostname
func (c *InventoryClient) FetchLatestByHostname(ctx context.Context, hostname string) (*collectorv1.Inventory, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("inventory collector is not configured")
	}

	resp, err := c.client.GetLatestByHostname(c.withAuth(ctx), &collectorv1.GetLatestByHostnameRequest{
		Hostname: hostname,
	})
	if err != nil {
		c.log.Errorf("failed to fetch inventory for %s: %v", hostname, err)
		return nil, fmt.Errorf("failed to fetch inventory for %s: %w", hostname, err)
	}

	return resp.GetInventory(), nil
}
