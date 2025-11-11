package domains

import "time"

// AgentMetadata represents node metadata
type AgentMetadata struct {
	ID            int64     `db:"id"`
	NodeID        string    `db:"node_id"`
	OSName        *string   `db:"os_name"`
	OSVersion     *string   `db:"os_version"`
	Arch          *string   `db:"arch"`
	KernelVersion *string   `db:"kernel_version"`
	Hostname      *string   `db:"hostname"`
	IPAddress     *string   `db:"ip_address"`
	CPUCores      *int      `db:"cpu_cores"`
	MemoryMB      *int      `db:"memory_mb"`
	DiskGB        *int      `db:"disk_gb"`
	LastUpdated   time.Time `db:"last_updated"`
}
