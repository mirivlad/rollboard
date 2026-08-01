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
   - `set_cell_owner`
   - `offer_choice` (branching with options)
   - `if_cell_unowned`, `if_cell_owned_by_current`, `if_cell_owned_by_other`, `if_resource_ge`
   - `finish_game`, `log_message`
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
locks and persists the room snapshot before the in-process hub broadcasts a
monotonic sequence. A room references `game_versions.id`, never a mutable draft.

## Implementation boundaries

Redis is included in the Docker/Portainer deployment but is not yet used for
cross-process fan-out or presence. Invite links, command idempotency receipts and
event replay are planned; this document does not claim them as implemented.

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
