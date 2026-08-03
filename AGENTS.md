# AGENTS.md — Rollboard Agent Guide

## Project identity

Rollboard is a browser-based engine and editor for route-based board games.

Core loop:

```text
player rolls dice → token moves along board graph → landing cell triggers actions
```

Rollboard is not a hardcoded Monopoly clone. It is a generic engine for games such as:

* Monopoly-like property games;
* linear race games;
* dungeon/RPG board games;
* branching roll-and-move games;
* hotseat local play;
* future online multiplayer.

The project uses:

* Go backend;
* Svelte 5 + Vite + TypeScript frontend;
* PostgreSQL storage, with Redis for cross-replica fan-out;
* visual board editor;
* hotseat playtest and authoritative multiplayer rooms;
* generic action-based runtime;
* runtime-loaded translation catalogs.

---

## Required reading order

Before making changes, read:

1. `AGENTS.md` — permanent project rules.
2. `docs/CURRENT_STATE.md` — current checkpoint, active blockers, what not to start yet.
3. `docs/ARCHITECTURE.md` — engine model, GameDefinition, runtime concepts.
4. `docs/PLAYTEST_CHECKLIST.md` — browser/manual validation checklist.
5. `README.md` — user-facing run and deployment instructions.
6. `CONTRIBUTING.md` — workflow, testing rules and style.
7. `docs/I18N.md` — how translations load and how to add one.

If any of these files are missing, create or update them as part of the work.

---

## Absolute rules

### 1. Server is authoritative

The frontend must not decide game rules.

Frontend may:

* display board state;
* send player intentions;
* animate dice and movement;
* show pending actions;
* show logs.

Backend must decide:

* dice results;
* legal moves;
* movement path;
* cell actions;
* resource changes;
* ownership;
* win/loss;
* pending actions.

Frontend must never send:

```text
I rolled 6
```

Frontend sends:

```text
I want to roll
```

Backend returns the dice result and new state.

---

### 2. Do not hardcode demo games into runtime

Mini-Monopoly and Dungeon Race are demo data, not engine logic.

Bad:

```go
if cell.Type == "property" { ... }
if cell.Type == "trap" { ... }
if cell.Type == "treasure" { ... }
player.Money += ...
```

Good:

```go
ExecuteAction(...)
EvaluateCondition(...)
TransferResource(...)
SetCellOwner(...)
FinishGame(...)
```

Game-specific words such as `property`, `rent`, `trap`, `treasure`, `key`, `heal`, `money`, `gold` may appear in:

* demo definitions;
* demo tests;
* UI labels;
* docs.

They must not define generic runtime behavior directly.

---

### 3. Keep runtime generic

Generic runtime primitives are allowed.

Examples:

```text
gain_resource
lose_resource
transfer_resource
offer_choice
set_cell_owner
if_cell_unowned
if_cell_owned_by_current
if_cell_owned_by_other
if_resource_ge
finish_game
log_message
```

Adding a new generic action is acceptable.

Adding a game-specific branch to runtime is not acceptable.

If a new feature only works for one demo game, stop and redesign it as generic data/action behavior.

---

### 4. Browser UI checks matter more than API-only checks

API tests are not browser UI tests.

Do not claim UI is fixed unless the rendered browser UI was opened and clicked.

API/proxy tests do not verify:

* visible editor panels;
* cell selection;
* drag behavior;
* mouse coordinate correctness;
* board scaling;
* clipped cells;
* token rendering;
* dice result visibility;
* movement animation;
* broken CSS/layout;
* stale frontend state.

If browser access is unavailable, report clearly:

```text
Browser UI verification was not performed. User/browser-agent verification is required.
```

Never write “browser UI tested” if only API calls were tested.

---

## Development workflow

From repository root:

```bash
make stop-dev
./scripts/dev.sh
```

Expected services:

```text
Backend:  http://127.0.0.1:8080
Frontend: http://127.0.0.1:5173
```

Before starting work, check for stale processes:

```bash
ps aux | grep rollboard | grep -v grep || true
ps aux | grep vite | grep -v grep || true
ss -ltnp | grep -E ':8080|:8081|:8082|:8083|:8084|:5173' || true
```

Stop project leftovers:

```bash
make stop-dev
```

Do not leave dev/test processes running.

---

## Required checks before reporting success

Run:

```bash
make stop-dev
make smoke
make smoke
make smoke
make check
make test
```

Then:

```bash
cd frontend
npm install
npm run check
npm run build
```

Then verify that test servers were cleaned up:

```bash
ps aux | grep '/tmp/rollboard-server' | grep -v grep || true
ss -ltnp | grep -E ':8080|:8081|:8082|:8083|:8084|:5173' || true
```

After `make smoke`, no `/tmp/rollboard-server` process should remain.

---

## Git rules

Do not commit:

```text
node_modules/
dist/
data/*.db
data/*.db-shm
data/*.db-wal
.env
tmp/
logs/
temporary binaries
```

Before commit:

```bash
make stop-dev
make check
make test
make smoke
cd frontend && npm run check && npm run build
```

Commit message must describe the actual user-visible change.

---

## Board geometry invariants

The editor is grid-based.

Preferred model:

```text
cols
rows
cellSize
```

Derived:

```text
board.width = cols * cellSize
board.height = rows * cellSize
```

Rules:

* board width must be divisible by cellSize;
* board height must be divisible by cellSize;
* cell x must be divisible by cellSize;
* cell y must be divisible by cellSize;
* cell x must be between `0` and `(cols - 1) * cellSize`;
* cell y must be between `0` and `(rows - 1) * cellSize`.

UI, saved `GameDefinition`, backend validation and playtest must use the same values.

If UI displays:

```text
1056 x 384
```

then saved definition must contain:

```text
width = 1056
height = 384
```

not stale values like `1100 x 400`.

---

## Playtest board rendering invariants

Playtest board must use one coordinate system.

All layers use:

```text
cell.x
cell.y
board.cellSize
board.width
board.height
```

Layer order:

```text
grid
edges
cells
owner markers / overlays
tokens
animation overlays
```

Rules:

* BoardView renders exact logical board size.
* Fit-to-view or scrolling is allowed, but all layers must transform together.
* Edges connect cell centers.
* Tokens render inside their current cell.
* During animation, only the moving token may render at animated position.
* The same token must not be rendered both on its cell and on the animated path.
* After animation, clear animation state.

---

## Dice behavior invariants

Dice config lives in:

```text
game.rules.dice.count
game.rules.dice.sides
```

Backend validation must reject:

```text
dice.count < 1
dice.count > 10
dice.sides < 2
dice.sides > 100
```

Roll response or move event must include:

```json
{
  "dice": [3, 5],
  "total": 8,
  "path": ["cell_a", "cell_b", "cell_c"]
}
```

UI sequence must be:

```text
Roll clicked
→ rolling animation
→ final dice values displayed
→ total displayed
→ short pause
→ token movement starts
→ landing action / pending choice / turn done
```

Frontend must not generate final dice values.

Runtime must not use fixed deterministic RNG in normal dev/play mode. Deterministic RNG is allowed only in tests and must be injected explicitly.

---

## Error handling rules

Backend should return structured JSON errors:

```json
{
  "error": "short message",
  "details": "optional details"
}
```

Frontend should show operation-specific messages:

```text
Save failed: ...
Validate failed: ...
Playtest start failed: ...
Roll failed: ...
```

Do not show only:

```text
Internal Server Error
```

---

## Dev script rules

Scripts must not leave stale processes.

Any script that starts a server must stop it on:

* success;
* failure;
* Ctrl+C;
* SIGINT;
* SIGTERM;
* timeout.

Use cleanup traps:

```bash
cleanup() {
  # stop children safely
}

trap cleanup EXIT INT TERM
```

`make smoke` must be repeatable. Running it multiple times must not leave `/tmp/rollboard-server` processes behind.

---

## Documentation rules

Keep permanent rules in `AGENTS.md`.

Keep current status in:

```text
docs/CURRENT_STATE.md
```

Keep manual/browser checklists in:

```text
docs/PLAYTEST_CHECKLIST.md
```

Keep architecture explanation in:

```text
docs/ARCHITECTURE.md
```

Keep future plans in:

```text
docs/ROADMAP.md
```

Do not turn `AGENTS.md` into a stale sprint board.

---

## Reporting rules

Final reports must be honest.

Always include:

1. What was changed.
2. Which files were changed.
3. Which checks were run.
4. Whether real browser UI was tested.
5. Browser used, if tested.
6. Known remaining issues.
7. Commit hash.

Do not say:

```text
Browser UI tested
```

unless a real browser was opened and clicked.

Do not say:

```text
Fixed
```

for UI features that are not visible in the rendered UI.

Use:

```text
Implemented, but browser verification required.
```

when browser testing was not possible.

---

## Stop conditions

Stop and ask/report before continuing if:

* UI and backend validation disagree;
* browser behavior contradicts automated test results;
* runtime needs game-specific hardcoding;
* a script leaves stale processes;
* playtest cannot be visually verified;
* a feature is implemented but not visible in UI;
* the same bug reappears after a “fix”.

Do not proceed to larger features while foundational UI/runtime consistency is broken.
