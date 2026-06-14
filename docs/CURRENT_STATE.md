# Rollboard — Current State

Last updated: 2026-06-14
Current commit: (pending — edge conditions stage)

## Status

**Verified playable prototype with branching routes.**
- Builder → wired board → playtest cycle proven with user-created board
- Browser E2E confirms custom edges render and tokens move along them
- Edge conditions implemented and browser-verified (see below)
- All 22 engine tests pass; 0 Svelte warnings; 0 build errors

The engine can be edited, saved, loaded, and playtested in hotseat mode. Four demo games are bundled: Mini-Monopoly, Dungeon Race, Branching Demo, Manual Branch Demo.

## Edge Conditions — Implemented and Browser-Verified

| Condition Type | Demo Game | Browser E2E | Status |
|---|---|---|---|
| `dice_total_even` | Branching Demo | ✅ rolled 2 → even_cell | PASS |
| `dice_total_odd` | Branching Demo | ✅ rolled odd → odd_cell → finish | PASS |
| `manual_choice` | Manual Branch Demo | ✅ "Safe Path" button shown | PASS |
| `pay_resource` | Manual Branch Demo | ✅ "Pay 2 Gold" button shown | PASS |
| `route_choice` pending action | Manual Branch Demo | ✅ Choose path UI with options | PASS |
| resource subtraction on pay | Manual Branch Demo | ✅ gold 5→3 after Pay 2 Gold | PASS |
| movement after route choice | Manual Branch Demo | ✅ token moves to finish after choice | PASS |
| event log for route choice | Manual Branch Demo | ✅ "moved from start to choice_fork" etc. | PASS |
| edge condition labels in editor | Both demos | ✅ "Safe Path", "pay 2 gold" visible | PASS |
| edge inspector edit condition | Both demos | ✅ condition type selectable | PASS |

## Known Issues

1. **Playtest board layout** — board may look cramped or clipped depending on window size. The view does not auto-scale to fit the board inside the viewport on smaller screens. A fit-to-view or scrollable area is needed.

2. **Dice result visual** — dice result is shown in the sidebar before movement animation starts, but on a narrow/wrapped sidebar the dice images may not be immediately visible. The flow is: roll → dice result shown → 600ms pause → token animation.

3. **Dungeon Race game ending** — Dungeon Race uses a single linear path. With 1d6, a player can overshoot the finish cell (the engine moves step-by-step following edges; if the path doesn't loop back, movement stops at the last edge). This may cause the game to end correctly or not, depending on the edge connectivity. See `engine.go:MoveCurrentPlayer` — if there's no edge from a cell, movement stops regardless of remaining steps.

4. **New game ID generation** — creating a new game with an empty ID auto-generates a slug from the title via `generateSlug`. If multiple games have the same title, the second save will fail (unique constraint). No auto-deduplication.

5. **No game delete endpoint** — `DELETE /api/games/:id` returns 501 Not Implemented. No way to remove games from the UI.

6. **No undo** — once a roll is made and movement occurs, it's recorded. No undo/rewind.

7. **No online multiplayer** — hotseat only.

8. **ARCHITECTURE.md created** — was missing, now present as of this update.

## Recent Fixes (2026-06-14 v3 — Final verification sprint)

- **Custom wired board E2E**: Proved editor → connect cells → save → validate → playtest → roll → token moves along user-defined edges. Tested with Start→A→B→Finish linear path. Event Log confirms route.
- **Svelte 5 warnings eliminated**: `state_referenced_locally` for `game` prop fixed — all reactive state initialized with defaults (not from `game`), `$effect` watches `game.id` reactively. `npm run check` → **0 errors, 0 warnings**.
- **Browser E2E screenshots**: 7 real PNG screenshots in `artifacts/browser-smoke/` — editor, playtest, dice result, custom board, after-move. Not API-only.
- **Game switch toolbar update**: Switching between Dungeon Race (1d6) → Mini-Monopoly (1d6) → back shows fresh values, not stale.
- **Dice persistence**: 1d6→1d8→Save→switch games→back→still 1d8.
- **Player 2 victory check**: confirmed Player 2 does NOT win from Player 1's turn (UI + regression test).

## Recent Fixes (2026-06-14 v2 — Rollboard Stabilization Sprint)

- **Board geometry sync**: `BoardEditor.svelte` now normalizes `game.board.width`/`height` on game load — ensures `width = cols × cellSize`, `height = rows × cellSize`. `App.svelte` `saveGame()` syncs dimensions before sending. Fixes validation seeing stale 1100×400 when UI shows 1056×384.

- **Game Rules Editor**: Toolbar now shows editable fields: Title, Cols, Rows, Cell Size, Dice (count + sides), Start Bonus (resource + amount). All fields sync on game switch. Dice settings persist through Save/Reload.

- **Backend panic recovery**: Sanitized JSON error response — no longer leaks `%v` in `{"error":"internal server error: %v"}`. Returns standard `{"error":"internal server error","details":"panic recovered; see backend logs"}`. Stack trace goes to backend log.

- **Regression tests**: Added 3 tests — `TestDungeonRaceRollDoesNotPanic`, `TestMiniMonopolyRollDoesNotPanic`, `TestDungeonRaceWinnerIsCurrentPlayer`. All pass. Confirms first roll in fresh session doesn't panic, winner is correctly Player 1.

- **BoardView centering**: Board container now centers the playfield horizontally with `display: flex; justify-content: center; align-items: flex-start; padding: 16px;`. Smaller windows get scroll.

- **Script cleanup**: Added `make stop-dev` target. Created `scripts/stop-dev.sh` for process cleanup. Browser smoke test (`scripts/browser-smoke.sh`) — 6/6 checks pass.

- **Dice behavior**: Already correctly implemented — rolling animation → result pause (600ms) → token animation. All dice configurations visible and editable in rules editor.

- **Random seed**: Go 1.20+ auto-seeds `math/rand`. No fixed seed in dev/play mode. Deterministic only in engine tests where specific roll values are passed directly.

## Verification (2026-06-14 v3 — Final)

- `go build ./...` — OK
- `go vet ./...` — OK
- `go test ./... -count=1` — **22/22 tests pass** 
- `npm run build` — OK (JS 88KB, CSS 14KB)
- `npm run check` — **0 errors, 0 warnings**
- `make smoke ×3` — **6/6, 6/6, 6/6** — no process leaks, ports free
- Browser E2E screenshots: 7 PNGs in `artifacts/browser-smoke/`
- Browser used: Chromium via Hermes Agent (Playwright)
- Console errors during E2E: 0
- Network errors during E2E: 0
- Current commit: cf81d4a

## Verification (2026-06-14 v2)

- `go build ./...` — OK
- `go vet ./...` — OK
- `go test ./... -count=1` — 22 tests pass
- `npm run build` — OK
- `make smoke` — runs without leftover processes
- `make smoke ×3` — no process leaks
- `make stop-dev` — cleans up Rollboard processes
- Browser smoke (`scripts/browser-smoke.sh`) — 6/6 API checks pass
