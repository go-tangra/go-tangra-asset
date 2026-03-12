package metrics

import (
	"context"

	"github.com/go-tangra/go-tangra-asset/internal/data"
)

// statusLabels maps asset status codes to human-readable labels.
var statusLabels = map[int32]string{
	1: "deployable",
	2: "assigned",
	3: "broken",
	4: "archived",
}

// Seed loads initial gauge values from the database.
// Called once at startup so Prometheus has accurate values from the start.
func (c *Collector) Seed(ctx context.Context, statsRepo *data.StatisticsRepo) {
	c.log.Info("Seeding Prometheus metrics from database...")

	assetCount, err := statsRepo.GetAssetCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed asset count: %v", err)
	} else {
		c.AssetsTotal.Set(float64(assetCount))
	}

	for code, label := range statusLabels {
		count, err := statsRepo.GetAssetCountByStatus(ctx, code)
		if err != nil {
			c.log.Errorf("Failed to seed asset status %s: %v", label, err)
		} else {
			c.AssetsByStatus.WithLabelValues(label).Set(float64(count))
		}
	}

	consumableCount, err := statsRepo.GetConsumableCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed consumable count: %v", err)
	} else {
		c.ConsumablesTotal.Set(float64(consumableCount))
	}

	supplierCount, err := statsRepo.GetSupplierCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed supplier count: %v", err)
	} else {
		c.SuppliersTotal.Set(float64(supplierCount))
	}

	categoryCount, err := statsRepo.GetCategoryCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed category count: %v", err)
	} else {
		c.CategoriesTotal.Set(float64(categoryCount))
	}

	locationCount, err := statsRepo.GetLocationCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed location count: %v", err)
	} else {
		c.LocationsTotal.Set(float64(locationCount))
	}

	c.log.Info("Prometheus metrics seeded successfully")
}
