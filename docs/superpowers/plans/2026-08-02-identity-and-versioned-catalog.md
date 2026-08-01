# Identity and Versioned Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add secure account/guest identity and author-owned draft/publication workflow with immutable game versions.

**Architecture:** PostgreSQL owns users, guests, sessions, games, drafts and published versions. An `identity.Service` hashes passwords with Argon2id and only persists token digests. A `catalog.Service` validates definitions before copying a draft into an immutable version; room work will consume `game_version_id`, never a mutable draft.

**Tech Stack:** Go 1.24, pgx/v5, Argon2id, secure cookies, PostgreSQL migrations, Svelte 5 API client.

## Global constraints

- All IDs are UUIDs generated with `crypto/rand`.
- Passwords, raw session tokens and raw guest tokens are never stored or logged.
- Guest identity may be claimed once by an authenticated account.
- Only the game owner can read/write drafts or publish versions.
- Published versions are immutable JSONB snapshots and must validate through `game.ValidateDefinition`.
- No SQLite compatibility layer is introduced.

### Task 1: Identity schema and repository

**Files:** Create `backend/internal/storage/postgres/migrations/000002_identity_catalog.sql`, `backend/internal/identity/service.go`, `backend/internal/identity/service_test.go`, `backend/internal/catalog/service.go`, `backend/internal/catalog/service_test.go`.

- [ ] Write integration tests that register an account, reject a duplicate email, create a guest, claim the guest, save an owner draft, publish version 1, modify the draft, then assert version 1 remains byte-for-byte unchanged.
- [ ] Run the tests with `ROLLBOARD_TEST_DATABASE_URL`; confirm they fail because the tables/services do not exist.
- [ ] Add tables `users`, `guest_identities`, `auth_sessions`, `games`, `game_drafts`, and `game_versions`; use `citext`-free lowercase email uniqueness, `BYTEA` token digests, `TIMESTAMPTZ` expiry and foreign keys.
- [ ] Implement `Register`, `CreateGuest`, `ClaimGuest`, `CreateSession`, `Authenticate`, `CreateGame`, `GetDraft`, `SaveDraft`, `Publish`, and `GetVersion` with context-aware transactions.
- [ ] Run `go test ./internal/identity ./internal/catalog ./internal/storage/postgres -count=1`, then commit `feat: add identity and versioned game catalog`.

### Task 2: Authentication and catalog HTTP contract

**Files:** Modify `backend/internal/httpapi/handler.go`; create `backend/internal/httpapi/auth_catalog_test.go`; modify `backend/cmd/server/main.go`; create `backend/internal/httpapi/auth.go`.

- [ ] Write HTTP tests for register/login/logout/me, guest creation/claim, unauthenticated draft rejection, cross-owner rejection, publish failure for invalid definition, and successful immutable version retrieval.
- [ ] Confirm tests fail, then add `/api/auth/*`, `/api/games`, `/api/games/{id}/draft`, `/api/games/{id}/publish`, and `/api/games/{id}/versions/{number}` routes.
- [ ] Use opaque `HttpOnly` cookies, CSRF header checks on mutations, configured `Secure`/`SameSite`, and the existing `{error, details, code}` error shape.
- [ ] Run `go test ./internal/httpapi ./... -count=1 && go vet ./...`, then commit `feat: expose account and publication APIs`.

### Task 3: Frontend identity and author dashboard shell

**Files:** Modify `frontend/src/lib/api.ts`, `frontend/src/lib/types.ts`, `frontend/src/App.svelte`; create `frontend/src/components/AuthPanel.svelte`, `frontend/src/components/GameDashboard.svelte`; create frontend tests with Vitest and Testing Library.

- [ ] Add failing component tests for guest entry, account registration/login state, empty dashboard, draft visibility and immutable published version badge.
- [ ] Add Vitest/Testing Library dependencies and execute the failing tests.
- [ ] Implement typed API calls and focused components; keep old prototype editor reachable only through an owned draft, not global anonymous writes.
- [ ] Run `npm run check`, `npm run build`, and focused frontend tests; commit `feat: add author identity dashboard`.

### Task 4: Verification and docs

**Files:** Modify `README.md`, `docs/ARCHITECTURE.md`, `docs/CURRENT_STATE.md`, `docs/PLAYTEST_CHECKLIST.md`; extend `scripts/smoke.sh`.

- [ ] Add a smoke flow: register → create draft → save definition → publish → retrieve immutable version.
- [ ] Run `make stop-dev`, `make smoke` three times, `make check`, `make test`, frontend check/build, PostgreSQL integration tests, and process/port cleanup checks.
- [ ] Perform real browser verification of registration, guest entry, draft save and publishing; record browser and console/network results honestly.
- [ ] Commit `docs: document identity and game publication workflow`.
