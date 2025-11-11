
---

# Agent / Agent-Svc POC (README / Spec)

**Short description**
A simple, production-minded POC for remote command execution on edge nodes. It uses:

* `agent-svc` (control plane) backed by **Postgres** (global command queue & logs, node registry & metadata)
* `node-agent` (edge agent) backed by **SQLite** (local durability for commands & chunk buffer)
* **Kong** as the public gateway (validates JWTs issued by agent-svc)
* Transport: HTTP (push-based) via Kong — node → gateway → agent-svc. No inbound to node.

Goals: safe remote execution (RunCommand, UpdateAgent, UpdatePackage), chunked stdout streaming for real-time feel, durable storage, idempotency and retries, node metadata and heartbeat.

---

## 1 — Final file & folder structure (POC-ready)

Top-level (root repo contains `deploy/docker-compose.yml` that builds each service):

```
agent/
├── agent-svc/
│   ├── cmd/api/main.go
│   ├── app/
│   │   ├── bootstrap.go
│   │   ├── config.go
│   │   ├── clients/storage_adapter.go
│   │   ├── domains/
│   │   │   ├── agent_registry.go
│   │   │   ├── agent_metadata.go
│   │   │   ├── command_queue.go
│   │   │   └── command_status.go
│   │   ├── dto/
│   │   │   ├── request.go
│   │   │   ├── response.go
│   │   │   └── command_types.go        # command structs & registry
│   │   ├── handlers/
│   │   │   ├── agent_handler.go        # register, heartbeat
│   │   │   └── command_handler.go      # submit, view commands
│   │   ├── services/
│   │   │   ├── jwt_service.go
│   │   │   ├── command_service.go
│   │   │   ├── log_service.go
│   │   │   └── metadata_service.go
│   │   └── utils/
│   │       ├── validator.go            # validates JSON → struct
│   │       └── retry_policy.go
│   ├── proto/agent.proto
│   ├── storage/postgres/
│   │   ├── migrations/
│   │   │   ├── 001_nodes.sql
│   │   │   ├── 002_node_commands.sql
│   │   │   ├── 003_command_logs.sql
│   │   │   └── 004_agent_metadata.sql
│   │   └── pg_store.go
│   ├── Dockerfile
│   └── README.md

├── node-agent/
│   ├── cmd/agent/main.go
│   ├── app/
│   │   ├── bootstrap.go
│   │   ├── config.go
│   │   ├── identity/
│   │   │   ├── manager.go              # identity.json read/write, JWT storage
│   │   │   └── metadata_collector.go   # OS, kernel, IP, CPU, memory, disk
│   │   ├── executor/
│   │   │   ├── command_executor.go
│   │   │   ├── stdout_chunker.go       # buffer/emit logic (time/size)
│   │   │   └── result_sender.go        # push chunks + final status
│   │   ├── handlers/
│   │   │   ├── pull_handler.go         # optional GET /commands/next
│   │   │   └── push_handler.go         # posts logs, status
│   │   ├── services/
│   │   │   ├── runtime.go              # main loop
│   │   │   ├── heartbeat.go
│   │   │   └── offline_buffer.go
│   │   ├── storage/
│   │   │   └── sqlite_store.go         # local commands/logs tables + DAO
│   │   └── utils/
│   │       └── backoff.go
│   ├── conf/config.yaml
│   ├── var/
│   │   └── identity.json               # stored at /var/lib/node-agent/identity.json
│   ├── Dockerfile
│   └── README.md

├── kong-gateway/
│   ├── kong.yml                        # declarative config, JWT plugin config
│   └── README.md

└── deploy/
    ├── docker-compose.yml              # builds agent-svc, node-agent, postgres, kong
    └── env/ (env files)
```

> Each service is its own buildable unit with a Dockerfile and can be run independently; `deploy/docker-compose.yml` aggregates them for the POC.

---

## 2 — Database schemas

### PostgreSQL (agent-svc) — `storage/postgres/migrations/*`

`001_nodes.sql`

```sql
CREATE TABLE IF NOT EXISTS nodes (
  id BIGSERIAL PRIMARY KEY,
  node_id TEXT UNIQUE NOT NULL,
  public_key TEXT,
  attrs JSONB DEFAULT '{}'::jsonb,
  jwt_issued_at TIMESTAMPTZ DEFAULT now(),
  last_seen_at TIMESTAMPTZ DEFAULT now(),
  disabled BOOLEAN DEFAULT FALSE
);
```

`002_node_commands.sql`

```sql
CREATE TABLE IF NOT EXISTS node_commands (
  id BIGSERIAL PRIMARY KEY,
  command_id UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
  node_id TEXT NOT NULL,
  command_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',  -- queued|running|streaming|success|failed|timeout
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  exit_code INT,
  error_msg TEXT,
  CONSTRAINT fk_node FOREIGN KEY(node_id) REFERENCES nodes(node_id)
);
CREATE INDEX idx_node_commands_nodeid ON node_commands(node_id);
```

`003_command_logs.sql`

```sql
CREATE TABLE IF NOT EXISTS command_logs (
  id BIGSERIAL PRIMARY KEY,
  command_id UUID NOT NULL REFERENCES node_commands(command_id) ON DELETE CASCADE,
  offset BIGINT NOT NULL,                     -- sequential chunk index (0,1,2...)
  stream TEXT CHECK (stream IN ('stdout','stderr')) NOT NULL DEFAULT 'stdout',
  data TEXT NOT NULL,                         -- chunk payload (text/base64)
  encoding TEXT DEFAULT 'utf-8',
  size_bytes INT GENERATED ALWAYS AS (length(data)) STORED,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (command_id, offset, stream)
);
CREATE INDEX idx_command_logs_commandid ON command_logs(command_id);
```

`004_agent_metadata.sql`

```sql
CREATE TABLE IF NOT EXISTS agent_metadata (
  id BIGSERIAL PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  os_name TEXT, os_version TEXT, arch TEXT, kernel_version TEXT,
  hostname TEXT, ip_address TEXT, cpu_cores INT, memory_mb INT, disk_gb INT,
  last_updated TIMESTAMPTZ DEFAULT now(),
  UNIQUE (node_id)
);
```

> Note: enable `pgcrypto` or `uuid-ossp` for `gen_random_uuid()` in your init script.

---

### SQLite (node-agent) — `storage/migrations/*` (local store)

`001_node_commands_local.sql`

```sql
CREATE TABLE IF NOT EXISTS node_commands_local (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  command_id TEXT UNIQUE NOT NULL,
  command_type TEXT NOT NULL,
  payload TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',  -- queued|running|success|failed
  retries INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  exit_code INTEGER,
  error_msg TEXT
);
CREATE INDEX IF NOT EXISTS idx_nc_local_cmdid ON node_commands_local(command_id);
```

`002_command_logs_local.sql`

```sql
CREATE TABLE IF NOT EXISTS command_logs_local (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  command_id TEXT NOT NULL,
  offset INTEGER NOT NULL,
  stream TEXT CHECK (stream IN ('stdout','stderr')) NOT NULL,
  data TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',   -- pending|acked|failed
  retries INTEGER DEFAULT 0,
  last_try DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(command_id, offset, stream)
);
CREATE INDEX IF NOT EXISTS idx_logs_local_cmdid ON command_logs_local(command_id);
```

---

## 3 — Request/Response Models (DTOs)

All API communication uses standard HTTP REST with JSON request/response models defined in `app/dto/`:

- **Request DTOs** (`app/dto/request.go`): `RegisterRequest`, `HeartbeatRequest`, `SubmitCommandRequest`, `PushLogsRequest`, `CommandStatusRequest`
- **Response DTOs** (`app/dto/response.go`): `RegisterResponse`, `HeartbeatResponse`, `SubmitCommandResponse`, `PushLogsResponse`, `CommandStatusResponse`, `CommandResponse`, `ErrorResponse`
- **Command Types** (`app/dto/command_types.go`): `RunCommand`, `UpdateAgent`, `UpdatePackage` structs with validation

All endpoints use standard HTTP REST patterns with JSON bodies. See `API.md` for complete API documentation.

---

## 4 — HTTP API endpoints & payloads (push-based model)

All endpoints are behind Kong. Agent authenticates using `Authorization: Bearer <JWT>` after registration.

### 1) Register node

* **POST** `/v1/agents/register`
* **Payload**

```json
{
  "node_id":"node-7",
  "public_key":"ssh-rsa AAAA...",
  "attrs":{"os":"ubuntu","zone":"eu-west-1a"}
}
```

* **Response**

```json
{ "token": "<JWT>", "node_id":"node-7", "expires_in":86400 }
```

* **Validation**: `node_id` required; `public_key` optional.

---

### 2) Heartbeat

* **POST** `/v1/agents/heartbeat`
* **Headers**: `Authorization: Bearer <JWT>`
* **Payload**

```json
{ "node_id": "node-7" }
```

* **Action**: update `nodes.last_seen_at`.

---

### 3) Submit command (Admin → agent-svc)

* **POST** `/v1/commands/submit`
* **Auth**: admin token (not the node token)
* **Payload**

```json
{
  "command_type":"RunCommand",
  "targets":["node-7","node-8"],        // or single node via node_id
  "payload": { "cmd":"ls -la /tmp", "timeout_sec":120 }
}
```

* **Action**: creates `node_commands` rows per target (payload must validate against command type schema).

---

### 4) (Optional) Poll next command — long polling

* **GET** `/v1/commands/next?node_id=node-7&wait=30`
* **Auth**: node JWT
* **Response**:

```json
{ "command_id":"uuid", "command_type":"RunCommand", "payload": { ... } }
```

* **Behavior**: server holds up to `wait` seconds for new command (long-poll).

---

### 5) Push logs (chunked)

* **POST** `/v1/commands/logs`
* **Auth**: node JWT
* **Payload**

```json
{
  "command_id":"uuid",
  "chunks":[
    { "offset": 0, "stream":"stdout", "data":"line1\n" },
    { "offset": 1, "stream":"stdout", "data":"line2\n" }
  ]
}
```

* **Server behavior**

  * For each chunk: `INSERT INTO command_logs ... ON CONFLICT DO NOTHING`
  * Return HTTP `201` with ack details, e.g. `{"acked_offsets":[0,1]}`.
* **Validation**

  * `command_id` exists and assigned to this node
  * `offset` >= 0, `stream` in {stdout,stderr}, `data` non-empty

---

### 6) Post command status (final)

* **POST** `/v1/commands/status`
* **Auth**: node JWT
* **Payload**

```json
{
  "command_id":"uuid",
  "status":"success",
  "exit_code":0,
  "error_msg":null
}
```

* **Server**: updates `node_commands` row, sets `updated_at`.

---

## 5 — Command template registry & validation

Store a small registry (in `agent-svc/app/dto/command_types.go`) mapping `command_type` → Go struct used for validation.

Example structs (Go):

```go
type RunCommand struct {
  Cmd string `json:"cmd" validate:"required"`
  Args []string `json:"args,omitempty"`
  TimeoutSec int `json:"timeout_sec,omitempty"`
}

type UpdateAgent struct {
  Version string `json:"version" validate:"required"`
  URL string `json:"url" validate:"required,url"`
}

type UpdatePackage struct {
  Packages []string `json:"packages" validate:"required"`
  Action string `json:"action" validate:"required,oneof=install remove upgrade"`
}

var CommandRegistry = map[string]interface{}{
  "RunCommand": RunCommand{},
  "UpdateAgent": UpdateAgent{},
  "UpdatePackage": UpdatePackage{},
}
```

### Validation process (server-side)

1. On `/v1/commands/submit`:

   * Read `command_type` and `payload` (raw JSON).
   * Look up `schema` := `CommandRegistry[command_type]`.
   * Unmarshal raw `payload` into a new instance of the `schema` type.
   * Use `go-playground/validator` (or equivalent) to validate field tags.
   * If valid → persist to `node_commands` (payload JSONB).
   * If invalid → return `400` with validation errors.

**Pseudo-code**

```go
schema := registry[commandType]            // reflect.Type
instance := reflect.New(reflect.TypeOf(schema))
json.Unmarshal(rawPayload, instance.Interface())
if err := validate.Struct(instance.Interface()); err != nil {
  return 400, err
}
db.InsertNodeCommand(nodeID, commandType, rawPayload)
```

This keeps validation centralized and extensible: to add new command types, add struct + register in map.

---

## 6 — Node agent behavior (chunking, push & local storage)

**Identity & metadata**

* On first run: `var/identity.json` is created with `node_id`, metadata (OS, ip), and `jwt_token` returned by `/v1/agents/register`.
* Identity file path: `/var/lib/node-agent/identity.json` (configurable).

**Execution & chunking**

* Execute shelled commands with `exec.CommandContext`.
* Read `stdout` and `stderr` concurrently.
* Chunking strategy (configurable):

  * Flush when buffer >= `CHUNK_SIZE` (e.g., 1KB) **OR**
  * Flush after `CHUNK_INTERVAL` (e.g., 2–3s)
* Each chunk gets an `offset` (incremental per command) and saved to local SQLite `command_logs_local` with `status='pending'`.
* Uploader routine sends batches (`chunks` array) to `/v1/commands/logs`. On success (HTTP 201) it marks them as `acked` locally. On failure it retries with exponential backoff and increments `retries`.

**Idempotency & ordering**

* The server enforces uniqueness with `UNIQUE(command_id, offset, stream)` and `ON CONFLICT DO NOTHING` inserts to avoid duplicates.
* The agent must persist `offset` in SQLite to avoid reusing the same offset on restarts.
* When resuming after crash, agent re-reads `pending` chunks and re-sends them; server dedupes.

**Final status**

* After process exit, send `/v1/commands/status` (success/failed + exit_code). The server updates `node_commands`.

---

## 7 — Idempotency, ordering & failure modes

**Out-of-order**: server orders chunks by `offset` and DB query uses `ORDER BY offset`. Agents may send chunks out-of-order; DB uniqueness handles duplicates.

**Lost chunks**: agent stores chunks locally until server acks. Retry policy: exponential backoff (e.g., 1s,2s,4s,8s,... up to limit).

**Partial uploads**: last processed offset persisted locally. On reconnect agent resumes sending next offsets.

**Duplicate sends**: server uses `ON CONFLICT … DO NOTHING`, safe upsert semantics.

---

## 8 — Cleanup & retention

**Local (node-agent)**:

* Immediately mark chunks `acked` on server success and delete them in a background cleanup job (e.g., after 15 minutes).
* Keep completed `node_commands_local` for short-term (e.g., 24 hours), then delete.

**Server (agent-svc)**:

* Retain `command_logs` for a configured retention (e.g., 7–30 days). Periodic cleaner job purges older rows.
* Keep `nodes` and `agent_metadata` as long as needed for auditing.

---

## 9 — JWT, registration & Kong

**JWT**

* Created by `agent-svc` (`jwt_service.go`) during `POST /v1/agents/register`, contains `sub=node_id`, `iss=agent-svc`, `iat`, `exp`.
* POC: HS256 HMAC with secret from env (production: use RS256 and KMS-managed keys).
* JWT stored in agent's identity file for further calls.

**Kong**

* Kong configured to validate JWT on agent routes (`/v1/agents/*`, `/v1/commands/*`). Kong uses same shared secret (for HS256) or public key for RS256 verification.
* Kong proxies authenticated requests to agent-svc.

---

## 10 — Deployment (docker-compose essentials)

`deploy/docker-compose.yml` includes services:

* `postgres` (volumes), exposes 5432
* `agent-svc` built from `agent-svc/` Dockerfile
* `node-agent` built from `node-agent/` Dockerfile (local state mounted)
* `kong` with `kong.yml` mounted

Environment variables:

* `JWT_SIGNING_SECRET` for agent-svc + configure same secret/plugin in Kong
* DB settings (PG host/user/password)
* Agent config (identity path, CHUNK_SIZE, CHUNK_INTERVAL)

---

## 11 — Minimal cURL examples

Register:

```bash
curl -X POST http://kong:8000/v1/agents/register \
 -H "Content-Type: application/json" \
 -d '{"node_id":"node-7","attrs":{"os":"ubuntu"}}'
```

Push chunks:

```bash
curl -X POST http://kong:8000/v1/commands/logs \
 -H "Authorization: Bearer $JWT" \
 -H "Content-Type: application/json" \
 -d '{
   "command_id":"uuid",
   "chunks":[ {"offset":0,"stream":"stdout","data":"line1\n"} ]
 }'
```

Post status:

```bash
curl -X POST http://kong:8000/v1/commands/status \
 -H "Authorization: Bearer $JWT" \
 -H "Content-Type: application/json" \
 -d '{"command_id":"uuid","status":"success","exit_code":0}'
```

---

## 12 — Final notes (implementation tips)

* Use `validator` (go-playground) for struct validation.
* Use `sqlx` or `pgx` for DB access. Use prepared statements for repeated inserts (chunks).
* Use `ON CONFLICT DO NOTHING` for chunk inserts to guarantee idempotency.
* Persist offsets and pending chunks in SQLite until acked.
* Use long-poll (`/v1/commands/next`) for better real-time behavior if you later want near-immediate delivery without streaming.
* Keep node identity and JWT under `/var/lib/node-agent/identity.json` with tight filesystem permissions.
* Monitor `nodes.last_seen_at` for offline detection.

---

## 13 — Where to start (POC implementation order)

1. Implement Postgres schemas & migrations.
2. Implement `agent-svc` register + JWT generation + heartbeat + basic command submit API + chunk insert handler.
3. Implement `node-agent` identity manager, registration, and local SQLite with chunk store.
4. Implement executor + chunker + uploader with retries and simple tests.
5. Add Kong config for JWT enforcement and run via `docker-compose`.
6. Add cleanup jobs and metadata syncing.

---