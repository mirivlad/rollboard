# Contributing to Rollboard

## Before you start

Read [AGENTS.md](AGENTS.md). It holds the rules that keep Rollboard a *generic*
engine, and the most important one is short:

> **No game-specific runtime code.** Cell behaviour comes from
> `ActionDefinition` data. If a change makes the engine know what "property" or
> "dungeon" means, it belongs in a game definition instead.

The second rule is equally load-bearing:

> **The server is authoritative.** The browser may display state, send
> intentions and animate results. It must never decide a rule outcome.

## Setting up

```bash
git clone https://github.com/mirivlad/rollboard.git
cd rollboard
cd frontend && npm install && cd ..
./scripts/dev.sh
```

You need Go 1.24+, Node.js 20+, and Docker for PostgreSQL and Redis.

## Before opening a pull request

```bash
make check
```

That runs gofmt, `go vet`, the full Go suite **with integration tests enabled**,
`svelte-check`, vitest, the frontend build, and demo validation. CI runs the
same thing.

If your change touches the interface, regenerate the screenshots too:

```bash
./scripts/ui-shots.sh
```

## Testing rules

**A skipped test is a failed test.** Integration tests guard themselves behind
`ROLLBOARD_TEST_DATABASE_URL`; `make test` sets it. CI fails the build if any
test reports SKIP, because this project has already shipped an unauthenticated
write endpoint past a suite that was quietly skipping the coverage that would
have caught it.

Run integration tests against `rollboard_test` only. They truncate tables, and
`scripts/test.sh` refuses any other database on purpose.

## Adding a translation

Copy `locales/en.json`, translate the values, restart. No rebuild, no code
change. See [docs/I18N.md](docs/I18N.md) for plural forms and error messages.

The test suite checks that every catalog agrees with English on keys and
placeholders, so a translation that drifts fails the build.

## Style

- **Go**: `gofmt` is enforced. Errors are wrapped with context. Handlers
  resolve the actor before doing anything else.
- **Svelte**: Svelte 5 runes. Reactive modules end in `.svelte.ts`.
- **CSS**: use the tokens in `frontend/src/styles/tokens.css`. Do not introduce
  a raw hex value; if a role is missing, add a token for it. Every interactive
  element needs a visible `:focus-visible` state, which the base stylesheet
  provides as long as you use real `button`, `a` and `input` elements.
- **Comments** explain *why*, not what. If a line looks odd but is deliberate,
  say what breaks without it.

## Security

Anything that reads or writes a draft, session or room must be scoped to the
calling actor. A resource belonging to someone else should read as **missing**,
not forbidden, so endpoints cannot be used to probe for IDs.

If you find a vulnerability, please open a private security advisory on GitHub
rather than a public issue.

## Commits

Conventional-commit prefixes: `feat:`, `fix:`, `docs:`, `test:`, `ci:`,
`chore:`, `perf:`, `style:`.

Write the body for someone who will read it in a year without this context:
what was wrong, why the fix is shaped this way, and how you know it works.

## License

Contributions are accepted under the [AGPL-3.0](LICENSE), the same license the
project uses.
