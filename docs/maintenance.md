# Admin Platform Maintenance

## Fast Diagnosis

1. `./dev status` checks Admin, PostgreSQL, Relay, Mac Agent, and Collector status.
2. Read `.run/admin-server.log` and `.run/collector.log`.
3. Inspect `.run/collector-state/spool`; files there are acknowledged-upload batches waiting to retry.
4. Query `/api/v1/overview`, then `/api/v1/diagnostics/events` by bounded `from/to`, `source`, `profile`, `trace_id`, `turn_ref`, or `session_id`.
5. Confirm the runtime database identity with `/api/healthz`; it must report `codexremote_admin_app` and `UTC`.
6. For a saved incident, download `/api/v1/incidents/{id}/snapshot` and verify `schema_version`, evidence window, source counts, truncation, and privacy policy.

## Recovery

- Admin unavailable: leave Collector running. Its current pending batch remains in spool and the checkpoint does not advance.
- PostgreSQL unavailable: restore PostgreSQL, run `make migrate`, then restart Admin. Collector retries automatically.
- Collector unavailable: restart it with `./dev restart`; checkpoints resume from the last acknowledged byte.
- Log rotation or truncation: Collector detects a new file identity or smaller size and reads from byte zero. Content fingerprints prevent duplicate display after rotation.
- Corrupt structured line: Collector advances past the complete line and emits `collector.parse_failed` with file basename, offset, byte length, and parser error.
- Simulator reinstall: container discovery is repeated each cycle, so the new App Data Container path is picked up automatically.
- `./dev restart` reports an unhealthy Admin while Collector is running: check whether port `18880` or the old launchd label is still present. The launcher waits for both labels and the listener to disappear before submitting replacements; do not bypass this preflight with back-to-back `launchctl remove` and `submit` calls.

## Capacity And Security

- Event requests: at most 1,000 events and 4 MiB HTTP body.
- Incident snapshots: at most 500 events in a window of at most one hour before and one hour after the anchor event. Events and artifact manifests commit in one transaction.
- Collector batches: at most 500 events, 4 MiB source bytes, and 256 KiB per line.
- Artifacts: at most 4 GiB per file; validate free disk space before requesting full sysdiagnose.
- Admin refuses non-loopback listeners. Any future LAN or Internet exposure requires an authenticated TLS proxy, source restriction, and a reviewed retention policy.
- PostgreSQL migration and runtime roles are separate; neither is a superuser.

## Upgrade Checklist

1. Back up PostgreSQL and artifacts.
2. Run `make test`, `make vet`, and `make build`.
3. Run `make migrate` with the migrator role.
4. Restart Admin and Collector externally with `./dev restart`.
5. Verify `/api/healthz`, `/api/v1/services`, Collector heartbeat, spool count, and one new event from each active source.
6. Exercise event filtering, event detail, artifacts, and the device task page in a browser.
7. Create a controlled incident from an event, restart Admin, verify the frozen snapshot is unchanged, then remove the controlled fixture.

## Validation Record

For each non-trivial diagnostics change, record the exact date, commands, source counts, Collector spool count, browser paths exercised, and any failed iPhone tests in the repository changelog or release note. A successful build is not evidence that Simulator interactions work.

The 2026-08-13 local baseline passed Admin tests, vet, build, PostgreSQL role/timezone checks, browser event/detail/device/artifact flows, and Relay/Mac/Simulator ingestion. The iPhone unit suite passed 9/9. The UI suite passed 21/24; the connection-toggle, interrupted-turn notice, and project-loading-indicator cases failed again when isolated and remain release blockers outside the Admin implementation.

### 2026-08-13 launchd restart race

- Symptom: `./dev restart` rebuilt successfully, Collector restarted, but Admin never bound `127.0.0.1:18880`; Collector safely retained and retried its spooled batch.
- Scope: local development restart only; PostgreSQL, checkpoints, artifacts, and source logs were unaffected.
- Root cause: `launchctl remove` is asynchronous. Submitting the replacement Admin before the old label/listener was fully removed allowed the pending removal to clear the new submitted job.
- Fast diagnosis: compare `launchctl list`, `lsof -nP -iTCP:18880 -sTCP:LISTEN`, and the final timestamp in `.run/admin-server.log`; run the same binary in the foreground to separate launcher failure from application startup failure.
- Prevention: `./dev` now waits up to 10 seconds for both launchd labels and the listener to disappear before submitting replacements.
- Verification: restart twice, confirm `/api/healthz`, `/api/v1/services`, Collector spool `0`, and a new event after each restart.
