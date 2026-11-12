# go-kibana

[![CI/CD](https://github.com/ottramst/go-kibana/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/ottramst/go-kibana/actions/workflows/main.yml)
[![CodeQL](https://github.com/ottramst/go-kibana/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/ottramst/go-kibana/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/ottramst/go-kibana/badge)](https://scorecard.dev/viewer/?uri=github.com/ottramst/go-kibana)
[![Go Reference](https://pkg.go.dev/badge/github.com/ottramst/go-kibana.svg)](https://pkg.go.dev/github.com/ottramst/go-kibana)
[![Go Report Card](https://goreportcard.com/badge/github.com/ottramst/go-kibana)](https://goreportcard.com/report/github.com/ottramst/go-kibana)

An idiomatic Go client for the [Kibana REST API](https://www.elastic.co/docs/api/doc/kibana).

> [!NOTE]
> go-kibana is pre-1.0: the API may still change between minor versions. Breaking changes are
> listed in the [CHANGELOG](CHANGELOG.md). See the [roadmap](#roadmap) for what is planned.

## Features

- **Context-aware:** every method takes a `context.Context` for cancellation and deadlines.
- **Spaces built in:** scope a client to a Kibana space with `WithSpace` or `client.InSpace("id")`.
- **Typed errors:** `*kibana.ErrorResponse` carries Kibana's status code and message, and works
  with `errors.Is(err, kibana.ErrNotFound)`, `ErrConflict`, `ErrForbidden` and the other sentinels.
- **Round-trip friendly types:** read an object, change it, and write it back. Read-only fields
  are stripped from requests automatically.
- **Escape hatch:** `client.NewRequest` and `client.Do` reach any endpoint that has no method yet,
  with authentication, space scoping and error handling included.
- **Tested against real Kibana:** acceptance tests run against every supported Kibana version in CI.
- **One small dependency:** [go-querystring](https://github.com/google/go-querystring).

### Supported APIs

| API | Status |
| --- | --- |
| Spaces | CRUD ✅ · copy saved objects and resolve conflicts ✅ · share saved objects ✅ · disable legacy URL aliases ✅ |
| Roles | CRUD ✅ · query with an iterator ✅ · bulk create or update ✅ |
| User sessions | invalidate ✅ |
| Status, features, Task Manager health | get status ✅ · list features ✅ · Task Manager health ✅ |
| Short URLs | create, get, resolve and delete ✅ |
| Tags | planned ([v0.3.0](https://github.com/ottramst/go-kibana/milestone/3)) |
| Saved objects, encrypted saved objects | planned ([v0.4.0](https://github.com/ottramst/go-kibana/milestone/4)) |
| Data views | planned ([v0.5.0](https://github.com/ottramst/go-kibana/milestone/5)) |

[COVERAGE.md](COVERAGE.md) lists every endpoint of the Kibana OpenAPI specs and the method that
implements it.

## Installation

```shell
go get github.com/ottramst/go-kibana
```

## Usage

```go
import "github.com/ottramst/go-kibana"
```

Create a client with the Kibana URL and credentials, then use its services:

```go
client, err := kibana.NewClient("https://kibana.example.com:5601",
	kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
if err != nil {
	log.Fatal(err)
}

spaces, _, err := client.Spaces.List(ctx, nil)
```

Methods return the decoded result, the `*kibana.Response` (which wraps `*http.Response`) and an
error. Methods for endpoints without a response body, such as `Delete`, return only the
`*kibana.Response` and the error.

### Authentication

| Option | Header |
| --- | --- |
| `kibana.WithAPIKey(encoded)` | `Authorization: ApiKey <encoded>`. Use the base64 `encoded` value of an Elasticsearch API key. |
| `kibana.WithBasicAuth(user, password)` | `Authorization: Basic ...` |

For other schemes, such as a bearer token, set the header per request with
`kibana.WithHeader("Authorization", "Bearer ...")`, or for every request with an `http.RoundTripper`
passed in `kibana.WithHTTPClient`.

If Kibana runs under a base path (`server.basePath`), include it in the URL:
`kibana.NewClient("https://example.com/kibana", ...)`.

### Creating and updating objects

```go
space, _, err := client.Spaces.Create(ctx, &kibana.Space{
	ID:               "marketing",
	Name:             "Marketing",
	DisabledFeatures: []string{"dev_tools"},
})
if err != nil {
	return err
}

// Update replaces the whole object: read it, change it, write it back.
space, _, err = client.Spaces.Get(ctx, "marketing")
if err != nil {
	return err
}
space.Description = "Campaign dashboards"
space, _, err = client.Spaces.Update(ctx, space.ID, space)
```

Empty text fields are sent too, so setting `space.Description = ""` clears the description (likewise
`Initials` and `ImageURL`; an empty `Color` or `Solution` keeps the current value). Read-only
fields, such as `Space.Reserved` or `Role.TransientMetadata`, are never sent.

Roles use Kibana's feature privileges as plain maps, so every feature of your Kibana version works.
Every Kibana privilege must list its spaces (`"*"` for all spaces); `CreateOrUpdate` rejects an empty
list, which Kibana would otherwise treat as all spaces:

```go
_, err := client.Roles.CreateOrUpdate(ctx, "marketing_analyst", &kibana.Role{
	Elasticsearch: kibana.RoleElasticsearch{
		Indices: []kibana.IndexPrivilege{{Names: []string{"logs-*"}, Privileges: []string{"read"}}},
	},
	Kibana: []kibana.KibanaPrivilege{{
		Feature: map[string][]string{"discover": {"read"}, "dashboard": {"all"}},
		Spaces:  []string{"marketing"},
	}},
}, nil)
```

### Spaces

Space-aware endpoints act on the default space unless the client is scoped to another one:

```go
marketing := client.InSpace("marketing") // requests go to /s/marketing/api/...

// or, for a whole new client:
scoped, err := kibana.NewClient(kibanaURL, kibana.WithAPIKey(apiKey), kibana.WithSpace("marketing"))
```

`InSpace` returns a cheap copy that shares the HTTP client.

### Errors

```go
_, _, err := client.Spaces.Get(ctx, "marketing")
if errors.Is(err, kibana.ErrNotFound) {
	// the space does not exist
}

var errResp *kibana.ErrorResponse
if errors.As(err, &errResp) {
	log.Printf("Kibana answered %d: %s", errResp.StatusCode, errResp.Message)
}
```

### Timeouts, retries and custom transports

The default HTTP client has no timeout: pass a context with a deadline, or bring your own client.
The client does not retry. To retry, for example on the 503 Kibana returns while it starts, plug in a
retrying transport:

```go
client, err := kibana.NewClient(kibanaURL,
	kibana.WithAPIKey(apiKey),
	kibana.WithHTTPClient(&http.Client{Timeout: 30 * time.Second, Transport: myRetryingTransport}),
)
```

### Endpoints without a method

```go
req, err := client.NewRequest(ctx, http.MethodGet, "/api/status", nil)
if err != nil {
	return err
}
var status map[string]any
if _, err := client.Do(req, &status); err != nil {
	return err
}
```

The path is the escaped API path from the [Kibana API reference](https://www.elastic.co/docs/api/doc/kibana)
(escape IDs with `url.PathEscape`). The request gets the client's authentication, space scoping
and `kbn-xsrf` header; set `Content-Type` with `kibana.WithHeader` when the body is an `io.Reader`. Responses with
a non-2xx status return an `*ErrorResponse`. If the value passed to `Do` is an `io.Writer`, the
response body is copied into it as is.

More examples are in the [package documentation](https://pkg.go.dev/github.com/ottramst/go-kibana#pkg-examples).

## Compatibility

| | Supported |
| --- | --- |
| Go | The two newest releases (currently 1.26 and 1.27) |
| Kibana | 9.5.x and 8.19.x, tested in CI against 9.5.6 and 8.19.25 |

### Support policy

The client supports, and tests in CI, the latest patch of the newest Kibana 9.x minor release and
of 8.19, the last 8.x release line. Older versions get best-effort support. Endpoints that only some
supported versions have (for example tags, from Kibana 9.5) are documented on their methods.
Serverless projects should work for the endpoints they expose, but are not tested.

## Roadmap

The path to v1.0 is tracked in [milestones](https://github.com/ottramst/go-kibana/milestones). Each
epic issue lists its endpoints as sub-issues:

| Milestone | Scope |
| --- | --- |
| [v0.1.0](https://github.com/ottramst/go-kibana/milestone/1) | Foundation: client core, CI, acceptance tests, spaces and roles CRUD |
| [v0.2.0](https://github.com/ottramst/go-kibana/milestone/2) | Spaces saved object operations, role query and bulk, user sessions |
| [v0.3.0](https://github.com/ottramst/go-kibana/milestone/3) | Status, features, Task Manager health, short URLs, tags |
| [v0.4.0](https://github.com/ottramst/go-kibana/milestone/4) | Saved objects: import/export, key rotation, and the deprecated CRUD/bulk/find endpoints |
| [v0.5.0](https://github.com/ottramst/go-kibana/milestone/5) | Data views and runtime fields |
| [v1.0.0](https://github.com/ottramst/go-kibana/milestone/6) | API freeze and compatibility guarantees |

Alerting, connectors, Fleet, the Security solution and other areas are tracked as
[backlog epics](https://github.com/ottramst/go-kibana/issues?q=is%3Aissue+is%3Aopen+label%3Aepic+no%3Amilestone).
Contributions are welcome.

## Development

```shell
make test                      # unit tests
make lint                      # golangci-lint
docker compose up -d --wait    # Elasticsearch + Kibana on localhost:5601 (elastic / changeme)
make testacc                   # unit + acceptance tests against it
make coverage                  # regenerate COVERAGE.md
docker compose down -v         # stop the stack (about 4 GB of RAM)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

[MIT](LICENSE)
