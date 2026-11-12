# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`github.com/ottramst/go-kibana` is a hand-written, idiomatic Go client for the [Kibana REST API](https://www.elastic.co/docs/api/doc/kibana) (single package `kibana`, Go 1.26+, one dependency: `go-querystring`). It is pre-1.0. Breaking changes are allowed but must be listed under `BREAKING CHANGES` in `CHANGELOG.md`.

The client supports and tests the latest patch of the newest Kibana 9.x minor and of 8.19, the last 8.x line (currently 9.5.6 and 8.19.25). Kibana's API differs between these versions. Check new code against both: endpoints missing from one version, different HTTP methods for the same endpoint, and request fields one version rejects.

v1.0 scope is the "core platform" APIs: spaces, roles, user sessions, status, features, Task Manager health, short URLs, tags, saved objects and data views. Work is planned as GitHub milestones (v0.1.0 to v1.0.0), with one epic issue per area and one sub-issue per endpoint group. Other areas, such as alerting, connectors, Fleet and the Security solution, are backlog epics. The OpenAPI spec is the reference, but the code is hand-written, not generated.

## Common Commands

```bash
make default     # fmt, lint, test
make build       # go build ./...
make fmt         # gofmt -s
make lint        # golangci-lint (v2 config, zero tolerance)
make test        # unit tests
make testacc     # unit + acceptance tests (needs a running stack, KIBANA_ACC=1)
make coverage    # download the pinned OpenAPI specs to .cache/ and regenerate COVERAGE.md
```

Run a single test:
```bash
go test -v -run TestSpacesService_Update ./...
KIBANA_ACC=1 go test -v -run TestAccRoles ./...
```

## Running Acceptance Tests Locally

```bash
docker compose up -d --wait                                   # Elasticsearch + Kibana 9.5.6, about 4 GB RAM, 1-2 minutes
# ELASTIC_STACK_VERSION=8.19.25 docker compose up -d --wait   # the other supported version
make testacc
docker compose down -v                                        # needed before switching versions
```

The stack runs with security enabled and a trial license, on ports bound to 127.0.0.1. Tear it down after use; it takes about 4 GB of RAM. Credentials are `elastic` / `changeme`, Kibana is on `http://localhost:5601`, and Elasticsearch on `:9200`. A one-shot `kibana-setup` container sets the `kibana_system` password. Tests read `KIBANA_URL`, `KIBANA_API_KEY` or `KIBANA_USERNAME`/`KIBANA_PASSWORD`, falling back to those defaults.

## Architecture

**Client core (`kibana.go`, `request.go`, `errors.go`):**
- `NewClient(baseURL, ...ClientOption)`. Options: `WithAPIKey`, `WithBasicAuth`, `WithHTTPClient`, `WithUserAgent`, `WithSpace`. The base URL keeps any `server.basePath` and loses its trailing slash. The default `http.Client` has no timeout; contexts govern. The User-Agent is `go-kibana/<module version>`, read from build info.
- Services use the go-github pattern: `type SpacesService service`, all sharing `Client.common`. A new service needs a field on `Client` and a line in `initServices()`. `InSpace(id)` copies the `Client` and calls `initServices()` on the copy, so services must never cache the client elsewhere.
- `NewRequest(ctx, method, path, body, ...RequestOption)`:
  - `path` is the literal API path from the spec (`/api/...`), with segments already escaped and an optional `?query`. Paths with a `#`, and paths or query strings with characters that must be escaped (spaces, control characters, non-ASCII), are rejected. Escaped characters of the base path (`%2F`) are kept.
  - The space prefix `/s/{space}` goes between the base path and `path`, unless the space is empty or `default`.
  - Header order: `Accept`, `kbn-xsrf: true` (always, even on GET), `Content-Type` (JSON bodies only), `User-Agent`, `Authorization`, then the caller's `RequestOption`s. So callers can override anything.
  - Body: an `io.Reader` is sent raw, with no `Content-Type` (the caller sets it, e.g. for the planned multipart import); anything else is JSON-encoded.
- `Do(req, v)`:
  - Non-2xx returns `*ErrorResponse`, through `CheckResponse`.
  - `v == nil` drains the body; an `io.Writer` gets the raw body (for ndjson exports).
  - Anything else is JSON-decoded. An empty body (204) is success and leaves `v` untouched. Data after the JSON value is an error.
  - The body is always drained and closed.
- Service methods call the unexported `c.do(ctx, method, path, body, v, opts)`.
- Helpers:
  - `pathf("/api/x/%s", id)` escapes each segment with `url.PathEscape`. Never `fmt.Sprintf` a path.
  - `addQuery(path, opt)` encodes `url:"..."` struct tags with go-querystring, for any HTTP method.
  - `WithQueryParam` appends to the existing query string without re-encoding it.
  - `requireID(name, v)` rejects empty, `.` and `..` path parameters before sending.
- `ErrorResponse{StatusCode, StatusText (the "error" field), Message, Attributes, Body}`. `Is()` maps 400/401/403/404/409 to `ErrBadRequest`/`ErrUnauthorized`/`ErrForbidden`/`ErrNotFound`/`ErrConflict`. Fields of unexpected types are skipped. Bodies that are not a JSON object, such as proxy pages, become `Message`, cut to 512 bytes; `Body` keeps the whole body (up to 1 MiB).

**Service conventions** (`spaces.go` is the template):
- Method names: `List`, `Get`, `Create`, `Update`, `Delete`. Other operations are named after the endpoint, e.g. `CopySavedObjects` or `Query`.
- Signatures: `(ctx, ids..., body/options, ...RequestOption) (*T, *Response, error)`, or `(*Response, error)` when Kibana returns no body.
- Every method's doc comment ends with two lines that `tools/coverage` parses:
  ```go
  // Kibana API: GET /api/spaces/space/{id}
  // Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-<operationId lowercased>
  ```
  The annotation path must match the spec exactly, including the `{param}` names.
- Prefer one type for reads and writes, so read → modify → write works, and strip read-only fields in an unexported conversion: `Space.writable()`, `newRolePayload()`. Never mutate the caller's value.
- Fields use plain values. Use `omitempty` only where Kibana rejects the empty value (e.g. `Space.Color`) or the field must not be sent; otherwise send it, so that `""` clears it: Kibana's PUT endpoints keep fields that are left out (see the Spaces quirks). Use pointers only when the zero value means something different from "not set". There are no `String()`/`Bool()` helpers; Go 1.26 has `new(expr)`.
- Reject inputs that Kibana would silently widen, such as a role privilege without spaces.
- Options structs (`ListSpacesOptions`, `CreateOrUpdateRoleOptions`) hold query parameters, are pointers, and may be nil. Request bodies that are not a domain object (such as `Space` or `Role`) use a `…Request` struct (e.g. `CopySavedObjectsRequest`).
- Paginated endpoints get an iterator variant returning `iter.Seq2[*T, error]` (e.g. `RolesService.QueryAll`) next to the single-page method, built on the internal offset paginator `paginate` (`pagination.go`): lazy, restartable, yields an error once and stops, and stops on an empty page.
- Deprecated endpoints (saved objects CRUD/bulk/find) get a `// Deprecated:` paragraph naming the replacement.

**Kibana API quirks** (verified against 8.19.25 and 9.5.6):
- Kibana rejects state-changing requests without `kbn-xsrf`. Since 9.0, internal APIs (`/internal/...`) also need `x-elastic-internal-origin`. Never call internal APIs. Don't send `elastic-api-version` by default; stateful Kibana defaults to the oldest version.
- `/s/{space}` scoping works on every route: Kibana strips it before routing. Copy-to-space takes the source space from the path.
- Spaces:
  - Create and update bodies are `additionalProperties: false`, so unknown fields are a 400. The read-only `authorizedPurposes` and `_reserved` are stripped. Kibana keeps `_reserved` when it is left out.
  - `description`, `initials` and `imageUrl` accept `""`, which clears them. `color: ""` is a 400 ("must be a 6 digit hex color"), so `Color` is `omitempty` and cannot be cleared.
  - `projectRouting` only exists in 9.5+; sending it to 8.19 is a 400, which is why it is `omitempty`.
  - PUT returns 200 with the space. Text fields left out keep their value, `disabledFeatures` left out becomes `[]`. DELETE returns 204 and deletes the space's saved objects.
  - Conflicts are 409.
  - The list's `authorizedPurposes` also contains `findSavedObjects`, which is not in the spec's purpose enum.
- Spaces saved-object operations (copy, resolve, sharing; verified on 8.19.25 and 9.5.6, which return the same shapes; the specs have no response schemas, only examples):
  - The source space is the request's space (`/s/{space}`), i.e. `client.InSpace(src)`.
  - `createNewCopies` defaults to **true**. `overwrite: true` without `createNewCopies: false` is a 400 on copy ("cannot use [overwrite] with [createNewCopies]"), but on resolve Kibana silently ignores `overwrite` and creates a new copy. The client rejects both combinations.
  - Copying into a space that does not exist **succeeds** and writes objects into it. Nothing checks target spaces.
  - Copies of shareable types (e.g. `index-pattern`) get a new `destinationId` even with `createNewCopies: false`; copying again reports a `conflict` with that `destinationId`.
  - Copy and resolve return 200 when objects fail: `{"<space>": {success, successCount, successResults[], errors[]}}`. An `errors[]` item is an object error (`{type, id, meta, error: {type, destinationId?, destinations?, references?}}`) or, for a failure of the whole space, a Boom payload whose `error` is a string. A missing source object fails the whole request with a 404 whose `attributes.objects` lists it.
  - Resolve `retries` must list every object to copy into a space, including the ones copied before.
  - `_disable_legacy_url_aliases` returns 204, also for aliases that don't exist; an empty `aliases` list is a 403.
  - `_update_objects_spaces` rejects `null` lists (400) and both lists empty (400). Per-object failures are `objects[].error` (`{statusCode, error, message}`). Removing an object from all its spaces deletes it.
  - `_get_shareable_references` adds `isMissing`, `spacesWithMatchingOrigins` and `spacesWithMatchingAliases` (not in the spec). Objects of types that can't be shared report `spaces: []`.
- Roles:
  - `PUT /api/security/role/{name}` returns **204** with an empty body. The pre-redesign client failed on this.
  - The PUT body only accepts `description`, `metadata`, `elasticsearch` (required) and `kibana`.
  - `kibana[].base` is required: send `[]` for feature privileges, never `null`. `_reserved` is response-only.
  - `kibana[].spaces` left out defaults to `["*"]` (all spaces), while `[]` or `null` is a 400. The client rejects empty `Spaces` before sending.
  - GET adds `transient_metadata`, `_transform_error`, `_unrecognized_applications` and `run_as: []`.
  - `?createOnly=true` on an existing role is a 409.
  - A missing role's 404 has the unhelpful message `"[{}]: undefined"`, so match on the status, not the message.
  - Feature IDs are dynamic per version and license, hence `map[string][]string`.
  - `POST /api/security/role/_query` (same shapes on 8.19.25 and 9.5.6; the 8.19 spec has no response schema): returns `{roles, count, total}` with roles shaped like `GET` role. It leaves out built-in roles unless `filters.showReservedRoles` is true. `size` defaults to 10, and `from + size` over 10,000 is a 400 from Elasticsearch. `query` is free text on the name and description.
  - `POST /api/security/roles` (bulk) returns `{created, updated, noop, errors}`. An unchanged role is `noop`. An invalid Elasticsearch privilege fails only that role (`errors[name] = {type, reason}`), with 200 for the request. Unknown Kibana feature names are accepted without error. A privilege without `spaces` is created with `["*"]`.
- The 8.19 and 9.5 specs disagree:
  - `_bulk_update` is POST in 8.19 and PUT in 9.5.
  - `DELETE /api/saved_objects/{type}/{id}` and `GET /api/features` are missing from the 8.19 spec, although 8.19 serves them.
  - Saved objects operationIds differ between the specs.
  - `/api/tags` only exists in 9.5+ (experimental). Data views v2 is 9.6+.
- Planned endpoints (see the sub-issues):
  - Saved objects import (multipart) returns 200 even when objects fail; check `success` and `errors[]`.
  - Export is ndjson that ends with a summary line.
  - `_find` sends `has_reference` as JSON inside the query string, and `type` as a repeated parameter.
  - `_rotate_key` needs `xpack.encryptedSavedObjects.keyRotation.decryptionOnlyKeys`, otherwise 400, and returns 429 while a rotation runs.

- User sessions: `POST /api/security/session/_invalidate` needs a superuser (403 otherwise) and returns `{total}`. `match: "query"` requires `query.provider.type`, and `match: "all"` with a `query` is a 400. Sessions only exist for interactive logins, so API key or basic auth requests create none.

**Tooling:** `tools/coverage/` is a separate module, so the library keeps one dependency.
- It parses the pinned specs and the `// Kibana API:` annotations, then writes `COVERAGE.md`.
- It fails on malformed annotations (e.g. a lower case method), annotations that match no spec, are not on a method, or are duplicated.
- Versions are passed oldest first; the newest spec's tag and x-state win. Untagged operations get the tag of the closest path.

## Test Patterns

- Unit tests are in package `kibana_test` and use the standard library only (no testify). Assertions read `t.Errorf("X = %q, want %q", got, want)`.
- Helpers in `helpers_test.go`:
  - `setup(t)` returns `(*http.ServeMux, *kibana.Client)`, using a basic-auth client against an `httptest.Server`.
  - `testHeader`, `testQuery`, `testJSONBody` (semantic JSON comparison), `fixture(t, "spaces/get.json")`, `writeJSON`.
- Register handlers with method patterns (`mux.HandleFunc("PUT /api/security/role/{name}", ...)`), so a wrong path (404) or method (405) fails the test.
- Cover for each method:
  - Happy path and the exact request body.
  - Read-only fields are not sent, and the caller's value is not mutated.
  - Error mapping (`errors.Is(err, kibana.ErrNotFound)`).
  - Escaping of path parameters (IDs with `/` and `?`).
  - Empty-ID and nil-body guards.
- Fixtures in `testdata/<service>/` are **captured from real Kibana responses** (curl against the compose stack). Never copy examples from the OpenAPI spec: it is CC BY-NC-ND licensed, and must not be committed or copied. It lives only in `.cache/`.
- Acceptance tests (`*_acc_test.go`, `TestAcc*`):
  - Use `accClient(t)`, which skips unless `KIBANA_ACC` is set.
  - Name objects with `randName(t, "acc")` and remove them in `cleanup(t, ...)`, which runs with a non-canceled context.
  - Changes to shared state, such as writing the default space, only run when `localStack()` (`KIBANA_URL` unset, i.e. the compose stack).
  - Each test runs a full lifecycle: create, conflict, get, read-modify-write update, list, delete, then get returning `ErrNotFound`.
  - Gate version-specific endpoints on the `kibanaVersion(t, client)` result.
- `example_test.go` holds compile-only `Example*` functions (no `// Output:`), shown on pkg.go.dev.

## CI

GitHub Actions workflows in `.github/workflows/`:
- `main.yml` ("CI/CD") calls the reusable `lint`, `build`, `test` and `coverage` workflows. The single `CI gate` job is the required status check. CI/CD, Security and CodeQL run on every pull request, including stacked ones that target another branch.
- `build`: go.mod tidy check, build, vet, and race-enabled unit tests on Go 1.26.x and 1.27.x, the two supported Go releases. The go.mod `go` directive is the oldest one.
- `test`: acceptance tests on a Kibana {8.19.25, 9.5.6} matrix via `docker compose up -d --wait`. The image tag comes from `ELASTIC_STACK_VERSION`, whose default in `docker-compose.yml` is the newest. Dependabot does not bump the stack; see CONTRIBUTING.md for the version bump checklist (test matrix, compose default, `KIBANA_SPEC_VERSIONS`, README, this file).
- `coverage`: tests the coverage tool, runs `make coverage`, and fails if `COVERAGE.md` changed.
- Also: CodeQL (`codeql.yml`), govulncheck and dependency review (`security.yml`), OpenSSF Scorecard, actionlint plus zizmor for workflow changes (zizmor is pinned in `workflow-lint.yml` and fails the job on findings, without uploading SARIF; reusable workflows are called with `./` paths plus `# zizmor: ignore[self-repository]`, because the `$/` syntax zizmor suggests fails actionlint), lock threads, and waiting-response triage.
- `release.yml`: pushing a `vX.Y.Z` tag creates the GitHub release from the `## vX.Y.Z` section of `CHANGELOG.md` (the heading must match exactly) and warms proxy.golang.org. It runs in the `release` environment, which only allows `v*` tags.
- All actions are pinned to commit SHAs with a version comment. Workflows triggered by events use top-level `permissions: {}` and grant permissions per job; the reusable workflows (`build`, `lint`, `test`, `coverage`) declare `contents: read`. Every job has a timeout, and checkouts use `persist-credentials: false`.
- Never commit downloaded files: `.cache/` (the OpenAPI specs) must stay in `.gitignore`.

## Rules

- Add a `CHANGELOG.md` entry under `## Unreleased` for user-visible changes (HashiCorp style: `BREAKING CHANGES`, `FEATURES`, `ENHANCEMENTS`, `BUG FIXES`, `NOTES`), linking the issue.
- Every new service method needs the `// Kibana API:` and `// Kibana API docs:` lines, unit tests, an acceptance test on both supported versions, and a regenerated `COVERAGE.md` (`make coverage`).
- Pull request titles follow Conventional Commits (`feat(spaces): ...`, `fix(roles)!: ...`). PRs are squash-merged and reference their issue (`Closes #N`).
- Keep the library to the standard library plus go-querystring. Tooling dependencies belong in `tools/`.
- Record newly discovered Kibana API quirks in the "Kibana API quirks" section above.
- Every pull request, including Dependabot and release pull requests, gets an independent review by the `pr-reviewer` subagent (`.claude/agents/pr-reviewer.md`) before it is merged. Spawn it with the PR number and the PR's purpose only, not your own conclusions, so the review is independent. Fix confirmed findings on the PR branch, and have the fix commits reviewed again before merging.
