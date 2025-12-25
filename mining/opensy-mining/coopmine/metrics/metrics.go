// Package metrics provides Prometheus metrics for CoopMine
package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all CoopMine Prometheus metrics
type Metrics struct {
	// Worker metrics
	WorkersTotal   prometheus.Gauge
	WorkersOnline  prometheus.Gauge
	WorkerHashrate *prometheus.GaugeVec

	// Share metrics
	SharesTotal   *prometheus.CounterVec
	SharesLatency prometheus.Histogram

	// Job metrics
	JobsTotal    prometheus.Counter
	JobsActive   prometheus.Gauge
	JobsDuration prometheus.Histogram

	// Block metrics
	BlocksFound prometheus.Counter

	// Pool connection metrics
	PoolConnected  prometheus.Gauge
	PoolReconnects prometheus.Counter
	PoolLatency    prometheus.Histogram

	// Hashrate metrics
	ClusterHashrate prometheus.Gauge

	// gRPC metrics
	GRPCConnections prometheus.Gauge
	GRPCRequests    *prometheus.CounterVec
	GRPCLatency     *prometheus.HistogramVec

	// System metrics
	UptimeSeconds prometheus.Counter

	registry *prometheus.Registry
	mu       sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "coopmine"
	}

	m := &Metrics{
		registry: prometheus.NewRegistry(),
	}

	// Worker metrics
	m.WorkersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "workers_total",
		Help:      "Total number of registered workers",
	})

	m.WorkersOnline = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "workers_online",
		Help:      "Number of currently online workers",
	})

	m.WorkerHashrate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "worker_hashrate",
		Help:      "Hashrate per worker in H/s",
	}, []string{"worker_id", "worker_name"})

	// Share metrics
	m.SharesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "shares_total",
		Help:      "Total number of shares submitted",
	}, []string{"status"}) // status: valid, invalid, stale

	m.SharesLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "shares_latency_seconds",
		Help:      "Share submission latency in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms to 4s
	})

	// Job metrics
	m.JobsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "jobs_total",
		Help:      "Total number of jobs received from pool",
	})

	m.JobsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "jobs_active",
		Help:      "Number of currently active jobs",
	})

	m.JobsDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "jobs_duration_seconds",
		Help:      "Job duration in seconds",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to 17min
	})

	// Block metrics
	m.BlocksFound = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_found_total",
		Help:      "Total number of blocks found",
	})

	// Pool metrics
	m.PoolConnected = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "pool_connected",
		Help:      "Whether connected to pool (1=connected, 0=disconnected)",
	})

	m.PoolReconnects = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pool_reconnects_total",
		Help:      "Total number of pool reconnections",
	})

	m.PoolLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "pool_latency_seconds",
		Help:      "Pool response latency in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to 10s
	})

	// Cluster hashrate
	m.ClusterHashrate = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "cluster_hashrate",
		Help:      "Total cluster hashrate in H/s",
	})

	// gRPC metrics
	m.GRPCConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "grpc_connections",
		Help:      "Number of active gRPC connections",
	})

	m.GRPCRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "grpc_requests_total",
		Help:      "Total gRPC requests",
	}, []string{"method", "status"})

	m.GRPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "grpc_latency_seconds",
		Help:      "gRPC request latency in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to 1s
	}, []string{"method"})

	// System metrics
	m.UptimeSeconds = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "uptime_seconds_total",
		Help:      "Total uptime in seconds",
	})

	// Register all metrics
	m.registry.MustRegister(
		m.WorkersTotal,
		m.WorkersOnline,
		m.WorkerHashrate,
		m.SharesTotal,
		m.SharesLatency,
		m.JobsTotal,
		m.JobsActive,
		m.JobsDuration,
		m.BlocksFound,
		m.PoolConnected,
		m.PoolReconnects,
		m.PoolLatency,
		m.ClusterHashrate,
		m.GRPCConnections,
		m.GRPCRequests,
		m.GRPCLatency,
		m.UptimeSeconds,
	)

	return m
}

// Handler returns an HTTP handler for the metrics endpoint
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordShareSubmission records a share submission
func (m *Metrics) RecordShareSubmission(status string, latencySeconds float64) {
	m.SharesTotal.WithLabelValues(status).Inc()
	m.SharesLatency.Observe(latencySeconds)
}

// RecordWorkerHashrate records a worker's hashrate
func (m *Metrics) RecordWorkerHashrate(workerID, workerName string, hashrate float64) {
	m.WorkerHashrate.WithLabelValues(workerID, workerName).Set(hashrate)
}

// RecordGRPCRequest records a gRPC request
func (m *Metrics) RecordGRPCRequest(method, status string, latencySeconds float64) {
	m.GRPCRequests.WithLabelValues(method, status).Inc()
	m.GRPCLatency.WithLabelValues(method).Observe(latencySeconds)
}

// SetPoolConnected sets the pool connection status
func (m *Metrics) SetPoolConnected(connected bool) {
	if connected {
		m.PoolConnected.Set(1)
	} else {
		m.PoolConnected.Set(0)
	}
}

// IncrementBlocks increments the block counter
func (m *Metrics) IncrementBlocks() {
	m.BlocksFound.Inc()
}

// UpdateClusterStats updates cluster-level statistics
func (m *Metrics) UpdateClusterStats(totalWorkers, onlineWorkers int, totalHashrate float64) {
	m.WorkersTotal.Set(float64(totalWorkers))
	m.WorkersOnline.Set(float64(onlineWorkers))
	m.ClusterHashrate.Set(totalHashrate)
}

// RemoveWorker removes a worker's metrics
func (m *Metrics) RemoveWorker(workerID, workerName string) {
	m.WorkerHashrate.DeleteLabelValues(workerID, workerName)
}

// ServeMetrics starts an HTTP server for metrics
func ServeMetrics(addr string, metrics *Metrics) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return http.ListenAndServe(addr, mux)
}
