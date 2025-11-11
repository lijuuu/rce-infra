# Agent / Agent-Svc POC

A production-minded POC for remote command execution on edge nodes.

## Architecture

- **agent-svc**: Control plane service (Postgres backend)
- **node-agent**: Edge agent (SQLite backend)
- **Kong**: API gateway with JWT validation
- **Postgres**: Global command queue & logs, node registry
- **SQLite**: Local durability for commands & chunk buffer

## Quick Start

1. Set environment variables in `deploy/env/.env` (copy from `.env.example`)

2. Start all services:
```bash
cd deploy
docker-compose up -d
```

3. Check logs:
```bash
docker-compose logs -f
```

## Services

### agent-svc
- Port: 8080 (internal)
- Exposed via Kong on port 8000

### node-agent
- Runs on each edge node
- Automatically registers on first run
- Polls for commands and executes them

### Kong
- Port: 8000 (proxy)
- Port: 8001 (admin API)

## API Usage

### Register Node (via Kong)
```bash
curl -X POST http://localhost:8000/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{"node_id":"node-1","attrs":{"os":"ubuntu"}}'
```

### Submit Command
```bash
curl -X POST http://localhost:8000/v1/commands/submit \
  -H "Content-Type: application/json" \
  -d '{
    "command_type":"RunCommand",
    "targets":["node-1"],
    "payload":{"cmd":"ls -la /tmp","timeout_sec":120}
  }'
```

### Push Logs (from node-agent)
```bash
curl -X POST http://localhost:8000/v1/commands/logs \
  -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{
    "command_id":"uuid",
    "chunks":[{"offset":0,"stream":"stdout","data":"line1\n"}]
  }'
```

## Development

### Building Services

```bash
# agent-svc
cd agent-svc
go build -o api ./cmd/api

# node-agent
cd node-agent
CGO_ENABLED=1 go build -o agent ./cmd/agent
```

### Running Locally

1. Start Postgres:
```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:15-alpine
```

2. Run migrations manually or let agent-svc run them on startup

3. Start agent-svc:
```bash
cd agent-svc
export JWT_SIGNING_SECRET=your-secret
export DB_HOST=localhost
./api
```

4. Start node-agent:
```bash
cd node-agent
export AGENT_SVC_URL=http://localhost:8080
./agent
```

## Command Types

- **RunCommand**: Execute shell commands
- **UpdateAgent**: Update agent binary
- **UpdatePackage**: Install/remove/upgrade packages

## Notes

- JWT tokens are issued during registration and stored in node identity file
- Commands are queued in Postgres and polled by node-agents
- Log chunks are stored with idempotency (ON CONFLICT DO NOTHING)
- Local SQLite provides durability during offline periods
- Chunks are buffered locally until acked by server

