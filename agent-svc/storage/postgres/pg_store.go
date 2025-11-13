package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
// The database must already exist - creation should be handled at the infrastructure/deployment level
func NewStore(connString string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close closes the connection pool
func (s *Store) Close() {
	s.pool.Close()
}

// RegisterNode registers a new node
func (s *Store) RegisterNode(ctx context.Context, nodeID string, attrs map[string]interface{}) error {
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		return fmt.Errorf("failed to marshal attrs: %w", err)
	}

	query := `
		INSERT INTO nodes (node_id, attrs, last_seen_at)
		VALUES ($1, $2::jsonb, $3)
		ON CONFLICT (node_id) 
		DO UPDATE SET 
			attrs = EXCLUDED.attrs,
			last_seen_at = EXCLUDED.last_seen_at
	`
	_, err = s.pool.Exec(ctx, query, nodeID, string(attrsJSON), time.Now())
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
	query := `SELECT id, node_id, attrs, last_seen_at, disabled FROM nodes WHERE node_id = $1`

	err := s.pool.QueryRow(ctx, query, nodeID).Scan(
		&node.ID, &node.NodeID, &node.Attrs, &node.LastSeenAt, &node.Disabled,
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

// GetNextCommand retrieves up to 5 queued commands for a node
func (s *Store) GetNextCommand(ctx context.Context, nodeID string) ([]*domains.NodeCommand, error) {
	query := `
		SELECT id, command_id, node_id, command_type, payload, status, created_at, updated_at, exit_code, error_msg
		FROM node_commands
		WHERE node_id = $1 AND status = 'queued'
		ORDER BY created_at ASC
		LIMIT 5
	`

	rows, err := s.pool.Query(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []*domains.NodeCommand
	var commandIDs []uuid.UUID

	for rows.Next() {
		var cmd domains.NodeCommand
		var payloadJSON []byte
		err := rows.Scan(
			&cmd.ID, &cmd.CommandID, &cmd.NodeID, &cmd.CommandType, &payloadJSON, &cmd.Status,
			&cmd.CreatedAt, &cmd.UpdatedAt, &cmd.ExitCode, &cmd.ErrorMsg,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(payloadJSON, &cmd.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}

		commands = append(commands, &cmd)
		commandIDs = append(commandIDs, cmd.CommandID)
	}

	if len(commands) == 0 {
		return nil, nil
	}

	if len(commandIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(commandIDs))
	args := make([]interface{}, len(commandIDs)+1)
	args[0] = time.Now()
	for i, cmdID := range commandIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = cmdID
	}

	updateQuery := fmt.Sprintf(`
		UPDATE node_commands 
		SET status = 'running', updated_at = $1
		WHERE command_id IN (%s) AND status = 'queued'
	`, strings.Join(placeholders, ","))

	result, err := s.pool.Exec(ctx, updateQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update command status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, nil
	}

	for _, cmd := range commands {
		cmd.Status = "running"
	}

	return commands, nil
}

// UpdateCommandStatus updates command status
func (s *Store) UpdateCommandStatus(ctx context.Context, commandID uuid.UUID, status string, exitCode *int, errorMsg *string) error {
	query := `
		UPDATE node_commands 
		SET status = $1, exit_code = $2, error_msg = $3, updated_at = $4
		WHERE command_id = $5
	`
	_, err := s.pool.Exec(ctx, query, status, exitCode, errorMsg, time.Now(), commandID)
	if err != nil {
		return err
	}

	if status == "success" || status == "failed" || status == "timeout" {
		if err := s.MarkAllChunksAsFinal(ctx, commandID); err != nil {
		}
	}

	return nil
}

// MarkAllChunksAsFinal marks all non-final chunks for a command as final
func (s *Store) MarkAllChunksAsFinal(ctx context.Context, commandID uuid.UUID) error {
	query := `
		UPDATE command_logs 
		SET is_final = TRUE
		WHERE command_id = $1 AND is_final = FALSE
	`
	_, err := s.pool.Exec(ctx, query, commandID)
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

	ackedChunkIndexes := make([]int64, 0, len(chunks))

	query := `
		INSERT INTO command_logs (command_id, chunk_index, stream, data, encoding, is_final)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (command_id, chunk_index, stream) DO UPDATE SET is_final = EXCLUDED.is_final
		RETURNING chunk_index
	`

	for _, chunk := range chunks {
		var chunkIndex int64
		err := s.pool.QueryRow(ctx, query, commandID, chunk.ChunkIndex, chunk.Stream, chunk.Data, chunk.Encoding, chunk.IsFinal).Scan(&chunkIndex)
		if err == nil {
			ackedChunkIndexes = append(ackedChunkIndexes, chunkIndex)
		} else if err != pgx.ErrNoRows {
			// If it's not a "no rows" error (which means conflict), return error
			return nil, err
		}
		// If ErrNoRows, it means conflict (already exists), skip it
	}

	return ackedChunkIndexes, nil
}

// GetCommandLogs retrieves logs for a command ordered by chunk_index
// Returns all logs for the command, even if it's not finished
// If afterChunkIndex is provided, only returns logs with chunk_index >= afterChunkIndex (inclusive)
func (s *Store) GetCommandLogs(ctx context.Context, commandID uuid.UUID, afterChunkIndex *int64) ([]domains.CommandLog, error) {
	query := `
		SELECT id, command_id, chunk_index, stream, data, encoding, is_final
		FROM command_logs
		WHERE command_id = $1
	`
	args := []interface{}{commandID}

	if afterChunkIndex != nil {
		query += ` AND chunk_index >= $2`
		args = append(args, *afterChunkIndex)
	}

	query += ` ORDER BY chunk_index ASC, stream ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domains.CommandLog
	for rows.Next() {
		var log domains.CommandLog
		err := rows.Scan(
			&log.ID, &log.CommandID, &log.ChunkIndex, &log.Stream, &log.Data,
			&log.Encoding, &log.IsFinal,
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

// ListNodes retrieves all registered nodes
func (s *Store) ListNodes(ctx context.Context) ([]domains.Node, error) {
	query := `SELECT id, node_id, attrs, last_seen_at, disabled FROM nodes ORDER BY last_seen_at DESC`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []domains.Node
	for rows.Next() {
		var node domains.Node
		err := rows.Scan(
			&node.ID, &node.NodeID, &node.Attrs, &node.LastSeenAt, &node.Disabled,
		)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// DeleteQueuedCommands deletes all queued commands and their associated log chunks
func (s *Store) DeleteQueuedCommands(ctx context.Context, nodeID *string) (int, error) {
	// Delete associated log chunks using subquery
	if nodeID != nil {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM command_logs 
			WHERE command_id IN (
				SELECT command_id FROM node_commands 
				WHERE status = 'queued' AND node_id = $1
			)
		`, *nodeID)
		if err != nil {
			return 0, err
		}
	} else {
		_, err := s.pool.Exec(ctx, `
			DELETE FROM command_logs 
			WHERE command_id IN (
				SELECT command_id FROM node_commands 
				WHERE status = 'queued'
			)
		`)
		if err != nil {
			return 0, err
		}
	}

	// Delete commands and get count
	var deleteQuery string
	if nodeID != nil {
		deleteQuery = `DELETE FROM node_commands WHERE status = 'queued' AND node_id = $1`
		result, err := s.pool.Exec(ctx, deleteQuery, *nodeID)
		if err != nil {
			return 0, err
		}
		return int(result.RowsAffected()), nil
	} else {
		deleteQuery = `DELETE FROM node_commands WHERE status = 'queued'`
		result, err := s.pool.Exec(ctx, deleteQuery)
		if err != nil {
			return 0, err
		}
		return int(result.RowsAffected()), nil
	}
}

// ListCommands retrieves commands, optionally filtered by nodeID
func (s *Store) ListCommands(ctx context.Context, nodeID *string, limit int) ([]domains.NodeCommand, error) {
	query := `
		SELECT id, command_id, node_id, command_type, payload, status, created_at, updated_at, exit_code, error_msg
		FROM node_commands
	`
	args := []interface{}{}
	argIdx := 1

	if nodeID != nil {
		query += ` WHERE node_id = $1`
		args = append(args, *nodeID)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`

	if limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []domains.NodeCommand
	for rows.Next() {
		var cmd domains.NodeCommand
		var payloadJSON []byte
		err := rows.Scan(
			&cmd.ID, &cmd.CommandID, &cmd.NodeID, &cmd.CommandType, &payloadJSON, &cmd.Status,
			&cmd.CreatedAt, &cmd.UpdatedAt, &cmd.ExitCode, &cmd.ErrorMsg,
		)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payloadJSON, &cmd.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
		commands = append(commands, cmd)
	}
	return commands, rows.Err()
}
