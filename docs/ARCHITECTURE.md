# Rollboard Architecture

## Overview

Rollboard is a generic engine for route-based board games.

- **Backend**: Go HTTP server with SQLite persistence
- **Frontend**: Svelte 5 + TypeScript + Vite
- **Communication**: REST JSON API over HTTP

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
    ├── httpapi/
    │   └── handler.go          — REST handlers: /api/health, /api/games, /api/sessions
    └── storage/sqlite/
        └── store.go            — SQLite CRUD for games and sessions
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
  → Backend validates + stores in SQLite

User starts playtest → POST /api/games/:id/playtest
  → StartSession creates GameSession → stored
  → PlaytestPanel renders live board

User clicks Roll → POST /api/sessions/:id/roll
  → RollDice (server RNG) → MoveCurrentPlayer → execute cell actions
  → Frontend shows dice result → animates token → shows pending action or turn done
```

## Key Rules

- **Board geometry**: grid-based. `width = cols * cellSize`, `height = rows * cellSize`.
  All cell coordinates must align to cellSize grid. Edges connect cell centers.
- **No game-specific runtime code**: all cell behavior is driven by ActionDefinition data.
- **Single coordinate system**: playtest and editor use the same board geometry.
