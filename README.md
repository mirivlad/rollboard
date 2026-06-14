# Rollboard — Roll-and-Move Board Game Engine

A browser-based engine for creating and playtesting roll-and-move board games.
Backend in Go, frontend in Svelte + Vite + TypeScript, storage in SQLite.

Game logic is defined entirely through data (ActionDefinition lists), not hardcoded in the runtime.
This makes the engine generic — adding a new game type requires only a JSON definition, not backend code changes.

## What's Implemented

- Visual board editor (add/delete/move cells, draw edges, edit properties)
- Directed-graph movement (cells are nodes, edges define paths)
- Generic ActionDefinition runtime (all game logic via data)
- Hotseat mode (pass-device play with explicit turn screens)
- Universal resource display (all keys from player.resources)
- Universal pending action UI (any option list rendered generically)
- Two built-in demos:
  - **Mini-Monopoly**: property purchase, rent collection, bonus/penalty cells
  - **Dungeon Race**: health, gold, keys, traps, treasure, heal, finish line
- Game persistence via SQLite
- Validation of game definitions
- Event log for all game actions
- Player elimination (bankruptcy)
- Configurable start pass-through bonus

## Requirements

- **Go** 1.22+ ([download](https://go.dev/dl/))
- **Node.js** 20+ ([download](https://nodejs.org/))
- **npm** (ships with Node.js)
- SQLite — no separate install needed. The driver (`github.com/mattn/go-sqlite3`) requires CGo and a working C compiler (gcc or mingw).

## Quick Start

```bash
# Clone the repo (once available)
git clone <repo-url>
cd rollboard

# Install frontend dependencies
cd frontend && npm install && cd ..

# Start both backend and frontend
./scripts/dev.sh
```

Or use Make:

```bash
make dev       # Start both backend and frontend
```

Open http://localhost:5173 in a browser.

## Make Commands

| Command        | Description                        |
|----------------|------------------------------------|
| `make dev`     | Start backend + frontend together  |
| `make backend` | Start backend server only          |
| `make frontend`| Start frontend dev server only     |
| `make test`    | Run backend tests                  |
| `make check`   | Run backend tests + frontend build |
| `make smoke`   | Run smoke tests (start backend, hit API, stop) |
| `make clean`   | Remove build artifacts and data    |
| `make build`   | Build production binaries          |

## Environment Variables

| Variable            | Default                  | Description            |
|---------------------|--------------------------|------------------------|
| `ROLLBOARD_ADDR`    | `127.0.0.1:8080`         | Backend listen address |
| `ROLLBOARD_DB_PATH` | `./data/rollboard.db`    | SQLite database path   |

Set these in a `.env` file (not committed) or export them in your shell:

```env
ROLLBOARD_ADDR=127.0.0.1:8080
ROLLBOARD_DB_PATH=./data/rollboard.db
```

## How to Use

### Open the UI

1. Run `./scripts/dev.sh`
2. Open http://localhost:5173

### Create a Mini-Monopoly Demo

1. In the sidebar, click **"Demo Mini-Monopoly"**
2. Inspect the board: cells, edges, cell properties
3. Click **Save** (hotkey: Ctrl+S)
4. Click **Validate** to confirm the definition is correct

### Create a Dungeon Race Demo

1. Click **"Demo Dungeon Race"**
2. Inspect the board, resources (health, gold, keys), and cell actions
3. Click **Save**
4. Click **Validate**

### Run a Hotseat Playtest

1. After creating and saving a game, click **Playtest**
2. Choose the number of players (2–6) and customize names/colors
3. Click **Start Playtest**
4. Turn intro screen shows whose turn it is → click **Start Turn**
5. Dice roll happens automatically → movement follows the graph
6. If an action is required (e.g., buy property), buttons appear
7. After the turn, click **Pass to Next Player**
8. Repeat until the game ends

## ActionDefinition System

Every cell can define `onLand` and `onPass` action lists — sequences of `ActionDefinition` objects that the engine executes when a player lands on or passes that cell.

Actions can:

- Modify resources (gain, lose, transfer)
- Offer choices via `offer_choice` with `options` containing follow-up `then` actions
- Branch on cell ownership (`if_cell_unowned`, `if_cell_owned_by_current`, `if_cell_owned_by_other`)
- Branch on resource values (`if_resource_ge`)
- End the game (`finish_game`)
- Log messages (`log_message`)

### Supported Action Types

| Type | Fields | Description |
|------|--------|-------------|
| `gain_resource` | `resource`, `amount`/`amountField` | Add to player's resource |
| `lose_resource` | `resource`, `amount`/`amountField` | Subtract from player's resource |
| `transfer_resource` | `resource`, `amount`/`amountField`, `target` (`"owner"` or `"current"`) | Transfer resource between players |
| `offer_choice` | `title`, `options` (array of `{id, title, then}`) | Present player with a choice; `then` executes on selection |
| `set_cell_owner` | `target` (`"current"`, `"owner"`, or a player ID) | Set ownership of the current cell |
| `if_cell_unowned` | `then`, `else` | Branch if current cell has no owner |
| `if_cell_owned_by_current` | `then`, `else` | Branch if current player owns the cell |
| `if_cell_owned_by_other` | `then`, `else` | Branch if another player owns the cell |
| `if_resource_ge` | `resource`, `amount`, `then`, `else` | Branch if player has ≥ amount of resource |
| `finish_game` | — | End the game; current player wins |
| `log_message` | `title` (message) | Add a log entry without modifying state |

### Example: Property Buy/Rent

```json
{
  "type": "if_cell_unowned",
  "then": [
    {
      "type": "offer_choice",
      "title": "Buy this property for $100?",
      "options": [
        {
          "id": "buy",
          "title": "Buy ($100)",
          "then": [
            { "type": "lose_resource", "resource": "money", "amount": 100 },
            { "type": "set_cell_owner", "target": "current" }
          ]
        },
        {
          "id": "skip",
          "title": "Don't Buy",
          "then": []
        }
      ]
    }
  ],
  "else": [
    {
      "type": "if_cell_owned_by_other",
      "then": [
        { "type": "transfer_resource", "resource": "money", "amount": 20, "target": "owner" }
      ]
    }
  ]
}
```

## Limitations (MVP)

- No authentication — anyone with access to the backend can read/write games
- No WebSocket / real-time — uses HTTP polling for state refresh
- No bot players — all players must be human (hotseat)
- No image file uploads — image URLs only
- Edge conditions are basic — only `always` type implemented
- No undo / rollback
- No property upgrades, mortgaging, or trading
- No complex dice rules — single dice rule per game
- No packaging — requires Go + Node.js to run

## Project Structure

```
├── Makefile
├── backend/
│   ├── cmd/server/main.go            # Entry point
│   ├── internal/game/
│   │   ├── definition.go             # GameDefinition, Board, Cell, Edge types
│   │   ├── session.go                # GameSession, PlayerState, PendingAction
│   │   ├── engine.go                 # Generic action executor (no game-specific logic)
│   │   ├── validation.go             # GameDefinition validation
│   │   └── engine_test.go            # Backend tests
│   ├── internal/httpapi/
│   │   └── handler.go                # HTTP API handlers
│   └── internal/storage/sqlite/
│       └── store.go                  # SQLite CRUD operations
├── frontend/
│   ├── src/
│   │   ├── App.svelte                # Main app shell + navigation
│   │   ├── lib/
│   │   │   ├── types.ts              # TypeScript interfaces
│   │   │   ├── api.ts                # API client
│   │   │   └── defaults.ts           # Demo definitions + default game
│   │   └── components/
│   │       ├── BoardEditor.svelte     # Editor layout + toolbar
│   │       ├── BoardCanvas.svelte     # Canvas with cells, edges, players
│   │       ├── CellView.svelte        # Single cell rendering
│   │       ├── EdgeLayer.svelte       # SVG edge arrows
│   │       ├── CellInspector.svelte   # Right panel for cell editing
│   │       └── PlaytestPanel.svelte   # Hotseat playtest UI
│   ├── package.json
│   ├── vite.config.ts
│   └── index.html
├── scripts/
│   ├── dev.sh                        # Start backend + frontend
│   ├── backend.sh                    # Start backend only
│   ├── frontend.sh                   # Start frontend only
│   ├── check.sh                      # Run tests + build
│   └── smoke.sh                      # Backend smoke test
└── .env.example
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/games` | List saved games |
| POST | `/api/games` | Create a new game |
| GET | `/api/games/{id}` | Get game definition |
| PUT | `/api/games/{id}` | Update game definition |
| POST | `/api/games/{id}/validate` | Validate game definition |
| POST | `/api/games/{id}/playtest` | Start hotseat playtest session (`{players: [{name, color}]}`) |
| GET | `/api/sessions/{id}` | Get current session state |
| POST | `/api/sessions/{id}/roll` | Roll dice and move current player |
| POST | `/api/sessions/{id}/actions` | Resolve pending action (`{actionId}`) |

## Troubleshooting

### Backend port busy

```
Error: listen tcp 127.0.0.1:8080: bind: address already in use
```

Kill the old process or change `ROLLBOARD_ADDR`:

```bash
lsof -ti :8080 | xargs kill -9
export ROLLBOARD_ADDR=127.0.0.1:8081
./scripts/backend.sh
```

### Frontend port busy

```
Error: listen tcp 127.0.0.1:5173: bind: address already in use
```

Kill the old process:

```bash
lsof -ti :5173 | xargs kill -9
```

Or edit `frontend/vite.config.ts` to change the port.

### Database path issue

If the backend fails to start with a SQLite error, ensure the directory for the database exists:

```bash
mkdir -p $(dirname "$ROLLBOARD_DB_PATH")
```

Or use the default:

```bash
export ROLLBOARD_DB_PATH=./data/rollboard.db
mkdir -p ./data
```

### node_modules missing

If the frontend build fails with module-not-found errors:

```bash
cd frontend && npm install && cd ..
```
