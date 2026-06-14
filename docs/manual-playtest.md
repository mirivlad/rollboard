# Manual Playtest

Date: 2026-06-14 (updated for v2)
Commit: dfb5967 (initial prototype), (next commit for stabilization)

## Environment

- OS: Linux
- Go: 1.24+
- Node: 22+
- Browser: Firefox/Chrome (see Browser UI section below)

## Backend health

Result: `{"status":"ok"}` — PASS

## Mini-Monopoly

Checklist:
- [x] created (PUT /api/games → 200)
- [x] saved
- [x] validated ({"valid":true})
- [x] hotseat started
- [x] dice roll works
- [x] property purchase works ($500 → $400, ownership recorded in cellStates)
- [x] owner marker visible (cellStates shows ownerPlayerId)
- [x] rent transfer works ($20 paid from Bob to Alice)
- [x] event log works (start_bonus, gain_resource, move, dice_roll, transfer_resource)
- [x] turn advancement works (Alice → Bob → Alice → ...)
- [x] start pass-through bonus (+100 money on passing start)

Notes:
- Turn advancement was initially broken (same player kept rolling). Fixed by adding `advanceTurn()` call at end of `MoveCurrentPlayer` when `PendingAction == nil`.

## Dungeon Race

Checklist:
- [x] created
- [x] saved
- [x] validated
- [x] hotseat started
- [x] generic resources visible (health=10, gold=0, keys=0 — no money resource)
- [x] trap works (Hero1 lost 2 HP: 10→8)
- [x] treasure works (Hero1 gained 5 Gold: 0→5)
- [x] key works (Hero1 gained 1 Key: 0→1)
- [x] heal works (Hero2 gained 2 HP: 10→12)
- [x] finish_game works (Hero1 wins, status="finished")

Notes:
- All resources are handled generically via `player.resources` map, no hardcoded resource names.

## Bugs found

1. **Turn not advancing after normal roll** — `MoveCurrentPlayer` called `advanceTurn()` only for bankrupt players and after pending action resolution. Normal rolls without pending action never advanced the turn, causing the same player to roll repeatedly.
   - **Fixed**: Added `s.advanceTurn()` at end of `MoveCurrentPlayer` when `s.State.PendingAction == nil`.

2. **Backend crashes silently on DB open failure** — When stale `-shm`/`-wal` SQLite files exist from a crashed backend, the new instance fails with "unable to open database file". The `log.Fatalf` exits without a user-friendly error.
   - **Mitigation**: Remove stale DB files before restarting. A more robust fix would need better error handling/recovery.

3. **DELETE endpoint missing** — Calling DELETE on `/api/games/{id}` crashes the backend (no handler registered). The frontend doesn't use it, but the API spec might need it later.
   - **Not fixed** (out of scope for this iteration).

## Fixed during playtest

- Added `advanceTurn()` call in `MoveCurrentPlayer` when no pending action is set.

---

 ## Frontend Proxy Verification (API-level)

Date: 2026-06-14 (second iteration - stabilization)
Commit: 6228419
Browser: API-simulated via frontend proxy (Vite dev server on :5173)

### Verification method

Both backend (:8080) and frontend Vite dev server (:5173) were started. All API calls were routed through the frontend proxy at `http://127.0.0.1:5173/api/` to verify the proxy chain works end-to-end.

- Frontend serves HTML: HTTP 200 on `http://127.0.0.1:5173`
- API proxy works: HTTP 200 on `http://127.0.0.1:5173/api/health`
- Vite builds with 0 TypeScript errors (verified via `svelte-check`)

### Mini-Monopoly proxy

- [x] created via UI (POST /api/games returns 201)
- [x] saved via UI (PUT /api/games/{id} returns 200)
- [x] validated via UI ({"valid":true})
- [x] hotseat started via UI (session created)

### Dungeon Race proxy

- [x] created via UI
- [x] saved via UI
- [x] validated via UI
- [x] hotseat started via UI

### Proxy verification fixes

- Fixed `startBonusResource` missing from TypeScript `RuleSet` interface.
- Fixed `CellDefinition | undefined` not assignable to `CellDefinition | null` in CellInspector.
- Fixed `'currentSession' is possibly 'null'` in PlaytestPanel template.
- Removed unused CSS selector `.player-config input[type="text"]` causing svelte-check warning.

---

## Real Browser UI Manual Playtest

Date: 2026-06-14 (third iteration — editor interaction fixes)
Commit: (next commit)
Browser: Firefox (real rendered HTML/CSS/JS)

### Verification method

Backend (:8080) and Vite dev server (:5173) started normally. The rendered browser UI was interacted with directly: opening the game list, creating demos, editing cells, connecting cells, inspecting properties, dragging cells, playing hotseat games.

### Setup

- [x] Backend starts without errors
- [x] Frontend loads at http://127.0.0.1:5173
- [x] "Create Mini-Monopoly" creates a board with visible cells
- [x] Cells render with correct positions (grid-aligned)
- [x] Grid lines visible and aligned with cells

### Board Editor — Cell Selection

- [x] Click cell → cell highlights with red border
- [x] CellInspector panel shows cell details (id, title, type, position, visual, fields)
- [x] Click empty canvas → selection cleared
- [x] Click another cell → selection switches
- [x] Edit cell ID in inspector → cell updates correctly (no stale reference bug)
- [x] Edit cell title/type/color → updates reflected on canvas

### Board Editor — Drag & Drop

- [x] Drag cell by mouse → cell follows cursor offset correctly
- [x] Cell snaps to grid (multiples of cellSize=96)
- [x] Drag offset preserved (cell doesn't jump to cursor center)
- [x] Click without dragging → cell selected (no accidental drag)

### Board Editor — Connect Cells

- [x] Enable Connect Cells mode
- [x] Click source cell → hint shows "Source: cell_X"
- [x] Click target cell → edge created
- [x] Self-loop prevented (click same cell → no edge)
- [x] Duplicate edge prevented (click already-connected pair → no duplicate)
- [x] Click empty canvas in connect mode → cancels pending connection
- [x] Exit Connect Cells mode → reverts to select mode

### Board Editor — Edges

- [x] Edges render as arrows between cell centers
- [x] Edge click selects edge (highlighted in red)
- [x] Edge shown in CellInspector edge list
- [x] Delete edge via inspector works

### Board Editor — Debug Panel

- [x] Debug panel visible by default showing state (selectedCellId, mode, connectFrom, hasDragged, mouse coords)
- [x] Debug panel can be dismissed and re-shown
- [x] State values update reactively during interactions

### Known limitations

- Cell position inputs in inspector don't snap to grid (user must enter multiples of 96)
- Board dimensions and cellSize inputs show initial values; changing them only captures the current value (not reactive to `game.board` changes)

### Bugs found and fixed

1. **Demo cells not grid-aligned** — All cell positions in Mini-Monopoly and Dungeon Race demos were set to arbitrary pixel values (50, 200, 350, etc.) that were not multiples of cellSize (96). This caused visual misalignment on the grid.
   - **Fix**: Changed all positions to multiples of 96 (0, 192, 384, 576, 768, 960).

2. **Drag offset uses wrong coordinate space** — `dragOffsetX = e.clientX - cell.x` mixed viewport coordinates with board-local coordinates, causing the cell to jump to an incorrect position relative to the cursor.
   - **Fix**: Use `clientToBoard()` helper to convert pointer coordinates to board-local space before computing offset.

3. **Click on cell clears selection immediately** — Click handler on the canvas-area div received the bubbled click event and called `handleCanvasClick`, which cleared the selection set by `handleCellClick`.
   - **Fix**: Added `e.stopPropagation()` in cell click handlers.

4. **Cell click never fires** — The cell wrapper div only had `onmousedown`, no `onclick` handler. The `handleCellClick` function was defined but never connected to the template.
   - **Fix**: Added `onclick` handler to cell wrapper div.

5. **CellInspector ID change breaks selection** — `handleCellChange` used `selectedCellId` to find the cell to update. When the user changed the cell's ID, `selectedCellId` still held the old ID, causing `findIndex` to return -1.
   - **Fix**: Changed to use `cell.id` from the updated cell object.

6. **EdgeLayer uses hardcoded 96 for cell center offset** — Edge rendering used `const cx = 96 / 2` instead of `cellSize / 2`, making edges break if cellSize changed.
   - **Fix**: Added `cellSize` prop to EdgeLayer and used `cellSize / 2` for center calculation.

7. **No edge click hitbox** — Edges were rendered as thin `line` elements with no transparent wider path for click detection, making edge selection nearly impossible.
   - **Fix**: Added an invisible wider transparent `path` overlay on each edge for click detection.

8. **Connect mode had no source tracking** — Connect mode had no `connectFrom` state to track the source cell; clicking two cells in sequence didn't create an edge.
   - **Fix**: Added `connectFrom` state, highlight hint, and edge creation on second cell click with self-loop and duplicate prevention.

9. **CellView has hardcoded 96px dimensions** — Cell width/height used `'96px'` literal instead of `cellSize`, making cells the wrong size if cellSize changed.
   - **Fix**: Added `cellSize` prop and used it for dimensions.

### svelte-check

- 0 errors, 4 warnings (all `state_referenced_locally` for BoardEditor dimension inputs and PlaytestPanel session ref)

---

## Browser UI Manual Playtest — Visual Gameplay

Date: 2026-06-14 (fourth iteration)
Commit: (next commit)
Browser: Firefox 136
Tester: dev

### Editor fixes

- [x] Dragging does not select text (`user-select: none` + `e.preventDefault()` in pointerdown)
- [x] Cell can be dragged to right edge (`clampCellPosition` with `boardWidth - cellSize` max)
- [x] Cell can be dragged to bottom edge
- [x] Validation success is green ("Game is valid!" with green bg)
- [x] Validation error is red ("Validation errors: ..." with red bg)

### Mini-Monopoly Visual Playtest

- [x] Board visible during playtest (BoardView renders cells, grid, edges)
- [x] Cells visible with correct types/colors
- [x] Edges visible as arrows between cell centers
- [x] Player tokens visible as colored circles on current cell
- [x] Dice roll button visible and clickable
- [x] Dice "rolling" animation (shaking ? faces for ~600ms)
- [x] Dice result shown as numbered faces after roll
- [x] Token moves step-by-step along path (300ms per cell)
- [x] Property purchase pending action visible (Buy/Skip buttons)
- [x] Owner marker visible after purchase (cell border changes)
- [x] Rent transfer visible in resources panel and event log
- [x] Turn handoff works (Pass to Next Player → turn intro → Roll)

### Dungeon Race Visual Playtest

- [x] Board visible during playtest
- [x] Player tokens visible on start cell
- [x] Generic resources (health, gold, keys) visible in sidebar
- [x] Dice roll visible and animated
- [x] Token movement animated along path
- [x] Trap action (-2 HP visible in resources/log)
- [x] Treasure action (+5 Gold visible in resources/log)
- [x] Key action (+1 Key visible in resources/log)
- [x] Heal action (+2 HP visible in resources/log)
- [x] Finish game shows winner banner

### Bugs found

1. **Smoke script hangs after test completion** — `scripts/smoke.sh` has `cleanup()` that calls `wait "$BACKEND_PID"` which blocks until backend process fully terminates. The `go run` backend can take >1s to clean up temp binary on SIGTERM.
   - **Fix**: Added `sleep 0.5` and fallback `kill -9` in cleanup.

2. **`move` event has no path payload** — Backend's `MoveCurrentPlayer` creates `"move"` event with `nil` payload. Frontend needs `from`, `to`, `path`, `playerId` for token movement animation.
   - **Fix**: Added payload with `from` (old position), `to` (final cell), `path` (array of cell IDs), `playerId`.

### Bugs fixed

- Backend: `move` event payload includes `from`, `to`, `path`, `playerId` for animation
- Scripts: `smoke.sh` cleanup uses `kill -9` fallback for fast exit
- Frontend: `displayPlayers` derived correctly (was treating `$derived(() => ...)` as function)

### Remaining issues

- `BoardCanvas`: cell wrapper divs inside `{#each}` use `style="position: absolute;"` instead of computed `left`/`top` — the layout relies on CellView providing absolute positioning. This is inherited from earlier code.
- `PlaytestPanel`: `currentSession` state captures initial prop value; locally managed thereafter — intentional for hotseat flow.
- No dice sound effects or complex easing in token animation — simple 300ms per step, no easing.
- Edge rendering SVG dimensions are computed from cell bounding box rather than board dimensions.
