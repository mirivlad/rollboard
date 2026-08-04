**English** · [Русский](README.ru.md)

# Rollboard

**A self-hostable platform for building and playing turn-based board games in the browser.**

Rollboard is a generic engine, not a Monopoly clone. A game is *data* — a board
graph plus a list of actions — so adding a new game means writing a definition,
never backend code. Build a board in the visual editor, publish an immutable
version, open a room, and play with other people in real time.

Go backend · Svelte 5 frontend · PostgreSQL · Redis · Docker

![Sign in](docs/screenshots/sign-in.png)

---

## Contents

- [What it does](#what-it-does)
- [Screenshots](#screenshots)
- [Quick start](#quick-start)
- [Deploying for real](#deploying-for-real)
- [Configuration](#configuration)
- [Translating](#translating)
- [How a game is defined](#how-a-game-is-defined)
- [Authoring guide](docs/AUTHORING.md)
- [Development](#development)
- [Current limitations](#current-limitations)
- [License](#license)

---

## What it does

- **Visual board editor** — place cells, draw directed edges, edit properties and actions.
- **Data-driven rules** — behaviour comes from `ActionDefinition` lists, not from code.
- **Directed-graph movement** — boards are graphs, so branching routes and one-way paths work.
- **Accounts and guests** — authors register; players can join a room without an account.
- **Private drafts, immutable published versions** — a room is pinned to a version, so a live game never changes underneath its players.
- **Real-time multiplayer rooms** — server-authoritative dice and movement over WebSocket, with room chat and host moderation.
- **Hotseat playtest** — pass one device around, with explicit turn screens.
- **Two languages, more without rebuilding** — see [Translating](#translating).
- **Light and dark themes**, keyboard accessible, works down to phone width.

The server is always authoritative. The browser sends intentions ("I want to
roll"), never results.

## Screenshots

**Author workspace** — private drafts and starting templates.

![Dashboard](docs/screenshots/dashboard.png)

**Board editor** — the board is a graph; edges carry conditions.

![Editor](docs/screenshots/editor.png)

**Action editor** — every action the engine can run, built from dropdowns. An
action can ask about the rest of the board ("cells of this type, owned by the
owner of this one") and compute amounts from the answer.

![Action editor](docs/screenshots/action-editor.png)

**Playtest** — dice, movement along the graph, and a choice produced entirely by the game's data.

![Playtest](docs/screenshots/playtest.png)

**Auction** — declining to buy puts the square in front of the whole table, and
bidding passes from player to player.

![Auction](docs/screenshots/auction.png)

**Multiplayer room** — roster, chat and server-authoritative play.

![Room](docs/screenshots/room.png)

**On a phone**, and with a visible keyboard focus ring.

<p>
  <img src="docs/screenshots/editor-mobile.png" alt="Editor on mobile" width="240">
  <img src="docs/screenshots/keyboard-focus.png" alt="Keyboard focus ring" width="480">
</p>

## Quick start

You need **Docker** with the Compose plugin. Nothing else.

```bash
git clone https://github.com/mirivlad/rollboard.git
cd rollboard
docker compose up --build
```

Open <http://localhost:8080>, create an account, and pick a template.

That single command builds the image and starts PostgreSQL, Redis and the
application. Migrations run automatically on startup.

### Running from source instead

For development you also need **Go 1.24+** and **Node.js 20+**:

```bash
cd frontend && npm install && cd ..
./scripts/dev.sh
```

This starts PostgreSQL and Redis in Docker, the Go backend on `:8080`, and the
Vite dev server on <http://localhost:5173>.

## Deploying for real

`docker compose up` is fine for trying Rollboard out, but it is not a public
deployment. For that:

### 1. Use the deployment stack

[`deploy/portainer-stack.yaml`](deploy/portainer-stack.yaml) pulls a published
image instead of building, and refuses to start without a database password and
an application origin.

```bash
export POSTGRES_PASSWORD='a long random string'
export ROLLBOARD_APP_ORIGIN='https://rollboard.example.com'
docker compose -f deploy/portainer-stack.yaml up -d
```

It also works as a Portainer stack: paste the file, set the same variables.

### 2. Put it behind TLS

Rollboard does not terminate TLS. Run it behind a reverse proxy and let that
handle certificates. The application only needs `ROLLBOARD_APP_ORIGIN` to match
the public URL, because that value drives both CORS and the WebSocket origin
check.

An example with Caddy:

```caddyfile
rollboard.example.com {
    reverse_proxy localhost:8080
}
```

**Set `ROLLBOARD_COOKIE_SECURE=true` whenever the site is served over HTTPS.**
Session cookies are otherwise sent over plain HTTP too.

**Set `ROLLBOARD_TRUSTED_PROXIES` to the proxy's address.** Every request
arrives from the proxy, so without this the sign-in rate limit counts all your
visitors as one person, and a few failed attempts from anybody lock out
everybody. The header is read *only* from addresses listed here — one from a
direct client is worthless, since anybody can write it.

```bash
# Caddy or nginx on the same host
ROLLBOARD_TRUSTED_PROXIES=127.0.0.1,::1
# nginx in another container on the default Docker bridge
ROLLBOARD_TRUSTED_PROXIES=172.16.0.0/12
```

With nginx, pass the address on:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header Host $host;
    # Live rooms are WebSockets.
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

### 3. Back up PostgreSQL

Everything durable lives in PostgreSQL: accounts, drafts, published versions,
rooms and the event journal. Redis carries only cross-process fan-out and can
be lost without data loss.

```bash
docker compose exec -T postgres pg_dump -U rollboard rollboard | gzip > rollboard-backup.sql.gz
```

To restore:

```bash
gunzip -c rollboard-backup.sql.gz | docker compose exec -T postgres psql -U rollboard rollboard
```

### 4. Upgrading

```bash
docker compose pull
docker compose up -d
```

Migrations are applied on startup inside a transaction, guarded by an advisory
lock, so several replicas starting at once is safe. **Take a backup first
anyway** — migrations are not reversible.

## Configuration

All configuration is environment variables. Copy [`.env.example`](.env.example)
as a starting point.

| Variable | Default | Description |
|----------|---------|-------------|
| `ROLLBOARD_ADDR` | `127.0.0.1:8080` | Listen address. The image sets `0.0.0.0:8080`. |
| `ROLLBOARD_DATABASE_URL` | `postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable` | PostgreSQL connection URL. |
| `ROLLBOARD_DATABASE_MAX_CONNS` | `20` | Maximum pooled connections **per replica**. |
| `ROLLBOARD_REDIS_URL` | `redis://127.0.0.1:6379/0` | Redis URL, used for cross-replica fan-out only. |
| `ROLLBOARD_APP_ORIGIN` | `http://127.0.0.1:5173` | Public origin. Drives CORS and the WebSocket origin check. **Must match your real URL.** |
| `ROLLBOARD_COOKIE_SECURE` | `false` | Set to `true` behind HTTPS. |
| `ROLLBOARD_SESSION_TTL` | `720h` | Session lifetime, as a Go duration. |
| `ROLLBOARD_AUTH_RATE_LIMIT` | `10` | Credential attempts per minute, per source IP, per replica. |
| `ROLLBOARD_TRUSTED_PROXIES` | unset | Addresses or CIDR blocks whose `X-Forwarded-For` is believed. **Set this behind a reverse proxy**, or all visitors share one rate-limit budget. |
| `ROLLBOARD_LOCALES_DIR` | unset (`/app/locales` in the image) | Translation catalogs. |
| `ROLLBOARD_UPLOADS_DIR` | unset (`/app/uploads` in the image) | Where author-uploaded images are stored. Leave unset to disable uploading. |
| `ROLLBOARD_UPLOAD_QUOTA_MB` | `50` | How much one account's images may total. |
| `ROLLBOARD_UPLOAD_TOTAL_MB` | `5000` | How much this deployment stores in all. |
| `ROLLBOARD_UPLOAD_RATE_LIMIT` | `30` | Uploads per minute, per account. |
| `ROLLBOARD_STATIC_DIR` | unset (`/app/frontend` in the image) | Built frontend to serve. Leave unset in development, where Vite serves it. |

Invalid values fail at startup with a clear message rather than at first use.

## Translating

The interface ships in **English** and **Russian**. Catalogs are plain JSON read
from disk at request time, so adding a language needs **no image rebuild and no
code change**:

```bash
cp locales/en.json locales/de.json
```

Translate the values, then restart the application. The language appears in the
switcher by itself. Untranslated keys fall back to English, so a partial
translation is useful immediately.

Full details, including plural forms and how to translate server error
messages, are in **[docs/I18N.md](docs/I18N.md)**.

## How a game is defined

A game is a board (cells and directed edges) plus rules (dice, resources, cell
types). Every cell can carry `onLand` and `onPass` action lists that the engine
executes generically.

```json
{
  "type": "if_cell_unowned",
  "then": [{
    "type": "offer_choice",
    "title": "Buy this property for 100?",
    "options": [
      { "id": "buy", "title": "Buy (100)", "then": [
        { "type": "lose_resource", "resource": "money", "amount": 100 },
        { "type": "set_cell_owner", "target": "current" }
      ]},
      { "id": "skip", "title": "Don't buy", "then": [] }
    ]
  }],
  "else": [{
    "type": "if_cell_owned_by_other",
    "then": [{ "type": "transfer_resource", "resource": "money", "amount": 20, "target": "owner" }]
  }]
}
```

That snippet is the whole of "property purchase and rent". There is no property
code in the engine.

**Action types:** `gain_resource`, `lose_resource`, `transfer_resource`,
`set_cell_owner`, `set_cell_level`, `set_cell_mortgaged`, `offer_choice`,
`random_branch`, `start_auction`, `if_cell_unowned`, `if_cell_owned_by_current`,
`if_cell_owned_by_other`, `if_cell_level_ge`, `if_cell_mortgaged`,
`if_cells_ge`, `for_each_cell`, `if_resource_ge`, `if_stat_ge`, `if_has_item`,
`grant_item`, `remove_item`, `equip_item`, `unequip_slot`, `use_item`,
`reveal_cells`, `move_player_to`, `skip_turns`, `finish_game`,
`eliminate_player`, `log_message`.

Every one of them is editable in the visual editor — the action list is
generated from a schema, and a test fails the build if the engine gains an
action the editor cannot reach. Amounts can be **computed** rather than fixed
("this cell's damage, minus my defence, at least zero"), which is what makes
armour worth wearing.

Actions can also ask about **other cells** — "how many stations does the owner
of this one hold?", "does one player own every square in this colour group?" —
and put a square up for **auction**, where every player bids in turn. Both are
built from dropdowns, and both are explained with worked examples in
**[docs/AUTHORING.md](docs/AUTHORING.md)**.

Beyond a board and resources, a definition can also declare **items** with
equipment slots and stat bonuses, **levels** with experience thresholds and
spendable points, **hidden cells** that stay face down until explored, and
**free movement**, where a roll is a budget spent on any square in reach rather
than a fixed path.

Two bundled templates show the range:

- **Monopoly** — 40 squares of pure data: buying, rent that scales with
  buildings, colour groups that double the rent once one player owns all of
  them, station rent that multiplies by the owner's holdings, auctions for
  squares nobody buys, mortgaging, jail, chance cards and bankruptcy.
- **Dungeon Crawl** — a 6×6 map you explore square by square, with stats,
  levels, loot you equip, traps, enemies whose difficulty is checked against
  your effective attack, and a dragon to kill.

Neither has a line of game-specific code in the engine.

**Edge conditions:** `always`, `dice_total_even`, `dice_total_odd`,
`dice_total_in`, `manual_choice`, `pay_resource`, `player_resource_at_least`.

The engine model, the session flow and the branching rules are described in
**[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

## Development

| Command | What it does |
|---------|--------------|
| `make dev` | PostgreSQL, Redis, backend and frontend together |
| `make test` | Full Go suite **with integration tests against real PostgreSQL and Redis** |
| `make test-unit` | Unit tests only, no Docker needed |
| `make check` | gofmt, vet, the Go suite, svelte-check, vitest, frontend build, demo validation |
| `make smoke` | End-to-end API smoke test |
| `make fmt` | gofmt the backend |
| `make build` | Production binaries |
| `./scripts/ui-shots.sh` | Drive a real browser and regenerate the screenshots |

`make test` requires Docker, and refuses to run against anything but the
`rollboard_test` database, because the integration tests truncate tables.

CI runs the same checks on every push and fails if any test *skips* — a suite
that silently skips its integration tests reports success while proving
nothing.

Project conventions are in [AGENTS.md](AGENTS.md); contribution guidance is in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Current limitations

Stated plainly, so you can judge whether Rollboard fits before deploying it:

- **No public game discovery.** Rooms are shared by ID or invite link; there is no browsable catalogue.
- **No presence indicator.** You cannot see who is currently connected, only who is a member.
- **Rate limiting is per replica.** Behind N replicas the effective ceiling is N times the configured limit. Behind a proxy it also needs `ROLLBOARD_TRUSTED_PROXIES`, or every visitor shares one budget.
- **No bots.** Every player must be a human.
- **No undo.** The event journal records what happened but cannot roll it back.
- **Bidding is stepped, not free-form.** An auction offers a few raises the server generated; a player cannot type an arbitrary amount.
- **No table lookups.** Tiered values are written as descending `if_*_ge` chains, which works but reads long.
- **Journal retention is unbounded.** Pruning policy is not implemented; busy deployments will want one.
- **No load testing has been done.** Treat capacity claims as unproven.
- **Uploads are images only**, at most 2 MB, with the type sniffed from the bytes rather than trusted from the filename. SVG is refused because it can carry script. Quotas bound them; images no longer used by any game are not collected automatically — authors delete them.
- **No OAuth**, and no author-supplied code of any kind.
- **Mini-games are reserved but not runnable.** `launch_minigame` is a typed, versioned placeholder that publication deliberately rejects, so untrusted code cannot execute in the application process.

The current state, including exactly what has been verified and how, is tracked
in [docs/CURRENT_STATE.md](docs/CURRENT_STATE.md).

## Documentation

| Document | Русский |
|----------|---------|
| [Authoring guide](docs/AUTHORING.md) — building games from dropdowns | [есть](docs/ru/AUTHORING.md) |
| [Architecture](docs/ARCHITECTURE.md) — engine model, session flow, boundaries | [есть](docs/ru/ARCHITECTURE.md) |
| [Translating](docs/I18N.md) — adding a language without rebuilding | [есть](docs/ru/I18N.md) |
| [Current state](docs/CURRENT_STATE.md) — what is verified, and how | [есть](docs/ru/CURRENT_STATE.md) |
| [Roadmap](docs/ROADMAP.md) | [есть](docs/ru/ROADMAP.md) |
| [Manual playtest](docs/manual-playtest.md), [checklist](docs/PLAYTEST_CHECKLIST.md) — release procedures | English only |

## Troubleshooting

**Port already in use.** Change `ROLLBOARD_ADDR`, or stop whatever holds the
port:

```bash
lsof -ti :8080 | xargs kill -9
```

**WebSocket fails to connect, or requests are blocked by CORS.**
`ROLLBOARD_APP_ORIGIN` does not match the URL you are actually using. It drives
both checks.

**Sign-in appears to succeed but you are immediately signed out.**
`ROLLBOARD_COOKIE_SECURE=true` while serving over plain HTTP; the browser
discards the cookie.

**Database connection refused.** Check the service is healthy:

```bash
docker compose ps postgres
docker compose logs postgres
```

## License

Copyright (C) 2026 mirivlad

Rollboard is free software: you can redistribute it and modify it under the
terms of the **GNU Affero General Public License, version 3** as published by
the Free Software Foundation. See [LICENSE](LICENSE) for the full text.

The AGPL is chosen deliberately over the plain GPL because Rollboard is a
network service. If you modify Rollboard and let other people use it over a
network — including as a hosted or commercial offering — you must offer those
users the source of your modified version under the same license. Running it
privately or self-hosting it for your own players carries no such obligation.
