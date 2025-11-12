# Command Execution Frontend

A modern React frontend built with Vite and shadcn/ui for managing nodes and executing commands.

## Features

- **Node Management**: View all registered nodes with health status
- **Command Execution**: Execute commands on selected nodes
- **Command History**: View recent command executions and their logs
- **Real-time Updates**: Auto-refreshes every 5 seconds

## Setup

```bash
cd frontend
npm install
```

## Running

```bash
npm run dev
```

The frontend will be available at `http://localhost:5173` (default Vite port).

Make sure the agent-svc is running on `http://localhost:8080`.

## Building

```bash
npm run build
```

## API Endpoints Used

- `GET /v1/agents` - List all nodes
- `GET /v1/commands` - List commands (with optional node_id filter)
- `POST /v1/commands/submit` - Submit a command
- `GET /v1/commands/:command_id/logs` - Get command logs
