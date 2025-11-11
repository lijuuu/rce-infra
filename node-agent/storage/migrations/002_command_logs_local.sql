CREATE TABLE IF NOT EXISTS command_logs_local (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  command_id TEXT NOT NULL,
  offset INTEGER NOT NULL,
  stream TEXT CHECK (stream IN ('stdout','stderr')) NOT NULL,
  data TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',   -- pending|acked|failed
  retries INTEGER DEFAULT 0,
  last_try DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(command_id, offset, stream)
);
CREATE INDEX IF NOT EXISTS idx_logs_local_cmdid ON command_logs_local(command_id);

