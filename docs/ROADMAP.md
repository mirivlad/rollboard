# Rollboard roadmap

## Product direction

Rollboard is evolving from a local SQLite prototype into a self-hostable platform
for authoring and running generic turn-based board games.

The approved platform design is in
`docs/superpowers/specs/2026-08-02-rollboard-platform-design.md`.

## Current milestone — platform replacement

Deliver the approved first release as a Docker/Portainer-ready stack:

1. PostgreSQL persistence, migrations, configuration, and stack packaging.
2. Account and guest identity, versioned games, draft/publication workflow.
3. Authoritative multiplayer rooms, reconnect, chat, moderation, and hotseat.
4. Guided authoring wizard, templates, advanced studio, lobby, and play UI.
5. Integration, browser, load, and deployment verification.

## Planned extension boundaries

- Versioned, sandboxed mini-game modules through `launch_minigame`.
- OAuth providers for hosted deployments.
- Import/export after stable definition schema and validation are established.
- Bot players and additional turn-based generic action primitives.

These items must preserve server authority and the generic runtime; they are not
permission to add game-specific runtime branches.
