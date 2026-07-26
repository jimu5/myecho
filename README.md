# myecho

`myecho` is a Go blog application. The backend uses Fiber, GORM, Jet templates, Swagger docs, and a YAML runtime config. The admin frontend lives in the `fe/myecho-admin` Git submodule.

Core blog features include posts, standalone pages, categories, tags, archive/search views, RSS, sitemap, comments with moderation, password-protected public content, media uploads, links, site settings, and installable Jet template themes.

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
make package  # build admin + linux backend and create dist/myecho-linux-amd64.tar.gz
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

## Public Routes

- `/`: public post list with category, tag, keyword, and date filters.
- `/articles/:id`: backward-compatible article detail route.
- `/posts/:slug`: slug-based post detail route.
- `/pages/:slug`: standalone page route; pages do not appear in the post list, RSS, or sitemap.
- `/article/categories`, `/tags`, `/archive`, `/links`: discovery and link pages.
- `/rss.xml`, `/sitemap.xml`: public feeds for displayable post content.

Public/top posts with an article password are listed by title and summary, but their body is shown only after `POST /api/articles/:id/password` succeeds and sets the unlock cookie.

## Admin Frontend

The admin app lives in the `fe/myecho-admin` submodule and uses npm. Build it into the backend static directory with:

```sh
make admin-build
```

The backend serves `/admin/*` only when `static/admin/index.html` exists; missing admin build output returns a clear 404.

## Deployment Package

Create a Linux amd64 deployment archive with:

```sh
make package
```

The archive is written to `dist/myecho-linux-amd64.tar.gz` and contains the backend binary, `config.example.yaml`, `views/`, `static/admin/`, and an empty `storage/` directory. On the server, extract it, copy `config.example.yaml` to `config.yaml`, edit the config, and run `./myecho -port :2999`.

You can override the target platform when needed:

```sh
make package PACKAGE_OS=linux PACKAGE_ARCH=arm64
```

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

Supported template override paths include `templates/index.jet.html`, `templates/article.jet.html`, `templates/article_password.jet.html`, `templates/category.jet.html`, `templates/tags.jet.html`, `templates/archive.jet.html`, `templates/link.jet.html`, `templates/404.jet.html`, and `templates/components/*.jet.html`. The admin theme preview uses a short-lived signed URL to render the real frontend with the selected theme before activation.

Every page template receives `NavigationStaticPages`. A theme that overrides `templates/components/header.jet.html` must render these entries itself, using each page's `DisplayName` and `URL`, to include the static pages selected for theme navigation in the admin.

## AI Development Notes

AI coding agents should start with `AGENTS.md`. It contains the project map, layer boundaries, verification commands, and submodule guidance.
