CREATE TABLE IF NOT EXISTS command_logs (
  id BIGSERIAL PRIMARY KEY,
  command_id UUID NOT NULL REFERENCES node_commands(command_id) ON DELETE CASCADE,
  offset BIGINT NOT NULL,                     -- sequential chunk index (0,1,2...)
  stream TEXT CHECK (stream IN ('stdout','stderr')) NOT NULL DEFAULT 'stdout',
  data TEXT NOT NULL,                         -- chunk payload (text/base64)
  encoding TEXT DEFAULT 'utf-8',
  size_bytes INT GENERATED ALWAYS AS (length(data)) STORED,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE (command_id, offset, stream)
);
CREATE INDEX idx_command_logs_commandid ON command_logs(command_id);

