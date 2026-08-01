# PostgreSQL and Docker Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace SQLite with a tested PostgreSQL storage foundation and make the current application reproducibly runnable as a Docker Compose and Portainer stack.

**Architecture:** Add a typed application configuration and a PostgreSQL-backed storage package behind a small store interface. Preserve the current game/session HTTP contract during this foundation task, then evolve it in the identity/versioning plan. PostgreSQL is durable; Redis is provisioned and health-checked now but no product correctness depends on it yet.

**Tech Stack:** Go 1.24, `pgx/v5` connection pool, embedded SQL migrations, PostgreSQL 16, Redis 7, Docker Compose, Svelte 5/Vite.

## Global Constraints

- Do not retain SQLite code, dependencies, runtime flags, or a fallback database mode.
- The server remains authoritative; no game result is accepted from the frontend.
- Keep grid geometry and dice validation invariants unchanged.
- Return JSON errors shaped as `{ "error", "details", "code" }`.
- Docker images run as a non-root user and contain no data files or secrets.
- Every server-starting script must clean up children on success, failure, SIGINT, SIGTERM, and timeout.
- Do not commit `.superpowers/`, databases, `node_modules/`, build output, or `.env`.

---

## File map

| Path | Responsibility |
| --- | --- |
| `backend/internal/config/config.go` | Parse and validate environment-backed application configuration. |
| `backend/internal/config/config_test.go` | Configuration defaults and invalid-input tests. |
| `backend/internal/storage/store.go` | Store contract used by HTTP handlers. |
| `backend/internal/storage/postgres/store.go` | PostgreSQL pool lifecycle and game/session implementation. |
| `backend/internal/storage/postgres/migrate.go` | Embedded, ordered migration runner. |
| `backend/internal/storage/postgres/migrations/000001_core.sql` | Durable tables for the compatibility game/session API. |
| `backend/internal/storage/postgres/store_test.go` | PostgreSQL integration tests gated by `ROLLBOARD_TEST_DATABASE_URL`. |
| `backend/internal/httpapi/handler.go` | Depend on `storage.Store`, not SQLite. |
| `backend/cmd/server/main.go` | Construct config/store, expose health/readiness, structured middleware. |
| `Dockerfile`, `compose.yaml`, `deploy/portainer-stack.yaml` | Production image and stack definitions. |
| `.env.example`, `scripts/dev.sh`, `scripts/smoke.sh`, `README.md` | PostgreSQL-first local/deployment instructions. |

### Task 1: Ignore visual-design session state and add configuration tests

**Files:**
- Modify: `.gitignore`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`

**Consumes:** Environment variables supplied by Compose and local development.

**Produces:** `config.Load() (Config, error)` and `Config.Validate() error`.

- [ ] **Step 1: Write failing config tests**

```go
func TestLoadUsesSafeDefaults(t *testing.T) {
    t.Setenv("ROLLBOARD_DATABASE_URL", "")
    cfg, err := Load()
    require.NoError(t, err)
    assert.Equal(t, "127.0.0.1:8080", cfg.Addr)
    assert.Equal(t, "postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable", cfg.DatabaseURL)
    assert.Equal(t, "redis://127.0.0.1:6379/0", cfg.RedisURL)
}

func TestLoadRejectsInvalidCookieSecurity(t *testing.T) {
    t.Setenv("ROLLBOARD_COOKIE_SECURE", "not-a-bool")
    _, err := Load()
    assert.ErrorContains(t, err, "ROLLBOARD_COOKIE_SECURE")
}
```

- [ ] **Step 2: Run the focused test and confirm failure**

Run: `cd backend && go test ./internal/config -run TestLoad -count=1`

Expected: package/path does not yet exist.

- [ ] **Step 3: Implement the configuration contract**

```go
type Config struct {
    Addr         string
    DatabaseURL  string
    RedisURL     string
    CookieSecure bool
    AppOrigin    string
}

func Load() (Config, error) {
    secure, err := envBool("ROLLBOARD_COOKIE_SECURE", false)
    if err != nil { return Config{}, err }
    return Config{
        Addr: env("ROLLBOARD_ADDR", "127.0.0.1:8080"),
        DatabaseURL: env("ROLLBOARD_DATABASE_URL", "postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable"),
        RedisURL: env("ROLLBOARD_REDIS_URL", "redis://127.0.0.1:6379/0"),
        CookieSecure: secure,
        AppOrigin: env("ROLLBOARD_APP_ORIGIN", "http://127.0.0.1:5173"),
    }, nil
}
```

Use `net/url.ParseRequestURI` to reject invalid database and Redis URLs. Add
`.superpowers/` to `.gitignore` as one exact line.

- [ ] **Step 4: Run tests and formatting**

Run: `cd backend && gofmt -w internal/config && go test ./internal/config -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the isolated change**

```bash
git add .gitignore backend/internal/config
git commit -m "feat: add Rollboard application configuration"
```

### Task 2: Add an embedded, transactional PostgreSQL migration runner

**Files:**
- Modify: `backend/go.mod`
- Create: `backend/internal/storage/postgres/migrate.go`
- Create: `backend/internal/storage/postgres/migrations/000001_core.sql`
- Create: `backend/internal/storage/postgres/migrate_test.go`

**Consumes:** `pgxpool.Pool` and `ROLLBOARD_TEST_DATABASE_URL`.

**Produces:** `postgres.Migrate(ctx, pool) error`; an idempotent `schema_migrations` table and `games`/`sessions` tables.

- [ ] **Step 1: Write the failing migration integration test**

```go
func TestMigrateIsIdempotent(t *testing.T) {
    dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
    if dsn == "" { t.Skip("ROLLBOARD_TEST_DATABASE_URL is required") }
    pool := newTestPool(t, dsn)
    require.NoError(t, Migrate(context.Background(), pool))
    require.NoError(t, Migrate(context.Background(), pool))
    var count int
    require.NoError(t, pool.QueryRow(context.Background(),
        `SELECT count(*) FROM schema_migrations WHERE version = 1`).Scan(&count))
    assert.Equal(t, 1, count)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd backend && ROLLBOARD_TEST_DATABASE_URL="$ROLLBOARD_DATABASE_URL" go test ./internal/storage/postgres -run TestMigrate -count=1`

Expected: package does not exist or migration function is undefined.

- [ ] **Step 3: Replace SQLite dependency and implement migrations**

Run: `cd backend && go get github.com/jackc/pgx/v5/pgxpool@v5.7.2 && go mod tidy`

Embed `migrations/*.sql` with `embed.FS`. In a transaction, create
`schema_migrations(version bigint primary key, applied_at timestamptz not null)`;
for every numeric filename not present, execute its SQL then insert its version.
Use this initial schema:

```sql
CREATE TABLE games (
  id text PRIMARY KEY,
  title text NOT NULL,
  version integer NOT NULL CHECK (version > 0),
  definition_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
  id text PRIMARY KEY,
  game_id text NOT NULL REFERENCES games(id),
  game_version integer NOT NULL CHECK (game_version > 0),
  session_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_game_id_idx ON sessions(game_id);
```

- [ ] **Step 4: Run the migration test against a disposable PostgreSQL container**

Run: `docker run --detach --rm --name rollboard-foundation-postgres -e POSTGRES_USER=rollboard -e POSTGRES_PASSWORD=rollboard -e POSTGRES_DB=rollboard_test -p 127.0.0.1:55432:5432 postgres:16-alpine`

Then run: `cd backend && ROLLBOARD_TEST_DATABASE_URL='postgres://rollboard:rollboard@127.0.0.1:55432/rollboard_test?sslmode=disable' go test ./internal/storage/postgres -run TestMigrate -count=1`

Expected: PASS. Stop the explicit test container with
`docker stop rollboard-foundation-postgres` after the foundation is verified.

- [ ] **Step 5: Commit the migration layer**

```bash
git add backend/go.mod backend/go.sum backend/internal/storage/postgres
git commit -m "feat: add PostgreSQL migrations"
```

### Task 3: Implement the PostgreSQL compatibility store

**Files:**
- Create: `backend/internal/storage/store.go`
- Create: `backend/internal/storage/postgres/store.go`
- Create: `backend/internal/storage/postgres/store_test.go`
- Modify: `backend/internal/storage/sqlite/store.go` (delete)

**Consumes:** `game.GameDefinition`, `game.GameSession`, `postgres.Migrate`.

**Produces:** `storage.Store` used by API callers.

- [ ] **Step 1: Write failing persistence tests**

```go
func TestStoreRoundTripsGameAndSession(t *testing.T) {
    store := newTestStore(t)
    gameDef := testDefinition(t, "postgres-game")
    require.NoError(t, store.CreateGame(ctx, gameDef))
    loaded, err := store.GetGame(ctx, gameDef.ID)
    require.NoError(t, err)
    require.Equal(t, gameDef, loaded)

    session := game.StartSession(gameDef, []game.PlayerConfig{{Name:"A"}, {Name:"B"}})
    require.NoError(t, store.SaveSession(ctx, session))
    loadedSession, err := store.GetSession(ctx, session.ID)
    require.NoError(t, err)
    require.Equal(t, session.ID, loadedSession.ID)
}

func TestStoreUpdateIncrementsVersion(t *testing.T) { /* assert v1 becomes v2 */ }
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `cd backend && ROLLBOARD_TEST_DATABASE_URL="$ROLLBOARD_TEST_DATABASE_URL" go test ./internal/storage/postgres -run TestStore -count=1`

Expected: FAIL because `Store` methods are unavailable.

- [ ] **Step 3: Define the interface and implement it with pgx**

```go
type Store interface {
    Close()
    Ping(context.Context) error
    ListGames(context.Context) ([]GameSummary, error)
    GetGame(context.Context, string) (*game.GameDefinition, error)
    CreateGame(context.Context, *game.GameDefinition) error
    UpdateGame(context.Context, *game.GameDefinition) error
    SaveSession(context.Context, *game.GameSession) error
    GetSession(context.Context, string) (*game.GameSession, error)
}
```

`postgres.New(ctx, databaseURL)` must create a `pgxpool.Pool`, call `Migrate`,
and return a store. Marshal values once with `encoding/json`, pass the bytes as
`jsonb`, and scan JSON into `[]byte`. Implement update as one statement with
`version = version + 1 RETURNING version`; assign the returned value to the
definition. Use `ON CONFLICT (id) DO UPDATE` only for `SaveSession`, preserving
the original `created_at`.

- [ ] **Step 4: Remove SQLite implementation and prove no reference remains**

Run: `rm backend/internal/storage/sqlite/store.go && rmdir backend/internal/storage/sqlite`

Run: `rg -n 'sqlite|ROLLBOARD_DB_PATH|mattn/go-sqlite3' backend README.md scripts || true`

Expected: no production references after later documentation task; tests may only
contain an explicit assertion that the old variables are absent.

- [ ] **Step 5: Run store tests**

Run: `cd backend && gofmt -w internal/storage && ROLLBOARD_TEST_DATABASE_URL="$ROLLBOARD_TEST_DATABASE_URL" go test ./internal/storage/postgres -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the store**

```bash
git add backend/internal/storage
git commit -m "feat: persist games and sessions in PostgreSQL"
```

### Task 4: Wire startup, readiness, and structured errors to PostgreSQL

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/httpapi/handler.go`
- Create: `backend/internal/httpapi/handler_test.go`

**Consumes:** `config.Config`, `storage.Store`.

**Produces:** `GET /api/health`, `GET /healthz`, and `GET /readyz`; HTTP handlers no longer import a concrete database package.

- [ ] **Step 1: Write failing handler tests**

```go
func TestReadyzReturns503WhenStorePingFails(t *testing.T) {
    api := New(failingStore{})
    rr := httptest.NewRecorder()
    api.Readyz(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
    assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
    assert.JSONEq(t, `{"error":"service not ready","code":"NOT_READY"}`, rr.Body.String())
}

func TestWriteErrorHasStableShape(t *testing.T) { /* assert error, details, code */ }
```

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `cd backend && go test ./internal/httpapi -run 'TestReadyz|TestWriteError' -count=1`

Expected: FAIL because readiness and stable error fields are absent.

- [ ] **Step 3: Make API context-aware and storage-agnostic**

Change `API.store` to `storage.Store`; pass `r.Context()` to every store call.
Add `Healthz` returning `{ "status": "ok" }` and `Readyz` which calls
`store.Ping`. Replace `writeError(status, message)` with
`writeError(w, status, code, message, details)` and use explicit codes such as
`INVALID_JSON`, `NOT_FOUND`, `METHOD_NOT_ALLOWED`, `CONFLICT`, and
`INTERNAL_ERROR`. Do not expose raw database errors in `details` for a 500.

In `main.go`, load `config.Load`, create `postgres.New(context.Background(),
cfg.DatabaseURL)`, register routes, and set CORS to `cfg.AppOrigin` rather than
`*`. `recoveryMiddleware` writes `INTERNAL_ERROR` only when headers have not been
sent. `loggerMiddleware` logs method, path, status, and generated request ID.

- [ ] **Step 4: Run API and whole-backend checks**

Run: `cd backend && gofmt -w cmd/server internal/httpapi && go test ./... -count=1 && go vet ./...`

Expected: PASS.

- [ ] **Step 5: Commit server wiring**

```bash
git add backend/cmd/server backend/internal/httpapi
git commit -m "feat: run Rollboard on PostgreSQL"
```

### Task 5: Package the stack for Docker Compose and Portainer

**Files:**
- Create: `Dockerfile`
- Create: `compose.yaml`
- Create: `deploy/portainer-stack.yaml`
- Create: `deploy/postgres-init/01-create-test-db.sql`
- Modify: `.env.example`
- Create: `scripts/docker-smoke.sh`

**Consumes:** `ROLLBOARD_DATABASE_URL`, `ROLLBOARD_REDIS_URL`, application health endpoints.

**Produces:** `docker compose up --build` starts app, PostgreSQL, and Redis; a Portainer-ready equivalent stack.

- [ ] **Step 1: Write the Docker smoke script first**

```bash
#!/usr/bin/env bash
set -euo pipefail
cleanup() { docker compose down -v --remove-orphans; }
trap cleanup EXIT INT TERM
docker compose up --build -d
for attempt in $(seq 1 30); do
  curl -fsS http://127.0.0.1:8080/readyz >/dev/null && break
  sleep 1
done
curl -fsS http://127.0.0.1:8080/readyz | jq -e '.status == "ready"'
docker compose restart app
curl -fsS http://127.0.0.1:8080/readyz | jq -e '.status == "ready"'
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `./scripts/docker-smoke.sh`

Expected: FAIL because the Compose files and image do not exist.

- [ ] **Step 3: Implement the production definitions**

`Dockerfile` has a Node build stage (`npm ci && npm run build`), a Go build
stage (`CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'`), and a minimal final
stage that creates UID 10001, copies the binary and `frontend/dist`, sets
`USER 10001`, and exposes 8080.

`compose.yaml` declares `postgres:16-alpine` with `POSTGRES_DB=rollboard`, named
volume `postgres_data`, init mount for the test database, and `pg_isready`
healthcheck; `redis:7-alpine` with `redis-cli ping` healthcheck; and `app` with
`depends_on` health conditions, port `8080:8080`, no writable source mounts, and
URLs using service names. `deploy/portainer-stack.yaml` contains the same three
services, relative-free volume names, and `${ROLLBOARD_*}` variables for
Portainer's environment editor.

- [ ] **Step 4: Run Docker smoke and inspect health state**

Run: `./scripts/docker-smoke.sh && docker compose ps`

Expected: smoke exits 0; all services report healthy before cleanup.

- [ ] **Step 5: Commit deployment artifacts**

```bash
git add Dockerfile compose.yaml deploy .env.example scripts/docker-smoke.sh
git commit -m "feat: add Docker and Portainer deployment stack"
```

### Task 6: Convert developer scripts and user documentation

**Files:**
- Modify: `scripts/dev.sh`
- Modify: `scripts/smoke.sh`
- Modify: `scripts/check.sh`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/CURRENT_STATE.md`

**Consumes:** Compose services and PostgreSQL URLs from Task 5.

**Produces:** Repeatable local development and smoke checks without SQLite or stale processes.

- [ ] **Step 1: Make smoke assertions fail on an unavailable PostgreSQL service**

Add a shell assertion near the top of `scripts/smoke.sh`:

```bash
until pg_isready --dbname="$ROLLBOARD_DATABASE_URL" >/dev/null 2>&1; do
  echo "waiting for PostgreSQL"; sleep 1
done
```

Run: `make stop-dev && ROLLBOARD_DATABASE_URL='postgres://bad' make smoke`

Expected: non-zero exit with a bounded timeout, no child server left behind.

- [ ] **Step 2: Implement bounded startup and cleanup**

`dev.sh` runs `docker compose up -d postgres redis`, waits with a 30-attempt
loop, then starts Go and Vite in the background. Its cleanup must be registered
with `trap cleanup EXIT INT TERM`, stop both PIDs, and wait for them. `smoke.sh`
uses a disposable `rollboard_test` database, records the spawned PID, and stops
it through one cleanup function. No script uses `kill -9` or broad `pkill`.

- [ ] **Step 3: Update docs exactly**

Replace SQLite requirements/environment variables in `README.md` with
`ROLLBOARD_DATABASE_URL`, `ROLLBOARD_REDIS_URL`, Compose quick start, and
Portainer instructions. Update `ARCHITECTURE.md` storage wording and
`CURRENT_STATE.md` to say that the PostgreSQL foundation is implemented but
identity, versions, online rooms, chat, and UI replacement remain pending.

- [ ] **Step 4: Run the required repeatability checks**

Run:

```bash
make stop-dev
make smoke
make smoke
make smoke
make check
make test
cd frontend && npm install && npm run check && npm run build
ps aux | grep '/tmp/rollboard-server' | grep -v grep || true
ss -ltnp | grep -E ':8080|:8081|:8082|:8083|:8084|:5173' || true
```

Expected: all checks pass; no temporary server or listened project port remains.

- [ ] **Step 5: Commit and report the foundation**

```bash
git add scripts README.md docs
git commit -m "docs: document PostgreSQL development workflow"
git status --short
```

Expected: no generated assets, databases, or `.superpowers/` staged.
