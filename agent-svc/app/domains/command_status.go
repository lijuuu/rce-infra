package domains

import "time"

// CommandLog represents a log chunk
type CommandLog struct {
	ID         int64     `db:"id"`
	CommandID  string    `db:"command_id"`
	ChunkIndex int64     `db:"chunk_index"`
	Stream     string    `db:"stream"`
	Data       string    `db:"data"`
	Encoding   string    `db:"encoding"`
	SizeBytes  int       `db:"size_bytes"`
	IsFinal    bool      `db:"is_final"`
	CreatedAt  time.Time `db:"created_at"`
}
