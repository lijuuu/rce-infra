#!/bin/bash

# Script to clear all databases (PostgreSQL and SQLite) in containers
# Usage: ./clear-databases.sh [--yes] to skip confirmation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check for --yes flag
SKIP_CONFIRM=false
if [ "$1" == "--yes" ] || [ "$1" == "-y" ]; then
    SKIP_CONFIRM=true
fi

echo "=== Clearing All Databases ==="
echo ""
echo -e "${RED}WARNING: This will delete ALL data from:${NC}"
echo "  - PostgreSQL database (nodes, commands, logs, metadata)"
echo "  - All SQLite databases in node-agent containers"
echo ""

if [ "$SKIP_CONFIRM" = false ]; then
    read -p "Are you sure you want to continue? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        echo "Aborted."
        exit 0
    fi
fi

# Load environment variables from .env if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Default values
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-agentdb}

echo -e "${YELLOW}Clearing PostgreSQL database...${NC}"

# Check if postgres container is running
if ! docker ps | grep -q "postgres"; then
    echo -e "${RED}PostgreSQL container is not running!${NC}"
    exit 1
fi

# Clear PostgreSQL database
echo "Truncating all tables in PostgreSQL..."
docker exec postgres psql -U "$DB_USER" -d "$DB_NAME" <<EOF
-- Disable foreign key checks temporarily
SET session_replication_role = 'replica';

-- Truncate all tables (ignore errors if tables don't exist)
DO \$\$
BEGIN
    TRUNCATE TABLE command_logs CASCADE;
EXCEPTION WHEN undefined_table THEN NULL;
END \$\$;

DO \$\$
BEGIN
    TRUNCATE TABLE node_commands CASCADE;
EXCEPTION WHEN undefined_table THEN NULL;
END \$\$;

DO \$\$
BEGIN
    TRUNCATE TABLE agent_metadata CASCADE;
EXCEPTION WHEN undefined_table THEN NULL;
END \$\$;

DO \$\$
BEGIN
    TRUNCATE TABLE nodes CASCADE;
EXCEPTION WHEN undefined_table THEN NULL;
END \$\$;

-- Re-enable foreign key checks
SET session_replication_role = 'origin';

-- Reset sequences (ignore errors if sequences don't exist)
DO \$\$
BEGIN
    ALTER SEQUENCE nodes_id_seq RESTART WITH 1;
EXCEPTION WHEN undefined_object THEN NULL;
END \$\$;

DO \$\$
BEGIN
    ALTER SEQUENCE node_commands_id_seq RESTART WITH 1;
EXCEPTION WHEN undefined_object THEN NULL;
END \$\$;

DO \$\$
BEGIN
    ALTER SEQUENCE command_logs_id_seq RESTART WITH 1;
EXCEPTION WHEN undefined_object THEN NULL;
END \$\$;

DO \$\$
BEGIN
    ALTER SEQUENCE agent_metadata_id_seq RESTART WITH 1;
EXCEPTION WHEN undefined_object THEN NULL;
END \$\$;
EOF

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ PostgreSQL database cleared successfully${NC}"
else
    echo -e "${RED}✗ Failed to clear PostgreSQL database${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Clearing SQLite databases in node-agent containers...${NC}"

# Find all node-agent containers (handles both docker-compose and docker swarm naming)
NODE_AGENT_CONTAINERS=$(docker ps --filter "name=node-agent" --format "{{.Names}}" 2>/dev/null || true)

if [ -z "$NODE_AGENT_CONTAINERS" ]; then
    echo -e "${YELLOW}No node-agent containers found (this is okay if they're not running)${NC}"
else
    CLEARED_COUNT=0
    # Clear SQLite databases in each node-agent container
    for container in $NODE_AGENT_CONTAINERS; do
        echo "Processing container: $container"
        
        # Check if container is running
        if ! docker ps --format "{{.Names}}" | grep -q "^${container}$"; then
            echo "  Container $container is not running, skipping..."
            continue
        fi
        
        # Find all SQLite database files in the container
        DB_FILES=$(docker exec "$container" find /var/lib/node-agent -name "*.db" -type f 2>/dev/null || true)
        
        if [ -z "$DB_FILES" ]; then
            echo "  No SQLite databases found in $container"
        else
            for db_file in $DB_FILES; do
                echo "  Clearing: $db_file"
                docker exec "$container" sqlite3 "$db_file" <<EOF
-- Delete all data from tables
DELETE FROM command_logs_local;
DELETE FROM node_commands_local;

-- Reset auto-increment counters
DELETE FROM sqlite_sequence WHERE name IN ('node_commands_local', 'command_logs_local');
EOF
                if [ $? -eq 0 ]; then
                    echo -e "  ${GREEN}✓ Cleared: $db_file${NC}"
                    CLEARED_COUNT=$((CLEARED_COUNT + 1))
                else
                    echo -e "  ${RED}✗ Failed to clear: $db_file${NC}"
                fi
            done
        fi
    done
    
    if [ $CLEARED_COUNT -gt 0 ]; then
        echo -e "${GREEN}✓ Cleared $CLEARED_COUNT SQLite database(s)${NC}"
    else
        echo -e "${YELLOW}No SQLite databases were found to clear${NC}"
    fi
fi

echo ""
echo -e "${GREEN}=== All databases cleared successfully! ===${NC}"

