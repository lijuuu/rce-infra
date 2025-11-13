package domains

import "time"

// Node represents a registered node
type Node struct {
	ID         int64                  `db:"id"`
	NodeID     string                 `db:"node_id"`
	Attrs      map[string]interface{} `db:"attrs"`
	LastSeenAt time.Time              `db:"last_seen_at"`
	Disabled   bool                   `db:"disabled"`
}
