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
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	chunkSize := 1024
	if cs := os.Getenv("CHUNK_SIZE"); cs != "" {
		if v, err := strconv.Atoi(cs); err == nil {
			chunkSize = v
		}
	}

	chunkInterval := 2
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

	cfg := &Config{
		AgentSvcURL:          getEnv("AGENT_SVC_URL", "http://kong:8000"), // Kong HTTP port
		IdentityPath:         getEnv("IDENTITY_PATH", "/var/lib/node-agent/identity.json"),
		ChunkSize:            chunkSize,
		ChunkIntervalSec:     chunkInterval,
		HeartbeatIntervalSec: heartbeatInterval,
		DBPath:               getEnv("DB_PATH", "/var/lib/node-agent/agent.db"),
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
