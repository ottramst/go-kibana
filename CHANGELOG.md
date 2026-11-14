## Unreleased

BREAKING CHANGES:

* `Response` no longer embeds `*http.Response`, whose body was already read and closed. It has `StatusCode`, `Status` and `Header` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* `ErrorResponse` no longer has a `Response` field; it has `Method`, `URL` (without credentials) and `Header` instead. The format of `Error()` is not part of the API ([#43](https://github.com/ottramst/go-kibana/issues/43))
* `CheckResponse` is no longer exported; `Do` returns the `*ErrorResponse` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* status: `Get` takes `*GetStatusOptions`, and returns no status with the error on a 503: use `StatusFromError`. `Status.Status` (`StatusInfo`) is now `Status.Health` (`StatusHealth`), and the `Status*` level constants are `StatusLevel*` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* Every method takes a nil-able options struct before the request options, so that options can be added without breaking callers: pass `nil`. The new options types are named after the method, e.g. `GetSpaceOptions`, `DeleteRoleOptions` or `ListDataViewsOptions`; iterators share the options of their single-page method ([#43](https://github.com/ottramst/go-kibana/issues/43))
* Options and `…Request` structs have an unexported field: they can no longer be compared with `==`, and literals must name their fields ([#43](https://github.com/ottramst/go-kibana/issues/43))
* tags: `Update` is now `CreateOrUpdate`, since it creates missing tags; `TagList` is `ListTagsResult` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* roles: `BulkRolesResult` is `BulkCreateOrUpdateRolesResult`, `RoleElasticsearch` is `ElasticsearchPrivileges`, and `SortAsc`/`SortDesc` are `SortDirectionAsc`/`SortDirectionDesc` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* spaces: the deprecated `Copy*` aliases of the import result types are removed; `CopyRetry` is `CopySavedObjectsRetry`, and `ShareableReference.InboundReferences` holds `SavedObjectReference`s (`InboundReference` is removed) ([#43](https://github.com/ottramst/go-kibana/issues/43))
* saved objects: the `ImportError*` constants are `ImportErrorType*`, the `Resolve*` outcome constants are `ResolveOutcome*`, `BulkGetSavedObjectsRequest` is `BulkGetSavedObjectsItem`, and `BulkDelete` takes `BulkDeleteSavedObjectsOptions` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* Task Manager: the health constants are `TaskManagerHealthStatus*` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* short URLs: `LegacyShortURLLocator` is `LegacyShortURLLocatorID` ([#43](https://github.com/ottramst/go-kibana/issues/43))
* `RuntimeField.Type`, `ImportWarning.Type`, `SubFeaturePrivilegeGroup.GroupType` and `ResolvedSavedObject.AliasPurpose` have named string types (`RuntimeFieldType`, `ImportWarningType`, `SubFeaturePrivilegeGroupType`, `ResolveAliasPurpose`) with constants ([#43](https://github.com/ottramst/go-kibana/issues/43))

FEATURES:

* `ErrServiceUnavailable` matches 503 Service Unavailable responses ([#43](https://github.com/ottramst/go-kibana/issues/43))
* status: `StatusFromError` returns the status Kibana sends with a 503 ([#43](https://github.com/ottramst/go-kibana/issues/43))

## v0.5.0

FEATURES:

* New `DataViews` service: `List`, `Get`, `Create`, `Update` and `Delete` data views, and `GetDefault` and `SetDefault` for the default data view of a space ([#37](https://github.com/ottramst/go-kibana/issues/37), [#38](https://github.com/ottramst/go-kibana/issues/38))
* data views: `UpdateFieldsMetadata` changes the custom labels, descriptions, popularity and formats of fields ([#39](https://github.com/ottramst/go-kibana/issues/39))
* data views: `CreateRuntimeField`, `CreateOrUpdateRuntimeField`, `GetRuntimeField`, `UpdateRuntimeField` and `DeleteRuntimeField`, including composite runtime fields ([#40](https://github.com/ottramst/go-kibana/issues/40))
* data views: `SwapReferences` and `PreviewSwapReferences` move the references of saved objects from one data view to another ([#41](https://github.com/ottramst/go-kibana/issues/41))

NOTES:

* Tested against Kibana 8.19.25 and 9.5.6, which behave identically for data views. Kibana answers 400, not 409, for an existing data view ID; `SetDefault` always sends `force`, because Kibana otherwise keeps an existing default without an error; and Kibana fails to return composite runtime fields with `GetRuntimeField` (400, and 404 for their subfields), although it creates them ([#36](https://github.com/ottramst/go-kibana/issues/36))

## v0.4.0

FEATURES:

* New `SavedObjects` service: `Export` streams an ndjson export to an `io.Writer`, `Import` imports one from an `io.Reader` (streamed as `multipart/form-data`), and `ResolveImportErrors` retries the objects that failed ([#29](https://github.com/ottramst/go-kibana/issues/29), [#30](https://github.com/ottramst/go-kibana/issues/30), [#31](https://github.com/ottramst/go-kibana/issues/31))
* New `EncryptedSavedObjects` service: `RotateKey` rotates the encryption key of encrypted saved objects ([#32](https://github.com/ottramst/go-kibana/issues/32))
* `ErrTooManyRequests` matches 429 Too Many Requests responses ([#32](https://github.com/ottramst/go-kibana/issues/32))
* saved objects: the deprecated single-object API, `Create`, `Get`, `Update` and `Delete`, with the `SavedObject` type, whose `Attributes` are raw JSON ([#33](https://github.com/ottramst/go-kibana/issues/33))
* saved objects: the deprecated bulk API, `BulkCreate`, `BulkGet`, `BulkUpdate`, `BulkDelete` and `BulkResolve`; per-object errors are reported in the result ([#34](https://github.com/ottramst/go-kibana/issues/34))
* saved objects: the deprecated `Find`, with `FindAll` iterating over all matches (`iter.Seq2`), and `Resolve` ([#35](https://github.com/ottramst/go-kibana/issues/35))

NOTES:

* spaces: the copy result types are now type aliases of the new import result types, which have the same format, and are deprecated: `CopySavedObjectsResult` is `ImportSavedObjectsResult`, `CopySavedObjectsSuccess` is `ImportSuccess`, `CopySavedObjectsError` is `ImportError`, `CopyErrorDetail` is `ImportErrorDetail`, `CopyErrorDestination` is `ImportErrorDestination`, `CopyErrorType` is `ImportErrorType`, and the `CopyError*` constants are the `ImportError*` constants. Existing code keeps compiling. The result now also has `Warnings` ([#30](https://github.com/ottramst/go-kibana/issues/30))
* The acceptance test stack configures a decryption-only encryption key, to test key rotation ([#32](https://github.com/ottramst/go-kibana/issues/32))
* Tested against Kibana 8.19.25 and 9.5.6, which return the same shapes for all saved objects endpoints. Kibana deprecated the single-object, bulk, find and resolve endpoints in 8.7, so their methods are marked `Deprecated`. Their replacements are the APIs of the object types, such as data views, or import and export; `Resolve` and `BulkResolve` have no replacement. `BulkUpdate` sends PUT, the only method both versions accept, although the Kibana 8.19 API specification documents POST

## v0.3.0

FEATURES:

* New `Status` service: `Get` returns the Kibana version and the status of its services. When Kibana answers 503, it returns the decoded status together with the error ([#20](https://github.com/ottramst/go-kibana/issues/20))
* New `Features` service: `List` returns the Kibana features and their privileges, for building role feature privileges ([#21](https://github.com/ottramst/go-kibana/issues/21))
* New `TaskManager` service: `Health` returns the Task Manager health; the statistics sections are kept as raw JSON ([#22](https://github.com/ottramst/go-kibana/issues/22))
* New `ShortURLs` service: `Create`, `Get`, `Resolve` (by slug) and `Delete` ([#24](https://github.com/ottramst/go-kibana/issues/24), [#25](https://github.com/ottramst/go-kibana/issues/25))
* New `Tags` service (Kibana 9.5+, experimental): `List`, `ListAll` (`iter.Seq2`), `Get`, `Create`, `Update` (creates missing tags) and `Delete` ([#27](https://github.com/ottramst/go-kibana/issues/27))

NOTES:

* Tested against Kibana 8.19.25 and 9.5.6. Features, short URLs and tags are technical preview or experimental in Kibana. The tags API only exists in Kibana 9.5+, and Kibana 8.19 answers `ErrNotFound`. The response types were verified against both versions where the specs are untyped or wrong: the 8.19 spec has short URL dates as strings, but both versions send epoch milliseconds

## v0.2.0

FEATURES:

* spaces: `CopySavedObjects` and `ResolveCopySavedObjectsErrors` copy saved objects between spaces and resolve copy conflicts; per-object errors are reported in the result ([#12](https://github.com/ottramst/go-kibana/issues/12))
* spaces: `DisableLegacyURLAliases` ([#13](https://github.com/ottramst/go-kibana/issues/13))
* spaces: `GetShareableReferences` and `UpdateObjectsSpaces` share saved objects between spaces ([#14](https://github.com/ottramst/go-kibana/issues/14))
* roles: `Query` returns a page of roles, and `QueryAll` iterates over all matching roles (`iter.Seq2`) ([#16](https://github.com/ottramst/go-kibana/issues/16))
* roles: `BulkCreateOrUpdate` creates or updates several roles in one request; per-role errors are reported in the result ([#17](https://github.com/ottramst/go-kibana/issues/17))
* New `UserSessions` service: `Invalidate` invalidates user sessions ([#18](https://github.com/ottramst/go-kibana/issues/18))

NOTES:

* Tested against Kibana 8.19.25 and 9.5.6. The OpenAPI specs have no response schemas for copy and sharing (in either version) or for role query and bulk (in 8.19), so the response types were verified against both versions

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
