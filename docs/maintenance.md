# Admin Platform Maintenance

## Fast Diagnosis

1. `./dev status` checks Admin, PostgreSQL, Relay, Mac Agent, and Collector status.
2. Read `.run/admin-server.log` and `.run/collector.log`.
3. Inspect `.run/collector-state/spool`; files there are acknowledged-upload batches waiting to retry.
4. Query `/api/overview`, then `/api/events` by `source`, `profile`, `trace_id`, `turn_ref`, or `session_id`.
5. Confirm the runtime database identity with `/api/healthz`; it must report `codexremote_admin_app` and `UTC`.
6. For a saved incident, download `/api/v1/incidents/{id}/snapshot` and verify `schema_version`, evidence window, source counts, truncation, and privacy policy.

## Recovery

- Admin unavailable: leave Collector running. Its current pending batch remains in spool and the checkpoint does not advance.
- PostgreSQL unavailable: restore PostgreSQL, run `make migrate`, then restart Admin. Collector retries automatically.
- Collector unavailable: restart it with `./dev restart`; checkpoints resume from the last acknowledged byte.
- Log rotation or truncation: Collector detects a new file identity or smaller size and reads from byte zero. Content fingerprints prevent duplicate display after rotation.
- Corrupt structured line: Collector advances past the complete line and emits `collector.parse_failed` with file basename, offset, byte length, and parser error.
- Simulator reinstall: container discovery is repeated each cycle, so the new App Data Container path is picked up automatically.

## Capacity And Security

- Event requests: at most 1,000 events and 4 MiB HTTP body.
- Incident snapshots: at most 500 events in a window of at most one hour before and one hour after the anchor event. Events and artifact manifests commit in one transaction.
- Collector batches: at most 500 events, 4 MiB source bytes, and 256 KiB per line.
- Artifacts: at most 4 GiB per file; validate free disk space before requesting full sysdiagnose.
- Admin refuses non-loopback listeners. Any future LAN or Internet exposure requires an authenticated TLS proxy, source restriction, and a reviewed retention policy.
- PostgreSQL migration and runtime roles are separate; neither is a superuser.

## Upgrade Checklist

1. Back up PostgreSQL and artifacts.
2. Run `go test ./...`, `go vet ./...`, and `make build`.
3. Run `make migrate` with the migrator role.
4. Restart Admin and Collector externally with `./dev restart`.
5. Verify `/api/healthz`, Collector heartbeat, spool count, and one new event from each active source.
6. Exercise event filtering, event detail, artifacts, and the device task page in a browser.
7. Create a controlled incident from an event, restart Admin, verify the frozen snapshot is unchanged, then remove the controlled fixture.

## Validation Record

For each non-trivial diagnostics change, record the exact date, commands, source counts, Collector spool count, browser paths exercised, and any failed iPhone tests in the repository changelog or release note. A successful build is not evidence that Simulator interactions work.

The 2026-08-13 local baseline passed Admin tests, vet, build, PostgreSQL role/timezone checks, browser event/detail/device/artifact flows, and Relay/Mac/Simulator ingestion. The iPhone unit suite passed 9/9. The UI suite passed 21/24; the connection-toggle, interrupted-turn notice, and project-loading-indicator cases failed again when isolated and remain release blockers outside the Admin implementation.
