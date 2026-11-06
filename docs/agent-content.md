# Agent content API

`GET /api/openapi.json` serves the canonical OpenAPI 3.1 contract in [openapi.json](openapi.json). The older Swagger 2 files describe legacy endpoints and are not the agent API contract.

## Authentication and access

An ordinary user token uses `Authorization: token <login-token>`. A scoped agent credential uses `Authorization: Bearer mye_<token>`. Agent credentials are created with `POST /api/account/tokens` and revoked with `DELETE /api/account/tokens/{id}`; both operations require an ordinary login token. Listing returns metadata only. The raw `mye_` credential is shown once at creation, so store it in a secret manager or environment variable and do not commit or log it.

Omitted or zero `expires_in_days` means 90 days; the maximum is 365 days. Grant only the scopes the agent needs. The supported scopes are `articles:read`, `articles:write`, `articles:publish`, `articles:delete`, `taxonomy:write`, `media:read`, `media:write`, `media:delete`, `content:export`, `content:import`, `comments:read`, `comments:write`, `settings:read`, and `settings:write`.

Scopes do not change the account role or ownership rules. Public article reads remain public; `GET /api/all_articles` and editing are limited to content the account can manage. An `author` can manage only their own articles and media; an author without trusted publishing can save drafts or submit them for review. An `editor` can manage site-wide content. For draft-only editorial work, grant an editor `articles:read` and `articles:write` without `articles:publish`. Publishing or reviewing requires the appropriate role plus `articles:publish`. Comment operations require an Editor/Admin role; settings writes require Admin. Content import and export also require an administrator role; import additionally requires `content:import` and `articles:publish`. Agent credentials cannot mint credentials or use article batch operations, themes, system backup/restore, or other unlisted routes.

## Article editing

Use `GET /api/all_articles?page=1&page_size=50` to list manageable articles and `GET /api/articles/{id}` to read one. Lists are paginated by default. The CLI accepts `--page-size` values from 1 to 100. Authors see only their own items. The detail response contains `edit_version` and, when available to that account, its latest editable submission snapshot.

Create with `POST /api/articles`. The request needs `title` and `content`; `type` is `1` for a post or `2` for a page. Use `content_format: "markdown"` or `"html"` (`markdown` is the default). Omitted status defaults to `4` (draft). Other status values are `1` published, `2` pinned, `3` private, `5` submit for review, and `6` recycle bin. `post_time` is an RFC 3339 timestamp. Category and tag references use `category_uid` and `tag_uids`.

Agent article creation requires an `Idempotency-Key` header of 1–128 printable ASCII characters with no whitespace. A key is scoped to the user. Repeating the same key with an identical request body replays the original 201 response; reusing it with a different body returns 409. The CLI does not retry create requests automatically. If a create result is uncertain, check the article list before trying again with a new key. Permanently deleting an article scrubs a cached idempotency response but retains a key tombstone; replaying that key returns 409 and cannot recreate the deleted article.

For updates, read the article, preserve the `edit_version`, edit the content, and send `PATCH /api/articles/{id}` with `expected_edit_version`. Agent PATCH requests require this field. PATCH uses a full editable article representation: include the current fields you want to preserve, including `title` and `content`; omitted values can reset to defaults or clear fields. A stale version returns HTTP 409. Fetch the latest article or submission, compare it with the intended changes, and reconcile before saving. Never blindly retry a version conflict.

Saving with status `5` creates an immutable pending-review snapshot; it does not replace the online content until an editor approves it. Editors can inspect `/api/submissions`, fetch `/api/submissions/{id}`, and approve or reject with `POST /api/submissions/{id}/review` using `{"action":"approve"}` or `{"action":"reject","note":"..."}`. Rejections require a note. The submitting author can withdraw a pending item with `POST /api/submissions/{id}/withdraw`.

Revision history is available at `/api/articles/{id}/revisions` and `/api/articles/{id}/revisions/{revision_id}`. Restoring a revision creates an editable draft; it does not publish it. `DELETE /api/articles/{id}` moves an article to the recycle bin and preserves its content and history; `POST /api/articles/{id}/restore` returns it as a draft. `DELETE /api/articles/{id}/permanent` is irreversible and only works for items already in the recycle bin.

Preview links are immutable snapshots. `POST /api/articles/{id}/previews` requires the current `expected_edit_version`; anyone with the returned link can view it until expiry or revocation. Treat preview URLs as secrets. `DELETE /api/articles/{id}/previews` revokes previews created for that article by the current user.

## Taxonomy, media, and portability

Public taxonomy reads are `GET /api/article/categories/all`, `GET /api/link/categories/all`, and `GET /api/tags/all`. Category and tag writes use `taxonomy:write`; category creation uses `/api/article/categories` or `/api/link/categories`, updates/deletes use `/api/categories/{id}`, and tag create/update/delete use `/api/tags` and `/api/tags/{id}`. Article requests refer to category and tag UIDs, not numeric IDs.

Media lives under `/mos`: `GET /mos/files` lists media, `POST /mos/files/upload` uploads a multipart field named `file`, `GET /mos/files/{id}/usages` shows references, `PUT /mos/files/{id}` updates metadata, and `DELETE /mos/files/{id}` removes an unused file. Authors can manage only their uploads; editors/admins can manage all media. A referenced file cannot be deleted.

`GET /api/content/export?include_media=true` downloads a ZIP of Markdown files with YAML front matter; media is omitted by default. `POST /api/content/import?dry_run=true` previews a ZIP in a multipart `file` field. Preview `plans` show each filename, slug, type, action, current/expected versions, and changed fields. Apply only after reviewing the plan by setting `dry_run=false`. Apply rechecks the versions and reports conflicts instead of overwriting intervening edits. New records can be created without a version. Existing article updates require an exported positive `edit_version`; legacy archives without it can still create new articles but will conflict when they match an existing article. Pending submissions and trashed articles are also conflicts. Use `author_map` as a JSON object form field when applying an archive whose authors need mapping.

## Responses and errors

Successful JSON APIs use `{ "code": 0, "msg": "ok", "data": ..., "meta": {} }`. Paginated responses put the array in `data` and return `meta.total`, `meta.page`, and `meta.page_size`. Content export is a ZIP response rather than a JSON envelope. Error responses use `{ "code": <integer>, "msg": "..." }`.

Common error codes are `4001` bad request/conflict, `4002` parse failure, `4011` unauthenticated, `4031` validation/ownership rejection, `4033` role or scope denial, `4041` not found, and `5001` internal database error. Handle the HTTP status as well as the JSON code; in particular, `409` means the state changed and needs reconciliation.

## CLI

The `myecho` CLI uses the same API and credential rules. Put `--url` before the command or set `MYECHO_URL`; set `MYECHO_TOKEN` to a login or agent token. JSON envelopes go to stdout, errors go to stderr, and failures return a nonzero exit code.

```sh
myecho [--url URL] article list [--page N --page-size N --status N --keyword TEXT --category-uid UID --tag-uid UID --author-id ID --type post|page --date-from YYYY-MM-DD --date-to YYYY-MM-DD]
myecho article get --id ID
myecho article create --file FILE --title TITLE --idempotency-key KEY [--slug SLUG --category-uid UID --tag-uid UID ... --summary TEXT --seo-title TEXT --seo-description TEXT --share-image URL --type post|page --content-format markdown|html --status N --data JSON]
myecho article save --id ID [--file FILE] [--data JSON] [--expected-edit-version N]
myecho article submit --id ID [--file FILE] [--data JSON] [--expected-edit-version N]
myecho article publish --id ID [--file FILE] [--data JSON] [--expected-edit-version N]
myecho article preview --id ID
myecho article revisions --id ID
myecho article trash --id ID
myecho article restore-trash --id ID
myecho article restore-revision --id ID --revision-id ID
myecho category list
myecho tag list
myecho media list
myecho media upload --file FILE
myecho token list
myecho token create --name NAME --scopes SCOPE[,SCOPE...] [--expires-in-days N]
myecho token revoke --id ID
myecho content export --output archive.zip [--include-media]
myecho content import --file archive.zip [--apply] [--author-map JSON]
```

`article create` defaults to status `4` (draft), requires a caller-supplied stable idempotency key, and never retries. `save`, `submit`, and `publish` set status to `4`, `5`, and `1` respectively; they first read the current article, merge the explicitly supplied fields over the article's own editable submission when present, and send the latest `expected_edit_version`. `save` requires `--file` or `--data`; `submit` and `publish` can submit the current version unchanged. Add `--expected-edit-version N` to pin an edit; if the current version no longer matches, the CLI exits before PATCH. The CLI never retries a version conflict. Token management requires an ordinary login token; an agent credential cannot mint or revoke credentials. `--expires-in-days` defaults to the server's 90-day expiry and accepts 1–365.

`myecho content export --output archive.zip [--include-media]` writes a ZIP to a new path and refuses to overwrite an existing file. `myecho content import --file archive.zip [--apply] [--author-map JSON]` previews by default; pass `--apply` only after inspecting the plan. `--author-map JSON` supplies author mappings during apply.

Build the CLI with `make cli` (output `bin/myecho-cli`) or run it from source with `go run ./cmd/myecho`.

For a first draft workflow, set `MYECHO_URL` to the running server and `MYECHO_TOKEN` to an ordinary login token from `POST /api/login` (JSON `name` or `email`, and `password`). Create a restricted credential:

```sh
./bin/myecho-cli token create --name content-agent --scopes articles:read,articles:write,media:read,media:write
```

Set `MYECHO_TOKEN` to the returned `data.token`, then use `category list` and `tag list` to discover UIDs. Create a draft with `article create --file article.md --title "Example" --idempotency-key example-draft-v1`. Read the returned article ID and version before using `article save --id ID --file article.md --expected-edit-version VERSION`, `article preview --id ID`, and `article submit --id ID`. Replace `ID` and `VERSION` with the values from the latest response. Review and approval stay with the editor's ordinary account; this credential has no publication or deletion scope.

Agent credentials and cached create responses are excluded from backups. A successful full backup restore invalidates both; issue new agent credentials after restoring.
