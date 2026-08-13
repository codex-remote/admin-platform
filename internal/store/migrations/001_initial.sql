CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS artifacts (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  kind TEXT NOT NULL,
  source TEXT NOT NULL,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  sha256 TEXT NOT NULL,
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS events (
  id BIGSERIAL PRIMARY KEY,
  timestamp TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  source TEXT NOT NULL,
  profile TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL,
  category TEXT NOT NULL,
  event TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  trace_id TEXT NOT NULL DEFAULT '',
  turn_ref TEXT NOT NULL DEFAULT '',
  sequence BIGINT NOT NULL DEFAULT 0,
  fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  artifact_id BIGINT REFERENCES artifacts(id) ON DELETE SET NULL,
  fingerprint TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS events_timestamp_idx ON events(timestamp DESC, id DESC);
CREATE INDEX IF NOT EXISTS events_source_idx ON events(source, profile, timestamp DESC);
CREATE INDEX IF NOT EXISTS events_trace_idx ON events(trace_id, timestamp DESC) WHERE trace_id <> '';
CREATE INDEX IF NOT EXISTS events_turn_idx ON events(turn_ref, timestamp DESC) WHERE turn_ref <> '';
CREATE INDEX IF NOT EXISTS events_session_idx ON events(session_id, sequence) WHERE session_id <> '';

CREATE TABLE IF NOT EXISTS captures (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  device_id TEXT NOT NULL,
  device_name TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
  full_logs BOOLEAN NOT NULL DEFAULT false,
  output_path TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  artifact_id BIGINT REFERENCES artifacts(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS captures_status_idx ON captures(status, created_at);
