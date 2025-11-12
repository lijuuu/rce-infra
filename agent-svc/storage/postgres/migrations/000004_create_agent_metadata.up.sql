CREATE TABLE IF NOT EXISTS agent_metadata (
  id BIGSERIAL PRIMARY KEY,
  node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
  os_name TEXT, os_version TEXT, arch TEXT, kernel_version TEXT,
  hostname TEXT, ip_address TEXT, cpu_cores INT, memory_mb INT, disk_gb INT,
  last_updated TIMESTAMPTZ DEFAULT now(),
  UNIQUE (node_id)
);

