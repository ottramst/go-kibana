# Upgrading to v1

v1.0.0 is the first stable release of go-kibana. From v1.0.0, the module follows
[semantic versioning](https://semver.org/): breaking changes need a new major version.

v1.0.0 reworks the API once, so that it can stay stable. This page lists every change from v0.5.0
that can break your code, with what to use instead.

## Client core

| v0.5.0 | v1.0.0 |
| --- | --- |
| `Response` embeds `*http.Response` | `Response{StatusCode, Status, Header}`. The body was already read before. For other fields of the HTTP response, such as `TLS` or the final `Request` after redirects, use a custom `http.RoundTripper` in `WithHTTPClient`. |
| `ErrorResponse.Response` | `ErrorResponse.Method`, `URL` (without credentials) and `Header` |
| `CheckResponse(resp)` | Removed. `Client.Do` returns the `*ErrorResponse`. |
| — | `ErrServiceUnavailable` for 503 responses |
| `Status.Get(ctx)` returns the status together with the error on a 503 | `Status.Get(ctx, nil)` returns only the error; `status, ok := kibana.StatusFromError(err)` |
| `status.Status.Overall` (`StatusInfo`) | `status.Health.Overall` (`StatusHealth`) |
| `StatusAvailable`, `StatusDegraded`, `StatusUnavailable`, `StatusCritical` | `StatusLevelAvailable`, `StatusLevelDegraded`, `StatusLevelUnavailable`, `StatusLevelCritical` |

## Method signatures

Every method takes a nil-able options struct before the variadic request options. Pass `nil` unless you
need an option:

| v0.5.0 | v1.0.0 |
| --- | --- |
| `client.Spaces.Get(ctx, "marketing")` | `client.Spaces.Get(ctx, "marketing", nil)` |
| `client.Tags.Update(ctx, id, tag)` | `client.Tags.CreateOrUpdate(ctx, id, tag, nil)` |
| `client.SavedObjects.BulkDelete(ctx, refs, &kibana.DeleteSavedObjectOptions{Force: true})` | `client.SavedObjects.BulkDelete(ctx, refs, &kibana.BulkDeleteSavedObjectsOptions{Force: true})` |

The new options types are named `<Method><Noun>Options`, e.g. `GetSpaceOptions`, `CreateShortURLOptions`,
`DeleteRoleOptions`, `ListDataViewsOptions` or `GetTaskManagerHealthOptions`. Iterators share the options
of their single-page method (`QueryRolesOptions` for `Query` and `QueryAll`; `SwapReferencesOptions` for
`SwapReferences` and `PreviewSwapReferences`).

Options and `…Request` structs now have an unexported field. Literals of them must name their fields
(`&kibana.ListSpacesOptions{Purpose: kibana.SpacePurposeAny}`), and they can't be compared with `==`.

## Renamed and removed identifiers

| v0.5.0 | v1.0.0 |
| --- | --- |
| `TagsService.Update` | `TagsService.CreateOrUpdate` |
| `TagList` | `ListTagsResult` |
| `BulkRolesResult` | `BulkCreateOrUpdateRolesResult` |
| `RoleElasticsearch` | `ElasticsearchPrivileges` |
| `SortAsc`, `SortDesc` | `SortDirectionAsc`, `SortDirectionDesc` |
| `CopySavedObjectsResult`, `CopySavedObjectsSuccess`, `CopySavedObjectsError`, `CopyErrorDetail`, `CopyErrorDestination`, `CopyErrorType` (deprecated aliases) | `ImportSavedObjectsResult`, `ImportSuccess`, `ImportError`, `ImportErrorDetail`, `ImportErrorDestination`, `ImportErrorType` |
| `CopyErrorConflict`, `CopyErrorAmbiguousConflict`, `CopyErrorMissingReferences`, `CopyErrorUnsupportedType`, `CopyErrorUnknown` (deprecated) | `ImportErrorTypeConflict`, `ImportErrorTypeAmbiguousConflict`, `ImportErrorTypeMissingReferences`, `ImportErrorTypeUnsupportedType`, `ImportErrorTypeUnknown` |
| `ImportErrorConflict`, `ImportErrorAmbiguousConflict`, `ImportErrorMissingReferences`, `ImportErrorUnsupportedType`, `ImportErrorUnknown` | `ImportErrorTypeConflict`, `ImportErrorTypeAmbiguousConflict`, `ImportErrorTypeMissingReferences`, `ImportErrorTypeUnsupportedType`, `ImportErrorTypeUnknown` |
| `CopyRetry` | `CopySavedObjectsRetry` |
| `InboundReference` | `SavedObjectReference` (same fields) |
| `ResolveExactMatch`, `ResolveAliasMatch`, `ResolveConflict` | `ResolveOutcomeExactMatch`, `ResolveOutcomeAliasMatch`, `ResolveOutcomeConflict` |
| `BulkGetSavedObjectsRequest` | `BulkGetSavedObjectsItem` |
| `DeleteSavedObjectOptions` for `BulkDelete` | `BulkDeleteSavedObjectsOptions` |
| `TaskManagerHealthOK`, `TaskManagerHealthWarning`, `TaskManagerHealthError`, `TaskManagerHealthUninitialized` | `TaskManagerHealthStatusOK`, `TaskManagerHealthStatusWarning`, `TaskManagerHealthStatusError`, `TaskManagerHealthStatusUninitialized` |
| `LegacyShortURLLocator` | `LegacyShortURLLocatorID` |
| `RuntimeField.Type`, `RuntimeSubField.Type` (`string`) | `RuntimeFieldType`, with `RuntimeFieldTypeKeyword` and the other types as constants |
| `ImportWarning.Type` (`string`) | `ImportWarningType` (`ImportWarningTypeSimple`, `ImportWarningTypeActionRequired`) |
| `SubFeaturePrivilegeGroup.GroupType` (`string`) | `SubFeaturePrivilegeGroupType` (`SubFeaturePrivilegeGroupTypeIndependent`, `SubFeaturePrivilegeGroupTypeMutuallyExclusive`) |
| `ResolvedSavedObject.AliasPurpose` (`string`) | `ResolveAliasPurpose` (`ResolveAliasPurposeConversion`, `ResolveAliasPurposeImport`) |

## Request and result types

| v0.5.0 | v1.0.0 |
| --- | --- |
