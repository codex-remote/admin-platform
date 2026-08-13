#!/bin/zsh
set -euo pipefail

bootstrap_url="${ADMIN_BOOTSTRAP_DATABASE_URL:-postgres://root@127.0.0.1:5432/postgres?sslmode=disable}"
database_name="codexremote_admin"
owner_role="codexremote_admin_owner"
migrator_role="codexremote_admin_migrator"
runtime_role="codexremote_admin_app"

psql "$bootstrap_url" -v ON_ERROR_STOP=1 \
  -v owner_role="$owner_role" \
  -v migrator_role="$migrator_role" \
  -v runtime_role="$runtime_role" <<'SQL'
SELECT format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE', :'owner_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'owner_role') \gexec
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE', :'migrator_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'migrator_role') \gexec
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE', :'runtime_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'runtime_role') \gexec
SELECT format('GRANT %I TO %I', :'owner_role', :'migrator_role') \gexec
SQL

if ! psql "$bootstrap_url" -Atqc "SELECT 1 FROM pg_database WHERE datname='$database_name'" | rg -qx 1; then
  createdb --maintenance-db="$bootstrap_url" --owner="$owner_role" "$database_name"
fi

migrator_psql_url="postgres://${migrator_role}@127.0.0.1:5432/${database_name}?sslmode=disable"
migrator_app_url="${migrator_psql_url}&timezone=UTC"
ADMIN_MIGRATOR_DATABASE_URL="$migrator_app_url" go run ./cmd/migrate

psql "$bootstrap_url" -v ON_ERROR_STOP=1 \
  -v database_name="$database_name" \
  -v runtime_role="$runtime_role" <<'SQL'
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'database_name', :'runtime_role') \gexec
SQL

psql "$migrator_psql_url" -v ON_ERROR_STOP=1 -v runtime_role="$runtime_role" <<'SQL'
SELECT format('GRANT USAGE ON SCHEMA public TO %I', :'runtime_role') \gexec
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %I', :'runtime_role') \gexec
SELECT format('GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO %I', :'runtime_role') \gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', :'runtime_role') \gexec
SELECT format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO %I', :'runtime_role') \gexec
SQL

print "PostgreSQL ready: ${database_name} (migrator=${migrator_role}, runtime=${runtime_role})"
