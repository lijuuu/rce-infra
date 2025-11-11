package domains

import (
	"time"

	"github.com/google/uuid"
)

// NodeCommand represents a command in the queue
type NodeCommand struct {
	ID          int64                  `db:"id"`
	CommandID   uuid.UUID              `db:"command_id"`
	NodeID      string                 `db:"node_id"`
	CommandType string                 `db:"command_type"`
	Payload     map[string]interface{} `db:"payload"`
	Status      string                 `db:"status"`
	CreatedAt   time.Time              `db:"created_at"`
	UpdatedAt   time.Time              `db:"updated_at"`
	ExitCode    *int                   `db:"exit_code"`
	ErrorMsg    *string                `db:"error_msg"`
}
