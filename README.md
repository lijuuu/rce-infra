# Command Execution POC

Remote command execution system for edge nodes.

## Architecture

- **agent-svc**: Control plane service (PostgreSQL backend)
- **node-agent**: Edge agent (SQLite backend)
- **Kong**: API gateway with JWT validation
- **PostgreSQL**: Global command queue and logs
- **SQLite**: Local durability for commands and chunk buffer

## Quick Start

```bash
cd deploy
docker-compose up -d
```

## Services

- **agent-svc**: Port 8080 (internal), exposed via Kong on port 8000
- **node-agent**: Runs on each edge node, auto-registers, polls for commands
- **Kong**: Port 8000 (proxy), Port 8001 (admin API)

## API Usage

### Submit Command
```bash
curl -X POST http://localhost:8000/v1/commands/submit \
  -H "Content-Type: application/json" \
  -d '{
    "command_type": "RunCommand",
    "node_id": "node-1",
    "payload": {"cmd": "ls -la /tmp", "timeout_sec": 120}
  }'
```

### List Commands
```bash
curl http://localhost:8000/v1/commands
```

### Get Command Logs
```bash
curl http://localhost:8000/v1/commands/{command_id}/logs
```

## Development

### Build
```bash
# agent-svc
cd agent-svc && go build -o api ./cmd/api

# node-agent
cd node-agent && CGO_ENABLED=1 go build -o agent ./cmd/agent
```

### Run Locally
1. Start PostgreSQL: `docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:15-alpine`
2. Start agent-svc: `cd agent-svc && export JWT_SIGNING_SECRET=secret && ./api`
3. Start node-agent: `cd node-agent && export AGENT_SVC_URL=http://localhost:8080 && ./agent`

## Documentation

See [API_DOCUMENTATION.md](./API_DOCUMENTATION.md) for detailed API reference.
