# go-kibana

[![CI/CD](https://github.com/ottramst/go-kibana/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/ottramst/go-kibana/actions/workflows/main.yml)
[![CodeQL](https://github.com/ottramst/go-kibana/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/ottramst/go-kibana/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/ottramst/go-kibana/badge)](https://scorecard.dev/viewer/?uri=github.com/ottramst/go-kibana)
[![Go Reference](https://pkg.go.dev/badge/github.com/ottramst/go-kibana.svg)](https://pkg.go.dev/github.com/ottramst/go-kibana)
[![Go Report Card](https://goreportcard.com/badge/github.com/ottramst/go-kibana)](https://goreportcard.com/report/github.com/ottramst/go-kibana)

An idiomatic Go client for the [Kibana REST API](https://www.elastic.co/docs/api/doc/kibana): spaces,
roles, saved objects, data views and more, tested against real Kibana 9.x and 8.19.

```go
client, err := kibana.NewClient("https://kibana.example.com:5601",
	kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
if err != nil {
	log.Fatal(err)
}
spaces, _, err := client.Spaces.List(ctx, nil)
```

```shell
go get github.com/ottramst/go-kibana
```

Authenticate with `WithAPIKey` (the base64 `encoded` value of an Elasticsearch API key) or
`WithBasicAuth`. If Kibana runs under a base path (`server.basePath`), include it in the URL.

## Features

- **Typed and context-aware:** every method takes a `context.Context` and an options struct (`nil` for
  the defaults), and returns typed results.
- **Spaces built in:** scope a client to a Kibana space with `client.InSpace("marketing")`.
- **Errors you can match:** `errors.Is(err, kibana.ErrNotFound)`, `ErrConflict` and the other sentinels;
  `*kibana.ErrorResponse` has Kibana's status code, message and body.
- **Read, modify, write:** read an object, change it and pass it back. Read-only fields are never sent.
- **Iterators** over paginated results, and **opt-in retries** with backoff and `Retry-After`.
- **Escape hatch:** `NewRequest` and `Do` call any endpoint, with authentication and spaces handled.
- **One small dependency:** [go-querystring](https://github.com/google/go-querystring).

## Supported APIs

| API | Methods |
| --- | --- |
| Spaces | CRUD · copy saved objects and resolve conflicts · share saved objects · disable legacy URL aliases |
| Roles | CRUD · query with an iterator · bulk create or update |
| User sessions | invalidate |
| Platform | status · features · Task Manager health |
| Short URLs | create, get, resolve, delete |
| Tags (Kibana 9.5+) | CRUD with an iterator |
| Saved objects | import, export, resolve import errors · CRUD, bulk, find and resolve (deprecated by Kibana) |
| Encrypted saved objects | rotate the encryption key |
| Data views | CRUD and the default data view · field metadata · runtime fields · reference swaps |

[COVERAGE.md](COVERAGE.md) maps every endpoint of the Kibana OpenAPI specs to its method.

## Cookbook

The [package documentation](https://pkg.go.dev/github.com/ottramst/go-kibana) explains the conventions;
each recipe links to a complete example.

**Update an object** ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-SpacesService.Update)).
Updates replace the object, so read it first. Empty text fields are sent, so `""` clears a field.

```go
space, _, err := client.Spaces.Get(ctx, "marketing", nil)
space.Description = "Campaign dashboards"
space, _, err = client.Spaces.Update(ctx, space.ID, space, nil)
```

**Grant access to one space** ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-RolesService.CreateOrUpdate)).
Feature privileges are plain maps, so every feature of your Kibana version works. A privilege must
list its spaces (`"*"` for all).

```go
_, err := client.Roles.CreateOrUpdate(ctx, "marketing_analyst", &kibana.Role{
	Elasticsearch: kibana.ElasticsearchPrivileges{
		Indices: []kibana.IndexPrivilege{{Names: []string{"logs-*"}, Privileges: []string{"read"}}},
	},
	Kibana: []kibana.KibanaPrivilege{{
		Feature: map[string][]string{"discover": {"read"}, "dashboard": {"all"}},
		Spaces:  []string{"marketing"},
	}},
}, nil)
```

**Copy dashboards to another space or Kibana** ([between spaces](https://pkg.go.dev/github.com/ottramst/go-kibana#example-SpacesService.CopySavedObjects),
[between instances](https://pkg.go.dev/github.com/ottramst/go-kibana#example-SavedObjectsService.Import-FromAnotherKibana)).
`Spaces.CopySavedObjects` copies within one Kibana. Across instances, stream `SavedObjects.Export` into
`SavedObjects.Import`. Both succeed when some objects fail, so check `result.Errors`.

**Handle errors** ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-ErrorResponse)).

```go
_, _, err := client.Spaces.Get(ctx, "marketing", nil)
var errResp *kibana.ErrorResponse
switch {
case errors.Is(err, kibana.ErrNotFound):
	// The space does not exist.
case errors.As(err, &errResp):
	log.Printf("Kibana answered %d: %s", errResp.StatusCode, errResp.Message)
}
```

`Status.Get` returns an error when Kibana answers 503; `kibana.StatusFromError(err)` returns the
status Kibana sent with it ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-StatusService.Get)).

**Iterate over all results** ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-SavedObjectsService.FindAll)).
`QueryAll`, `ListAll` and `FindAll` fetch the pages as you go.

```go
for obj, err := range client.SavedObjects.FindAll(ctx, []string{"dashboard"}, nil) {
	if err != nil {
		return err
	}
	fmt.Println(obj.ID)
}
```

**Call an endpoint without a method** ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-Client.Do)).
`NewRequest` takes the escaped API path and adds authentication, the space and the headers Kibana
needs. `Do` decodes the response, or copies it into an `io.Writer`, and returns an `*ErrorResponse`
for non-2xx responses.

```go
req, err := client.NewRequest(ctx, http.MethodGet, "/api/alerting/rules/_find?per_page=10", nil)
var rules struct{ Total int `json:"total"` }
_, err = client.Do(req, &rules)
```

**Set timeouts and retries** ([example](https://pkg.go.dev/github.com/ottramst/go-kibana#example-WithRetry)).
The default HTTP client has no timeout, and the client only retries with `WithRetry`. It retries
429, 502, 503 and 504 responses and network errors, but not requests Kibana may have processed, such as
a POST answered with 502.

```go
client, err := kibana.NewClient(kibanaURL,
	kibana.WithAPIKey(apiKey),
	kibana.WithHTTPClient(&http.Client{Timeout: 2 * time.Minute}), // spans all attempts
	kibana.WithRetry(kibana.RetryPolicy{}),                         // 4 attempts, 1s to 30s backoff
)
```

For other authentication schemes, such as a bearer token, use `kibana.WithHeader` per request or an
`http.RoundTripper` in `WithHTTPClient`.

## Stability

From v1.0.0, go-kibana follows [semantic versioning](https://semver.org/): code that compiles against a
v1 release keeps compiling against later v1 releases. Breaking changes need a new major version
(`/v2`), and CI checks every pull request against the latest release. Adding fields, methods and
functions is not a breaking change, and whether a struct can be compared with `==` is not part of the
API. There are two exceptions, which follow Kibana:

- Endpoints that Kibana marks as technical preview or experimental (tags, short URLs and features) may
  change in a minor release if Kibana changes them.
- Methods of endpoints that Kibana deprecated (the saved objects CRUD, bulk, find and resolve
  endpoints) keep compiling, but return Kibana's error once Kibana removes the endpoint.

Upgrading from v0.x? See [Upgrading to v1](docs/upgrading-to-v1.md). Changes are listed in the
[CHANGELOG](CHANGELOG.md).

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

v1.0 covers the core platform APIs. Alerting, connectors, Fleet, the Security solution and other areas
are tracked as [backlog epics](https://github.com/ottramst/go-kibana/issues?q=is%3Aissue+is%3Aopen+label%3Aepic+no%3Amilestone)
and [milestones](https://github.com/ottramst/go-kibana/milestones). Contributions are welcome.

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
