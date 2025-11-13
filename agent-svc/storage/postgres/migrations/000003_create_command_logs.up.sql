CREATE TABLE IF NOT EXISTS command_logs (
  id BIGSERIAL PRIMARY KEY,
  command_id UUID NOT NULL REFERENCES node_commands(command_id) ON DELETE CASCADE,
  chunk_index BIGINT NOT NULL,                     -- sequential chunk index (0,1,2...)
  stream TEXT CHECK (stream IN ('stdout','stderr')) NOT NULL DEFAULT 'stdout',
  data TEXT NOT NULL,                         -- chunk payload (text/base64)
  encoding TEXT DEFAULT 'utf-8',
  is_final BOOLEAN DEFAULT FALSE,              -- true if this is the final chunk (work is done)
  UNIQUE (command_id, chunk_index, stream)
);
CREATE INDEX IF NOT EXISTS idx_command_logs_commandid ON command_logs(command_id);
CREATE INDEX IF NOT EXISTS idx_command_logs_is_final ON command_logs(command_id, is_final) WHERE is_final = TRUE;