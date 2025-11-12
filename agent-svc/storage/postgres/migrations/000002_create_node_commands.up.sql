CREATE TABLE IF NOT EXISTS node_commands (
  id BIGSERIAL PRIMARY KEY,
  command_id UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
  node_id TEXT NOT NULL,
  command_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',  -- queued|running|streaming|success|failed|timeout
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  exit_code INT,
  error_msg TEXT,
  CONSTRAINT fk_node FOREIGN KEY(node_id) REFERENCES nodes(node_id)
);
CREATE INDEX IF NOT EXISTS idx_node_commands_nodeid ON node_commands(node_id);