CREATE TABLE IF NOT EXISTS node_commands_local (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  command_id TEXT UNIQUE NOT NULL,
  command_type TEXT NOT NULL,
  payload TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',  -- queued|running|success|failed
  retries INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  exit_code INTEGER,
  error_msg TEXT
);
CREATE INDEX IF NOT EXISTS idx_nc_local_cmdid ON node_commands_local(command_id);

