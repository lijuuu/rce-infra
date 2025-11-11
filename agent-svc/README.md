# Agent Service

The control plane service for remote command execution system.

## Features

- Node registration and management
- Command queue management
- JWT token generation and validation
- Log chunk storage with idempotency
- Command status tracking
- Agent metadata management

## Configuration

Environment variables:

- `SERVER_PORT`: Server port (default: 8080)
- `JWT_SIGNING_SECRET`: JWT signing secret (required)
- `DB_HOST`: PostgreSQL host (default: localhost)
- `DB_PORT`: PostgreSQL port (default: 5432)
- `DB_USER`: PostgreSQL user (default: postgres)
- `DB_PASSWORD`: PostgreSQL password (default: postgres)
- `DB_NAME`: Database name (default: agentdb)
- `DB_SSL_MODE`: SSL mode (default: disable)

## API Endpoints

- `POST /v1/agents/register` - Register a new node
- `POST /v1/agents/heartbeat` - Send heartbeat
- `POST /v1/commands/submit` - Submit a command
- `GET /v1/commands/next` - Poll for next command (long polling)
- `POST /v1/commands/logs` - Push log chunks
- `POST /v1/commands/status` - Update command status

## Building

```bash
go build -o api ./cmd/api
```

## Running

```bash
./api
```

