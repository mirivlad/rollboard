# Rollboard — self-hostable turn-based board-game platform

A browser-based platform for creating and running generic turn-based board games.
Backend in Go, frontend in Svelte + Vite + TypeScript, durable storage in PostgreSQL.

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
  - **Branching Demo**: dice-based even/odd branching routes
  - **Manual Branch Demo**: player-chosen paths with resource costs
- Game persistence via PostgreSQL
- Account registration, guest entry, opaque cookie sessions and CSRF protection
- Private drafts and immutable published game versions
- Multiplayer rooms pinned to a published version, guest joins and host moderation
- Server-authoritative room start/roll/action processing over authenticated WebSocket
- Persisted room-only chat with mute enforcement
- Validation of game definitions
- Event log for all game actions
- Player elimination (bankruptcy)
- Configurable start pass-through bonus
- **Edge conditions**: dice_total_even, dice_total_odd, manual_choice, pay_resource, player_resource_at_least
- **Branching routes**: route_choice pending action for player-selected paths

## Requirements

- **Go** 1.22+ ([download](https://go.dev/dl/))
- **Node.js** 20+ ([download](https://nodejs.org/))
- **npm** (ships with Node.js)
- Docker Compose plugin for local PostgreSQL and Redis services.

## Quick Start

```bash
# Clone the repo (once available)
git clone <repo-url>
cd rollboard

# Install frontend dependencies
cd frontend && npm install && cd ..

# Start local PostgreSQL/Redis plus backend and frontend
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
| `make smoke`   | Run PostgreSQL-backed smoke tests |
| `make clean`   | Remove build artifacts and data    |
| `make build`   | Build production binaries          |

## Environment Variables

| Variable            | Default                  | Description            |
|---------------------|--------------------------|------------------------|
| `ROLLBOARD_ADDR`    | `127.0.0.1:8080`         | Backend listen address |
| `ROLLBOARD_DATABASE_URL` | `postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable` | PostgreSQL connection URL |
| `ROLLBOARD_REDIS_URL` | `redis://127.0.0.1:6379/0` | Redis connection URL |

Set these in a `.env` file (not committed) or export them in your shell:

```env
ROLLBOARD_ADDR=127.0.0.1:8080
ROLLBOARD_DATABASE_URL=postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable
ROLLBOARD_REDIS_URL=redis://127.0.0.1:6379/0
```

## How to Use

### Open the UI

1. Run `./scripts/dev.sh`
2. Open http://localhost:5173

### Create and publish a game

1. Create an account — guests can join rooms but cannot publish games.
2. In **Your games**, choose **Blank board**, **Mini-Monopoly**, or **Dungeon Race**.
3. The default guided step sets the title and dice; select **Advanced studio** before choosing a template to open the full editor immediately.
4. Edit the definition, save the private draft, then select **Publish**.
5. Open **Rooms**, select the published version, and create a room.
6. Share the displayed room ID with other players. They may join as guests.

### Run an online room

1. The account that created the room starts it after at least two players join.
2. Only the current player sees **Roll dice**; the browser sends an intention and the server returns the result.
3. If the engine creates a choice, only the player named by the server can choose an option.
4. Use the room chat for the current room. The host can mute or remove members through the API; UI controls for moderation are planned.

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

## Current limitations

- The author dashboard reloads private drafts and owned published versions. Public catalog discovery/search and shareable invite links are still planned.
- The realtime hub is single-process. Redis is provisioned by Docker/Portainer, but cross-replica fan-out, presence and command idempotency are not implemented yet.
- Browser verification covers two simultaneous clients creating/joining a room, live roster updates, starting, rolling, resolving a purchase choice and room chat; wider responsive-device coverage is still required before release.
- No bot players — all players must be human.
- No image file uploads — image URLs only
- No undo / rollback
- No property upgrades, mortgaging, or trading
- No complex dice rules — single dice rule per game
- No OAuth or arbitrary author-supplied code.

## Project Structure

```
├── Makefile
├── backend/
│   ├── cmd/server/main.go            # Entry point
│   └── internal/
│       ├── game/                      # Generic definition, runtime and validation
│       ├── httpapi/                   # HTTP and WebSocket handlers
│       ├── identity/                  # Accounts, guests and sessions
│       ├── catalog/                   # Drafts and immutable versions
│       ├── room/                      # Rooms, membership, chat and moderation
│       ├── realtime/                  # Sequenced WebSocket room hub
│       └── storage/postgres/          # PostgreSQL migrations and storage
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
│   │       ├── PlaytestPanel.svelte   # Hotseat playtest UI
│   │       ├── RoomLobby.svelte       # Create/join room UI
│   │       └── RoomPlay.svelte        # Realtime room and chat UI
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
| POST | `/api/auth/register` | Register an account and create a session |
| POST | `/api/auth/guest` | Create a guest session |
| GET | `/api/games` | List the authenticated author's drafts |
| GET | `/api/games/versions` | List the authenticated author's published versions |
| POST | `/api/rooms` | Create a room pinned to a published version |
| GET | `/api/rooms/{id}` | Get room state (room members only) |
| POST | `/api/rooms/{id}/join` | Join a room |
| GET | `/api/rooms/{id}/ws` | Authenticated realtime room connection |

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

### Database connection issue

Rollboard requires PostgreSQL. Check that the local Compose service is healthy:

```bash
docker compose ps postgres
docker compose logs postgres
```

For a remote database, set `ROLLBOARD_DATABASE_URL` to a valid PostgreSQL URL and restart the backend.

### node_modules missing

If the frontend build fails with module-not-found errors:

```bash
cd frontend && npm install && cd ..
```
