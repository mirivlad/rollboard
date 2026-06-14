# Rollboard — Current State

Last updated: 2026-06-14
Current commit: 49dc020

## Status

Playable local prototype — stabilization phase.

The engine can be edited, saved, loaded, and playtested in hotseat mode. Two demo games (Mini-Monopoly, Dungeon Race) are bundled.

## Known Issues

1. **Playtest board layout** — board may look cramped or clipped depending on window size. The view does not auto-scale to fit the board inside the viewport on smaller screens. A fit-to-view or scrollable area is needed.

2. **Dice result visual** — dice result is shown in the sidebar before movement animation starts, but on a narrow/wrapped sidebar the dice images may not be immediately visible. The flow is: roll → dice result shown → 600ms pause → token animation.

3. **Dungeon Race game ending** — Dungeon Race uses a single linear path. With 1d6, a player can overshoot the finish cell (the engine moves step-by-step following edges; if the path doesn't loop back, movement stops at the last edge). This may cause the game to end correctly or not, depending on the edge connectivity. See `engine.go:MoveCurrentPlayer` — if there's no edge from a cell, movement stops regardless of remaining steps.

4. **New game ID generation** — creating a new game with an empty ID auto-generates a slug from the title via `generateSlug`. If multiple games have the same title, the second save will fail (unique constraint). No auto-deduplication.

5. **No game delete endpoint** — `DELETE /api/games/:id` returns 501 Not Implemented. No way to remove games from the UI.

6. **No undo** — once a roll is made and movement occurs, it's recorded. No undo/rewind.

7. **No online multiplayer** — hotseat only.

8. **ARCHITECTURE.md created** — was missing, now present as of this update.

## Recent Fixes (2026-06-14 v2 — Rollboard Stabilization Sprint)

- **Board geometry sync**: `BoardEditor.svelte` now normalizes `game.board.width`/`height` on game load — ensures `width = cols × cellSize`, `height = rows × cellSize`. `App.svelte` `saveGame()` syncs dimensions before sending. Fixes validation seeing stale 1100×400 when UI shows 1056×384.

- **Game Rules Editor**: Toolbar now shows editable fields: Title, Cols, Rows, Cell Size, Dice (count + sides), Start Bonus (resource + amount). All fields sync on game switch. Dice settings persist through Save/Reload.

- **Backend panic recovery**: Sanitized JSON error response — no longer leaks `%v` in `{"error":"internal server error: %v"}`. Returns standard `{"error":"internal server error","details":"panic recovered; see backend logs"}`. Stack trace goes to backend log.

- **Regression tests**: Added 3 tests — `TestDungeonRaceRollDoesNotPanic`, `TestMiniMonopolyRollDoesNotPanic`, `TestDungeonRaceWinnerIsCurrentPlayer`. All pass. Confirms first roll in fresh session doesn't panic, winner is correctly Player 1.

- **BoardView centering**: Board container now centers the playfield horizontally with `display: flex; justify-content: center; align-items: flex-start; padding: 16px;`. Smaller windows get scroll.

- **Script cleanup**: Added `make stop-dev` target. Created `scripts/stop-dev.sh` for process cleanup. Browser smoke test (`scripts/browser-smoke.sh`) — 6/6 checks pass.

- **Dice behavior**: Already correctly implemented — rolling animation → result pause (600ms) → token animation. All dice configurations visible and editable in rules editor.

- **Random seed**: Go 1.20+ auto-seeds `math/rand`. No fixed seed in dev/play mode. Deterministic only in engine tests where specific roll values are passed directly.

## Verification (2026-06-14 v2)

- `go build ./...` — OK
- `go vet ./...` — OK
- `go test ./... -count=1` — 22 tests pass
- `npm run build` — OK
- `make smoke` — runs without leftover processes
- `make smoke ×3` — no process leaks
- `make stop-dev` — cleans up Rollboard processes
- Browser smoke (`scripts/browser-smoke.sh`) — 6/6 API checks pass
