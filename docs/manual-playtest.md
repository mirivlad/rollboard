# Manual Playtest

Date: 2026-06-14
Commit: (not yet committed)

## Environment

- OS: Linux
- Go: 1.24+
- Node: 22+
- Browser: (API-only test via Python urllib)

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
