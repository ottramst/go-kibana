# go-kibana

A Kibana API client enabling Go programs to interact with Kibana in a simple and uniform way

[![Build Status](https://github.com/ottramst/go-kibana/workflows/Lint%20and%20Test/badge.svg)](https://github.com/ottramst/go-kibana/actions?workflow=Lint%20and%20Test)
[![Sourcegraph](https://sourcegraph.com/github.com/ottramst/go-kibana/-/badge.svg)](https://sourcegraph.com/github.com/ottramst/go-kibana?badge)
[![GoDoc](https://godoc.org/github.com/ottramst/go-kibana?status.svg)](https://godoc.org/github.com/ottramst/go-kibana)
[![Go Report Card](https://goreportcard.com/badge/github.com/ottramst/go-kibana)](https://goreportcard.com/report/github.com/ottramst/go-kibana)

## Coverage

This API client package covers most of the existing Kibana API calls and is updated regularly
to add new and/or missing endpoints. Currently, the following services are supported:

- [ ] Spaces
  - [x] Create space
  - [x] Update space
  - [x] Get space
  - [x] Get all spaces
  - [x] Delete space
  - [ ] Copy saved objects to space
  - [ ] Resolve copy to space conflicts
  - [ ] Disable legacy URL aliases

## Usage

```go
import "github.com/ottramst/go-kibana"
```

Construct a new Kibana client, then use the various services on the client to
access different parts of the Kibana API. For example, to list all
spaces:

```go
kib, err := kibana.NewClient("<APIKEY>")
if err != nil {
  log.Fatalf("Failed to create client: %v", err)
}
spaces, _, err := kib.Spaces.GetAllSpaces(&kibana.GetAllSpacesOptions{})
```

There are a few `With...` option functions that can be used to customize
the API client. For example, to set a custom base URL:

```go
kib, err := kibana.NewClient("<APIKEY>", kibana.WithBaseURL("https://kibana.com"))
if err != nil {
  log.Fatalf("Failed to create client: %v", err)
}
spaces, _, err := kib.Spaces.GetAllSpaces(&kibana.GetAllSpacesOptions{})
```

Some API methods have optional parameters that can be passed. For example,
to update a specific space:

```go
kib, _ := kibana.NewClient("<APIKEY>")
opt := &UpdateSpaceOptions{Name: kibana.String("Test Space"), Description: kibana.String("This is a test space")}
projects, _, err := kib.Spaces.UpdateSpace("test-space", opt)
```

### Examples

TODO

For complete usage of go-kibana, see the full [package docs](https://godoc.org/github.com/ottramst/go-kibana).

## Issues

- If you have an issue: report it on the [issue tracker](https://github.com/ottramst/go-kibana/issues)
