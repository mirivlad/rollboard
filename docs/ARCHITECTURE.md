# Rollboard Architecture

## Overview

Rollboard is a generic engine for route-based board games.

- **Backend**: Go HTTP server with PostgreSQL persistence
- **Frontend**: Svelte 5 + TypeScript + Vite
- **Communication**: REST JSON API plus authenticated WebSocket room events

## Backend Structure

```
backend/
├── cmd/server/main.go        — entry point, flag parsing, middleware
└── internal/
    ├── game/
    │   ├── definition.go      — types: GameDefinition, Board, CellDefinition,
    │   │                         EdgeDefinition, ActionDefinition, RuleSet
    │   ├── session.go          — types: GameSession, GameState, PlayerState,
    │   │                         CellState, PendingAction
    │   ├── engine.go           — runtime: StartSession, RollDice, MoveCurrentPlayer,
    │   │                         executeActions, ResolvePendingAction, advanceTurn
    │   ├── validation.go       — ValidateDefinition: geometry, graph, resources
    │   ├── engine_test.go      — tests for engine + demo validation
    │   └── validation_test.go  — tests for validation logic
    ├── httpapi/                — REST and WebSocket upgrade handlers
    ├── identity/               — accounts, guests and opaque sessions
    ├── catalog/                — drafts and immutable published versions
    ├── room/                   — room, membership, moderation and chat persistence
    ├── realtime/               — in-process sequenced room hub
    └── storage/postgres/
        └── store.go            — PostgreSQL CRUD for games and sessions
```

### Engine Model

The engine is **generic** — no game-specific code.

**Core principles:**

1. **GameDefinition** describes a board (grid of cells, directed edges) and rules (dice, resources, cell types).
2. **GameSession** is created from a GameDefinition + player list. It holds mutable state.
3. **Actions** (ActionDefinition) are data-driven — they describe what happens when a player lands on a cell:
   - `gain_resource`, `lose_resource`, `transfer_resource`
   - `set_cell_owner`, `set_cell_level`, `set_cell_mortgaged`
   - `offer_choice` (player-chosen branching), `random_branch` (server-chosen branching)
   - `if_cell_unowned`, `if_cell_owned_by_current`, `if_cell_owned_by_other`,
     `if_cell_level_ge`, `if_cell_mortgaged`, `if_resource_ge`
   - `move_player_to`, `skip_turns`, `reveal_cells`
   - `grant_item`, `remove_item`, `equip_item`, `unequip_slot`, `use_item`
   - `if_has_item`, `if_stat_ge`
   - `finish_game`, `eliminate_player`, `log_message`

4. **Items** are the definition's catalogue of things a player carries. A
   resource is a number; an item is a named thing with its own effects, which
   is what a sword has to be and a counter cannot. Bonuses apply only while an
   item is equipped and are never folded into the stored resource, so taking
   equipment off is not lossy. `if_stat_ge` reads the effective value;
   `if_resource_ge` reads the raw one.
5. **Hidden cells** stay face down until landed on or scouted. The stripping
   happens when a session is serialised for the wire, and only for the wire:
   storage keeps the full definition, because a face-down cell persisted in its
   stripped form would come back empty.

   None of these know about any particular game. `set_cell_level` is houses in a
   property game and fortification in a war game; `skip_turns` is a jail
   sentence and a stun effect.
4. **Server is authoritative** — dice are rolled server-side, movement is computed server-side.
5. Frontend only sends intentions ("I want to roll"), never results.

### Session Flow

```
StartSession
  → Turn intro (UI only)
  → RollDice → MoveCurrentPlayer → executeActions(OnLand)
    → pending action? → ResolvePendingAction → advanceTurn
    → no pending action? → advanceTurn
  → Turn done
  → next player turn intro
```

## Frontend Structure

```
frontend/src/
├── App.svelte                    — main app: routing (editor ↔ playtest)
├── lib/
│   ├── types.ts                  — shared TypeScript types mirroring backend
│   ├── defaults.ts               — default game + demo definitions
│   └── api.ts                    — REST API client
└── components/
    ├── BoardEditor.svelte         — editor with toolbar (cols/rows/dice/cells/edges)
    ├── BoardCanvas.svelte         — editor canvas: grid, cells, edges, drag-n-drop
    ├── BoardView.svelte           — playtest board: grid, cells, edges, tokens
    ├── CellView.svelte            — single cell renderer
    ├── CellInspector.svelte       — cell/tile property editor panel
    ├── EdgeLayer.svelte           — SVG layer for arrow edges
    ├── TokenLayer.svelte          — player token positions on cells
    └── PlaytestPanel.svelte       — full playtest UI: setup → turn → roll → action → results
```

### Data Flow

```
User edits board → BoardEditor mutates GameDefinition.board
  → Save → POST /api/games/:id (serialized as JSON)
  → Backend validates + stores the author-owned draft in PostgreSQL

User starts playtest → POST /api/games/:id/playtest
  → StartSession creates GameSession → stored
  → PlaytestPanel renders live board

User clicks Roll → POST /api/sessions/:id/roll
  → RollDice (server RNG) → MoveCurrentPlayer → execute cell actions
  → Frontend shows dice result → animates token → shows pending action or turn done
```

Online rooms use the same generic engine, but their client sends a WebSocket
intent (`start`, `roll`, `action`, `chat`) rather than a rule result. PostgreSQL
locks and persists the room snapshot before the hub publishes a monotonic
sequence. Redis Pub/Sub distributes that accepted snapshot, transition or chat
event to the local hubs of every application replica. A room references
`game_versions.id`, never a mutable draft. Redis is not an authority: PostgreSQL
stores the matching wire envelope and a per-actor client-command UUID receipt in
the same transaction. Retrying `start`, `roll`, `action` or `chat` returns that
stored result without re-executing the command. On reconnect, the hub replays up
to 64 contiguous journal events newer than `since`; any gap or longer range
safely produces the latest authenticated PostgreSQL snapshot instead.

## Cell queries and auctions

Building the full 40-square Monopoly template mapped the edges of the action
set, and two of the gaps it found are now filled. Both are general
capabilities available to every author from the editor, not special cases for
the bundled templates — see **[AUTHORING.md](AUTHORING.md)** for worked
examples.

**Cell queries.** A `CellQuery` selects cells by type, by a field the author
defined, and by owner (`none`, `current`, `cellOwner`, `other`). `if_cells_ge`
branches on how many match, `for_each_cell` runs a list once per match with
that cell as the context, and a formula term of kind `cells` turns a count into
a number, which is what lets rent multiply by holdings. `sameAsCell` compares a
field against the cell being resolved, so "the rest of this colour group" is one
query that works for every group rather than one query per colour.

The owner filter distinguishes the visitor from the landlord deliberately.
Rent scales with what the *owner* holds (`cellOwner`), not with what the player
standing on the square holds, and the two are easy to confuse when writing the
rule out in prose.

**Auctions.** `start_auction` runs an open ascending auction: bidding passes
round the table, and the pending action belongs to one bidder at a time. That
shape was chosen because a `PendingAction` addresses exactly one player, so a
simultaneous sealed bid has nowhere to live — and because bidding in turn makes
the auction resumable. It lives in the session state, so a player who reloads
mid-auction gets it back and a room replaying its journal replays the bidding.

The server generates the bid options (a few raises and "pass") and accepts only
an option it offered, re-checking the balance when the answer arrives. The
winner pays the bank and the author's `then` list runs *as the winner*, so
"give this cell to the current player" hands it to whoever won rather than to
whoever landed on the square.

Actions nest three ways — branches, a teleport that runs the destination's
`onLand`, and a query that runs a list per matching cell — so execution is
bounded at 32 levels. A definition that loops back on itself stops with an
`action_depth_exceeded` event instead of taking the process down.

## What the action language cannot express yet

- **No table lookups.** Tiered values are written as descending `if_*_ge`
  chains, which works but is verbose.
- **No free-form bids.** An auction offers stepped amounts the server
  generated, not a number the player types.
- **Payments floor at zero rather than failing.** `lose_resource` and
  `transfer_resource` clamp, so a definition that wants bankruptcy must guard
  the payment with `if_resource_ge` and call `eliminate_player` in the else
  branch. Nothing in the engine decides that for you, which is deliberate: what
  counts as ruin belongs to the game, not to the runtime.

## Implementation boundaries

Redis is used only for cross-process fan-out. PostgreSQL is the authority for
room state, event journal and command receipts. Presence and rate limiting are
not implemented yet.

### Mini-game extension boundary

`launch_minigame` is a typed, reserved action with an immutable module ID and
version. The current validator rejects it because no runner is wired. A future
runner must execute outside the Rollboard process, receive only an explicit
`MiniGameInvocation`, return schema-validated output, and let the room service
apply that result inside its normal authoritative transaction. Author-supplied
mini-game code must never access PostgreSQL or mutate a session directly.

## Key Rules

- **Board geometry**: grid-based. `width = cols * cellSize`, `height = rows * cellSize`.
  All cell coordinates must align to cellSize grid. Edges connect cell centers.
- **No game-specific runtime code**: all cell behavior is driven by ActionDefinition data.
- **Single coordinate system**: playtest and editor use the same board geometry.

## Edge Conditions

Edges can have conditions that determine whether they are available for traversal.

### Condition Types

| Type | Fields | Description |
|------|--------|-------------|
| `always` | — | Always available (default) |
| `dice_total_even` | — | Available when dice total is even |
| `dice_total_odd` | — | Available when dice total is odd |
| `manual_choice` | `label` | Player manually chooses this path; triggers `route_choice` pending action |
| `pay_resource` | `resource`, `amount`, `label` | Requires spending a resource to use; triggers `route_choice` pending action |
| `player_resource_at_least` | `resource`, `amount` | Available only if player has ≥ N of the resource |

### Route Choice Flow

When movement reaches a cell with multiple available outgoing edges that have `manual_choice` or `pay_resource` conditions:

1. Movement stops at the current cell
2. Engine creates a `PendingAction` with `type: "route_choice"` and `options` array
3. Frontend renders option buttons (one per available edge)
4. Player clicks a button → frontend sends `POST /api/sessions/{id}/actions` with the chosen edge ID
5. Engine validates the choice, subtracts resources if `pay_resource`, and continues movement
6. If `pay_resource` and insufficient resources, returns error

### pay_resource vs player_resource_at_least

- `player_resource_at_least`: **gate** — edge is only available if player has enough resource. No cost to use.
- `pay_resource`: **cost** — edge is always shown, but choosing it subtracts the resource amount.

### MoveContext

The `MoveContext` is passed to `evaluateCondition()` and provides:
- `Dice`: the dice rolls for this move
- `Total`: sum of dice rolls
- `Player`: the current player state (for resource checks)
- `Step`: current step number in multi-step movement

### PendingMovement

When a `route_choice` interrupts movement mid-path, the engine saves a `PendingMovement`:
- `PlayerID`: whose movement was interrupted
- `CurrentCellID`: cell where movement stopped
- `RemainingSteps`: steps left to take after choice
- `Dice`, `Total`: original dice roll
- `PathSoFar`: cells already traversed

This allows the engine to resume movement from the chosen cell with remaining steps.
