# myecho

`myecho` is a Go blog application. The backend uses Fiber, GORM, Jet templates, Swagger docs, and a YAML runtime config. The admin frontend lives in the `fe/myecho-admin` Git submodule.

## Repository Layout

- `main.go`: application startup and middleware setup.
- `api_router.go`, `router.go`: route registration.
- `handler/api`: JSON API handlers.
- `handler/view`: server-rendered page handlers.
- `handler/rtype`: request and response types.
- `service`: business logic.
- `dal`: database/cache access.
- `model`: GORM models.
- `middleware`: Fiber middleware.
- `config`: static and YAML config loading.
- `views`: Jet templates and static assets.
- `docs`: Swagger output.
- `fe/myecho-admin`: admin frontend submodule.

## Requirements

- Go 1.23 or newer. The module currently declares toolchain `go1.24.7`.
- A `config.yaml` file in the repository root.

## Quick Start

```sh
git submodule update --init --recursive
make run
```

The default listen address is `:2999`. Override it with:

```sh
go run ./ -port :3000
```

## Common Commands

```sh
make build    # build current platform binary
make run      # run the backend
make fmt      # gofmt backend Go files
make vet      # run go vet ./...
make test     # run go test ./...
make tidy     # run go mod tidy
make clean    # remove built binaries
```

## Configuration

`config.yaml` is read at startup. Database type can be `sqlite` or `postgresql`; an empty or unknown value falls back to SQLite using `database.db_name`.

Minimal local SQLite config:

```yaml
database:
  type: sqlite
  host: 127.0.0.1
  port: 3306
  user: admin
  password: admin
  db_name: myecho
app_config:
  allow_register: true
```

## AI Development Notes

AI coding agents should start with `AGENTS.md`. It contains the project map, layer boundaries, verification commands, and submodule guidance.
