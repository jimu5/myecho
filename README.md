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
cp config.example.yaml config.yaml
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
make check    # run vet and tests
make coverage # generate Go coverage summary
make admin-test  # run admin frontend tests
make admin-build # build admin frontend into static/admin
make tidy     # run go mod tidy
make clean    # remove built binaries
```

## Configuration

`config.yaml` is read at startup and is intentionally not tracked. Start from the example:

```sh
cp config.example.yaml config.yaml
```

Database type can be `sqlite` or `postgresql`; an empty, `mysql`, or unknown value falls back to SQLite using `database.db_name`.

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

## API Response Shape

JSON APIs return a common envelope:

```json
{ "code": 0, "msg": "ok", "data": {}, "meta": {} }
```

Pagination responses put the list in `data` and page details in `meta.total`, `meta.page`, and `meta.page_size`. Errors use lowercase `code` and `msg`.

## Admin Frontend

The admin app lives in the `fe/myecho-admin` submodule and uses npm. Build it into the backend static directory with:

```sh
make admin-build
```

The backend serves `/admin/*` only when `static/admin/index.html` exists; missing admin build output returns a clear 404.

## Theme Packages

Theme packages are uploaded from the admin theme page as `.zip` files. A package must contain `theme.json`; referenced assets are extracted under `storage/themes/<theme-name>` and served from `/themes/<theme-name>/`. Packages may also include Jet templates under `templates/`; missing templates automatically fall back to the built-in `views` directory.

Example `theme.json`:

```json
{
  "schema_version": 1,
  "name": "clean_theme",
  "display_name": "Clean Theme",
  "author": "Myecho",
  "version": "1.0.0",
  "description": "A clean blog theme",
  "css": "style.css",
  "js": "script.js",
  "preview": "preview.png",
  "config": {
    "primaryColor": "#0f766e"
  },
  "config_schema": [
    { "key": "primaryColor", "label": "Primary color", "type": "color", "default": "#0f766e" }
  ]
}
```

Supported template override paths include `templates/index.jet.html`, `templates/article.jet.html`, `templates/category.jet.html`, `templates/link.jet.html`, and `templates/components/*.jet.html`. The admin theme preview uses a short-lived signed URL to render the real frontend with the selected theme before activation.

## AI Development Notes

AI coding agents should start with `AGENTS.md`. It contains the project map, layer boundaries, verification commands, and submodule guidance.
