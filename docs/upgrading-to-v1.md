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

| v0.5.0 | v1.0.0 |
| --- | --- |

## Renamed and removed identifiers

| v0.5.0 | v1.0.0 |
| --- | --- |

## Request and result types

| v0.5.0 | v1.0.0 |
| --- | --- |
