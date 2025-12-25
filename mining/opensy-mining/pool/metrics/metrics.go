// Package metrics provides Prometheus metrics for the mining pool
package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all pool Prometheus metrics
type Metrics struct {
	// Connection metrics
	ConnectionsTotal    prometheus.Counter
	ConnectionsCurrent  prometheus.Gauge
	ConnectionsRejected *prometheus.CounterVec

	// Miner metrics
	MinersTotal   prometheus.Gauge
	WorkersTotal  prometheus.Gauge
	MinerHashrate *prometheus.GaugeVec

	// Share metrics
	SharesTotal      *prometheus.CounterVec
	SharesLatency    prometheus.Histogram
	SharesDifficulty prometheus.Histogram

	// Block metrics
	BlocksFound    prometheus.Counter
	BlocksOrphaned prometheus.Counter
	BlockReward    prometheus.Gauge

	// Job metrics
	JobsTotal  prometheus.Counter
	JobsActive prometheus.Gauge
	JobLatency prometheus.Histogram

	// Payout metrics
	PayoutsTotal   prometheus.Counter
	PayoutsAmount  prometheus.Counter
	PayoutsPending prometheus.Gauge

	// Network metrics
	NetworkDifficulty prometheus.Gauge
	NetworkHeight     prometheus.Gauge
	NetworkHashrate   prometheus.Gauge

	// Pool metrics
	PoolHashrate     prometheus.Gauge
	PoolUptime       prometheus.Counter
	PoolFeeCollected prometheus.Counter

	// RPC metrics
	RPCRequests *prometheus.CounterVec
	RPCLatency  *prometheus.HistogramVec
	RPCErrors   prometheus.Counter

	// Database metrics
	DBConnections prometheus.Gauge
	DBLatency     *prometheus.HistogramVec
	DBErrors      prometheus.Counter

	// Redis metrics
	RedisConnections prometheus.Gauge
	RedisLatency     prometheus.Histogram
	RedisErrors      prometheus.Counter

	// Security metrics
	BannedIPs     prometheus.Gauge
	RateLimited   prometheus.Counter
	InvalidShares prometheus.Counter

	registry *prometheus.Registry
	mu       sync.RWMutex
}

// New creates a new metrics instance
func New(namespace string) *Metrics {
	if namespace == "" {
		namespace = "opensy_pool"
	}

	m := &Metrics{
		registry: prometheus.NewRegistry(),
	}

	// Connection metrics
	m.ConnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "connections_total",
		Help:      "Total number of miner connections",
	})

	m.ConnectionsCurrent = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "connections_current",
		Help:      "Current number of active connections",
	})

	m.ConnectionsRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "connections_rejected_total",
		Help:      "Total rejected connections by reason",
	}, []string{"reason"})

	// Miner metrics
	m.MinersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "miners_total",
		Help:      "Total unique miners",
	})

	m.WorkersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "workers_total",
		Help:      "Total workers across all miners",
	})

	m.MinerHashrate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "miner_hashrate",
		Help:      "Hashrate per miner in H/s",
	}, []string{"address"})

	// Share metrics
	m.SharesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "shares_total",
		Help:      "Total shares submitted",
	}, []string{"status"}) // valid, invalid, stale, duplicate

	m.SharesLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "shares_latency_seconds",
		Help:      "Share processing latency",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
	})

	m.SharesDifficulty = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "shares_difficulty",
		Help:      "Distribution of share difficulties",
		Buckets:   prometheus.ExponentialBuckets(1000, 2, 15),
	})

	// Block metrics
	m.BlocksFound = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_found_total",
		Help:      "Total blocks found by the pool",
	})

	m.BlocksOrphaned = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "blocks_orphaned_total",
		Help:      "Total orphaned blocks",
	})

	m.BlockReward = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "block_reward",
		Help:      "Current block reward in satoshis",
	})

	// Job metrics
	m.JobsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "jobs_total",
		Help:      "Total jobs created",
	})

	m.JobsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "jobs_active",
		Help:      "Currently active jobs",
	})

	m.JobLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "job_latency_seconds",
		Help:      "Job distribution latency",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 12),
	})

	// Payout metrics
	m.PayoutsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "payouts_total",
		Help:      "Total payouts made",
	})

	m.PayoutsAmount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "payouts_amount_total",
		Help:      "Total amount paid out in satoshis",
	})

	m.PayoutsPending = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "payouts_pending",
		Help:      "Pending payout amount in satoshis",
	})

	// Network metrics
	m.NetworkDifficulty = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "network_difficulty",
		Help:      "Current network difficulty",
	})

	m.NetworkHeight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "network_height",
		Help:      "Current blockchain height",
	})

	m.NetworkHashrate = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "network_hashrate",
		Help:      "Estimated network hashrate",
	})

	// Pool metrics
	m.PoolHashrate = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "pool_hashrate",
		Help:      "Pool hashrate in H/s",
	})

	m.PoolUptime = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pool_uptime_seconds",
		Help:      "Pool uptime in seconds",
	})

	m.PoolFeeCollected = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pool_fee_collected_total",
		Help:      "Total pool fees collected in satoshis",
	})

	// RPC metrics
	m.RPCRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "rpc_requests_total",
		Help:      "Total RPC requests by method",
	}, []string{"method"})

	m.RPCLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "rpc_latency_seconds",
		Help:      "RPC request latency by method",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"method"})

	m.RPCErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "rpc_errors_total",
		Help:      "Total RPC errors",
	})

	// Database metrics
	m.DBConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "db_connections",
		Help:      "Current database connections",
	})

	m.DBLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "db_latency_seconds",
		Help:      "Database query latency",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 14),
	}, []string{"operation"})

	m.DBErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "db_errors_total",
		Help:      "Total database errors",
	})

	// Redis metrics
	m.RedisConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "redis_connections",
		Help:      "Current Redis connections",
	})

	m.RedisLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "redis_latency_seconds",
		Help:      "Redis operation latency",
		Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 12),
	})

	m.RedisErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "redis_errors_total",
		Help:      "Total Redis errors",
	})

	// Security metrics
	m.BannedIPs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "banned_ips",
		Help:      "Number of currently banned IPs",
	})

	m.RateLimited = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "rate_limited_total",
		Help:      "Total rate-limited requests",
	})

	m.InvalidShares = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "invalid_shares_total",
		Help:      "Total invalid shares (security metric)",
	})

	// Register all metrics
	m.registry.MustRegister(
		m.ConnectionsTotal,
		m.ConnectionsCurrent,
		m.ConnectionsRejected,
		m.MinersTotal,
		m.WorkersTotal,
		m.MinerHashrate,
		m.SharesTotal,
		m.SharesLatency,
		m.SharesDifficulty,
		m.BlocksFound,
		m.BlocksOrphaned,
		m.BlockReward,
		m.JobsTotal,
		m.JobsActive,
		m.JobLatency,
		m.PayoutsTotal,
		m.PayoutsAmount,
		m.PayoutsPending,
		m.NetworkDifficulty,
		m.NetworkHeight,
		m.NetworkHashrate,
		m.PoolHashrate,
		m.PoolUptime,
		m.PoolFeeCollected,
		m.RPCRequests,
		m.RPCLatency,
		m.RPCErrors,
		m.DBConnections,
		m.DBLatency,
		m.DBErrors,
		m.RedisConnections,
		m.RedisLatency,
		m.RedisErrors,
		m.BannedIPs,
		m.RateLimited,
		m.InvalidShares,
	)

	return m
}

// Handler returns the HTTP handler for metrics
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Registry returns the prometheus registry
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// RecordShare records a share submission
func (m *Metrics) RecordShare(status string, difficulty float64, latency float64) {
	m.SharesTotal.WithLabelValues(status).Inc()
	m.SharesLatency.Observe(latency)
	m.SharesDifficulty.Observe(difficulty)
}

// RecordConnection records a connection event
func (m *Metrics) RecordConnection(accepted bool, rejectReason string) {
	m.ConnectionsTotal.Inc()
	if accepted {
		m.ConnectionsCurrent.Inc()
	} else {
		m.ConnectionsRejected.WithLabelValues(rejectReason).Inc()
	}
}

// RecordDisconnection records a disconnection
func (m *Metrics) RecordDisconnection() {
	m.ConnectionsCurrent.Dec()
}

// RecordBlock records a found block
func (m *Metrics) RecordBlock(orphaned bool) {
	if orphaned {
		m.BlocksOrphaned.Inc()
	} else {
		m.BlocksFound.Inc()
	}
}

// RecordRPC records an RPC call
func (m *Metrics) RecordRPC(method string, latency float64, err error) {
	m.RPCRequests.WithLabelValues(method).Inc()
	m.RPCLatency.WithLabelValues(method).Observe(latency)
	if err != nil {
		m.RPCErrors.Inc()
	}
}

// RecordDB records a database operation
func (m *Metrics) RecordDB(operation string, latency float64, err error) {
	m.DBLatency.WithLabelValues(operation).Observe(latency)
	if err != nil {
		m.DBErrors.Inc()
	}
}

// UpdatePoolStats updates pool-level statistics
func (m *Metrics) UpdatePoolStats(hashrate float64, miners, workers int64) {
	m.PoolHashrate.Set(hashrate)
	m.MinersTotal.Set(float64(miners))
	m.WorkersTotal.Set(float64(workers))
}

// UpdateNetworkStats updates network statistics
func (m *Metrics) UpdateNetworkStats(difficulty float64, height int64) {
	m.NetworkDifficulty.Set(difficulty)
	m.NetworkHeight.Set(float64(height))
}
