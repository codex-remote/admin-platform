# Changelog

## Unreleased

`0.x` releases remain open to rapid iteration. Compatibility will be decided from the actual impact of each change and recorded here; `0.0.1` does not freeze a compatibility baseline.

- Adopted Apache License 2.0, public contribution guidance, multi-ecosystem Dependabot updates, and the canonical `github.com/codex-remote/admin-platform` module path.

## 0.0.1 - 2026-08-13

- Removed the remaining pre-rebuild residue: obsolete Admin workspace configuration, unused server/store helpers, compatibility wording, placeholder global search UI, unused browser-test dependencies, duplicate font formats, and production source maps; unknown legacy `/api/*` routes now return typed JSON 404 responses instead of the SPA shell.
- Added enforced ESLint checks and narrow-screen dashboard layouts; verified that the 390 x 844 workbench keeps overview content within the viewport while wide event records scroll only inside their table.
- Replaced the legacy DOM dashboard with the D-scheme React/TypeScript workbench: Overview, Events, Incidents, Captures, Artifacts, and Services are backed by real data; Users and User Behavior are visible only as non-interactive `规划中` navigation items.
- Added bounded `/api/v1` diagnostics queries with RFC 3339 time windows, multi-value filters, compound cursors, server-side facets and histograms, stable metadata envelopes, and typed error responses.
- Moved Collector ingestion, heartbeat, capture leases, artifacts, services, devices, and live invalidation to `/api/v1`; removed all unversioned Admin routes and legacy HTML/CSS/JavaScript.
- Added the React/Vite build to the Go embed and CI pipeline, local IBM Plex assets, strict TypeScript, capability tests, API query/cursor tests, and a single `make test`/`make build` workflow.
- Fixed a launchd restart race by waiting for prior Admin/Collector labels and port `18880` to be fully released before submitting replacement processes.
- Added a documented Admin capability lifecycle: available modules are operable, Users and User Behavior are visible as non-interactive `规划中`, and uncertain candidates remain hidden from the product but discoverable with explicit activation criteria.
- Accepted the D-scheme Admin Web architecture and hard-cut development plan: Linear-like shell, Datadog-style event exploration, Sentry-style incidents, versioned APIs, existing PostgreSQL reuse, and mandatory removal of the legacy Web/API implementation after E2E.
- Validated the 2026-08-13 local Simulator ingestion loop: the native app connected to the simulator Relay, Collector acknowledged the new app-container and Relay JSONL offsets with an empty spool, PostgreSQL retained the events across an Admin/Collector restart, and the dashboard event/detail/Incident views passed headed-browser checks with no console errors.
- Added versioned Incident snapshots with transactional event evidence, artifact manifests, provenance, JSON export, and a dedicated management view.
- Added a centralized diagnostic metadata allowlist and a migration that removes previously stored non-allowlisted fields.
- Kept event-row DOM nodes stable across live refreshes so event selection and Incident creation are not interrupted by new log batches.
- Fixed empty event, capture, and artifact API lists to serialize as `[]`.
- Added PostgreSQL-backed Admin Server with versioned migrations and separated database roles.
- Added independent Diagnostics Collector with acknowledged checkpoints, durable spool, Relay/Mac Agent JSONL ingestion, and Simulator app-container ingestion.
- Added local operations dashboard, event streaming, diagnostics artifacts, service health, and device capture controls.
