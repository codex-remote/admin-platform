CREATE TABLE IF NOT EXISTS collectors (
  id TEXT PRIMARY KEY,
  updated_at TIMESTAMPTZ NOT NULL,
  hostname TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'online',
  pending_batches INTEGER NOT NULL DEFAULT 0,
  details_json JSONB NOT NULL DEFAULT '{}'::jsonb
);
