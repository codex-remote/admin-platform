CREATE TABLE IF NOT EXISTS incidents (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 160),
  summary TEXT NOT NULL DEFAULT '' CHECK (char_length(summary) <= 2000),
  severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
  anchor_event_id BIGINT,
  window_start TIMESTAMPTZ NOT NULL,
  window_end TIMESTAMPTZ NOT NULL,
  session_id TEXT NOT NULL DEFAULT '',
  trace_id TEXT NOT NULL DEFAULT '',
  turn_ref TEXT NOT NULL DEFAULT '',
  event_count INTEGER NOT NULL DEFAULT 0,
  artifact_count INTEGER NOT NULL DEFAULT 0,
  truncated BOOLEAN NOT NULL DEFAULT false,
  snapshot_version INTEGER NOT NULL DEFAULT 1,
  snapshot_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  CHECK (window_end >= window_start)
);

CREATE INDEX IF NOT EXISTS incidents_created_at_idx ON incidents(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS incidents_status_idx ON incidents(status, created_at DESC);

CREATE TABLE IF NOT EXISTS incident_events (
  incident_id BIGINT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL,
  source_event_id BIGINT,
  snapshot_json JSONB NOT NULL,
  PRIMARY KEY (incident_id, ordinal)
);

CREATE INDEX IF NOT EXISTS incident_events_source_idx ON incident_events(source_event_id);

CREATE TABLE IF NOT EXISTS incident_artifacts (
  incident_id BIGINT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL,
  artifact_id BIGINT REFERENCES artifacts(id) ON DELETE SET NULL,
  manifest_json JSONB NOT NULL,
  PRIMARY KEY (incident_id, ordinal)
);
