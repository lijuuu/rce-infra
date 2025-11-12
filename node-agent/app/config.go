package app

import (
	"fmt"
	"os"
	"strconv"
)

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

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	chunkSize := 16384 // Default 16KB for generous batch size
	if cs := os.Getenv("CHUNK_SIZE"); cs != "" {
		if v, err := strconv.Atoi(cs); err == nil {
			chunkSize = v
		}
	}

	chunkInterval := 1 // Default 1 second for real-time priority
	if ci := os.Getenv("CHUNK_INTERVAL_SEC"); ci != "" {
		if v, err := strconv.Atoi(ci); err == nil {
			chunkInterval = v
		}
	}

	heartbeatInterval := 30
	if hi := os.Getenv("HEARTBEAT_INTERVAL_SEC"); hi != "" {
		if v, err := strconv.Atoi(hi); err == nil {
			heartbeatInterval = v
		}
	}

	workerCount := 2
	if wc := os.Getenv("WORKER_COUNT"); wc != "" {
		if v, err := strconv.Atoi(wc); err == nil && v > 0 {
			workerCount = v
		}
	}

	channelSize := 100
	if cs := os.Getenv("CHANNEL_SIZE"); cs != "" {
		if v, err := strconv.Atoi(cs); err == nil && v > 0 {
			channelSize = v
		}
	}

	// Use HOSTNAME (set by Docker) to ensure unique paths per container replica
	hostname := getEnv("HOSTNAME", "node-agent")
	basePath := "/var/lib/node-agent"

	// If IDENTITY_PATH or DB_PATH are explicitly set, use them; otherwise use hostname-based paths
	identityPath := getEnv("IDENTITY_PATH", fmt.Sprintf("%s/%s/identity.json", basePath, hostname))
	dbPath := getEnv("DB_PATH", fmt.Sprintf("%s/%s/agent.db", basePath, hostname))

	cfg := &Config{
		AgentSvcURL:          getEnv("AGENT_SVC_URL", "http://kong:8000"), // Kong HTTP port
		IdentityPath:         identityPath,
		ChunkSize:            chunkSize,
		ChunkIntervalSec:     chunkInterval,
		HeartbeatIntervalSec: heartbeatInterval,
		DBPath:               dbPath,
		WorkerCount:          workerCount,
		ChannelSize:          channelSize,
	}

	if cfg.AgentSvcURL == "" {
		return nil, fmt.Errorf("AGENT_SVC_URL must be set (Kong HTTP endpoint)")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
