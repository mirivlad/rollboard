# Rooms, Multiplayer and Chat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver authoritative turn-based multiplayer rooms with reconnectable realtime state, host moderation, room-only chat and parallel hotseat support.

**Architecture:** PostgreSQL persists rooms, player slots, chat and the authoritative `game.GameSession` snapshot pinned to an immutable `game_version_id`. The Go process owns a room hub that broadcasts versioned state events over WebSocket; clients submit intentions and never rule outcomes. Redis is reserved for cross-instance fan-out and presence after the single-process hub contract is proven.

**Tech Stack:** Go 1.24, pgx/v5, PostgreSQL 16, WebSocket transport, Svelte 5, Vite, Vitest/Testing Library.

## Global Constraints

- A room references `game_versions.id`, never `game_drafts` or a mutable game definition.
- The server validates membership, current turn and every game action before mutating a session.
- Player identities are account users or existing guests; chat messages contain no raw tokens.
- Hotseat remains a local mode and does not require a WebSocket connection.
- Every browser mutation sends the existing CSRF header; WebSocket upgrade authenticates the opaque session cookie.
- Chat is persisted per room; only the host may mute or remove a player.
- State messages carry monotonic `sequence` values for reconnect and stale-event detection.

---

### Task 1: Room persistence and authorization

**Files:**
- Create: `backend/internal/storage/postgres/migrations/000003_rooms.sql`
- Create: `backend/internal/room/service.go`
- Create: `backend/internal/room/service_integration_test.go`
- Modify: `backend/internal/catalog/service.go`

**Interfaces:**
- Produces `room.Service.Create(ctx, host identity.Actor, gameVersionID string, settings CreateInput) (Room, error)`.
- Produces `Join(ctx, actor identity.Actor, roomID string) (RoomMember, error)`, `Get(ctx, roomID string)`, and host-only `Remove`/`Mute` methods.
- Uses `catalog.GetVersionByID(ctx, id)` to load the immutable definition that seeds a `game.GameSession`.

- [ ] **Step 1: Write failing integration tests** for creating a room from version 1, rejecting a draft/game ID, joining an available slot, rejecting a second account on the same slot, and rejecting host moderation by another member.

```go
room, err := rooms.Create(ctx, host, version.ID, room.CreateInput{Title: "Friday game", MaxPlayers: 4})
if err != nil || room.GameVersionID != version.ID { t.Fatal(err) }
if _, err := rooms.Remove(ctx, guest, room.ID, member.ID); !errors.Is(err, room.ErrNotHost) { t.Fatal(err) }
```

- [ ] **Step 2: Run** `ROLLBOARD_TEST_DATABASE_URL=... go test ./internal/room -run TestRoom -count=1`; expect missing package/schema failure.
- [ ] **Step 3: Add schema** with UUID room/member/message IDs, `rooms.game_version_id` foreign key, room status, max player check, unique `(room_id, actor_kind, actor_id)`, membership index and timestamped messages.
- [ ] **Step 4: Implement minimal transactional service**. Lock rooms during joins, use `game.StartSession` only from `game_versions.definition_json`, and persist session JSON in `rooms.session_json`.
- [ ] **Step 5: Re-run integration tests** and `go test ./internal/room ./internal/catalog ./internal/storage/postgres -count=1`.
- [ ] **Step 6: Commit** `feat: add authoritative room persistence`.

### Task 2: HTTP room contract

**Files:**
- Modify: `backend/internal/httpapi/handler.go`
- Modify: `backend/internal/httpapi/handler_test.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Produces `POST /api/rooms`, `GET /api/rooms/{id}`, `POST /api/rooms/{id}/join`, and host-only moderation endpoints.
- Every creation/join/moderation mutation resolves `identity.Actor`; account-only creation is explicit.

- [ ] **Step 1: Write failing HTTP tests** for unauthenticated creation (401), guest creation rejection (403), account creation with CSRF (201), guest join (201), and non-host mute rejection (403).
- [ ] **Step 2: Run** `go test ./internal/httpapi -run TestRoom -count=1`; expect unregistered routes.
- [ ] **Step 3: Add HTTP handlers** with `{error,details,code}` errors, actor extraction and strict CSRF checks.
- [ ] **Step 4: Re-run** focused HTTP tests and `go test ./... -count=1 && go vet ./...`.
- [ ] **Step 5: Commit** `feat: expose multiplayer room APIs`.

### Task 3: Realtime room hub and authoritative actions

**Files:**
- Create: `backend/internal/realtime/hub.go`
- Create: `backend/internal/realtime/hub_test.go`
- Modify: `backend/internal/room/service.go`
- Modify: `backend/internal/httpapi/handler.go`

**Interfaces:**
- `Hub.Connect(roomID, actor, since uint64) (Client, error)` sends a `room_state` event and then ordered events.
- Client intentions are `roll`, `action`, and `next_turn`; all route through room service before persistence/broadcast.
- Event envelope: `{"type":"room_state|room_event","sequence":N,"payload":...}`.

- [ ] **Step 1: Write failing hub tests** showing two clients receive equal ordered state after one authorised roll, while an out-of-turn client receives `NOT_YOUR_TURN` and no state mutation.
- [ ] **Step 2: Run** `go test ./internal/realtime -count=1`; expect missing hub.
- [ ] **Step 3: Implement in-process hub** with per-room mutex, monotonic sequence, bounded client channels, cleanup on disconnect and snapshot-on-reconnect.
- [ ] **Step 4: Add authenticated WebSocket endpoint** `/api/rooms/{id}/ws`; reject missing/invalid session before upgrade.
- [ ] **Step 5: Re-run hub and HTTP tests**, then `go test ./... -count=1`.
- [ ] **Step 6: Commit** `feat: add realtime authoritative room hub`.

### Task 4: Persisted chat and moderation

**Files:**
- Modify: `backend/internal/room/service.go`
- Modify: `backend/internal/room/service_integration_test.go`
- Modify: `backend/internal/realtime/hub.go`
- Modify: `backend/internal/httpapi/handler.go`

- [ ] **Step 1: Write failing tests** for persisted room messages, muted sender rejection, host mute/unmute, and broadcast to two connected members.
- [ ] **Step 2: Run** targeted room/realtime tests; expect missing methods/events.
- [ ] **Step 3: Implement** `SendMessage`, bounded 1–1000 rune content validation, latest-message retrieval and `chat_message` events; no global chat endpoint is introduced.
- [ ] **Step 4: Re-run** targeted and all Go tests.
- [ ] **Step 5: Commit** `feat: add room chat and moderation`.

### Task 5: Room UI and browser verification

**Files:**
- Modify: `frontend/src/lib/api.ts`, `frontend/src/lib/types.ts`, `frontend/src/App.svelte`
- Create: `frontend/src/components/RoomLobby.svelte`, `frontend/src/components/RoomPlay.svelte`, `frontend/src/components/RoomChat.svelte`
- Create: focused Vitest component tests

- [ ] **Step 1: Write failing component tests** for room create button availability to accounts, guest join name, chat send form, and sequence-based state replacement.
- [ ] **Step 2: Implement typed REST/WebSocket client** and components. The board uses existing `BoardView`; only backend events mutate state.
- [ ] **Step 3: Run** `npm test`, `npm run check`, `npm run build`.
- [ ] **Step 4: Browser test** account creates a room, guest joins, two tabs see one authoritative roll and a room chat message; inspect console/network and record results.
- [ ] **Step 5: Commit** `feat: add multiplayer room interface`.

### Task 6: Deployment, docs and end-to-end verification

**Files:**
- Modify: `scripts/smoke.sh`, `README.md`, `docs/ARCHITECTURE.md`, `docs/CURRENT_STATE.md`, `docs/PLAYTEST_CHECKLIST.md`, `docs/ROADMAP.md`

- [ ] **Step 1: Extend smoke** through account registration → draft → publish → room create → guest join → server-authoritative action.
- [ ] **Step 2: Run** `make stop-dev`, `make smoke` three times, `make check`, `make test`, frontend tests/check/build and process-port cleanup checks.
- [ ] **Step 3: Verify Docker and Portainer manifest** with PostgreSQL/Redis health checks and no exposed credentials in logs.
- [ ] **Step 4: Update docs** to remove stale SQLite/MVP claims and document local/Docker/Portainer startup plus browser verification results.
- [ ] **Step 5: Commit** `docs: document multiplayer rooms and deployment`.

## Self-review

- Immutable version pinning is covered by Tasks 1–3.
- Server authority, reconnect sequencing and out-of-turn rejection are covered by Task 3.
- Room-only persisted chat and host moderation are covered by Task 4.
- Guest and account flows, hotseat continuity and browser-visible multiplayer UI are covered by Tasks 2 and 5.
- Docker, Portainer, smoke, documentation and process cleanup are covered by Task 6.
- The plan contains no SQLite fallback and no game-specific runtime branches.
