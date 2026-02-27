package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

// InventorySummary represents a summary entry from the inventory collector
type InventorySummary struct {
	ID           int64  `json:"id"`
	Hostname     string `json:"hostname"`
	Username     string `json:"username"`
	SystemUUID   string `json:"systemUuid"`
	SystemSerial string `json:"systemSerial"`
	CollectedAt  string `json:"collectedAt"`
}

// InventoryDetail represents a full inventory record from the collector
type InventoryDetail struct {
	ID        int64         `json:"id"`
	Inventory InventoryData `json:"inventory"`
}

// InventoryData contains the full SMBIOS inventory
type InventoryData struct {
	Hostname   string          `json:"hostname"`
	System     *SystemInfo     `json:"system"`
	BIOS       *BIOSInfo       `json:"bios"`
	Baseboard  *BaseboardInfo  `json:"baseboard"`
	Chassis    *ChassisInfo    `json:"chassis"`
	Processors []ProcessorInfo `json:"processors"`
	Memory     *MemoryInfo     `json:"memory"`
	Monitors   []MonitorInfo   `json:"monitor"`
}

// SystemInfo represents SMBIOS system information
type SystemInfo struct {
	Manufacturer string `json:"manufacturer"`
	ProductName  string `json:"product_name"`
	Version      string `json:"version"`
	SerialNumber string `json:"serial_number"`
	UUID         string `json:"uuid"`
	SKUNumber    string `json:"sku_number"`
	Family       string `json:"family"`
}

// BIOSInfo represents SMBIOS BIOS information
type BIOSInfo struct {
	Vendor      string `json:"vendor"`
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
}

// BaseboardInfo represents SMBIOS baseboard information
type BaseboardInfo struct {
	Manufacturer string `json:"manufacturer"`
	Product      string `json:"product"`
	Version      string `json:"version"`
	SerialNumber string `json:"serial_number"`
}

// ChassisInfo represents SMBIOS chassis information
type ChassisInfo struct {
	Manufacturer string `json:"manufacturer"`
	Type         string `json:"type"`
	SerialNumber string `json:"serial_number"`
	AssetTag     string `json:"asset_tag"`
}

// ProcessorInfo represents SMBIOS processor information
type ProcessorInfo struct {
	SocketDesignation string `json:"socket_designation"`
	Manufacturer      string `json:"manufacturer"`
	Version           string `json:"version"`
	MaxSpeed          string `json:"max_speed"`
	CurrentSpeed      string `json:"current_speed"`
	CoreCount         int    `json:"core_count"`
	ThreadCount       int    `json:"thread_count"`
}

// MemoryInfo represents SMBIOS memory information
type MemoryInfo struct {
	TotalCapacity string             `json:"total_capacity"`
	Devices       []MemoryDeviceInfo `json:"devices"`
}

// MemoryDeviceInfo represents a single memory device
type MemoryDeviceInfo struct {
	Size         string `json:"size"`
	Type         string `json:"type"`
	Speed        string `json:"speed"`
	Manufacturer string `json:"manufacturer"`
	SerialNumber string `json:"serial_number"`
	Locator      string `json:"locator"`
}

// MonitorInfo represents monitor/display information
type MonitorInfo struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	SerialNumber string `json:"serial_number"`
	ProductID    string `json:"product_id"`
	YearOfMfg    int    `json:"year_of_manufacture"`
}

// InventoryClient handles HTTP communication with the inventory collector service
type InventoryClient struct {
	baseURL   string
	apiSecret string
	client    *http.Client
	log       *log.Helper
}

// NewInventoryClient creates a new inventory collector client from environment variables
func NewInventoryClient(ctx *bootstrap.Context) *InventoryClient {
	return &InventoryClient{
		baseURL:   getEnvOrDefault("ASSET_INVENTORY_URL", ""),
		apiSecret: getEnvOrDefault("ASSET_INVENTORY_API_SECRET", ""),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: ctx.NewLoggerHelper("inventory/data/asset-service"),
	}
}

// IsConfigured returns true if the inventory collector URL is set
func (c *InventoryClient) IsConfigured() bool {
	return c.baseURL != ""
}

// FetchInventories fetches all inventory summaries from the collector
func (c *InventoryClient) FetchInventories(ctx context.Context) ([]InventorySummary, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("inventory collector is not configured")
	}

	url := fmt.Sprintf("%s/v1/inventories?page_size=10000", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiSecret != "" {
		req.Header.Set("X-API-Key", c.apiSecret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Errorf("failed to fetch inventories: %v", err)
		return nil, fmt.Errorf("failed to fetch inventories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Errorf("inventory API returned status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("inventory API returned status %d", resp.StatusCode)
	}

	var result struct {
		Items []InventorySummary `json:"inventories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode inventories response: %w", err)
	}

	c.log.Infof("fetched %d inventory summaries", len(result.Items))
	return result.Items, nil
}

// FetchLatestByHostname fetches the latest full inventory for a specific hostname
func (c *InventoryClient) FetchLatestByHostname(ctx context.Context, hostname string) (*InventoryData, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("inventory collector is not configured")
	}

	url := fmt.Sprintf("%s/v1/inventories/latest/%s", c.baseURL, hostname)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiSecret != "" {
		req.Header.Set("X-API-Key", c.apiSecret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Errorf("failed to fetch inventory for %s: %v", hostname, err)
		return nil, fmt.Errorf("failed to fetch inventory for %s: %w", hostname, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Errorf("inventory API returned status %d for %s: %s", resp.StatusCode, hostname, string(body))
		return nil, fmt.Errorf("inventory API returned status %d for %s", resp.StatusCode, hostname)
	}

	var detail InventoryDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("failed to decode inventory for %s: %w", hostname, err)
	}

	return &detail.Inventory, nil
}
