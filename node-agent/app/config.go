package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// YAMLConfig represents the structure of config.yaml
type YAMLConfig struct {
	Agent struct {
		SvcURL       string `yaml:"svc_url"`
		IdentityPath string `yaml:"identity_path"`
		Chunk        struct {
			Size        int `yaml:"size"`
			IntervalSec int `yaml:"interval_sec"`
		} `yaml:"chunk"`
		Heartbeat struct {
			IntervalSec int `yaml:"interval_sec"`
		} `yaml:"heartbeat"`
		Storage struct {
			DBPath string `yaml:"db_path"`
		} `yaml:"storage"`
		Execution struct {
			DefaultTimeoutSec int `yaml:"default_timeout_sec"`
			WorkerCount       int `yaml:"worker_count"`
			ChannelSize       int `yaml:"channel_size"`
		} `yaml:"execution"`
	} `yaml:"agent"`
}

// Config holds node agent configuration
type Config struct {
	AgentSvcURL          string
	IdentityPath         string
	ChunkSize            int
	ChunkIntervalSec     int
	HeartbeatIntervalSec int
	DBPath               string
	WorkerCount          int
	ChannelSize          int
}

// LoadConfig loads configuration from YAML file with environment variable overrides
func LoadConfig() (*Config, error) {
	// Determine config file path (can be overridden via CONFIG_PATH env var)
	configPath := getEnv("CONFIG_PATH", "/app/conf/config.yaml")

	// Try to load YAML config
	yamlCfg := &YAMLConfig{}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, yamlCfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
		}
	} else {
		// If config file doesn't exist, use defaults (backward compatibility)
		yamlCfg.Agent.SvcURL = "http://kong:8000"
		yamlCfg.Agent.Chunk.Size = 16384
		yamlCfg.Agent.Chunk.IntervalSec = 1
		yamlCfg.Agent.Heartbeat.IntervalSec = 30
		yamlCfg.Agent.Execution.WorkerCount = 2
		yamlCfg.Agent.Execution.ChannelSize = 100
	}

	// Use HOSTNAME (set by Docker) to ensure unique paths per container replica
	hostname := getEnv("HOSTNAME", "node-agent")
	basePath := "/var/lib/node-agent"

	// Build config with YAML values, allowing env var overrides
	cfg := &Config{
		AgentSvcURL:          getEnv("AGENT_SVC_URL", yamlCfg.Agent.SvcURL),
		ChunkSize:            getEnvInt("CHUNK_SIZE", yamlCfg.Agent.Chunk.Size),
		ChunkIntervalSec:     getEnvInt("CHUNK_INTERVAL_SEC", yamlCfg.Agent.Chunk.IntervalSec),
		HeartbeatIntervalSec: getEnvInt("HEARTBEAT_INTERVAL_SEC", yamlCfg.Agent.Heartbeat.IntervalSec),
		WorkerCount:          getEnvInt("WORKER_COUNT", yamlCfg.Agent.Execution.WorkerCount),
		ChannelSize:          getEnvInt("CHANNEL_SIZE", yamlCfg.Agent.Execution.ChannelSize),
	}

	// Handle identity path: env var > YAML > hostname-based default
	if identityPath := getEnv("IDENTITY_PATH", ""); identityPath != "" {
		cfg.IdentityPath = identityPath
	} else if yamlCfg.Agent.IdentityPath != "" {
		cfg.IdentityPath = yamlCfg.Agent.IdentityPath
	} else {
		cfg.IdentityPath = filepath.Join(basePath, hostname, "identity.json")
	}

	// Handle DB path: env var > YAML > hostname-based default
	if dbPath := getEnv("DB_PATH", ""); dbPath != "" {
		cfg.DBPath = dbPath
	} else if yamlCfg.Agent.Storage.DBPath != "" {
		cfg.DBPath = yamlCfg.Agent.Storage.DBPath
	} else {
		cfg.DBPath = filepath.Join(basePath, hostname, "agent.db")
	}

	// Validate required fields
	if cfg.AgentSvcURL == "" {
		return nil, fmt.Errorf("AGENT_SVC_URL must be set (Kong HTTP endpoint)")
	}

	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 2
	}

	if cfg.ChannelSize <= 0 {
		cfg.ChannelSize = 100
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return defaultValue
}
