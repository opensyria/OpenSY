// OpenSY Mining Pool Server
// Main entry point for the Stratum mining pool
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/opensyria/opensy-mining/pool"
)

// Build info (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func main() {
	cfg := parseFlags()

	// Setup logging
	logger := setupLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	logger.Info("Starting OpenSY Mining Pool",
		"version", Version,
		"commit", Commit,
	)

	// Create pool service
	poolCfg := pool.Config{
		StratumAddr:       cfg.StratumAddr,
		InitialDifficulty: cfg.InitialDifficulty,
		MinDifficulty:     cfg.MinDifficulty,
		MaxDifficulty:     cfg.MaxDifficulty,
		VardiffEnabled:    cfg.VardiffEnabled,

		DBHost:     cfg.DBHost,
		DBPort:     cfg.DBPort,
		DBUser:     cfg.DBUser,
		DBPassword: cfg.DBPassword,
		DBName:     cfg.DBName,

		RedisAddr:     cfg.RedisAddr,
		RedisPassword: cfg.RedisPassword,

		NodeURL:  cfg.NodeURL,
		NodeUser: cfg.NodeUser,
		NodePass: cfg.NodePass,

		ConfirmationDepth: 100, // OpenSY uses 100-block maturity
		StatsInterval:     10 * time.Second,

		Logger: logger,
	}

	poolService, err := pool.New(poolCfg)
	if err != nil {
		logger.Error("Failed to create pool service", "error", err)
		os.Exit(1)
	}

	// Start pool service
	if err := poolService.Start(); err != nil {
		logger.Error("Failed to start pool service", "error", err)
		os.Exit(1)
	}

	// Start metrics/API server
	go startAPIServer(cfg.MetricsAddr, poolService, logger)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info("Received shutdown signal", "signal", sig)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		poolService.Stop()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Graceful shutdown complete")
	case <-ctx.Done():
		logger.Warn("Shutdown timed out")
	}
}

// Config holds CLI configuration
type Config struct {
	// Stratum
	StratumAddr       string
	InitialDifficulty uint64
	MinDifficulty     uint64
	MaxDifficulty     uint64
	VardiffEnabled    bool

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisAddr     string
	RedisPassword string

	// Node RPC
	NodeURL  string
	NodeUser string
	NodePass string

	// Metrics
	MetricsAddr string

	// Logging
	LogLevel  string
	LogFormat string
}

func parseFlags() Config {
	cfg := Config{}

	// Stratum
	flag.StringVar(&cfg.StratumAddr, "stratum-addr", ":3333", "Stratum server listen address")
	flag.Uint64Var(&cfg.InitialDifficulty, "initial-difficulty", 10000, "Initial mining difficulty")
	flag.Uint64Var(&cfg.MinDifficulty, "min-difficulty", 1000, "Minimum difficulty")
	flag.Uint64Var(&cfg.MaxDifficulty, "max-difficulty", 1000000000, "Maximum difficulty")
	flag.BoolVar(&cfg.VardiffEnabled, "vardiff", true, "Enable variable difficulty")

	// Database
	flag.StringVar(&cfg.DBHost, "db-host", "localhost", "PostgreSQL host")
	flag.IntVar(&cfg.DBPort, "db-port", 5432, "PostgreSQL port")
	flag.StringVar(&cfg.DBUser, "db-user", "pool", "PostgreSQL user")
	flag.StringVar(&cfg.DBPassword, "db-password", "pool", "PostgreSQL password")
	flag.StringVar(&cfg.DBName, "db-name", "opensy_pool", "PostgreSQL database name")

	// Redis
	flag.StringVar(&cfg.RedisAddr, "redis-addr", "localhost:6379", "Redis address")
	flag.StringVar(&cfg.RedisPassword, "redis-password", "", "Redis password")

	// Node RPC
	flag.StringVar(&cfg.NodeURL, "node-url", "http://127.0.0.1:8332", "OpenSY node RPC URL")
	flag.StringVar(&cfg.NodeUser, "node-user", "", "Node RPC username")
	flag.StringVar(&cfg.NodePass, "node-pass", "", "Node RPC password")

	// Metrics
	flag.StringVar(&cfg.MetricsAddr, "metrics-addr", ":9100", "Metrics/API server address")

	// Logging
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&cfg.LogFormat, "log-format", "text", "Log format (text, json)")

	// Version flag
	showVersion := flag.Bool("version", false, "Show version")

	flag.Parse()

	if *showVersion {
		fmt.Printf("OpenSY Mining Pool %s (%s) built %s\n", Version, Commit, BuildDate)
		os.Exit(0)
	}

	// Environment variable overrides
	if v := os.Getenv("OPENSY_NODE_URL"); v != "" {
		cfg.NodeURL = v
	}
	if v := os.Getenv("OPENSY_NODE_USER"); v != "" {
		cfg.NodeUser = v
	}
	if v := os.Getenv("OPENSY_NODE_PASS"); v != "" {
		cfg.NodePass = v
	}
	if v := os.Getenv("OPENSY_DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("OPENSY_DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("OPENSY_REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}

	return cfg
}

func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func startAPIServer(addr string, poolService *pool.Service, logger *slog.Logger) {
	mux := http.NewServeMux()

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// Pool stats
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := poolService.Stats()
		if err != nil {
			http.Error(w, "Failed to get stats", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
	"online_miners": %d,
	"online_workers": %d,
	"hashrate": %.2f,
	"blocks_found": %d,
	"last_block_height": %d,
	"network_difficulty": %d
}`,
			stats.OnlineMiners,
			stats.OnlineWorkers,
			stats.Hashrate,
			stats.BlocksFound,
			stats.LastBlockHeight,
			stats.NetworkDiff,
		)
	})

	// Active miners
	mux.HandleFunc("/miners/active", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"active_miners": %d}`, poolService.ActiveMiners())
	})

	logger.Info("API/Metrics server started", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("API server error", "error", err)
	}
}
