# Rollboard platform design

Date: 2026-08-02

## Purpose

Rollboard becomes a self-hostable platform for creating and running generic
turn-based board games. It replaces the SQLite, single-device prototype with a
production-oriented system that supports visual authoring, published game
versions, online multiplayer, room chat, and hotseat.

The first release is deliberately a turn-based engine. It does not execute
author-supplied JavaScript, Lua, or other arbitrary code. This keeps the runtime
safe, deterministic, portable, and suitable for public hosting.

## Product decisions

- Authors and returning players use accounts. A guest can join through an invite
  link with a display name and can later claim that identity by creating an
  account.
- Games have draft, private, unlisted, and public states. Publishing creates an
  immutable version. A running room always uses the exact version it started
  with.
- The editor starts with a template picker or an empty game. A guided wizard is
  the primary flow; an advanced studio exposes the same underlying definition.
  Switching modes never converts or loses data.
- The runtime supports only server-authoritative, turn-based mechanics. It
  supports graph branches, player choices, optional turn timeouts, reconnects,
  and asynchronous continuation.
- A room has one persisted chat stream. System events are separate from player
  messages. The host can remove or mute participants.
- Hotseat and online play use the same room and runtime. Hotseat adds a local
  handoff screen between turns instead of a second rules engine.

## System architecture

Rollboard is a modular Go monolith with a Svelte single-page application. The
compiled frontend is served by the application container in production. Modules
communicate through Go interfaces and typed domain values rather than internal
HTTP calls.

```text
Browser ── HTTPS / WebSocket ── Rollboard app
                                   ├── identity and permissions
                                   ├── game catalog and versioning
                                   ├── editor validation
                                   ├── authoritative game runtime
                                   ├── room and chat service
                                   └── realtime gateway
                                       │                 │
                                   PostgreSQL          Redis
                                 durable source      ephemeral fan-out,
                                   of truth          presence, rate limits
```

PostgreSQL is required in every supported deployment and replaces SQLite. Redis
is required by the production stack for cross-instance WebSocket fan-out,
presence, and rate limits; no durable game, identity, or chat data is kept only
in Redis. A Redis restart may temporarily remove presence indicators but cannot
lose a game or chat message.

The application can run as one replica. With Redis configured, multiple replicas
may be placed behind a WebSocket-capable reverse proxy. PostgreSQL transactions,
not distributed locks, serialize changes to a room.

## Domain model and persistence

All identifiers are UUIDs. Timestamps are UTC. Tables use database migrations
that run before the server becomes ready.

| Area | Persistent records | Notes |
| --- | --- | --- |
| Identity | `users`, `guest_identities`, `auth_sessions` | Password and session token hashes only; guest identity can be claimed by a user. |
| Catalog | `games`, `game_drafts`, `game_versions` | One author-owned game; editable draft; immutable published JSON definition and version number. |
| Rooms | `rooms`, `room_members`, `room_invites` | Stores pinned version, host, mode, status, limits, token digest and expiry. |
| Runtime | `session_snapshots`, `session_events`, `command_receipts` | Snapshot has current state and revision; events enable replay/reconnect; receipts make commands idempotent. |
| Chat | `chat_messages`, `room_moderation` | Messages and mute/kick audit records are room-scoped. |

`GameDefinition` is stored as validated JSONB in drafts and versions. It contains
metadata, visual theme, board geometry, cells, directed edges, resources, dice
rules, actions, conditions, and finish rules. The grid invariants remain
mandatory: dimensions derive from `cols * cellSize` and `rows * cellSize`; every
cell coordinate aligns to the grid and lies inside the board.

Starting a room copies neither a mutable draft nor an implicit latest version:
it pins `game_version_id`. Old rooms remain playable when an author publishes a
new version. Runtime state is a snapshot with a monotonic revision. Each accepted
command appends ordered `session_events` in the same PostgreSQL transaction as
the next snapshot.

## Runtime and commands

The server is authoritative. The client sends an intent, never a dice result,
movement path, resource balance, owner, winner, or pending action result.

The core command set is:

- `start_room`, `join_room`, `leave_room`, `claim_seat`;
- `roll`, `resolve_pending_action`, `acknowledge_hotseat_handoff`;
- `send_chat_message`, `mute_member`, `remove_member`;
- `reconnect` with the last event sequence seen by the browser.

Every mutating command includes a client-generated idempotency key. The server
locks the room row in a transaction, verifies actor permissions and session
revision, applies the command through the generic engine, writes events and the
snapshot, stores the command receipt, commits, and only then publishes the event
to Redis/WebSocket subscribers. A duplicate command returns the original result
instead of rolling again.

The roll response and corresponding move event include confirmed values:

```json
{
  "dice": [3, 5],
  "total": 8,
  "path": ["cell_a", "cell_b", "cell_c"],
  "revision": 42
}
```

The browser animates only this confirmed path, then reconciles to the latest
snapshot. A reconnect gets a snapshot when its sequence is too old, otherwise it
gets ordered missing events. This prevents stale state and duplicate tokens.

The generic action model remains data-driven. Initial primitives are resource
gain/loss/transfer, ownership, typed player choice, conditional branches, log
messages, finish game, and optional turn timeout. Property, rent, trap,
treasure, money, and gold remain definition data, not runtime branches.

## Editor experience

The author dashboard lists games and their state. Creating a game starts with a
template (race, economy, adventure) or an empty definition. Templates are
ordinary versioned definitions, never engine logic.

The wizard consists of:

1. Basics: title, description, visibility, player count, and template.
2. Board: grid size, cells, visual labels, and directed connections.
3. Rules: dice, resources, turn configuration, actions, and conditions.
4. Appearance: colors, icons, and accessible player tokens.
5. Test and publish: server validation, local hotseat preview, publication.

The advanced studio has structural navigation, a central board canvas, an
inspector, and always-visible validation feedback. It edits exactly the data
edited by the wizard. The action editor renders a typed block tree with form
controls; raw JSON is not the primary authoring interaction.

Publish is blocked for invalid geometry, dangling graph references, invalid dice
configuration, invalid action payloads, missing start conditions, or a version
that cannot be started by the runtime.

## Room, play, chat, and hotseat UX

An online host selects a published version and creates a room. The host receives
an expiring, revocable invite link. Invitees authenticate or choose a guest name
before taking a seat. The lobby shows seats, readiness, connection status, the
pinned game version, and room controls.

The play screen has three stable regions: player/turn information and legal
commands, the board, and room chat. It visibly shows active player, dice rule,
last confirmed action, pending choices, and connection state. The animation order
is roll request, rolling feedback, confirmed dice and total, short pause, token
movement, then landing action or turn handoff.

Hotseat uses the same screen and server state. When a local turn ends, a full
handoff view hides player-specific controls until the next local player confirms
they have taken the device. A hotseat room can be changed to online before the
game starts; after start, room mode is fixed to preserve player semantics.

Chat is durable and paginated. System actions are events rendered separately
from chat messages. The host can mute or remove a room member; server permission
checks enforce both actions.

## HTTP and WebSocket contracts

REST covers authentication, catalog browsing, draft editing, publication,
validation, room creation/invites, and snapshot reads. WebSocket is used for a
room's ordered runtime events, chat events, presence, and typed error events.

All API errors use:

```json
{ "error": "short message", "details": "actionable details", "code": "MACHINE_CODE" }
```

The UI prefixes errors with the failed operation, for example `Save failed`,
`Publish failed`, `Join failed`, or `Roll failed`. It never exposes raw server
HTML as a user-facing message.

WebSocket messages include a type, room ID, event sequence, correlation ID, and
payload. The gateway rejects unknown types, unauthorized access, oversized
messages, and commands illegal for the sender or room state.

## Identity and security

Account registration uses email and password. Passwords are hashed with Argon2id.
Browser sessions are opaque, random, hashed at rest, and delivered in `HttpOnly`,
`Secure`, appropriately `SameSite` cookies. CSRF protection applies to
cookie-backed mutating HTTP requests and WebSocket upgrades verify allowed origin.

Guest and invite tokens are random, expire, are revocable, and are stored as
digests. Registration can be disabled through environment configuration for a
closed installation. OAuth, billing, direct messages, and arbitrary author code
are outside the first release.

The application emits structured logs with request and room correlation IDs but
never logs passwords, session tokens, invite tokens, or full sensitive payloads.
Rate limits cover login, registration, invite joins, WebSocket connections,
commands, and chat messages.

## Deployment

The repository provides a production `compose.yaml`, a Portainer stack example,
a multi-stage application Dockerfile, `.env.example`, and migration commands.
The stack has:

- `app`: non-root Go process serving frontend, API, and WebSocket;
- `postgres`: supported PostgreSQL image, named volume, healthcheck and
  credentials from environment/secrets;
- `redis`: supported image, healthcheck, ephemeral data only.

The app waits for PostgreSQL readiness, performs migrations safely, then exposes
`/healthz` and `/readyz`. Docker images never contain local databases, secrets,
or development dependencies. HTTPS terminates at the deployment reverse proxy;
the app trusts forwarded headers only from explicitly configured proxies.

## Extension boundary for mini-games

The first release reserves `launch_minigame` and a versioned `MiniGameModule`
contract but does not execute it. A future module receives only explicit input,
runs in an isolated sandbox, returns a server-validated result, and appends that
result through the same session command/event transaction. Mini-game code never
runs in the authoring backend process or accesses the database directly.

## Verification and acceptance criteria

The replacement is accepted only when direct evidence exists for all items:

- migration tests prove a fresh PostgreSQL database initializes and the app no
  longer requires SQLite;
- unit tests cover definition validation, action execution, dice limits,
  branching, choices, turn timeout behavior, and hotseat handoff;
- integration tests cover transactions, conflicting commands, idempotency,
  publication/version pinning, guest claim, invites, and migrations;
- WebSocket tests cover two players receiving the same ordered events, reconnect
  catch-up, chat, mute/kick, and Redis restart recovery;
- browser tests in a real rendered browser cover template → wizard → studio →
  publish, online invite/join/play/chat, and hotseat handoff;
- Docker Compose smoke tests verify healthchecks, migrations, readiness,
  PostgreSQL persistence across app restart, and no stale test processes;
- required repository checks run: `make stop-dev`, `make smoke` three times,
  `make check`, `make test`, frontend type check/build, then cleanup checks.

Browser verification is reported honestly with the actual browser and remaining
issues. Passing API tests alone never proves editor or play UI.

## Delivery sequence

Implementation proceeds in dependency order without reducing first-release scope:

1. PostgreSQL storage, migrations, configuration, Compose/Portainer foundation.
2. Identity, game drafts/versions, authorization, and catalog APIs.
3. Transactional room runtime, event journal, WebSocket gateway, reconnect, and
   durable chat.
4. Dashboard, template picker, wizard, advanced studio, lobby, play room, and
   hotseat flow.
5. End-to-end hardening, browser tests, load checks, documentation, and
   deployment verification.
