# Admin Platform Engineering Guidelines

- Treat this directory as an independent Git repository. Keep Admin Server, Admin Web, Collector, migrations, tests, CI, maintenance docs, and changelog changes inside this repository.
- Do not depend on a parent-repository commit, sibling-repository source import, `go.work`, Git submodule, or symlink. Coordinate cross-repository changes through versioned schemas, fixtures, and an explicit compatibility matrix.
- Keep Admin Server, Admin Web, and Diagnostics Collector in this repository, but build Collector as an independent process.
- Collector must submit through the versioned HTTP API and must never connect directly to PostgreSQL.
- Keep PostgreSQL migration and runtime roles separate. The runtime process must not use a superuser.
- Advance a collection checkpoint only after Admin acknowledges the corresponding batch. Persist pending batches before upload.
- Never record prompts, responses, credentials, tokens, relay URLs, or full user file paths in diagnostic events.
- Treat log schemas as machine-readable contracts: stable event names, RFC 3339 timestamps, correlation IDs, typed fields, and explicit versions.
- Keep local Admin bound to loopback unless a reviewed authentication and network exposure change requires otherwise.
- Treat `Codex Remote/01-架构设计/Admin Web 产品与前端架构.md`, `Codex Remote/01-架构设计/Admin 能力目录与导航策略.md`, `Codex Remote/04-开发计划/Admin Web 重构开发计划.md`, and ADR-006 as the Admin Web target contract.
- Keep one typed navigation catalog. Available capabilities must have working routes; planned capabilities may be shown as non-interactive `规划中` items without routes or requests; candidate and retired capabilities must remain out of product code and discoverable only in the capability document.
- Do not create placeholder pages, empty API endpoints, feature flags, database tables, mock counts, or example data merely to advertise a planned capability.
- The Admin Web replacement is a hard cut: do not retain legacy pages, unversioned Admin routes, response fallbacks, feature flags, or duplicate old/new components after the replacement passes E2E.
- Keep page, filter, time-range, and selected-object state in the URL; do not duplicate it across global JavaScript state and DOM classes.
- Reuse the existing PostgreSQL data model and add only measured query indexes or forward migrations. Do not create a second database, mirror tables, or Redis for the Web redesign.
