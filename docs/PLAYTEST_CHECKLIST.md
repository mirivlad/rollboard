# PLAYTEST_CHECKLIST.md — Rollboard Browser Verification

Use this file for real browser testing.

Browser tests must be performed in rendered UI at:

```text
http://127.0.0.1:5173
```

API-only checks do not count as browser UI verification.

---

## Before browser test

Run:

```bash
make stop-dev
./scripts/dev.sh
```

Open DevTools Console and Network.

Check:

* no old backend process is responding;
* no unexplained 500 errors;
* frontend is the current version;
* `/api/health` works.

---

## Editor rules panel

* [ ] Game title input is visible.
* [ ] Board columns input is visible.
* [ ] Board rows input is visible.
* [ ] Cell size input is visible.
* [ ] Calculated board size is visible.
* [ ] Dice count input is visible.
* [ ] Dice sides input is visible.
* [ ] Start bonus resource input is visible.
* [ ] Start bonus amount input is visible.
* [ ] Dice settings can be changed.
* [ ] Dice settings survive Save → reload.

---

## Board geometry

* [ ] UI board size equals saved `GameDefinition.board.width/height`.
* [ ] Validate uses same width/height as UI.
* [ ] No validation error with stale width/height.
* [ ] Demo cells align to grid.
* [ ] Dragging a cell does not select text.
* [ ] Cell can move to top-left.
* [ ] Cell can move to top-right.
* [ ] Cell can move to bottom-left.
* [ ] Cell can move to bottom-right.
* [ ] Cell never leaves board.
* [ ] CellInspector opens on selection.
* [ ] CellInspector updates when selecting another cell.

---

## Connections

* [ ] Connect mode activates visibly.
* [ ] Source cell can be selected.
* [ ] Target cell creates edge.
* [ ] Duplicate edge is rejected.
* [ ] Self-loop is rejected.
* [ ] Edge is visible.
* [ ] Edge connects cell centers.
* [ ] Edge can be selected.
* [ ] Edge can be deleted.

---

## Validation UI

* [ ] Valid game message is green/success.
* [ ] Invalid game message is red/error.
* [ ] Error text is specific.
* [ ] No generic `Internal Server Error` without details.

---

## Dungeon Race playtest

* [ ] Demo validates immediately after creation/save.
* [ ] Playtest starts with 2 players.
* [ ] Sidebar shows correct dice rule.
* [ ] Board is visible.
* [ ] All cells are visible.
* [ ] No cell is clipped.
* [ ] Edges connect cell centers.
* [ ] Tokens start on Start.
* [ ] Tokens are not duplicated.
* [ ] Dice animation appears.
* [ ] Final dice result is shown.
* [ ] Total is shown.
* [ ] Movement starts after result is visible.
* [ ] Token moves step by step.
* [ ] Trap changes health.
* [ ] Treasure changes gold.
* [ ] Key changes keys.
* [ ] Heal changes health.
* [ ] Finish declares correct winner.

---

## Mini-Monopoly playtest

* [ ] Demo validates immediately after creation/save.
* [ ] Playtest starts with 2 players.
* [ ] Sidebar shows dice rule.
* [ ] Board is visible.
* [ ] Tokens start on Start.
* [ ] Dice result is shown before movement.
* [ ] Token moves along route.
* [ ] Property purchase pending action appears.
* [ ] Buying property changes resources.
* [ ] Owner marker appears.
* [ ] Another player landing on owned property pays rent.
* [ ] Event log explains the action.
* [ ] Turn handoff works.

---

## Multiplayer room

Use two independent browser sessions: an account host and a guest or second account.

* [ ] Host publishes a game version and creates a room.
* [ ] Guest joins with the displayed room ID.
* [ ] Host roster updates without a page reload.
* [ ] Both clients show `LIVE ROOM`; browser network shows a WebSocket upgrade.
* [ ] Only the host can start the lobby.
* [ ] After start, only the current player sees **Roll dice**.
* [ ] A roll changes the board and turn on both clients.
* [ ] A generic pending action shows only to its designated player.
* [ ] Resolving the action advances the turn on both clients.
* [ ] Chat appears on both clients and persists after reload.
* [ ] A muted player cannot send chat; host mute/unmute and removal return clear UI feedback once moderation controls are exposed.

---

## Process cleanup after test

After closing dev server with Ctrl+C:

```bash
ps aux | grep rollboard | grep -v grep || true
ps aux | grep vite | grep -v grep || true
ss -ltnp | grep -E ':8080|:8081|:8082|:8083|:8084|:5173' || true
```

Expected: no stale Rollboard dev/test processes.

---

## Test report format

When reporting browser verification, include:

```text
Browser:
Commit:
OS:
Result:
Bugs found:
Screenshots:
Console errors:
Network errors:
Remaining issues:
```

Do not mark unchecked items as passed.

---

## Edge Conditions / Branching Routes

- [x] Edge condition labels visible in editor (Safe Path, pay 2 gold)
- [x] Edge Inspector can edit condition type (manual_choice, pay_resource, dice_total_even, dice_total_odd, always)
- [x] dice_total_even works — rolled 2 → even_cell
- [x] dice_total_odd works — rolled odd → odd_cell → finish
- [x] manual_choice creates route_choice pending action with option buttons
- [x] pay_resource checks resource availability before showing option
- [x] pay_resource subtracts resource after choice (gold 5→3)
- [x] movement continues after route choice to selected cell
- [x] event log records route/movement/action events
- [x] browser E2E screenshots saved in artifacts/browser-smoke/

### Edge Condition Types Implemented

| Type | Description | Demo |
|---|---|---|
| `always` | Always available | All |
| `dice_total_even` | Available when dice total is even | Branching Demo |
| `dice_total_odd` | Available when dice total is odd | Branching Demo |
| `manual_choice` | Player chooses manually | Manual Branch Demo |
| `pay_resource` | Requires spending resource to use | Manual Branch Demo |
| `player_resource_at_least` | Available if player has ≥ N resource | (engine only) |
