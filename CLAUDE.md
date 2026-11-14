# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`github.com/ottramst/go-kibana` is a hand-written, idiomatic Go client for the [Kibana REST API](https://www.elastic.co/docs/api/doc/kibana) (single package `kibana`, Go 1.26+, one dependency: `go-querystring`). Until v1.0.0, breaking changes are allowed but must be listed under `BREAKING CHANGES` in `CHANGELOG.md` and in `docs/upgrading-to-v1.md`. From v1.0.0, the module follows semver: breaking changes need a `/v2` module path, and the `API compatibility` CI job enforces it (exceptions for technical preview and experimental endpoints only, per README "Stability").

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
- `NewClient(baseURL, ...ClientOption)`. Options: `WithAPIKey`, `WithBasicAuth`, `WithHTTPClient`, `WithUserAgent`, `WithSpace`, `WithRetry`. The base URL keeps any `server.basePath` and loses its trailing slash. The default `http.Client` has no timeout; contexts govern. The User-Agent is `go-kibana/<module version>`, read from build info.
- `WithRetry(RetryPolicy)` (`retry.go`) is applied after the other options: it wraps the transport of a copy of the `http.Client` (nil means `http.DefaultTransport`), so option order doesn't matter and the caller's client is unchanged.
  - GET/HEAD/PUT/DELETE/OPTIONS retry on `RetryOn` (default 429/502/503/504) and network errors. Other methods only retry on 429 and on 503 with `Retry-After`, because Kibana did not process them.
  - No retry after TLS errors (certificate verification, and alerts from the server such as a rejected client certificate, which surface as `*net.OpError{Op: "remote error"}`), when the context is done, or for bodies without `GetBody` (the streamed import).
  - The decision and the wait are made before the old response is drained (up to 64 KiB), so a response that isn't retried reaches `Do` unread. `Retry-After` is capped at `MaxBackoff`: Kibana sends `Retry-After: 30` with its startup and auth-not-ready 503s, so giving up instead would never retry them with a smaller `MaxBackoff`. A wait past the context deadline returns the last response. `http.Client.Timeout` shows up as a context deadline in the transport.
  - Tests replace the sleep with `WithRetrySleep` from `export_test.go`.
- Services use the go-github pattern: `type SpacesService service`, all sharing `Client.common`. A new service needs a field on `Client` and a line in `initServices()`. `InSpace(id)` copies the `Client` and calls `initServices()` on the copy, so services must never cache the client elsewhere.
- `NewRequest(ctx, method, path, body, ...RequestOption)`:
  - `path` is the literal API path from the spec (`/api/...`), with segments already escaped and an optional `?query`. Paths with a `#`, and paths or query strings with characters that must be escaped (spaces, control characters, non-ASCII), are rejected. Escaped characters of the base path (`%2F`) are kept.
  - The space prefix `/s/{space}` goes between the base path and `path`, unless the space is empty or `default`.
  - Header order: `Accept`, `kbn-xsrf: true` (always, even on GET), `Content-Type` (JSON bodies only), `User-Agent`, `Authorization`, then the caller's `RequestOption`s. So callers can override anything.
  - Body: an `io.Reader` is sent raw, with no `Content-Type` (the caller sets it, e.g. for the multipart import); anything else is JSON-encoded.
- `Do(req, v)`:
  - Returns a `*Response{StatusCode, Status, Header}`; it doesn't expose the `*http.Response`, whose body is already read. Non-2xx returns `*ErrorResponse`, built by the unexported `checkResponse`.
  - `v == nil` drains the body; an `io.Writer` gets the raw body (for ndjson exports).
  - Anything else is JSON-decoded. An empty body (204) is success and leaves `v` untouched. Data after the JSON value is an error.
  - The body is always drained and closed.
- Service methods call the unexported `c.do(ctx, method, path, body, v, opt, opts)`, which encodes `opt` like `addQuery`.
- Helpers:
  - `pathf("/api/x/%s", id)` escapes each segment with `url.PathEscape`. Never `fmt.Sprintf` a path.
  - `addQuery(path, opt)` encodes `url:"..."` struct tags with go-querystring, for any HTTP method.
  - `WithQueryParam` appends to the existing query string without re-encoding it.
  - `requireID(name, v)` rejects empty, `.` and `..` path parameters before sending.
- `ErrorResponse{Method, URL (without credentials), Header, StatusCode, StatusText (the "error" field), Message, Attributes, Body}`. The format of `Error()` is not part of the API. `Is()` maps 400/401/403/404/409/429/503 to `ErrBadRequest`/`ErrUnauthorized`/`ErrForbidden`/`ErrNotFound`/`ErrConflict`/`ErrTooManyRequests`/`ErrServiceUnavailable`. Fields of unexpected types are skipped. Bodies that are not a JSON object, such as proxy pages, become `Message`, cut to 512 bytes; `Body` keeps the whole body (up to 1 MiB).

**Service conventions** (`spaces.go` is the template). These are the v1 API rules; changes that break them need a new major version after v1.0.0. The v1.0.0 milestone PRs (#43) bring the existing code in line with them, so until v1.0.0 some code still differs (e.g. bodies passed as bare slices or maps); `docs/upgrading-to-v1.md` tracks every change.
- **Names:**
  - Methods: `List`, `Get`, `Create`, `Update`, `Delete`. A PUT that creates or replaces (an upsert) is `CreateOrUpdate`. Other operations are named after the endpoint, e.g. `CopySavedObjects` or `Query`.
  - Iterators are the single-page method plus `All` (`QueryAll`, `ListAll`, `FindAll`). They return `iter.Seq2[*T, error]` and are built on the internal paginators in `pagination.go`: lazy, restartable, yield an error once and stop, and stop on an empty page.
  - Options types are `<Method><Noun>Options` (`ListSpacesOptions`, `GetRoleOptions`); an iterator shares the options of its single-page method. Body types are `…Request`, items of JSON-array bodies are `…Item`, and results are `<Verb><Noun>Result`. Types shared by several methods are named after the main one (`ImportSavedObjectsResult`, `RuntimeFieldResult`).
  - Constants are `<TypeName><MeaningfulName>`, e.g. `SessionMatchAll` or `TaskManagerHealthStatusWarning` for `"warn"`.
- **Signatures:** `(ctx, ids..., [stream], [domain object | *XRequest], opt *XOptions, opts ...RequestOption)`, returning `(*T, *Response, error)`, or `(*Response, error)` when Kibana returns no body. No method returns a result together with a non-nil error.
  - Required lists without which the call makes no sense are positional, like IDs: `Find(ctx, types, opt)` (the repeated `type` query parameter) and `ResolveImportErrors(ctx, r, retries, opt)` (the `retries` form field). Optional query parameters and form fields go into Options.
  - Every method has a nil-able `opt`, even if the options struct is still empty, so that query parameters can be added later without breaking callers.
  - Options hold query parameters (`url:"..."`), and flags that Kibana takes in the body next to the domain object (`url:"-"`, e.g. `CreateDataViewOptions.Override`). `…Request` structs hold only body fields.
  - A JSON object body is passed as a domain object (`*Space`, `*Role`) or a `…Request` struct, never as a bare slice or map. JSON-array bodies (the saved objects bulk endpoints) take slices.
  - Every Options and `…Request` struct has an unexported `_ [0]func()` as its first field (first, because a trailing zero-size field adds padding). It makes the struct non-comparable, so that adding a slice or map field later is not a breaking change, and requires keyed literals.
- **Results:** envelopes around a resource or collection are unwrapped (`{"data_view": {...}}` returns `*DataView`). Scalar results and results with metadata are a `*<Verb><Noun>Result`. Returned collections are `[]*T` (or a `map[string]*T` keyed by Kibana); collections inside structs hold values. Exception: `BulkRoleError` has a value receiver, so the values of `map[string]BulkRoleError` are errors.
- **Annotations:** every method's doc comment ends with lines that `tools/coverage` parses, as separate paragraphs, so that pkg.go.dev doesn't join them:
  ```go
  // Kibana API: GET /api/spaces/space/{id}
  //
  // Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-<operationId lowercased>
  ```
  The annotation path must match the spec exactly, including the `{param}` names. A method that calls several endpoints has one `Kibana API:` paragraph each. The docs link keeps the spec's operationId as is, including typos (`createdataviewdefaultw`).
- **Types:**
  - Prefer one type for reads and writes, so read → modify → write works, and strip read-only fields in an unexported conversion: `Space.writable()`, `newRolePayload()`. Never mutate the caller's value.
  - Fields use plain values. Use `omitempty` only where Kibana rejects the empty value (e.g. `Space.Color`) or the field must not be sent; otherwise send it, so that `""` clears it: Kibana's PUT endpoints keep fields that are left out (see the Spaces quirks). Use pointers only when the zero value means something different from "not set". There are no `String()`/`Bool()` helpers; Go 1.26 has `new(expr)`.
  - Free-form JSON is `json.RawMessage` when it round-trips or callers decode it into their own types (`SavedObject.Attributes`, `DataViewFieldFormat.Params`), and `map[string]any` only for small values that are only written (`CreateShortURLRequest.Params`).
  - Enum-like strings have a named string type with constants. The constants are not exhaustive: Kibana may send other values, which still decode.
  - `SavedObjectRef{Type, ID}` is embedded in result types that identify an object (e.g. `ImportSuccess`), not in input types. It must not gain fields or methods, which would change every type that embeds it.
- Reject inputs that Kibana would silently widen, such as a role privilege without spaces.
- Deprecated endpoints (saved objects CRUD/bulk/find) get a `// Deprecated:` paragraph naming the replacement, or saying that there is none.

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
  - `_bulk_update` is POST in the 8.19 spec and PUT in 9.5, but both 8.19.25 and 9.5.6 only accept PUT: POST is routed to "create an object of type `_bulk_update`" and fails with 400.
  - `DELETE /api/saved_objects/{type}/{id}` and `GET /api/features` are missing from the 8.19 spec, although 8.19 serves them.
  - Saved objects operationIds differ between the specs.
  - `/api/tags` only exists in 9.5+ (experimental). Data views v2 is 9.6+.
- Saved objects import and export (same on 8.19.25 and 9.5.6):
  - Export returns `application/ndjson`: one object per line, then a details line `{exportedCount, missingRefCount, missingReferences, excludedObjectsCount, excludedObjects}` unless `excludeExportDetails`. The client writes every line to the caller's writer and returns the details line decoded; it keeps only the current line in memory. An export without objects is only the details line. `type` and `objects` together are a 400, as is neither. A missing object is a 400 whose `attributes.objects` lists it. `search` and `hasReference` work on 8.19 too, although only the 9.5 spec has them.
  - Import is `multipart/form-data` with a `file` part whose filename must end in `.ndjson` (400 otherwise); the details line is ignored. Chunked uploads work, so the client streams the form through `io.Pipe`. It returns 200 when objects fail, with `success: false` and `errors[]` in the same format as copy-to-space (hence the shared `Import*` types), plus `warnings`. `createNewCopies` with `overwrite` is a 400.
  - `_resolve_import_errors` takes the same file plus a `retries` form field holding a JSON array; without it, 400. Index patterns imported into another space get a new `destinationId`, as with copy.
  - `_rotate_key` needs `xpack.encryptedSavedObjects.keyRotation.decryptionOnlyKeys` (400 otherwise; the compose stack sets one), returns `{total, successful, failed}`, and 429 while another rotation runs. `successful + failed` can be less than `total` (skipped objects), and `failed` is occasionally 1 for Kibana's own objects (seen in CI on 9.5.6). `batch_size` over 10000 is a 400.
- Saved objects CRUD and bulk (deprecated since 8.7; same on 8.19.25 and 9.5.6):
  - Objects have `id, type, namespaces, attributes, references, managed, migrationVersion, coreMigrationVersion, typeMigrationVersion, created_at, updated_at, version`. `version` is an optimistic concurrency token: PUT with a stale one is a 409.
  - PUT merges top-level attributes, keeps the references when `references` is left out, and replaces them when it is sent (also `[]`). Its response only has `id, type, updated_at, version, namespaces, attributes` (plus `created_at` for an upsert). PUT on a missing object is a 404, unless `upsert` holds the attributes to create it with. The 9.5 PUT body rejects the migration versions.
  - Create with an existing ID is a 409, unless `?overwrite=true`. An unknown type is a 400. DELETE answers 200 with `{}`; it works on 8.19, although the 8.19 spec does not list it.
  - Bulk endpoints answer 200 with per-item errors (`{id, type, error: {statusCode, error, message}}`): `_bulk_create`, `_bulk_get` and `_bulk_update` return `{saved_objects}`, `_bulk_delete` `{statuses: [{id, type, success, error?}]}`, `_bulk_resolve` `{resolved_objects: [{saved_object, outcome}]}`, where a missing object is a `saved_object` with an `error` and the outcome `exactMatch`.
- Saved objects find and resolve (deprecated since 8.7; same on 8.19.25 and 9.5.6):
  - `_find` requires `type` (400 otherwise), repeated for several types; an unknown type returns no objects, not an error. `has_reference` and `has_no_reference` are JSON in the query string, either an object or an array (repeating the parameter also works); the client always sends an array. `namespaces` works on 8.19 too. Items add `score` and `sort`.
  - `page × per_page` over 10,000 is a 400 from Elasticsearch ("Result window is too large"). `per_page` defaults to 20. `aggs` on a `text` field such as `index-pattern.attributes.title` is a 400.
  - `resolve/{type}/{id}` returns `{saved_object, outcome}`; `alias_target_id` and `alias_purpose` only come with legacy URL aliases, which cannot be created through the public API, so `aliasMatch`/`conflict` are untested live.

- Data views (same on 8.19.25 and 9.5.6; the specs are identical apart from descriptions):
  - Bodies use the `data_view` envelope; the list is `{data_view: [{id, title, name, namespaces, managed (9.5 only)}]}`. `allowHidden` (include hidden and system indices) is not in the spec but is returned, and accepted by create (not update). Responses leave out `typeMeta` and `type` for data views that are not rollups.
  - Create and update bodies are strict (unknown fields are a 400) and reject `null` (e.g. `typeMeta: null`). Empty collections are accepted.
  - An existing ID is a **400** ("conflict"), not a 409, unless `override: true`, which replaces the data view with that ID. Several data views can have the same title. A title that matches no index is accepted, with or without `allowNoIndex`.
  - Update keeps the fields that are left out. `title: ""` is saved as an empty title (so the client leaves an empty Title out), `name: ""` and `timeFieldName: ""` clear them, `runtimeFieldMap: {}` removes all runtime fields, and `version`, `id`, `fieldAttrs`, `namespaces` and `allowHidden` are a 400. DELETE answers 204.
  - The first data view of a space becomes its default. GET `/default` returns `{data_view_id: ""}` when there is none. POST `/default` without `force: true` is a silent no-op (`{acknowledged: true}`) when a default exists, so the client always sends `force`. `data_view_id: null` removes the default, `""` is a 400, and unknown IDs are accepted. Deleting the default data view leaves its ID as the default.
- Data view fields:
  - `POST .../{viewId}/fields` returns the data view **without** `fields` (not `{acknowledged}` as the spec says). `null` removes a `customLabel`, `customDescription`, `count` or `format`; `""` is a 400 (minimum length 1), and `customDescription` has at most 300 characters. `format` is stored in `fieldFormats`, the others in `fieldAttrs`. Field names that are not mapped are accepted.
  - Runtime fields: create, create-or-update, get and update (also update, although the spec says it has no body) return `{data_view (without fields), fields: [field]}`. A name that exists, including a mapped field, is a 400 on create. Update is partial (only `type` or only `script`). `customLabel`, `customDescription`, `popularity` (→ `count`) and `format` are accepted by create, create-or-update and update (also on composite subfields, as `comp.a`) and stored as field metadata (`fieldAttrs`, `fieldFormats`). Inside a data view's `runtimeFieldMap` they are stored and returned with the runtime field instead. DELETE answers 200 with no body.
  - Composite runtime fields (`type: composite`, `fields: {sub: {type}}`) can be created and return a field per subfield (`comp.a`), but GET of the composite field is a 400 ("Cannot read properties of undefined (reading 'toSpec')") and GET or DELETE of a subfield is a 404.
- Swap references: `toId` must exist (400 otherwise). `forId` takes a string or an array; `[]` matches nothing, and leaving it out matches everything, so the client rejects an empty non-nil `ForID`. `deleteStatus` is only returned with `delete: true`; the source data view is deleted only when `remainingRefs` is 0.
- User sessions: `POST /api/security/session/_invalidate` needs a superuser (403 otherwise) and returns `{total}`. `match: "query"` requires `query.provider.type`, and `match: "all"` with a `query` is a 400. Sessions only exist for interactive logins, so API key or basic auth requests create none.
- Platform (verified on 8.19.25 and 9.5.6):
  - `GET /api/status` answers 503 with the full status body only when a **core** service (Elasticsearch, saved objects) is unavailable or critical. A degraded Kibana, or one with unavailable plugins, answers 200. `Status.Get` returns the 503 as an `*ErrorResponse` like any other error, and `StatusFromError` decodes the status from its body. A 503 without a status body (e.g. from a proxy, or "Kibana server is not ready yet") has none.
  - Unauthenticated callers, and since 9.5 callers without the `monitor` cluster privilege, get only `{"status":{"overall":{"level":...}}}` (no version). Plugin statuses add `reported`, which is not in the spec.
  - `GET /api/features` has the same shape on both versions, but the IDs differ (`dashboard` on 8.19.25, `dashboard_v2` on 9.5.6; 50 and 64 features). Features with only reserved privileges (e.g. `monitoring`) have `privileges: null`. `?ignoreValidLicenses=true` is accepted but not in the spec.
  - `GET /api/task_manager/_health` has a top-level `reason` when the status is not `OK`. Stats sections are `{status, timestamp, value}`, plus `reason` for `capacity_estimation`. The section values change between versions, so they stay `json.RawMessage`.
- Short URLs (same on 8.19.25 and 9.5.6):
  - `accessDate` and `createDate` are epoch milliseconds on both versions, although the 8.19 spec says strings. Responses add `url: ""`.
  - Short URLs belong to a space: a get or resolve from another space is a 404.
  - Resolve by slug counts as an access: Kibana increments `accessCount` and sets `accessDate` in the background, and returns the record as it was before. Get by ID changes nothing.
  - A taken slug and an unknown `locatorId` are both 409. A `LEGACY_SHORT_URL_LOCATOR` `url` that is not a Kibana path is a 400.
  - Create accepts slugs of 3 to 255 characters, but get, resolve and delete validate `{id}`/`{slug}` as 4 to 128 characters (400 otherwise).
  - `humanReadableSlug` is deprecated in Kibana and ignored (a random 5-character slug is generated), so the client does not send it.
  - DELETE answers 200 with the body `null`.
- Tags (`/api/tags`, 9.5.6; 8.19.25 answers 404 on every tag route):
  - Responses are `{id, data: {name, description?, color}, meta: {created_at, updated_at, managed, version}}`; `Tag` flattens `data`. Lists are `{data, meta: {page, per_page, total}}`; `per_page` defaults to 20, at most 1000.
  - Create and update bodies only accept `name`, `description` and `color` (`additionalProperties: false`). `color: ""` is a 400; a missing color gets a **random** color, also on PUT, so `Color` is `omitempty` and updates must send it.
  - PUT replaces every field (a missing `description` clears it) and is an upsert: 200 when it updated, 201 when it created the tag with that ID. Its response leaves out `created_at`. There is no optimistic concurrency: `meta.version` cannot be sent.
  - POST answers 201. Duplicate names in a space are 409. DELETE answers 204. Tags belong to a space (404 from another one).

**Tooling:** `tools/coverage/` and `tools/gorelease/` are separate modules, so the library keeps one dependency. `tools/gorelease` only pins `golang.org/x/exp/cmd/gorelease` (a `tool` directive) for the API compatibility job; gorelease needs a clean checkout. With a v0 base it never fails, so the check only works against v1+ releases.
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
  - Gate version-specific endpoints with `skipBelow(t, client, "9.5.0")`, or compare `kibanaVersion(t, client)` with `compareVersions`. Both use `Status.Get` and fail when the version is hidden (no `monitor` privilege) instead of skipping.
- `example_test.go` holds compile-only `Example*` functions (no `// Output:`), shown on pkg.go.dev.

## CI

GitHub Actions workflows in `.github/workflows/`:
- `main.yml` ("CI/CD") calls the reusable `lint`, `build`, `test` and `coverage` workflows. The single `CI gate` job is the required status check. CI/CD, Security and CodeQL run on every pull request, including stacked ones that target another branch.
- `build`: go.mod tidy check, build, vet, and race-enabled unit tests on Go 1.26.x and 1.27.x, the two supported Go releases. The go.mod `go` directive is the oldest one. Its `api-compat` job runs gorelease against the latest `v1.x.y` from the proxy with `-version` set to the next minor, and passes when there is no v1 release yet. The owner-applied `api-break-approved` label (read through `env:`, needs a re-run) turns a failure into a warning.
- `test`: acceptance tests on a Kibana {8.19.25, 9.5.6} matrix via `docker compose up -d --wait`. The image tag comes from `ELASTIC_STACK_VERSION`, whose default in `docker-compose.yml` is the newest. Dependabot does not bump the stack; see CONTRIBUTING.md for the version bump checklist (test matrix, compose default, `KIBANA_SPEC_VERSIONS`, README, this file).
- `coverage`: tests the coverage tool, runs `make coverage`, and fails if `COVERAGE.md` changed.
- Also: CodeQL (`codeql.yml`), govulncheck and dependency review (`security.yml`), OpenSSF Scorecard, actionlint plus zizmor for workflow changes (zizmor is pinned in `workflow-lint.yml` and fails the job on findings, without uploading SARIF; reusable workflows are called with `./` paths plus `# zizmor: ignore[self-repository]`, because the `$/` syntax zizmor suggests fails actionlint), lock threads, and waiting-response triage.
- `release.yml`: pushing a `vX.Y.Z` tag creates the GitHub release from the `## vX.Y.Z` section of `CHANGELOG.md` (the heading must match exactly). It runs in the `release` environment, which only allows `v*` tags. If the release already exists (a re-run), it is left unchanged, with a warning if it is a draft. A separate `warm-proxy` job without permissions then warms proxy.golang.org: it waits 90 s for the tag to be visible, because the proxy caches a miss for up to about 30 minutes, retries for about 40 minutes, and only warns if the proxy still misses. Re-runs use the workflow file of the tagged commit. Never push a throwaway tag to test it: proxy versions cannot be removed.
- All actions are pinned to commit SHAs with a version comment. Workflows triggered by events use top-level `permissions: {}` and grant permissions per job; the reusable workflows (`build`, `lint`, `test`, `coverage`) declare `contents: read`. Every job has a timeout, and checkouts use `persist-credentials: false`.
- Never commit downloaded files: `.cache/` (the OpenAPI specs) must stay in `.gitignore`.

## Rules

- Until v1.0.0, every pull request that breaks the API adds its changes to `docs/upgrading-to-v1.md`.
- Add a `CHANGELOG.md` entry under `## Unreleased` for user-visible changes (HashiCorp style: `BREAKING CHANGES`, `FEATURES`, `ENHANCEMENTS`, `BUG FIXES`, `NOTES`), linking the issue.
- Every new service method needs the `// Kibana API:` and `// Kibana API docs:` lines, unit tests, an acceptance test on both supported versions, and a regenerated `COVERAGE.md` (`make coverage`).
- Pull request titles follow Conventional Commits (`feat(spaces): ...`, `fix(roles)!: ...`). PRs are squash-merged and reference their issue (`Closes #N`).
- Keep the library to the standard library plus go-querystring. Tooling dependencies belong in `tools/`.
- Record newly discovered Kibana API quirks in the "Kibana API quirks" section above.
- Plans for a milestone, or for any change that spans several pull requests, go through the `plan-critic` subagent (`.claude/agents/plan-critic.md`) before they are approved. Spawn it with the plan file path and the decisions the user has made only, fold the confirmed findings into the plan, and note in the plan which were taken. Run it again if the findings change the plan's structure, such as its pull requests or their order.
- Every pull request, including Dependabot and release pull requests, gets an independent review by the `pr-reviewer` subagent (`.claude/agents/pr-reviewer.md`) before it is merged. Spawn it with the PR number and the PR's purpose only, not your own conclusions, so the review is independent. Fix confirmed findings on the PR branch, and have the fix commits reviewed again before merging.
