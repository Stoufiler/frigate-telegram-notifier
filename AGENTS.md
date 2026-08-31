# Agent Guidelines

Instructions for AI coding agents working on this repository.

## Before You Code

- Read `CLAUDE.md` for project layout, conventions, and build commands
- Run `make lint` before claiming any change is done
- Run `make build` to verify compilation
- Use conventional commits: `feat:`, `fix:`, `refactor:`, `ci:`, `docs:`, `chore:`

## Architecture

Single-binary Go bot. No frameworks, no ORM, no config files at runtime — only env vars.
The MQTT message flow is: `frigate/reviews` → `dispatch()` → `handleNew` / `handleEnd` → Telegram.

All user-facing text is localized via `tr("key")`. Add new keys to both `locales/en.json` and `locales/fr.json`.

## Common Tasks

| Task | Command |
|------|---------|
| Build | `make build` |
| Lint | `make lint` |
| Format | `make fmt` |
| Test | `make test` |
| Changelog | `make changelog` |
| Docker | `make docker` |

## Do Not

- Add runtime config files (YAML, TOML, etc.) — env vars only
- Hardcode user-facing strings — use `tr()` with locales
- Add dependencies without a strong reason — this is a small, focused bot
- Skip linting — CI will fail
- Use `bbox` parameter — it was dropped in favor of `bounding_box` (Frigate 0.18)
