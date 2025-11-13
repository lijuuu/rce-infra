# Deployment Scripts

## clear-databases.sh

Script to clear all databases (PostgreSQL and SQLite) in containers.

### Usage

```bash
cd deploy
./clear-databases.sh
```

Or skip confirmation prompt:
```bash
./clear-databases.sh --yes
# or
./clear-databases.sh -y
```

### What it does

1. **PostgreSQL**: Truncates all tables in the `postgres` container:
   - `command_logs`
   - `node_commands`
   - `agent_metadata`
   - `nodes`
   - Resets all sequences

2. **SQLite**: Clears all SQLite databases in all running `node-agent` containers:
   - Finds all `.db` files in `/var/lib/node-agent`
   - Deletes all data from `command_logs_local` and `node_commands_local` tables
   - Resets auto-increment counters

### Requirements

- Docker must be running
- PostgreSQL container must be running (named `postgres`)
- Node-agent containers should be running (optional - script handles missing containers gracefully)

### Environment Variables

The script reads from `.env` file if present, or uses defaults:
- `DB_USER` (default: `postgres`)
- `DB_PASSWORD` (default: `postgres`)
- `DB_NAME` (default: `agentdb`)

### Notes

- The script handles scaled node-agent containers (docker-compose scale or docker swarm)
- It safely handles containers that are not running
- All operations are logged with color-coded output

