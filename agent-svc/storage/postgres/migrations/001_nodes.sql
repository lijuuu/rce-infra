CREATE TABLE IF NOT EXISTS nodes (
  id BIGSERIAL PRIMARY KEY,
  node_id TEXT UNIQUE NOT NULL,
  public_key TEXT,
  attrs JSONB DEFAULT '{}'::jsonb,
  jwt_issued_at TIMESTAMPTZ DEFAULT now(),
  last_seen_at TIMESTAMPTZ DEFAULT now(),
  disabled BOOLEAN DEFAULT FALSE
);

