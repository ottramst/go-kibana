## Unreleased

BREAKING CHANGES:

* The module now requires Go 1.26 or later ([#2](https://github.com/ottramst/go-kibana/issues/2))
* The client was redesigned ([#4](https://github.com/ottramst/go-kibana/issues/4)):
  * `NewClient(baseURL, opts...)` replaces `NewClient(apiKey, ...)` and `NewBasicAuthClient`; authenticate with `WithAPIKey` or `WithBasicAuth`.
  * Every service method takes a `context.Context` first.
  * Methods are named `List`, `Get`, `Create`, `Update` and `Delete`.
  * `RequestOptionFunc` is now `RequestOption`, `ClientOptionFunc` is now `ClientOption`, and the `String`/`Bool` pointer helpers are removed.
* spaces: `Space` is used for reads and writes, and `CreateSpaceOptions`/`UpdateSpaceOptions` are removed. `Description`, `Initials` and `ImageURL` are always sent, so they can be cleared ([#6](https://github.com/ottramst/go-kibana/issues/6))
* roles: `KibanaPrivilege.Feature` is now `map[string][]string`, so it works with every Kibana feature, and the anonymous structs are replaced with named types. `CreateOrUpdate` returns only the response, because Kibana answers 204 No Content. `CreateOrUpdate` rejects Kibana privileges without `Spaces`, which Kibana would otherwise grant in all spaces ([#7](https://github.com/ottramst/go-kibana/issues/7))

FEATURES:

* Space scoping: `WithSpace` and `Client.InSpace` send requests to `/s/{space}/api/...` ([#4](https://github.com/ottramst/go-kibana/issues/4))
* Typed errors: `*ErrorResponse` exposes the status code, message and body, and works with `errors.Is` against `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound` and `ErrConflict` ([#4](https://github.com/ottramst/go-kibana/issues/4))
* `WithHTTPClient`, `WithUserAgent`, and the request options `WithHeader` and `WithQueryParam` ([#4](https://github.com/ottramst/go-kibana/issues/4))
* `Client.NewRequest` and `Client.Do` can call endpoints that have no method yet ([#4](https://github.com/ottramst/go-kibana/issues/4))
* spaces: `Space` now includes `Solution`, `ProjectRouting` (Kibana 9.5+), `Reserved` and `AuthorizedPurposes` ([#6](https://github.com/ottramst/go-kibana/issues/6))
* roles: Added `description`, remote indices and remote cluster privileges, the `createOnly` option (`CreateOrUpdateRoleOptions`) and the `replaceDeprecatedPrivileges` option (`ListRolesOptions`, `GetRoleOptions`) ([#7](https://github.com/ottramst/go-kibana/issues/7))

BUG FIXES:

* roles: `CreateOrUpdate` no longer fails with `EOF` after every successful call ([#7](https://github.com/ottramst/go-kibana/issues/7))
* The `kbn-xsrf` header is now sent on every request, including `PATCH` ([#4](https://github.com/ottramst/go-kibana/issues/4))
* Query parameters are now sent on `POST`, `PUT` and `DELETE` requests too ([#4](https://github.com/ottramst/go-kibana/issues/4))
* Path parameters are now escaped, so IDs containing `/` or `?` reach the right endpoint ([#4](https://github.com/ottramst/go-kibana/issues/4))
* spaces: `List` sent `include_authorized_purpose` instead of `include_authorized_purposes` ([#6](https://github.com/ottramst/go-kibana/issues/6))
* spaces: `Update` no longer modifies the caller's value, and no longer panics on nil options ([#6](https://github.com/ottramst/go-kibana/issues/6))
* Requests no longer time out after a hardcoded 10 seconds. Use a context deadline or `WithHTTPClient` ([#4](https://github.com/ottramst/go-kibana/issues/4))

NOTES:

* README rewritten with usage, compatibility and support policy, and roadmap sections; `Example*` functions on pkg.go.dev; `CLAUDE.md` with architecture notes and verified Kibana API quirks ([#9](https://github.com/ottramst/go-kibana/issues/9))
* `COVERAGE.md` lists every endpoint of the Kibana 8.19.25 and 9.5.6 OpenAPI specs and the method that implements it. `make coverage` regenerates it from the `// Kibana API:` annotations, and CI fails when it is stale ([#8](https://github.com/ottramst/go-kibana/issues/8))
* Acceptance tests run against Kibana 8.19.25 and 9.5.6 in CI, using `docker-compose.yml` locally ([#5](https://github.com/ottramst/go-kibana/issues/5))
* Repository tooling modernized: golangci-lint v2, a `GNUmakefile`, a hardened CI/CD pipeline, Dependabot, and community health files ([#2](https://github.com/ottramst/go-kibana/issues/2), [#3](https://github.com/ottramst/go-kibana/issues/3))
