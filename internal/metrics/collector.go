package metrics

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	commonMetrics "github.com/go-tangra/go-tangra-common/metrics"
)

const namespace = "tangra"
const subsystem = "asset"

// Collector holds all Prometheus metrics for the asset module.
type Collector struct {
	log    *log.Helper
	server *commonMetrics.MetricsServer

	// Asset metrics
	AssetsByStatus *prometheus.GaugeVec
	AssetsTotal    prometheus.Gauge

	// gRPC request metrics
	RequestDuration *prometheus.HistogramVec
	RequestsTotal   *prometheus.CounterVec

	// Entity metrics
	ConsumablesTotal prometheus.Gauge
	SuppliersTotal   prometheus.Gauge
	CategoriesTotal  prometheus.Gauge
	LocationsTotal   prometheus.Gauge
}

// NewCollector creates and registers all asset Prometheus metrics.
func NewCollector(ctx *bootstrap.Context) *Collector {
	c := &Collector{
		log: ctx.NewLoggerHelper("asset/metrics"),

		AssetsByStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "assets_by_status",
			Help:      "Number of assets by status.",
		}, []string{"status"}),

		AssetsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "assets_total",
			Help:      "Total number of assets.",
		}),

		ConsumablesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "consumables_total",
			Help:      "Total number of consumables.",
		}),

		SuppliersTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "suppliers_total",
			Help:      "Total number of suppliers.",
		}),

		CategoriesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "categories_total",
			Help:      "Total number of categories.",
		}),

		LocationsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "locations_total",
			Help:      "Total number of locations.",
		}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "Histogram of gRPC request durations in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests by method and status.",
		}, []string{"method", "status"}),
	}

	prometheus.MustRegister(
		c.AssetsByStatus,
		c.AssetsTotal,
		c.ConsumablesTotal,
		c.SuppliersTotal,
		c.CategoriesTotal,
		c.LocationsTotal,
		c.RequestDuration,
		c.RequestsTotal,
	)

	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = ":9910"
	}
	c.server = commonMetrics.NewMetricsServer(addr, nil, ctx.GetLogger())

	go func() {
		if err := c.server.Start(); err != nil {
			c.log.Errorf("Metrics server failed: %v", err)
		}
	}()

	return c
}

// Stop shuts down the metrics HTTP server.
func (c *Collector) Stop(ctx context.Context) {
	if c.server != nil {
		c.server.Stop(ctx)
	}
}

// Middleware returns a Kratos middleware that records gRPC request metrics.
func (c *Collector) Middleware() middleware.Middleware {
	return commonMetrics.NewServerMiddleware(c.RequestDuration, c.RequestsTotal)
}

// --- Asset helpers ---

// AssetCreated increments the asset counters for the given status.
func (c *Collector) AssetCreated(status string) {
	c.AssetsByStatus.WithLabelValues(status).Inc()
	c.AssetsTotal.Inc()
}

// AssetDeleted decrements the asset counters for the given status.
func (c *Collector) AssetDeleted(status string) {
	c.AssetsByStatus.WithLabelValues(status).Dec()
	c.AssetsTotal.Dec()
}

// AssetStatusChanged adjusts the status gauge when an asset's status changes.
func (c *Collector) AssetStatusChanged(oldStatus, newStatus string) {
	c.AssetsByStatus.WithLabelValues(oldStatus).Dec()
	c.AssetsByStatus.WithLabelValues(newStatus).Inc()
}

// --- Consumable helpers ---

// ConsumableCreated increments the consumable counter.
func (c *Collector) ConsumableCreated() {
	c.ConsumablesTotal.Inc()
}

// ConsumableDeleted decrements the consumable counter.
func (c *Collector) ConsumableDeleted() {
	c.ConsumablesTotal.Dec()
}

// --- Supplier helpers ---

// SupplierCreated increments the supplier counter.
func (c *Collector) SupplierCreated() {
	c.SuppliersTotal.Inc()
}

// SupplierDeleted decrements the supplier counter.
func (c *Collector) SupplierDeleted() {
	c.SuppliersTotal.Dec()
}

// --- Category helpers ---

// CategoryCreated increments the category counter.
func (c *Collector) CategoryCreated() {
	c.CategoriesTotal.Inc()
}

// CategoryDeleted decrements the category counter.
func (c *Collector) CategoryDeleted() {
	c.CategoriesTotal.Dec()
}

// --- Location helpers ---

// LocationCreated increments the location counter.
func (c *Collector) LocationCreated() {
	c.LocationsTotal.Inc()
}

// LocationDeleted decrements the location counter.
func (c *Collector) LocationDeleted() {
	c.LocationsTotal.Dec()
}
