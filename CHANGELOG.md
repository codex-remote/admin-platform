# Changelog

## Unreleased

- Added a documented Admin capability lifecycle: available modules are operable, Users and User Behavior are visible as non-interactive `规划中`, and uncertain candidates remain hidden from the product but discoverable with explicit activation criteria.
- Accepted the D-scheme Admin Web architecture and hard-cut development plan: Linear-like shell, Datadog-style event exploration, Sentry-style incidents, versioned APIs, existing PostgreSQL reuse, and mandatory removal of the legacy Web/API implementation after E2E.
- Validated the 2026-08-13 local Simulator ingestion loop: the native app connected to the simulator Relay, Collector acknowledged the new app-container and Relay JSONL offsets with an empty spool, PostgreSQL retained the events across an Admin/Collector restart, and the dashboard event/detail/Incident views passed headed-browser checks with no console errors.
- Added versioned Incident snapshots with transactional event evidence, artifact manifests, provenance, JSON export, and a dedicated management view.
- Added a centralized diagnostic metadata allowlist and a migration that removes previously stored non-allowlisted fields.
- Kept event-row DOM nodes stable across live refreshes so event selection and Incident creation are not interrupted by new log batches.
- Fixed empty event, capture, and artifact API lists to serialize as `[]`; the dashboard now also tolerates `null` list values from older servers.
- Added PostgreSQL-backed Admin Server with versioned migrations and separated database roles.
- Added independent Diagnostics Collector with acknowledged checkpoints, durable spool, Relay/Mac Agent JSONL ingestion, and Simulator app-container ingestion.
- Added local operations dashboard, event streaming, diagnostics artifacts, service health, and device capture controls.
