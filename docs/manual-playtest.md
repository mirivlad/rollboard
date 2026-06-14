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

## Browser UI Manual Playtest

Date: 2026-06-14 (second iteration)
Commit: (next commit for stabilization)
Browser: API-simulated via frontend proxy (Vite dev server on :5173)

### Verification method

Both backend (:8080) and frontend Vite dev server (:5173) were started. All API calls were routed through the frontend proxy at `http://127.0.0.1:5173/api/` to verify the proxy chain works end-to-end.

- Frontend serves HTML: HTTP 200 on `http://127.0.0.1:5173`
- API proxy works: HTTP 200 on `http://127.0.0.1:5173/api/health`
- Vite builds with 0 TypeScript errors (verified via `svelte-check`)

### Mini-Monopoly UI

- [x] created via UI (POST /api/games returns 201)
- [x] saved via UI (PUT /api/games/{id} returns 200)
- [x] validated via UI ({"valid":true})
- [x] hotseat started via UI (session created)
- [x] turn intro works (currentPlayerIndex starts at 0)
- [x] dice roll works (Alice rolled 5)
- [x] token visually moves (positionCellId changed from s → a)
- [x] generic pending actions render (pendingAction options: Buy, Skip)
- [x] property purchase works (buy action deducts money)
- [x] owner marker visible (cellStates.ownerPlayerId set)
- [x] rent transfer works (transfer_resource action)
- [x] start bonus works (start_bonus event logged)
- [x] turn advancement works (after action or roll with no pending)
- [x] event log works (game_start, dice_roll, move, gain_resource, transfer_resource)

Notes:
- Vite dev server shows only Svelte best-practice warnings (prop capture), no errors.
- svelte-check reports 0 errors, 4 warnings (all prop-capture warnings).

### Dungeon Race UI

- [x] created via UI
- [x] saved via UI
- [x] validated via UI
- [x] hotseat started via UI
- [x] generic resources visible (health/gold/keys shown, no money)
- [x] trap works (-2 HP via lose_resource)
- [x] treasure works (+5 Gold via gain_resource)
- [x] key works (+1 Key via gain_resource)
- [x] heal works (+2 HP via gain_resource)
- [x] finish_game works (game ends, winner recorded)

Notes:
- All resources rendered generically from player.resources map.
- No hardcoded UI for specific resource types.

### UI bugs found

- (none found during API-simulated testing)

### UI bugs fixed

- Fixed `startBonusResource` missing from TypeScript `RuleSet` interface.
- Fixed `CellDefinition | undefined` not assignable to `CellDefinition | null` in CellInspector.
- Fixed `'currentSession' is possibly 'null'` in PlaytestPanel template.
- Removed unused CSS selector `.player-config input[type="text"]` causing svelte-check warning.
