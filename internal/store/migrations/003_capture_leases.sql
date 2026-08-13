ALTER TABLE captures ADD COLUMN IF NOT EXISTS claimed_by TEXT NOT NULL DEFAULT '';
ALTER TABLE captures ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE captures ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS captures_lease_idx ON captures(status, lease_expires_at, created_at);
