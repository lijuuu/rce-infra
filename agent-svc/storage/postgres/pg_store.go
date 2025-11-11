package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-svc/app/domains"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store represents the Postgres storage implementation
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new Postgres store
// It will attempt to create the database if it doesn't exist
func NewStore(connString string) (*Store, error) {
	// First, try to connect to the database
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		// If connection fails, try to create the database
		if err := ensureDatabaseExists(connString); err != nil {
			return nil, fmt.Errorf("failed to create connection pool: %w", err)
		}
		// Retry connection after creating database
		pool, err = pgxpool.New(context.Background(), connString)
		if err != nil {
			return nil, fmt.Errorf("failed to create connection pool after database creation: %w", err)
		}
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// ensureDatabaseExists creates the database if it doesn't exist
func ensureDatabaseExists(connString string) error {
	// Parse connection string to extract database name
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return fmt.Errorf("failed to parse connection string: %w", err)
	}

	dbName := config.ConnConfig.Database
	if dbName == "" {
		return nil // No database specified, skip creation
	}

	// Connect to postgres database (default database that always exists)
	config.ConnConfig.Database = "postgres"
	adminConn, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil // Can't connect to admin database, skip creation (might not have permissions)
	}
	defer adminConn.Close()

	// Check if database exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`
	err = adminConn.QueryRow(context.Background(), checkQuery, dbName).Scan(&exists)
	if err != nil {
		return nil // Can't check, skip creation
	}

	if exists {
		return nil // Database already exists
	}

	// Create database (using identifier to prevent SQL injection)
	createQuery := fmt.Sprintf(`CREATE DATABASE %s`, pgx.Identifier{dbName}.Sanitize())
	_, err = adminConn.Exec(context.Background(), createQuery)
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	return nil
}

// Close closes the connection pool
func (s *Store) Close() {
	s.pool.Close()
}

// RunMigrations runs all SQL migrations
func (s *Store) RunMigrations(migrations []string) error {
	ctx := context.Background()
	for i, migration := range migrations {
		if _, err := s.pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}

// RegisterNode registers a new node
func (s *Store) RegisterNode(ctx context.Context, nodeID string, publicKey *string, attrs map[string]interface{}) error {
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		return fmt.Errorf("failed to marshal attrs: %w", err)
	}

	query := `
		INSERT INTO nodes (node_id, public_key, attrs, jwt_issued_at, last_seen_at)
		VALUES ($1, $2, $3::jsonb, $4, $4)
		ON CONFLICT (node_id) 
		DO UPDATE SET 
			public_key = EXCLUDED.public_key,
			attrs = EXCLUDED.attrs,
			jwt_issued_at = EXCLUDED.jwt_issued_at,
			last_seen_at = EXCLUDED.last_seen_at
	`
	_, err = s.pool.Exec(ctx, query, nodeID, publicKey, string(attrsJSON), time.Now())
	return err
}

// UpdateNodeLastSeen updates the last_seen_at timestamp
func (s *Store) UpdateNodeLastSeen(ctx context.Context, nodeID string) error {
	query := `UPDATE nodes SET last_seen_at = $1 WHERE node_id = $2`
	_, err := s.pool.Exec(ctx, query, time.Now(), nodeID)
	return err
}

// GetNode retrieves a node by ID
func (s *Store) GetNode(ctx context.Context, nodeID string) (*domains.Node, error) {
	var node domains.Node
	query := `SELECT id, node_id, public_key, attrs, jwt_issued_at, last_seen_at, disabled FROM nodes WHERE node_id = $1`

	err := s.pool.QueryRow(ctx, query, nodeID).Scan(
		&node.ID, &node.NodeID, &node.PublicKey, &node.Attrs, &node.JWTIssuedAt, &node.LastSeenAt, &node.Disabled,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// CreateCommand creates a new command in the queue
func (s *Store) CreateCommand(ctx context.Context, nodeID, commandType string, payload map[string]interface{}) (uuid.UUID, error) {
	commandID := uuid.New()
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO node_commands (command_id, node_id, command_type, payload, status)
		VALUES ($1, $2, $3, $4::jsonb, 'queued')
	`
	_, err = s.pool.Exec(ctx, query, commandID, nodeID, commandType, string(payloadJSON))
	if err != nil {
		return uuid.Nil, err
	}
	return commandID, nil
}

// GetNextCommand retrieves the next queued command for a node (for polling)
func (s *Store) GetNextCommand(ctx context.Context, nodeID string) (*domains.NodeCommand, error) {
	var cmd domains.NodeCommand
	query := `
		SELECT id, command_id, node_id, command_type, payload, status, created_at, updated_at, exit_code, error_msg
		FROM node_commands
		WHERE node_id = $1 AND status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	var payloadJSON []byte
	err := s.pool.QueryRow(ctx, query, nodeID).Scan(
		&cmd.ID, &cmd.CommandID, &cmd.NodeID, &cmd.CommandType, &payloadJSON, &cmd.Status,
		&cmd.CreatedAt, &cmd.UpdatedAt, &cmd.ExitCode, &cmd.ErrorMsg,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(payloadJSON, &cmd.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return &cmd, nil
}

// UpdateCommandStatus updates command status
func (s *Store) UpdateCommandStatus(ctx context.Context, commandID uuid.UUID, status string, exitCode *int, errorMsg *string) error {
	query := `
		UPDATE node_commands 
		SET status = $1, exit_code = $2, error_msg = $3, updated_at = $4
		WHERE command_id = $5
	`
	_, err := s.pool.Exec(ctx, query, status, exitCode, errorMsg, time.Now(), commandID)
	return err
}

// GetCommandByID retrieves a command by ID
func (s *Store) GetCommandByID(ctx context.Context, commandID uuid.UUID) (*domains.NodeCommand, error) {
	var cmd domains.NodeCommand
	query := `
		SELECT id, command_id, node_id, command_type, payload, status, created_at, updated_at, exit_code, error_msg
		FROM node_commands
		WHERE command_id = $1
	`

	var payloadJSON []byte
	err := s.pool.QueryRow(ctx, query, commandID).Scan(
		&cmd.ID, &cmd.CommandID, &cmd.NodeID, &cmd.CommandType, &payloadJSON, &cmd.Status,
		&cmd.CreatedAt, &cmd.UpdatedAt, &cmd.ExitCode, &cmd.ErrorMsg,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(payloadJSON, &cmd.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return &cmd, nil
}

// InsertLogChunks inserts log chunks with idempotency (ON CONFLICT DO NOTHING)
func (s *Store) InsertLogChunks(ctx context.Context, commandID uuid.UUID, chunks []domains.CommandLog) ([]int64, error) {
	if len(chunks) == 0 {
		return []int64{}, nil
	}

	ackedOffsets := make([]int64, 0, len(chunks))

	query := `
		INSERT INTO command_logs (command_id, offset, stream, data, encoding)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (command_id, offset, stream) DO NOTHING
		RETURNING offset
	`

	for _, chunk := range chunks {
		var offset int64
		err := s.pool.QueryRow(ctx, query, commandID, chunk.Offset, chunk.Stream, chunk.Data, chunk.Encoding).Scan(&offset)
		if err == nil {
			ackedOffsets = append(ackedOffsets, offset)
		} else if err != pgx.ErrNoRows {
			// If it's not a "no rows" error (which means conflict), return error
			return nil, err
		}
		// If ErrNoRows, it means conflict (already exists), skip it
	}

	return ackedOffsets, nil
}

// GetCommandLogs retrieves logs for a command ordered by offset
// If afterOffset is provided, only returns logs with offset > afterOffset
func (s *Store) GetCommandLogs(ctx context.Context, commandID uuid.UUID, afterOffset *int64) ([]domains.CommandLog, error) {
	query := `
		SELECT id, command_id, offset, stream, data, encoding, size_bytes, created_at
		FROM command_logs
		WHERE command_id = $1
	`
	args := []interface{}{commandID}

	if afterOffset != nil {
		query += ` AND offset > $2`
		args = append(args, *afterOffset)
	}

	query += ` ORDER BY offset ASC, stream ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domains.CommandLog
	for rows.Next() {
		var log domains.CommandLog
		err := rows.Scan(
			&log.ID, &log.CommandID, &log.Offset, &log.Stream, &log.Data,
			&log.Encoding, &log.SizeBytes, &log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// UpdateAgentMetadata updates or inserts agent metadata
func (s *Store) UpdateAgentMetadata(ctx context.Context, nodeID string, metadata *domains.AgentMetadata) error {
	query := `
		INSERT INTO agent_metadata (
			node_id, os_name, os_version, arch, kernel_version,
			hostname, ip_address, cpu_cores, memory_mb, disk_gb, last_updated
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (node_id) 
		DO UPDATE SET
			os_name = EXCLUDED.os_name,
			os_version = EXCLUDED.os_version,
			arch = EXCLUDED.arch,
			kernel_version = EXCLUDED.kernel_version,
			hostname = EXCLUDED.hostname,
			ip_address = EXCLUDED.ip_address,
			cpu_cores = EXCLUDED.cpu_cores,
			memory_mb = EXCLUDED.memory_mb,
			disk_gb = EXCLUDED.disk_gb,
			last_updated = EXCLUDED.last_updated
	`

	_, err := s.pool.Exec(ctx, query,
		nodeID, metadata.OSName, metadata.OSVersion, metadata.Arch, metadata.KernelVersion,
		metadata.Hostname, metadata.IPAddress, metadata.CPUCores, metadata.MemoryMB, metadata.DiskGB, time.Now(),
	)
	return err
}

// CleanupOldLogs deletes logs older than retention days
func (s *Store) CleanupOldLogs(ctx context.Context, retentionDays int) error {
	query := `
		DELETE FROM command_logs
		WHERE created_at < NOW() - INTERVAL '%d days'
	`
	_, err := s.pool.Exec(ctx, fmt.Sprintf(query, retentionDays))
	return err
}
