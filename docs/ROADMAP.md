# Rollboard roadmap

## Product direction

Rollboard has moved from a local SQLite prototype to a self-hostable platform
for authoring and running generic turn-based board games. PostgreSQL is the only
supported runtime store.

The approved platform design is in
`docs/superpowers/specs/2026-08-02-rollboard-platform-design.md`.

## Delivered

1. PostgreSQL persistence, migrations, configuration, and stack packaging.
2. Account and guest identity, versioned games, draft/publication workflow.
3. Authoritative multiplayer rooms, reconnect, chat, moderation, and hotseat.
4. Guided authoring wizard, templates, advanced studio, lobby, and play UI.
5. Owner-scoped authorization on every authoring route, plus credential rate limiting.
6. Integration tests that actually run, and CI that fails on a skipped test.
7. Design tokens, light and dark themes, keyboard accessibility, responsive layouts.
8. English and Russian, with further languages addable at runtime.

## Next

- Public room discovery and invite links.
- Presence.
- Event-journal retention policy.
- Load testing, which has not been done at all.
- Versioned, sandboxed mini-game modules through `launch_minigame`.

## Planned extension boundaries

- OAuth providers for hosted deployments.
- Import/export after stable definition schema and validation are established.
- Bot players and additional turn-based generic action primitives.

These items must preserve server authority and the generic runtime; they are not
permission to add game-specific runtime branches.
