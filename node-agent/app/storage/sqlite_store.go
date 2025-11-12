package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Store represents the SQLite storage implementation
type Store struct {
	db *sql.DB
}

// NewStore creates a new SQLite store
func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &Store{db: db}

	// Run migrations
	if err := store.runMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// runMigrations runs SQL migrations
func (s *Store) runMigrations() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS node_commands_local (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command_id TEXT UNIQUE NOT NULL,
			command_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'queued',
			retries INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			exit_code INTEGER,
			error_msg TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_nc_local_cmdid ON node_commands_local(command_id)`,
		`CREATE TABLE IF NOT EXISTS command_logs_local (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command_id TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			stream TEXT CHECK (stream IN ('stdout','stderr')) NOT NULL,
			data TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			retries INTEGER DEFAULT 0,
			last_try DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(command_id, chunk_index, stream)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_local_cmdid ON command_logs_local(command_id)`,
	}

	for _, migration := range migrations {
		if _, err := s.db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// LocalCommand represents a command in local storage
type LocalCommand struct {
	ID          int64
	CommandID   string
	CommandType string
	Payload     string
	Status      string
	Retries     int
	CreatedAt   string
	UpdatedAt   string
	ExitCode    *int
	ErrorMsg    *string
}

// SaveCommand saves a command locally
func (s *Store) SaveCommand(ctx context.Context, commandID, commandType, payload string) error {
	query := `
		INSERT INTO node_commands_local (command_id, command_type, payload, status)
		VALUES (?, ?, ?, 'queued')
		ON CONFLICT(command_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, commandID, commandType, payload)
	return err
}

// GetNextQueuedCommand retrieves the next queued command
func (s *Store) GetNextQueuedCommand(ctx context.Context) (*LocalCommand, error) {
	query := `
		SELECT id, command_id, command_type, payload, status, retries, created_at, updated_at, exit_code, error_msg
		FROM node_commands_local
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1
	`

	var cmd LocalCommand
	err := s.db.QueryRowContext(ctx, query).Scan(
		&cmd.ID, &cmd.CommandID, &cmd.CommandType, &cmd.Payload, &cmd.Status,
		&cmd.Retries, &cmd.CreatedAt, &cmd.UpdatedAt, &cmd.ExitCode, &cmd.ErrorMsg,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

// UpdateCommandStatus updates command status
func (s *Store) UpdateCommandStatus(ctx context.Context, commandID, status string, exitCode *int, errorMsg *string) error {
	query := `
		UPDATE node_commands_local
		SET status = ?, exit_code = ?, error_msg = ?, updated_at = CURRENT_TIMESTAMP
		WHERE command_id = ?
	`
	_, err := s.db.ExecContext(ctx, query, status, exitCode, errorMsg, commandID)
	return err
}

// LogChunk represents a log chunk in local storage
type LogChunk struct {
	ID         int64
	CommandID  string
	ChunkIndex int64
	Stream     string
	Data       string
	Status     string
	Retries    int
	LastTry    *string
	CreatedAt  string
}

// SaveLogChunk saves a log chunk locally
func (s *Store) SaveLogChunk(ctx context.Context, commandID string, chunkIndex int64, stream, data string) error {
	query := `
		INSERT INTO command_logs_local (command_id, chunk_index, stream, data, status)
		VALUES (?, ?, ?, ?, 'pending')
		ON CONFLICT(command_id, chunk_index, stream) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, commandID, chunkIndex, stream, data)
	return err
}

// GetPendingChunks retrieves pending chunks for a command
func (s *Store) GetPendingChunks(ctx context.Context, commandID string) ([]LogChunk, error) {
	query := `
		SELECT id, command_id, chunk_index, stream, data, status, retries, last_try, created_at
		FROM command_logs_local
		WHERE command_id = ? AND status = 'pending'
		ORDER BY chunk_index ASC, stream ASC
	`

	rows, err := s.db.QueryContext(ctx, query, commandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []LogChunk
	for rows.Next() {
		var chunk LogChunk
		err := rows.Scan(
			&chunk.ID, &chunk.CommandID, &chunk.ChunkIndex, &chunk.Stream, &chunk.Data,
			&chunk.Status, &chunk.Retries, &chunk.LastTry, &chunk.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}

// GetCommandsWithPendingChunks returns distinct command IDs that have pending chunks
func (s *Store) GetCommandsWithPendingChunks(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT command_id
		FROM command_logs_local
		WHERE status = 'pending'
		ORDER BY command_id
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commandIDs []string
	for rows.Next() {
		var commandID string
		if err := rows.Scan(&commandID); err != nil {
			return nil, err
		}
		commandIDs = append(commandIDs, commandID)
	}

	return commandIDs, rows.Err()
}

// MarkChunksAcked marks chunks as acked
func (s *Store) MarkChunksAcked(ctx context.Context, commandID string, chunkIndexes []int64) error {
	if len(chunkIndexes) == 0 {
		return nil
	}

	// Build placeholders for IN clause
	placeholders := ""
	args := make([]interface{}, len(chunkIndexes)+1)
	args[0] = commandID
	for i, chunkIndex := range chunkIndexes {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i+1] = chunkIndex
	}

	query := fmt.Sprintf(`
		UPDATE command_logs_local
		SET status = 'acked', last_try = CURRENT_TIMESTAMP
		WHERE command_id = ? AND chunk_index IN (%s)
	`, placeholders)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// IncrementChunkRetries increments retry count for chunks
func (s *Store) IncrementChunkRetries(ctx context.Context, commandID string, chunkIndexes []int64) error {
	if len(chunkIndexes) == 0 {
		return nil
	}

	placeholders := ""
	args := make([]interface{}, len(chunkIndexes)+1)
	args[0] = commandID
	for i, chunkIndex := range chunkIndexes {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i+1] = chunkIndex
	}

	query := fmt.Sprintf(`
		UPDATE command_logs_local
		SET retries = retries + 1, last_try = CURRENT_TIMESTAMP
		WHERE command_id = ? AND chunk_index IN (%s)
	`, placeholders)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// CleanupAckedChunks deletes acked chunks older than specified minutes
func (s *Store) CleanupAckedChunks(ctx context.Context, olderThanMinutes int) error {
	query := `
		DELETE FROM command_logs_local
		WHERE status = 'acked' AND datetime(created_at, '+' || ? || ' minutes') < datetime('now')
	`
	_, err := s.db.ExecContext(ctx, query, olderThanMinutes)
	return err
}

// CleanupCompletedCommands deletes completed commands older than specified hours
func (s *Store) CleanupCompletedCommands(ctx context.Context, olderThanHours int) error {
	query := `
		DELETE FROM node_commands_local
		WHERE status IN ('success', 'failed') AND datetime(created_at, '+' || ? || ' hours') < datetime('now')
	`
	_, err := s.db.ExecContext(ctx, query, olderThanHours)
	return err
}
