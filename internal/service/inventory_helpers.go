package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/go-tangra/go-tangra-asset/internal/data"
	"github.com/go-tangra/go-tangra-asset/internal/data/ent"
	assetV1 "github.com/go-tangra/go-tangra-asset/gen/go/asset/service/v1"
)

// computeInventoryChangedFields compares an existing asset with inventory data and returns the list of changed fields
func computeInventoryChangedFields(existing *ent.Asset, inv *data.InventoryData) []string {
	var changed []string

	if inv.Hostname != "" && !strings.EqualFold(inv.Hostname, existing.Name) {
		changed = append(changed, "name")
	}

	serial := ""
	if inv.System != nil {
		serial = inv.System.SerialNumber
	}
	if serial != "" && serial != existing.Serial {
		changed = append(changed, "serial")
	}

	modelName := buildModelName(inv)
	if modelName != "" && modelName != existing.ModelName {
		changed = append(changed, "model_name")
	}

	modelNumber := ""
	if inv.System != nil {
		modelNumber = inv.System.Version
	}
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
func buildModelName(inv *data.InventoryData) string {
	if inv.System == nil {
		return ""
	}
	manufacturer := strings.TrimSpace(inv.System.Manufacturer)
	productName := strings.TrimSpace(inv.System.ProductName)
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
func buildInventoryCreateOpts(inv *data.InventoryData) []func(*ent.AssetCreate) {
	var opts []func(*ent.AssetCreate)

	if inv.System != nil && inv.System.SerialNumber != "" {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetSerial(inv.System.SerialNumber) })
	}

	modelName := buildModelName(inv)
	if modelName != "" {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetModelName(modelName) })
	}

	if inv.System != nil && inv.System.Version != "" {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetModelNumber(inv.System.Version) })
	}

	meta := buildInventoryMetadata(inv)
	if meta != nil {
		opts = append(opts, func(c *ent.AssetCreate) { c.SetMetadata(meta) })
	}

	return opts
}

// buildInventoryUpdateMap creates an update map from inventory data for changed fields
func buildInventoryUpdateMap(inv *data.InventoryData, changedFields []string) map[string]interface{} {
	updates := make(map[string]interface{})

	for _, field := range changedFields {
		switch field {
		case "name":
			updates["name"] = inv.Hostname
		case "serial":
			if inv.System != nil {
				updates["serial"] = inv.System.SerialNumber
			}
		case "model_name":
			updates["model_name"] = buildModelName(inv)
		case "model_number":
			if inv.System != nil {
				updates["model_number"] = inv.System.Version
			}
		case "metadata":
			updates["metadata"] = buildInventoryMetadata(inv)
		}
	}

	return updates
}

// buildInventoryMetadata converts full SMBIOS data to a metadata map
func buildInventoryMetadata(inv *data.InventoryData) map[string]interface{} {
	meta := make(map[string]interface{})

	if inv.System != nil {
		meta["system"] = map[string]interface{}{
			"manufacturer":  inv.System.Manufacturer,
			"product_name":  inv.System.ProductName,
			"version":       inv.System.Version,
			"serial_number": inv.System.SerialNumber,
			"uuid":          inv.System.UUID,
			"sku_number":    inv.System.SKUNumber,
			"family":        inv.System.Family,
		}
	}

	if inv.BIOS != nil {
		meta["bios"] = map[string]interface{}{
			"vendor":       inv.BIOS.Vendor,
			"version":      inv.BIOS.Version,
			"release_date": inv.BIOS.ReleaseDate,
		}
	}

	if inv.Baseboard != nil {
		meta["baseboard"] = map[string]interface{}{
			"manufacturer":  inv.Baseboard.Manufacturer,
			"product":       inv.Baseboard.Product,
			"version":       inv.Baseboard.Version,
			"serial_number": inv.Baseboard.SerialNumber,
		}
	}

	if inv.Chassis != nil {
		meta["chassis"] = map[string]interface{}{
			"manufacturer":  inv.Chassis.Manufacturer,
			"type":          inv.Chassis.Type,
			"serial_number": inv.Chassis.SerialNumber,
			"asset_tag":     inv.Chassis.AssetTag,
		}
	}

	if len(inv.Processors) > 0 {
		procs := make([]interface{}, len(inv.Processors))
		for i, p := range inv.Processors {
			procs[i] = map[string]interface{}{
				"socket_designation": p.SocketDesignation,
				"manufacturer":      p.Manufacturer,
				"version":           p.Version,
				"max_speed":         p.MaxSpeed,
				"current_speed":     p.CurrentSpeed,
				"core_count":        p.CoreCount,
				"thread_count":      p.ThreadCount,
			}
		}
		meta["processors"] = procs
	}

	if inv.Memory != nil {
		memData := map[string]interface{}{
			"total_capacity": inv.Memory.TotalCapacity,
		}
		if len(inv.Memory.Devices) > 0 {
			devices := make([]interface{}, len(inv.Memory.Devices))
			for i, d := range inv.Memory.Devices {
				devices[i] = map[string]interface{}{
					"size":          d.Size,
					"type":          d.Type,
					"speed":         d.Speed,
					"manufacturer":  d.Manufacturer,
					"serial_number": d.SerialNumber,
					"locator":       d.Locator,
				}
			}
			memData["devices"] = devices
		}
		meta["memory"] = memData
	}

	if len(inv.Monitors) > 0 {
		monitors := make([]interface{}, len(inv.Monitors))
		for i, m := range inv.Monitors {
			monitors[i] = map[string]interface{}{
				"name":                m.Name,
				"manufacturer":        m.Manufacturer,
				"serial_number":       m.SerialNumber,
				"product_id":          m.ProductID,
				"year_of_manufacture": m.YearOfMfg,
			}
		}
		meta["monitors"] = monitors
	}

	if len(meta) == 0 {
		return nil
	}

	return meta
}

// buildInventoryPreview generates a sync preview by comparing inventory entries against existing assets
func (s *AssetService) buildInventoryPreview(ctx context.Context, tenantID uint32) (*assetV1.InventorySyncPreviewResponse, map[string]*data.InventoryData, error) {
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
		if s.Hostname == "" {
			continue
		}
		lower := strings.ToLower(s.Hostname)
		if _, exists := uniqueHostnames[lower]; !exists {
			uniqueHostnames[lower] = struct{}{}
			hostnames = append(hostnames, s.Hostname)
		}
	}

	// Fetch full inventory for each unique hostname
	inventoryMap := make(map[string]*data.InventoryData)
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
		if inv.System != nil && inv.System.SerialNumber != "" {
			existing = bySerial[inv.System.SerialNumber]
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
func getInventorySerial(inv *data.InventoryData) string {
	if inv.System != nil {
		return inv.System.SerialNumber
	}
	return ""
}
