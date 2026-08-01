# Rollboard — current state

Last updated: 2026-08-02

## Implemented and automatedly verified

- PostgreSQL migrations and Docker/Portainer manifests; SQLite is not a supported runtime store.
- Accounts, guest identities, opaque cookie sessions, Argon2id passwords and CSRF checks.
- Author-owned private drafts, reloadable author catalog, immutable published game versions and server-side validation.
- Generic hotseat runtime plus multiplayer rooms pinned to `game_versions.id`.
- Host-created rooms, guest/account joins, visible host mute/kick controls, durable room-only chat.
- Authoritative server start/roll/choice commands; out-of-turn commands are rejected.
- In-process sequenced WebSocket hub with authenticated upgrade and origin checking.
- Svelte author dashboard with template picker, guided basics or direct advanced editor, lobby, room view, WebSocket live state and chat UI.

## Verification completed on 2026-08-02

- `make stop-dev`
- `make smoke` three times
- `make check`
- `make test`
- `frontend: npm test`, `npm run check`, `npm run build`

All of the above passed. Smoke covers guest-to-account claim, draft creation,
publication, account room creation and guest join. Backend integration tests cover
authoritative start/roll and out-of-turn rejection.

## Release blockers / next work

- Two Chromium/Playwright contexts verified account publication → room creation → guest join → live roster refresh → authenticated WebSocket upgrades → host start → authoritative roll → generic purchase choice resolution → chat broadcast. Screenshot: `/tmp/rollboard-multiplayer.png` (test artifact, not committed).
- Redis is present in deployment but the realtime hub is still single-process; no cross-replica fan-out, presence or rate limiting exists yet.
- Public room discovery, invite links and reconnect event replay are not implemented.
- `scripts/backend.sh` and `scripts/browser-smoke.sh` require audit before being advertised as supported paths.
- The existing hotseat BoardView still needs responsive visual QA across target sizes.
