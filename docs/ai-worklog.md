# AI Worklog

## Current Contract

- Backend JSON APIs return `{ "code": 0, "msg": "ok", "data": ..., "meta": ... }`.
- Paginated APIs return list data in `data` and pagination metadata in `meta.total`, `meta.page`, and `meta.page_size`.
- Errors use lowercase `code` and `msg`.
- The admin frontend unwraps this envelope in `fe/myecho-admin/src/utils/myaxios.ts`.

## Local Setup

1. `git submodule update --init --recursive`
2. `cp config.example.yaml config.yaml`
3. `make check`
4. For frontend changes: `make admin-test`
5. To serve the admin bundle from the backend: `make admin-build`

## Decisions

- Passwords are stored with bcrypt. Legacy broken SHA-256 strings are accepted only during login migration and are upgraded after successful login.
- Remote file downloads are limited to `http` and `https`, reject private/local/link-local targets, require 2xx responses, and cap downloads at 50 MiB.
- `config.yaml` is local-only; commit changes to `config.example.yaml` instead.
- The admin frontend uses npm. Do not reintroduce `yarn.lock`.

## Fixed In This Pass

- Password storage and legacy login migration.
- Unified API success/error response shape.
- API 404 and missing admin build behavior.
- Remote file/favion download validation and size/status checks.
- Theme activation transaction safety.
- Article pagination when top and public articles share pages.
- Setting cache invalidation on delete.
- File metadata update extension guard.
- DB error propagation in tag/comment/link/setting/file paths.
- Local and CI verification entrypoints.

## Follow-Up Backlog

- Regenerate Swagger docs from annotations after the API envelope is fully documented.
- Add rate limiting or moderation controls for public comment creation.
- Decide whether unsupported `database.type: mysql` should remain SQLite fallback or become a startup error.
- Consider moving direct DB usage out of remaining API handlers into service/DAL methods.
