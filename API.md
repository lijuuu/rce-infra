# Agent API Documentation

## Overview

All communication is HTTP-based with standard JSON request/response models.

## Architecture

- **node-agent** → **Kong** (HTTP) → **agent-svc** (HTTP)
- Kong handles JWT validation
- All endpoints use standard HTTP REST patterns

## Base URL

- Via Kong: `http://kong:8000`
- Direct to agent-svc: `http://agent-svc:8080`

## Authentication

All endpoints except `/v1/agents/register` require JWT authentication:

```
Authorization: Bearer <JWT_TOKEN>
```

## Standard Request/Response Format

### Request Format
All POST requests use JSON body:
```json
{
  "field1": "value1",
  "field2": "value2"
}
```

### Response Format

**Success Response:**
```json
{
  "field1": "value1",
  "field2": "value2"
}
```

**Error Response:**
```json
{
  "error": "Error message",
  "details": {
    "field": "validation error"
  }
}
```

## Endpoints

### 1. Register Node

**POST** `/v1/agents/register`

**Request:**
```json
{
  "node_id": "node-7",
  "public_key": "ssh-rsa AAAA...",
  "attrs": {
    "os": "ubuntu",
    "zone": "eu-west-1a"
  }
}
```

**Response:** `200 OK`
```json
{
  "token": "<JWT_TOKEN>",
  "node_id": "node-7",
  "expires_in": 86400
}
```

---

### 2. Heartbeat

**POST** `/v1/agents/heartbeat`

**Headers:** `Authorization: Bearer <JWT_TOKEN>`

**Request:**
```json
{
  "node_id": "node-7"
}
```

**Response:** `200 OK`
```json
{
  "ok": true
}
```

---

### 3. Submit Command (Admin - One-to-One)

**POST** `/v1/commands/submit`

**Request:**
```json
{
  "command_type": "RunCommand",
  "node_id": "node-7",
  "payload": {
    "cmd": "ls -la /tmp",
    "timeout_sec": 120
  }
}
```

**Response:** `201 Created`
```json
{
  "command_id": "uuid"
}
```

---

### 4. Poll Next Command

**GET** `/v1/commands/next?node_id=node-7&wait=30`

**Headers:** `Authorization: Bearer <JWT_TOKEN>`

**Query Parameters:**
- `node_id` (required): Node identifier
- `wait` (optional): Max wait time in seconds (default: 30, max: 60)

**Response:** `200 OK`

**If command available:**
```json
{
  "command_id": "uuid",
  "command_type": "RunCommand",
  "payload": {
    "cmd": "ls -la",
    "timeout_sec": 120
  }
}
```

**If no command (timeout):**
```json
{}
```

---

### 5. Push Logs

**POST** `/v1/commands/logs`

**Headers:** `Authorization: Bearer <JWT_TOKEN>`

**Request:**
```json
{
  "command_id": "uuid",
  "chunks": [
    {
      "offset": 0,
      "stream": "stdout",
      "data": "line1\n"
    },
    {
      "offset": 1,
      "stream": "stdout",
      "data": "line2\n"
    }
  ]
}
```

**Response:** `201 Created`
```json
{
  "acked_offsets": [0, 1]
}
```

---

### 6. Update Command Status

**POST** `/v1/commands/status`

**Headers:** `Authorization: Bearer <JWT_TOKEN>`

**Request:**
```json
{
  "command_id": "uuid",
  "status": "success",
  "exit_code": 0,
  "error_msg": null
}
```

**Response:** `200 OK`
```json
{
  "ok": true
}
```

**Status Values:**
- `queued`
- `running`
- `streaming`
- `success`
- `failed`
- `timeout`

---

### 7. Get Command Logs (Admin)

**GET** `/v1/commands/:command_id/logs?after_offset=5`

**Query Parameters:**
- `after_offset` (optional): Only return logs with offset > after_offset

**Response:** `200 OK`
```json
{
  "command_id": "uuid",
  "logs": [
    {
      "offset": 0,
      "stream": "stdout",
      "data": "line1\n"
    },
    {
      "offset": 1,
      "stream": "stdout",
      "data": "line2\n"
    }
  ]
}
```

**Usage:**
- Fetch all logs: `GET /v1/commands/{command_id}/logs`
- Fetch logs after offset 5: `GET /v1/commands/{command_id}/logs?after_offset=5`

---

## Standard Error Codes

- `400 Bad Request` - Invalid request body or validation failed
- `401 Unauthorized` - Invalid or missing JWT token
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

---

## Command Types

### RunCommand
```json
{
  "cmd": "ls -la /tmp",
  "args": ["-l", "-a"],
  "timeout_sec": 120
}
```

### UpdateAgent
```json
{
  "version": "1.0.0",
  "url": "https://example.com/agent.tar.gz"
}
```

### UpdatePackage
```json
{
  "packages": ["nginx", "curl"],
  "action": "install"
}
```

**Action Values:** `install`, `remove`, `upgrade`

