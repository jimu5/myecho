# Copilot Instructions

Use `AGENTS.md` as the source of truth for project layout, commands, and contribution rules.

Important defaults:

- Prefer small, localized Go changes.
- Keep handlers, services, DAL, and models in their existing layers.
- Run `make fmt` and `make test` for backend changes when possible.
- Treat `fe/myecho-admin` as a separate submodule unless the task is explicitly frontend-related.

