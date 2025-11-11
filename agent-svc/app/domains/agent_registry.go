package domains

import "time"

// Node represents a registered node
type Node struct {
	ID          int64                  `db:"id"`
	NodeID      string                 `db:"node_id"`
	PublicKey   *string                `db:"public_key"`
	Attrs       map[string]interface{} `db:"attrs"`
	JWTIssuedAt time.Time              `db:"jwt_issued_at"`
	LastSeenAt  time.Time              `db:"last_seen_at"`
	Disabled    bool                   `db:"disabled"`
}
