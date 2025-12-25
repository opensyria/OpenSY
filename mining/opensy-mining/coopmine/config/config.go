// Package config provides configuration loading for CoopMine
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CoordinatorConfig holds coordinator configuration
type CoordinatorConfig struct {
	Cluster   ClusterConfig   `yaml:"cluster"`
	Pool      PoolConfig      `yaml:"pool"`
	GRPC      GRPCConfig      `yaml:"grpc"`
	Dashboard DashboardConfig `yaml:"dashboard"`
	Workers   WorkersConfig   `yaml:"workers"`
	Jobs      JobsConfig      `yaml:"jobs"`
	Shares    SharesConfig    `yaml:"shares"`
	Logging   LoggingConfig   `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
}

// ClusterConfig holds cluster identification
type ClusterConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// PoolConfig holds upstream pool configuration
type PoolConfig struct {
	Address              string        `yaml:"address"`
	Wallet               string        `yaml:"wallet"`
	Password             string        `yaml:"password"`
	ReconnectDelay       time.Duration `yaml:"reconnect_delay"`
	MaxReconnectAttempts int           `yaml:"max_reconnect_attempts"`
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Listen     string    `yaml:"listen"`
	MaxWorkers int       `yaml:"max_workers"`
	TLS        TLSConfig `yaml:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Cert              string `yaml:"cert"`
	Key               string `yaml:"key"`
	CA                string `yaml:"ca"`
	RequireClientCert bool   `yaml:"require_client_cert"`
}

// DashboardConfig holds dashboard configuration
type DashboardConfig struct {
	Listen      string   `yaml:"listen"`
	StaticDir   string   `yaml:"static_dir"`
	CORSEnabled bool     `yaml:"cors_enabled"`
	CORSOrigins []string `yaml:"cors_origins"`
}

// WorkersConfig holds worker management configuration
type WorkersConfig struct {
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	Timeout           time.Duration `yaml:"timeout"`
	MinHashrate       float64       `yaml:"min_hashrate"`
}

// JobsConfig holds job distribution configuration
type JobsConfig struct {
	Timeout     time.Duration `yaml:"timeout"`
	HistorySize int           `yaml:"history_size"`
}

// SharesConfig holds share validation configuration
type SharesConfig struct {
	TargetDifficulty uint64  `yaml:"target_difficulty"`
	ValidateHashes   bool    `yaml:"validate_hashes"`
	MaxRate          float64 `yaml:"max_rate"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// MetricsConfig holds Prometheus metrics configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
	Path    string `yaml:"path"`
}

// WorkerNodeConfig holds worker node configuration
type WorkerNodeConfig struct {
	Worker      WorkerIdentConfig `yaml:"worker"`
	Coordinator ConnectorConfig   `yaml:"coordinator"`
	Mining      MiningConfig      `yaml:"mining"`
	Resources   ResourcesConfig   `yaml:"resources"`
	Logging     LoggingConfig     `yaml:"logging"`
	Health      HealthConfig      `yaml:"health"`
}

// WorkerIdentConfig holds worker identification
type WorkerIdentConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// ConnectorConfig holds coordinator connection configuration
type ConnectorConfig struct {
	Address              string        `yaml:"address"`
	ReconnectDelay       time.Duration `yaml:"reconnect_delay"`
	MaxReconnectAttempts int           `yaml:"max_reconnect_attempts"`
	TLS                  TLSConfig     `yaml:"tls"`
}

// MiningConfig holds mining configuration
type MiningConfig struct {
	Threads          int           `yaml:"threads"`
	HugePages        bool          `yaml:"huge_pages"`
	Flags            RandomXFlags  `yaml:"flags"`
	HashrateInterval time.Duration `yaml:"hashrate_interval"`
}

// RandomXFlags holds RandomX-specific flags
type RandomXFlags struct {
	FullMem bool `yaml:"full_mem"`
	HardAES bool `yaml:"hard_aes"`
	JIT     bool `yaml:"jit"`
}

// ResourcesConfig holds resource limit configuration
type ResourcesConfig struct {
	MaxMemory   int   `yaml:"max_memory"`
	CPUAffinity []int `yaml:"cpu_affinity"`
}

// HealthConfig holds health monitoring configuration
type HealthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
}

// LoadCoordinatorConfig loads coordinator configuration from file
func LoadCoordinatorConfig(path string) (*CoordinatorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultCoordinatorConfig()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// DefaultCoordinatorConfig returns default coordinator configuration
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		Cluster: ClusterConfig{
			ID:   "default-cluster",
			Name: "CoopMine Cluster",
		},
		Pool: PoolConfig{
			Password:             "x",
			ReconnectDelay:       5 * time.Second,
			MaxReconnectAttempts: 0,
		},
		GRPC: GRPCConfig{
			Listen:     ":50051",
			MaxWorkers: 100,
		},
		Dashboard: DashboardConfig{
			Listen:      ":8080",
			StaticDir:   "./coopmine/dashboard/static",
			CORSEnabled: true,
		},
		Workers: WorkersConfig{
			HeartbeatInterval: 30 * time.Second,
			Timeout:           90 * time.Second,
			MinHashrate:       0,
		},
		Jobs: JobsConfig{
			Timeout:     5 * time.Minute,
			HistorySize: 100,
		},
		Shares: SharesConfig{
			TargetDifficulty: 10000,
			ValidateHashes:   false,
			MaxRate:          10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Listen:  ":9090",
			Path:    "/metrics",
		},
	}
}

// LoadWorkerConfig loads worker configuration from file
func LoadWorkerConfig(path string) (*WorkerNodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultWorkerConfig()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// DefaultWorkerConfig returns default worker configuration
func DefaultWorkerConfig() *WorkerNodeConfig {
	return &WorkerNodeConfig{
		Worker: WorkerIdentConfig{
			Name: "Mining Worker",
		},
		Coordinator: ConnectorConfig{
			Address:              "localhost:50051",
			ReconnectDelay:       5 * time.Second,
			MaxReconnectAttempts: 0,
		},
		Mining: MiningConfig{
			Threads:          0, // Auto-detect
			HugePages:        true,
			HashrateInterval: 10 * time.Second,
			Flags: RandomXFlags{
				FullMem: true,
				HardAES: true,
				JIT:     true,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Health: HealthConfig{
			Enabled: false,
			Listen:  ":9091",
		},
	}
}

// Validate validates coordinator configuration
func (c *CoordinatorConfig) Validate() error {
	if c.Cluster.ID == "" {
		return fmt.Errorf("cluster.id is required")
	}
	if c.Pool.Address == "" {
		return fmt.Errorf("pool.address is required")
	}
	if c.Pool.Wallet == "" {
		return fmt.Errorf("pool.wallet is required")
	}
	if c.GRPC.Listen == "" {
		return fmt.Errorf("grpc.listen is required")
	}
	if c.GRPC.TLS.Enabled {
		if c.GRPC.TLS.Cert == "" || c.GRPC.TLS.Key == "" {
			return fmt.Errorf("grpc.tls.cert and grpc.tls.key are required when TLS is enabled")
		}
	}
	return nil
}

// Validate validates worker configuration
func (c *WorkerNodeConfig) Validate() error {
	if c.Coordinator.Address == "" {
		return fmt.Errorf("coordinator.address is required")
	}
	if c.Mining.Threads < 0 {
		return fmt.Errorf("mining.threads must be >= 0")
	}
	if c.Coordinator.TLS.Enabled {
		if c.Coordinator.TLS.Cert == "" || c.Coordinator.TLS.Key == "" {
			return fmt.Errorf("coordinator.tls.cert and coordinator.tls.key are required when TLS is enabled")
		}
	}
	return nil
}
