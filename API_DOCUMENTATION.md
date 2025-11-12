# Agent Service API Documentation

Base URL: `http://localhost:8080/v1`

All endpoints return JSON responses. Error responses follow this format:
```json
{
  "error": "Error message",
  "details": {
    "field": "additional error details"
  }
}
```

---

## Health Endpoints

### GET /health
Health check endpoint.

**Response:**
```json
{
  "status": "healthy"
}
```

### GET /ready
Readiness check endpoint.

**Response:**
```json
{
  "status": "ready"
}
```

---

## Agent Endpoints

### POST /v1/agents/register
Register a new node agent. This endpoint does NOT require authentication.

**Request Body:**
```json
{
  "node_id": "string (required)",
  "public_key": "string (optional)",
  "attrs": {
    "hostname": "string",
    "os": "string",
    "cpu_cores": 4,
    "memory_gb": 8,
    "disk_gb": 100
  }
}
```

**Response (200 OK):**
```json
{
  "token": "JWT token string",
  "node_id": "string",
  "expires_in": 86400
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request body or validation failed
- `500 Internal Server Error`: Failed to register node or generate token

---

### POST /v1/agents/heartbeat
Update node heartbeat. Requires JWT authentication.

**Headers:**
```
Authorization: Bearer <JWT_TOKEN>
```

**Request Body:**
```json
{
  "node_id": "string (required)"
}
```

**Response (200 OK):**
```json
{
  "ok": true
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request body or validation failed
- `401 Unauthorized`: Invalid or missing token
- `500 Internal Server Error`: Failed to update heartbeat

---

### GET /v1/agents
List all registered nodes. Admin endpoint (no authentication required).

**Query Parameters:**
- None

**Response (200 OK):**
```json
{
  "nodes": [
    {
      "node_id": "string",
      "attrs": {
        "hostname": "string",
        "os": "string",
        "cpu_cores": 4,
        "memory_gb": 8,
        "disk_gb": 100
      },
      "last_seen_at": "2024-01-01T00:00:00Z",
      "disabled": false,
      "is_healthy": true
    }
  ]
}
```

**Notes:**
- `is_healthy`: `true` if `last_seen_at` is within the last 30 seconds (configurable via `HEARTBEAT_TIMEOUT_SEC`)
- `disabled`: Whether the node is disabled

---

## Command Endpoints

### POST /v1/commands/submit
Submit a command to a specific node. Admin endpoint (no authentication required).

**Request Body:**
```json
{
  "command_type": "RunCommand (required)",
  "node_id": "string (required)",
  "payload": {
    "cmd": "echo 'Hello World'",
    "timeout_sec": 30
  }
}
```

**Response (201 Created):**
```json
{
  "command_id": "uuid-string"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request body, validation failed, or node not found
- `500 Internal Server Error`: Failed to submit command

---

### GET /v1/commands/next
Poll for the next queued command for the authenticated node. Long polling endpoint. Requires JWT authentication.

**Headers:**
```
Authorization: Bearer <JWT_TOKEN>
```

**Query Parameters:**
- `wait` (optional): Maximum wait time in seconds (1-60, default: 30)

**Response (200 OK):**
```json
{
  "command_id": "uuid-string",
  "command_type": "RunCommand",
  "payload": {
    "cmd": "echo 'Hello World'",
    "timeout_sec": 30
  }
}
```

**Empty Response (200 OK):**
If no command is available within the wait time:
```json
{
  "command_id": "",
  "command_type": "",
  "payload": null
}
```

**Error Responses:**
- `401 Unauthorized`: Invalid or missing token
- `500 Internal Server Error`: Failed to get command

**Notes:**
- Uses long polling: waits up to `wait` seconds for a command to become available
- Polls every 1 second internally
- Returns empty response if timeout is reached

---

### POST /v1/commands/logs
Push command execution log chunks. Requires JWT authentication.

**Headers:**
```
Authorization: Bearer <JWT_TOKEN>
```

**Request Body:**
```json
{
  "command_id": "uuid-string (required)",
  "chunks": [
    {
      "chunk_index": 0,
      "stream": "stdout",
      "data": "Hello World\n",
      "is_final": false
    },
    {
      "chunk_index": 1,
      "stream": "stderr",
      "data": "Warning message\n",
      "is_final": false
    },
    {
      "chunk_index": 2,
      "stream": "stdout",
      "data": "Final output\n",
      "is_final": true
    }
  ]
}
```

**Field Descriptions:**
- `chunk_index`: Sequential chunk index (0, 1, 2, ...)
- `stream`: Either `"stdout"` or `"stderr"`
- `data`: The log data (text)
- `is_final`: `true` if this is the final chunk (work is done), `false` otherwise

**Response (201 Created):**
```json
{
  "acked_offsets": [0, 1, 2]
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request body, validation failed, command not found, or command doesn't belong to node
- `401 Unauthorized`: Invalid or missing token
- `500 Internal Server Error`: Failed to insert log chunks

**Notes:**
- Chunks are inserted with idempotency (duplicate chunks are ignored)
- Returns list of chunk indexes that were successfully inserted
- `is_final: true` should be set on the last chunk(s) when command execution completes

---

### POST /v1/commands/status
Update command execution status. Requires JWT authentication.

**Headers:**
```
Authorization: Bearer <JWT_TOKEN>
```

**Request Body:**
```json
{
  "command_id": "uuid-string (required)",
  "status": "success (required)",
  "exit_code": 0,
  "error_msg": "string (optional)"
}
```

**Status Values:**
- `queued`: Command is queued (not yet picked up)
- `running`: Command is currently executing
- `streaming`: Command is streaming output
- `success`: Command completed successfully
- `failed`: Command failed
- `timeout`: Command timed out

**Response (200 OK):**
```json
{
  "ok": true
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request body, validation failed, command not found, or command doesn't belong to node
- `401 Unauthorized`: Invalid or missing token
- `500 Internal Server Error`: Failed to update status

---

### GET /v1/commands
List commands. Admin endpoint (no authentication required).

**Query Parameters:**
- `node_id` (optional): Filter by node ID
- `limit` (optional): Maximum number of commands to return (1-100, default: 50)

**Response (200 OK):**
```json
{
  "commands": [
    {
      "command_id": "uuid-string",
      "node_id": "string",
      "command_type": "RunCommand",
      "payload": {
        "cmd": "echo 'Hello World'",
        "timeout_sec": 30
      },
      "status": "success",
      "exit_code": 0,
      "error_msg": null,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:05Z"
    }
  ]
}
```

**Error Responses:**
- `500 Internal Server Error`: Failed to list commands

---

### GET /v1/commands/:command_id/logs
Get logs for a specific command. Admin endpoint (no authentication required).

**Path Parameters:**
- `command_id`: UUID of the command

**Query Parameters:**
- `after_chunk_index` (optional): Only return logs with `chunk_index > after_chunk_index`

**Response (200 OK):**
```json
{
  "command_id": "uuid-string",
  "logs": [
    {
      "chunk_index": 0,
      "stream": "stdout",
      "data": "Hello World\n",
      "is_final": false
    },
    {
      "chunk_index": 1,
      "stream": "stderr",
      "data": "Warning message\n",
      "is_final": false
    },
    {
      "chunk_index": 2,
      "stream": "stdout",
      "data": "Final output\n",
      "is_final": true
    }
  ]
}
```

**Error Responses:**
- `400 Bad Request`: Invalid command_id
- `500 Internal Server Error`: Failed to fetch logs

**Notes:**
- Logs are returned ordered by `chunk_index` ascending, then by `stream`
- Use `after_chunk_index` for incremental log fetching (polling)
- `is_final: true` indicates the final chunk(s) when work is done

---

## Authentication

Most endpoints require JWT authentication via the `Authorization` header:
```
Authorization: Bearer <JWT_TOKEN>
```

**Getting a Token:**
1. Register a node using `POST /v1/agents/register`
2. The response includes a `token` field
3. Use this token in subsequent authenticated requests

**Token Expiration:**
- Tokens expire after 24 hours (86400 seconds)
- Re-register to get a new token

**Endpoints that DON'T require authentication:**
- `POST /v1/agents/register` (registration endpoint)
- `GET /v1/agents` (list nodes - admin)
- `GET /v1/commands` (list commands - admin)
- `GET /v1/commands/:command_id/logs` (get logs - admin)
- `POST /v1/commands/submit` (submit command - admin)
- `GET /health` and `GET /ready` (health checks)

**Endpoints that REQUIRE authentication:**
- `POST /v1/agents/heartbeat`
- `GET /v1/commands/next`
- `POST /v1/commands/logs`
- `POST /v1/commands/status`

---

## Command Status Flow

1. **queued**: Command is submitted and waiting to be picked up by node-agent
2. **running**: Node-agent has picked up the command and started execution
3. **streaming**: Command is actively streaming output (optional intermediate state)
4. **success**: Command completed successfully (exit code 0)
5. **failed**: Command failed (non-zero exit code or error)
6. **timeout**: Command execution exceeded the timeout limit

---

## Example Workflows

### Node Registration and Command Execution

1. **Register Node:**
```bash
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "node-123",
    "attrs": {
      "hostname": "worker-1",
      "os": "Linux"
    }
  }'
```

2. **Submit Command:**
```bash
curl -X POST http://localhost:8080/v1/commands/submit \
  -H "Content-Type: application/json" \
  -d '{
    "command_type": "RunCommand",
    "node_id": "node-123",
    "payload": {
      "cmd": "echo Hello World",
      "timeout_sec": 30
    }
  }'
```

3. **Node Polls for Command:**
```bash
curl -X GET "http://localhost:8080/v1/commands/next?wait=30" \
  -H "Authorization: Bearer <TOKEN>"
```

4. **Node Pushes Logs:**
```bash
curl -X POST http://localhost:8080/v1/commands/logs \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "command_id": "<COMMAND_ID>",
    "chunks": [
      {
        "chunk_index": 0,
        "stream": "stdout",
        "data": "Hello World\n",
        "is_final": true
      }
    ]
  }'
```

5. **Node Updates Status:**
```bash
curl -X POST http://localhost:8080/v1/commands/status \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "command_id": "<COMMAND_ID>",
    "status": "success",
    "exit_code": 0
  }'
```

6. **Admin Fetches Logs:**
```bash
curl http://localhost:8080/v1/commands/<COMMAND_ID>/logs
```

---

## Error Handling

All endpoints return appropriate HTTP status codes:
- `200 OK`: Successful request
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request (validation errors, missing fields)
- `401 Unauthorized`: Authentication required or invalid token
- `500 Internal Server Error`: Server error

Error responses include a descriptive error message and optional details:
```json
{
  "error": "validation failed",
  "details": {
    "error": "node_id is required"
  }
}
```

