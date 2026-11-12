## v0.2.0

FEATURES:

* spaces: `CopySavedObjects` and `ResolveCopySavedObjectsErrors` copy saved objects between spaces and resolve copy conflicts; per-object errors are reported in the result ([#12](https://github.com/ottramst/go-kibana/issues/12))
* spaces: `DisableLegacyURLAliases` ([#13](https://github.com/ottramst/go-kibana/issues/13))
* spaces: `GetShareableReferences` and `UpdateObjectsSpaces` share saved objects between spaces ([#14](https://github.com/ottramst/go-kibana/issues/14))
* roles: `Query` returns a page of roles, and `QueryAll` iterates over all matching roles (`iter.Seq2`) ([#16](https://github.com/ottramst/go-kibana/issues/16))
* roles: `BulkCreateOrUpdate` creates or updates several roles in one request; per-role errors are reported in the result ([#17](https://github.com/ottramst/go-kibana/issues/17))
* New `UserSessions` service: `Invalidate` invalidates user sessions ([#18](https://github.com/ottramst/go-kibana/issues/18))

NOTES:

* Tested against Kibana 8.19.25 and 9.5.6. Copy, sharing, role query and bulk responses have no schema in the 8.19 OpenAPI spec; the client was verified against both versions

## v0.1.0

BREAKING CHANGES:

* The module now requires Go 1.26 or later ([#2](https://github.com/ottramst/go-kibana/issues/2))
* The client was redesigned ([#4](https://github.com/ottramst/go-kibana/issues/4)):
  * `NewClient(baseURL, opts...)` replaces `NewClient(apiKey, ...)` and `NewBasicAuthClient`; authenticate with `WithAPIKey` or `WithBasicAuth`.
  * Every service method takes a `context.Context` first.
  * Methods are named `List`, `Get`, `Create`, `Update` and `Delete`.
  * `RequestOptionFunc` is now `RequestOption`, `ClientOptionFunc` is now `ClientOption`, and the `String`/`Bool` pointer helpers are removed.
  * `WithBaseURL` is removed: the base URL is the first argument of `NewClient`.
* spaces: `Space` is used for reads and writes, and `CreateSpaceOptions`/`UpdateSpaceOptions` are removed. `Description`, `Initials` and `ImageURL` are always sent, so they can be cleared. `GetAllSpacesOptions` is now `ListSpacesOptions`, with `Purpose SpacePurpose` and `IncludeAuthorizedPurposes bool` ([#6](https://github.com/ottramst/go-kibana/issues/6))
* roles: `Privilege` is now `KibanaPrivilege` and its `Feature` is a `map[string][]string`, so it works with every Kibana feature (the `Feature` struct is removed); `ElasticsearchIndex` is now `IndexPrivilege`, and the anonymous structs are replaced with named types. `CreateOrUpdate` takes the role as a `*Role`; `CreateOrUpdateRoleOptions` now only holds query options (`CreateOnly`). `CreateOrUpdate` returns only the response, because Kibana answers 204 No Content. `CreateOrUpdate` rejects Kibana privileges without `Spaces`, which Kibana would otherwise grant in all spaces ([#7](https://github.com/ottramst/go-kibana/issues/7))

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

* First release of the revived module, tested against Kibana 8.19.25 and 9.5.6 on Go 1.26 and 1.27. The [roadmap](https://github.com/ottramst/go-kibana/milestones) lists what comes next
* Requires `github.com/google/go-querystring` v1.2.0 ([#66](https://github.com/ottramst/go-kibana/pull/66))
* README rewritten with usage, compatibility and support policy, and roadmap sections; `Example*` functions on pkg.go.dev; `CLAUDE.md` with architecture notes and verified Kibana API quirks ([#9](https://github.com/ottramst/go-kibana/issues/9))
* `COVERAGE.md` lists every endpoint of the Kibana 8.19.25 and 9.5.6 OpenAPI specs and the method that implements it. `make coverage` regenerates it from the `// Kibana API:` annotations, and CI fails when it is stale ([#8](https://github.com/ottramst/go-kibana/issues/8))
* Acceptance tests run against Kibana 8.19.25 and 9.5.6 in CI, using `docker-compose.yml` locally ([#5](https://github.com/ottramst/go-kibana/issues/5))
* Repository tooling modernized: golangci-lint v2, a `GNUmakefile`, a hardened CI/CD pipeline, Dependabot, and community health files ([#2](https://github.com/ottramst/go-kibana/issues/2), [#3](https://github.com/ottramst/go-kibana/issues/3))
