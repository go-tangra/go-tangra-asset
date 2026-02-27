package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
	collectorv1 "github.com/go-tangra/go-tangra-inventory/gen/go/inventory/collector/v1"
)

// computeInventoryChangedFields compares an existing asset with inventory data and returns the list of changed fields
func computeInventoryChangedFields(existing *ent.Asset, inv *collectorv1.Inventory) []string {
	var changed []string

	if inv.GetHostname() != "" && !strings.EqualFold(inv.GetHostname(), existing.Name) {
		changed = append(changed, "name")
	}

	serial := inv.GetSystem().GetSerialNumber()
	if serial != "" && serial != existing.Serial {
		changed = append(changed, "serial")
	}

	modelName := buildModelName(inv)
	if modelName != "" && modelName != existing.ModelName {
		changed = append(changed, "model_name")
	}

	modelNumber := inv.GetSystem().GetVersion()
	if modelNumber != "" && modelNumber != existing.ModelNumber {
		changed = append(changed, "model_number")
	}

	newMeta := buildInventoryMetadata(inv)
	if newMeta != nil {
		existingMetaJSON, _ := json.Marshal(existing.Metadata)
		newMetaJSON, _ := json.Marshal(newMeta)
		if string(existingMetaJSON) != string(newMetaJSON) {
			changed = append(changed, "metadata")
		}
	}

	return changed
}

// buildModelName creates a model name from system manufacturer + product name
func buildModelName(inv *collectorv1.Inventory) string {
	sys := inv.GetSystem()
	if sys == nil {
		return ""
	}
	manufacturer := strings.TrimSpace(sys.GetManufacturer())
	productName := strings.TrimSpace(sys.GetProductName())
	if manufacturer == "" && productName == "" {
		return ""
	}
	if manufacturer == "" {
		return productName
	}
	if productName == "" {
		return manufacturer
	}
	return manufacturer + " " + productName
}

// buildInventoryCreateOpts creates functional options for AssetCreate from inventory data
func buildInventoryCreateOpts(inv *collectorv1.Inventory) []func(*ent.AssetCreate) {
	var opts []func(*ent.AssetCreate)

	if serial := inv.GetSystem().GetSerialNumber(); serial != "" {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetSerial(serial) })
	}

	modelName := buildModelName(inv)
	if modelName != "" {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetModelName(modelName) })
	}

	if version := inv.GetSystem().GetVersion(); version != "" {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetModelNumber(version) })
	}

	meta := buildInventoryMetadata(inv)
	if meta != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetMetadata(meta) })
	}

	return opts
}

// buildInventoryUpdateMap creates an update map from inventory data for changed fields
func buildInventoryUpdateMap(inv *collectorv1.Inventory, changedFields []string) map[string]interface{} {
	updates := make(map[string]interface{})

	for _, field := range changedFields {
		switch field {
		case "name":
			updates["name"] = inv.GetHostname()
		case "serial":
			updates["serial"] = inv.GetSystem().GetSerialNumber()
		case "model_name":
			updates["model_name"] = buildModelName(inv)
		case "model_number":
			updates["model_number"] = inv.GetSystem().GetVersion()
		case "metadata":
			updates["metadata"] = buildInventoryMetadata(inv)
		}
	}

	return updates
}

// formatBytes converts bytes to a human-readable string
func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// buildInventoryMetadata converts full SMBIOS data to a metadata map
func buildInventoryMetadata(inv *collectorv1.Inventory) map[string]interface{} {
	meta := make(map[string]interface{})

	if sys := inv.GetSystem(); sys != nil {
		meta["system"] = map[string]interface{}{
			"manufacturer":  sys.GetManufacturer(),
			"product_name":  sys.GetProductName(),
			"version":       sys.GetVersion(),
			"serial_number": sys.GetSerialNumber(),
			"uuid":          sys.GetUuid(),
			"wake_up_type":  sys.GetWakeUpType(),
			"sku_number":    sys.GetSkuNumber(),
			"family":        sys.GetFamily(),
		}
	}

	if bios := inv.GetBios(); bios != nil {
		meta["bios"] = map[string]interface{}{
			"vendor":       bios.GetVendor(),
			"version":      bios.GetVersion(),
			"release_date": bios.GetReleaseDate(),
		}
	}

	if bb := inv.GetBaseboard(); bb != nil {
		meta["baseboard"] = map[string]interface{}{
			"manufacturer":  bb.GetManufacturer(),
			"product":       bb.GetProduct(),
			"version":       bb.GetVersion(),
			"serial_number": bb.GetSerialNumber(),
			"asset_tag":     bb.GetAssetTag(),
		}
	}

	if chassis := inv.GetChassis(); chassis != nil {
		meta["chassis"] = map[string]interface{}{
			"manufacturer":    chassis.GetManufacturer(),
			"version":         chassis.GetVersion(),
			"serial_number":   chassis.GetSerialNumber(),
			"asset_tag_number": chassis.GetAssetTagNumber(),
			"sku_number":      chassis.GetSkuNumber(),
		}
	}

	if procs := inv.GetProcessors(); len(procs) > 0 {
		procsList := make([]interface{}, len(procs))
		for i, p := range procs {
			procsList[i] = map[string]interface{}{
				"socket_designation": p.GetSocketDesignation(),
				"manufacturer":      p.GetManufacturer(),
				"version":           p.GetVersion(),
				"max_speed":         fmt.Sprintf("%d MHz", p.GetMaxSpeedMhz()),
				"current_speed":     fmt.Sprintf("%d MHz", p.GetCurrentSpeedMhz()),
				"core_count":        p.GetCoreCount(),
				"core_enabled":      p.GetCoreEnabled(),
				"thread_count":      p.GetThreadCount(),
				"serial_number":     p.GetSerialNumber(),
				"asset_tag":         p.GetAssetTag(),
				"part_number":       p.GetPartNumber(),
			}
		}
		meta["processors"] = procsList
	}

	if mem := inv.GetMemory(); mem != nil {
		memData := map[string]interface{}{
			"total_physical_bytes": mem.GetTotalPhysicalBytes(),
			"total_capacity":      formatBytes(mem.GetTotalPhysicalBytes()),
		}
		if modules := mem.GetModules(); len(modules) > 0 {
			devices := make([]interface{}, len(modules))
			for i, m := range modules {
				devices[i] = map[string]interface{}{
					"device_locator":       m.GetDeviceLocator(),
					"bank_locator":         m.GetBankLocator(),
					"capacity_bytes":       m.GetCapacityBytes(),
					"size":                 formatBytes(m.GetCapacityBytes()),
					"form_factor":          m.GetFormFactor(),
					"type":                 m.GetMemoryType(),
					"speed":                fmt.Sprintf("%d MT/s", m.GetSpeedMtS()),
					"configured_speed":     fmt.Sprintf("%d MT/s", m.GetConfiguredSpeedMtS()),
					"manufacturer":         m.GetManufacturer(),
					"serial_number":        m.GetSerialNumber(),
					"asset_tag":            m.GetAssetTag(),
					"part_number":          m.GetPartNumber(),
				}
			}
			memData["devices"] = devices
		}
		meta["memory"] = memData
	}

	if monitors := inv.GetMonitor(); len(monitors) > 0 {
		monList := make([]interface{}, len(monitors))
		for i, m := range monitors {
			monList[i] = map[string]interface{}{
				"model":         m.GetModel(),
				"manufacturer":  m.GetManufacturer(),
				"serial_number": m.GetSerialNumber(),
			}
		}
		meta["monitors"] = monList
	}

	if len(meta) == 0 {
		return nil
	}

	return meta
}

// buildInventoryPreview generates a sync preview by comparing inventory entries against existing assets
func (s *AssetService) buildInventoryPreview(ctx context.Context, tenantID uint32) (*assetV1.InventorySyncPreviewResponse, map[string]*collectorv1.Inventory, error) {
	summaries, err := s.inventoryClient.FetchInventories(ctx)
	if err != nil {
		return nil, nil, assetV1.ErrorInventorySyncFailed("failed to fetch inventories: %v", err)
	}

	existingAssets, err := s.assetRepo.ListAll(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}

	// Build lookup maps for existing assets
	bySerial := make(map[string]*ent.Asset)
	byName := make(map[string]*ent.Asset)
	for _, a := range existingAssets {
		if a.Serial != "" {
			bySerial[a.Serial] = a
		}
		if a.Name != "" {
			byName[strings.ToLower(a.Name)] = a
		}
	}

	// Deduplicate summaries by hostname (keep latest)
	uniqueHostnames := make(map[string]struct{})
	var hostnames []string
	for _, s := range summaries {
		hostname := s.GetHostname()
		if hostname == "" {
			continue
		}
		lower := strings.ToLower(hostname)
		if _, exists := uniqueHostnames[lower]; !exists {
			uniqueHostnames[lower] = struct{}{}
			hostnames = append(hostnames, hostname)
		}
	}

	// Fetch full inventory for each unique hostname
	inventoryMap := make(map[string]*collectorv1.Inventory)
	var warnings []string
	for _, hostname := range hostnames {
		inv, fetchErr := s.inventoryClient.FetchLatestByHostname(ctx, hostname)
		if fetchErr != nil {
			warnings = append(warnings, "failed to fetch details for "+hostname+": "+fetchErr.Error())
			continue
		}
		inventoryMap[hostname] = inv
	}

	var changes []*assetV1.InventorySyncChange
	unchangedCount := int32(0)

	for hostname, inv := range inventoryMap {
		var existing *ent.Asset

		// Match by serial first, then by hostname
		if serial := inv.GetSystem().GetSerialNumber(); serial != "" {
			existing = bySerial[serial]
		}
		if existing == nil {
			existing = byName[strings.ToLower(hostname)]
		}

		if existing == nil {
			// New asset
			changes = append(changes, &assetV1.InventorySyncChange{
				Action:    assetV1.InventorySyncChange_ACTION_CREATE,
				Hostname:  hostname,
				Serial:    getInventorySerial(inv),
				ModelName: buildModelName(inv),
			})
		} else {
			changedFields := computeInventoryChangedFields(existing, inv)
			if len(changedFields) > 0 {
				changes = append(changes, &assetV1.InventorySyncChange{
					Action:        assetV1.InventorySyncChange_ACTION_UPDATE,
					Hostname:      hostname,
					Serial:        getInventorySerial(inv),
					ModelName:     buildModelName(inv),
					ChangedFields: changedFields,
					ExistingId:    existing.ID,
				})
			} else {
				unchangedCount++
			}
		}
	}

	newCount := int32(0)
	updateCount := int32(0)
	for _, c := range changes {
		switch c.Action {
		case assetV1.InventorySyncChange_ACTION_CREATE:
			newCount++
		case assetV1.InventorySyncChange_ACTION_UPDATE:
			updateCount++
		}
	}

	return &assetV1.InventorySyncPreviewResponse{
		TotalInventoryEntries: int32(len(inventoryMap)),
		NewCount:              newCount,
		UpdateCount:           updateCount,
		UnchangedCount:        unchangedCount,
		Changes:               changes,
		Warnings:              warnings,
	}, inventoryMap, nil
}

// getInventorySerial extracts the serial number from inventory data
func getInventorySerial(inv *collectorv1.Inventory) string {
	return inv.GetSystem().GetSerialNumber()
}
