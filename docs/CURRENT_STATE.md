# Rollboard — current state

Last updated: 2026-08-04

## Implemented and verified

- PostgreSQL migrations and Docker/Portainer manifests; SQLite is not a supported runtime store.
- Accounts, guest identities, opaque cookie sessions, Argon2id passwords and CSRF checks.
- Rate limiting on registration, login and guest entry, configurable through
  `ROLLBOARD_AUTH_RATE_LIMIT`.
- Author-owned private drafts, reloadable author catalog, immutable published game versions and server-side validation.
- Every authoring route is owner-scoped. Drafts, validation, playtest and hotseat
  sessions are reachable only by the account that owns them; another account's
  resource reads as missing rather than forbidden.
- PostgreSQL connection-pool limit and indexes for owner-scoped catalog queries.
- Generic hotseat runtime plus multiplayer rooms pinned to `game_versions.id`.
- Host-created rooms, guest/account joins, visible host mute/kick controls, durable room-only chat.
- Authoritative server start/roll/choice commands; out-of-turn commands are rejected.
- Sequenced WebSocket hub with authenticated upgrade, origin checking, Redis cross-replica fan-out and disconnect on room removal.
- Durable PostgreSQL room-event journal and per-actor UUID command receipts for start, roll, action and chat. Reconnects replay up to 64 contiguous events, otherwise safely receive the current snapshot.
- Svelte author dashboard with template picker, guided basics or direct advanced editor, lobby, room view, WebSocket live state and chat UI. Online rolls display the server-provided dice result and animate only the server-provided path.

## Test and CI status

`make test` runs the integration suite against a real PostgreSQL and Redis.
113 tests run and none skip. CI fails the build if any test reports SKIP.

Until 2026-08-04 this was not true: every integration test guarded itself behind
`ROLLBOARD_TEST_DATABASE_URL`, nothing set it, and the suite reported `ok` for
each package while skipping roughly twenty tests covering persistence, catalog,
rooms and identity. Treat any earlier "automatedly verified" claim in this
project's history as unproven.

GitHub Actions runs gofmt, `go vet`, the Go suite with integration tests
enabled, `svelte-check`, vitest, the frontend build, and a container image
build published to `ghcr.io/mirivlad/rollboard` on master.

## Verification completed on 2026-08-04

- `make check` (gofmt, vet, full Go suite, Redis fan-out, svelte-check, vitest, frontend build, demo validation)
- `make smoke`
- Docker image build
- The authorization fix was verified by re-running the original exploit against
  a live server: an anonymous `PUT /api/games/{id}` now returns 404 and leaves
  the owner's game untouched, a second account cannot read, roll or playtest
  another account's draft or session, and the owner's own
  create → save → playtest → roll path still works.

## Known limitations

- The rate limiter is per-process. Behind N replicas the effective ceiling is
  N times the configured limit. Move it to the Redis backplane if a deployment
  needs an exact global budget.
- Presence is not implemented. Journal retention/pruning and load testing still
  need explicit production policy before a large public launch.
- Public room discovery and invite links are not implemented.
- The interface is English-only; there is no localisation layer yet.
- No design tokens: colours are hard-coded per component, there are no focus
  styles, no light theme, and the editor has no responsive layout.
- `scripts/browser-smoke.sh` is still a stub that points at the manual
  checklist. A portable Playwright runner is not implemented.
