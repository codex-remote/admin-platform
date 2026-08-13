# CodexRemote Admin Platform

Local-first diagnostics control plane for CodexRemote. It provides a PostgreSQL-backed Admin Server and Web dashboard plus an independent Collector for Relay, Mac Agent, Simulator app-container logs, and allowlisted CoreDevice capture tasks.

The dashboard is the D-scheme diagnostics workbench defined in `../Codex Remote/01-架构设计/Admin Web 产品与前端架构.md`. It uses a React/TypeScript frontend embedded by the Go Admin Server, the existing PostgreSQL database, and one versioned `/api/v1` contract. The legacy DOM application and unversioned Admin APIs have been removed.

## Architecture

- `admin-server`: loopback HTTP API, dashboard, PostgreSQL queries, immutable incident snapshots, artifacts, and capture queue.
- `diagnostics-collector`: file checkpoints, durable upload spool, Simulator container discovery, and CoreDevice commands.
- `admin-migrate`: schema migration entry point using a separate database role.
- PostgreSQL stores indexed events and control-plane metadata. Raw artifacts stay in the local data directory.

Collector never connects to PostgreSQL. It persists a batch before upload and advances its checkpoint only after an HTTP success response. Event fingerprints make replay idempotent.

## Local Development

Prerequisites: Go 1.24+, Node.js 22+, PostgreSQL 14+, Xcode command-line tools, and `rg`. The bootstrap defaults to the local `root` PostgreSQL role only for role/database creation; Admin runtime uses `codexremote_admin_app`, which is not a superuser.

```bash
./dev start
./dev status
```

Open [http://127.0.0.1:18880](http://127.0.0.1:18880). Runtime data is under `.run/` and is ignored by Git. Stop the two independent services with `./dev stop`.

For a non-default PostgreSQL bootstrap credential, set `ADMIN_BOOTSTRAP_DATABASE_URL`. Set the same non-empty `ADMIN_INGEST_TOKEN` for Admin and Collector when ingestion authentication is required. The first-phase server refuses non-loopback listeners; remote access must use a reviewed authenticated TLS proxy in front of the loopback service.

## Simulator Loop

1. Start `../mac-agent/dev simulator`.
2. Build and launch `../iphone-app` on a booted Simulator.
3. Run `./dev start`.
4. Collector resolves the current App Data Container with `xcrun simctl get_app_container`, tails `Library/Application Support/Diagnostics/events*.jsonl`, and submits acknowledged batches.

Relay and Mac Agent JSONL files are discovered from sibling `.run/{debug,simulator,iphone}` directories. File identity, offset, truncation, rotation, partial-line handling, batch limits, and a disk spool are maintained locally.

## Verification

```bash
make test
make vet
make build
curl -fsS http://127.0.0.1:18880/api/healthz
curl -fsS http://127.0.0.1:18880/api/v1/overview
curl -fsS 'http://127.0.0.1:18880/api/v1/diagnostics/events?limit=10'
```

The JSON API is the machine/AI-readable interface. The dashboard is the human-readable interface; both query the same normalized records and correlation IDs.

Select an event and use **建立事故** to freeze a bounded evidence window. The Incident view stores normalized event snapshots and artifact manifests transactionally; `/api/v1/incidents/{id}/snapshot` exports the same versioned evidence used by the dashboard.

## Maintenance

- Back up PostgreSQL and `.run/admin-data/artifacts` together when evidence retention matters.
- Check `.run/collector-state/spool` for upload backlog and the dashboard Collector status for heartbeat freshness.
- A stale Collector heartbeat does not stop Relay, Mac Agent, or iPhone operation.
- Migrations are append-only numbered SQL files. Apply them with `make migrate` before deploying a new binary.
- Do not log prompts, responses, credentials, relay URLs, or complete user file paths.
- Event fields pass through a central metadata allowlist before PostgreSQL storage. Additions require a privacy review and tests in `internal/privacy`.
