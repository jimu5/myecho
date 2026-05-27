# AGENTS.md

## Project Overview

`myecho` is a Go blog application built with Fiber, GORM, Jet templates, and Swagger docs. The repository also tracks the admin frontend as a Git submodule at `fe/myecho-admin`.

## Repository Map

- `main.go`: application bootstrap, middleware registration, static routes, and router setup.
- `api_router.go`, `router.go`: API, theme, Swagger, and page route wiring.
- `handler/api`: JSON API handlers and validators.
- `handler/view`: server-rendered page handlers.
- `handler/rtype`: request and response DTOs.
- `service`: business logic and domain operations.
- `dal`: database/cache access and migration setup.
- `model`: GORM persistence models.
- `middleware`: Fiber middleware, error handling, auth, cache, and request helpers.
- `config`: static configuration and YAML config loading.
- `views`: Jet templates and static public assets.
- `docs`: generated Swagger artifacts.
- `utils`: small shared helpers.
- `fe/myecho-admin`: admin frontend submodule. Treat it as a separate repository.

## Common Commands

Run commands from the repository root unless noted.

```sh
make run      # go run ./
make build    # build current platform binary
make fmt      # gofmt repository Go files, excluding the frontend submodule
make vet      # go vet ./...
make test     # go test ./...
make tidy     # go mod tidy
```

The application expects `config.yaml` in the repository root. The default config falls back to SQLite when `database.type` is empty or unknown.

## Development Rules

- Keep business logic in `service`; keep persistence concerns in `dal` and `model`.
- Keep request/response shapes in `handler/rtype` and validation close to `handler/api/validator`.
- Do not edit generated Swagger files in `docs` by hand unless the task is explicitly about generated API docs.
- Do not make cross-cutting refactors while implementing focused fixes.
- Preserve the Fiber middleware order in `main.go` unless the change depends on ordering.
- Avoid touching `fe/myecho-admin` from the parent repository unless the task explicitly asks for frontend work; it is a submodule.
- Add or update focused tests when changing shared helpers, service behavior, validators, or data access code.
- Use `gofmt` on changed Go files before finishing.

## Testing Notes

- `make test` is the default verification command for backend changes.
- Some packages may require a usable `config.yaml` and local SQLite/PostgreSQL access because initialization reads runtime config.
- Tests should avoid depending on persistent local state. Prefer temporary SQLite database names when adding integration-style tests.

