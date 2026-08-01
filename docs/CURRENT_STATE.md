# Rollboard — current state

Last updated: 2026-08-02

## Implemented and automatedly verified

- PostgreSQL migrations and Docker/Portainer manifests; SQLite is not a supported runtime store.
- Accounts, guest identities, opaque cookie sessions, Argon2id passwords and CSRF checks.
- Author-owned private drafts, reloadable author catalog, immutable published game versions and server-side validation.
- PostgreSQL connection-pool limit and indexes for owner-scoped catalog queries.
- Generic hotseat runtime plus multiplayer rooms pinned to `game_versions.id`.
- Host-created rooms, guest/account joins, visible host mute/kick controls, durable room-only chat.
- Authoritative server start/roll/choice commands; out-of-turn commands are rejected.
- Sequenced WebSocket hub with authenticated upgrade, origin checking, Redis cross-replica fan-out and disconnect on room removal.
- Svelte author dashboard with template picker, guided basics or direct advanced editor, lobby, room view, WebSocket live state and chat UI. Online rolls display the server-provided dice result and animate only the server-provided path.

## Verification completed on 2026-08-02

- `make stop-dev`
- `make smoke` three times
- `make check`
- `make test`
- `frontend: npm test`, `npm run check`, `npm run build`
- `make check` runs a real Redis two-replica fan-out test.
- Docker image build and Docker Compose app startup reached `/api/health`.

All of the above passed. Smoke covers guest-to-account claim, draft creation,
publication, account room creation and guest join. Backend integration tests cover
authoritative start/roll and out-of-turn rejection.

## Browser verification completed on 2026-08-02

- Two Chromium/Playwright contexts verified account publication → room creation → guest join → live roster refresh → authenticated WebSocket upgrades → host start → server-authoritative roll with visible dice result → generic purchase choice resolution → chat broadcast → host mute enforcement. Screenshot: `/tmp/rollboard-multiplayer.png` (test artifact, not committed).

## Release blockers / next work

- Presence, command idempotency, rate limiting and durable reconnect event replay are not implemented; reconnects currently load a fresh PostgreSQL snapshot.
- Public room discovery and invite links are not implemented.
- A portable automated Playwright runner is not implemented; `scripts/browser-smoke.sh` intentionally directs developers to the manual checklist.
- The existing hotseat BoardView still needs responsive visual QA across target sizes.
