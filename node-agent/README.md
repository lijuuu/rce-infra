# Node Agent

The edge agent for remote command execution system.

## Features

- Automatic registration with agent-svc
- Command execution with chunked stdout/stderr streaming
- Local SQLite storage for durability
- Offline buffer with retry logic
- Heartbeat service
- Metadata collection

## Configuration

Environment variables:

- `AGENT_SVC_URL`: Agent service URL (default: http://kong:8000)
- `IDENTITY_PATH`: Path to identity file (default: /var/lib/node-agent/identity.json)
- `CHUNK_SIZE`: Chunk size in bytes (default: 1024)
- `CHUNK_INTERVAL_SEC`: Chunk interval in seconds (default: 2)
- `HEARTBEAT_INTERVAL_SEC`: Heartbeat interval in seconds (default: 30)
- `DB_PATH`: SQLite database path (default: /var/lib/node-agent/agent.db)

## Building

```bash
CGO_ENABLED=1 go build -o agent ./cmd/agent
```

## Running

```bash
./agent
```

## Identity

On first run, the agent will:
1. Collect system metadata
2. Register with agent-svc
3. Save identity (node_id, JWT token) to `IDENTITY_PATH`

Subsequent runs will use the saved identity.

