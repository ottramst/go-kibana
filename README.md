# go-kibana

A Kibana API client enabling Go programs to interact with Kibana in a simple and uniform way

[![Build Status](https://github.com/ottramst/go-kibana/workflows/Lint%20and%20Test/badge.svg)](https://github.com/ottramst/go-kibana/actions?workflow=Lint%20and%20Test)
[![Sourcegraph](https://sourcegraph.com/github.com/ottramst/go-kibana/-/badge.svg)](https://sourcegraph.com/github.com/ottramst/go-kibana?badge)
[![GoDoc](https://godoc.org/github.com/ottramst/go-kibana?status.svg)](https://godoc.org/github.com/ottramst/go-kibana)
[![Go Report Card](https://goreportcard.com/badge/github.com/ottramst/go-kibana)](https://goreportcard.com/report/github.com/ottramst/go-kibana)

## Coverage

This API client package covers most of the existing Kibana API calls and is updated regularly
to add new and/or missing endpoints. Currently, the following services are supported:

- [ ] Get features API
- [ ] Get Task Manager health
- [x] Spaces
  - [x] Create space
  - [x] Update space
  - [x] Get space
  - [x] Get all spaces
  - [x] Delete space
  - [ ] Copy saved objects to space
  - [ ] Resolve copy to space conflicts
  - [ ] Disable legacy URL aliases
- [x] Roles
  - [x] Create or update role
  - [x] Get specific role
  - [x] Get all roles
  - [ ] Delete role
- [ ] User Sessions
  - [ ] Invalidate user sessions
- [ ] Saved Objects
  - [ ] Get object
  - [ ] Bulk get objects
  - [ ] Find objects
  - [ ] Create saved objects
  - [ ] Bulk create saved objects
  - [ ] Update object
  - [ ] Delete object
  - [ ] Export objects
  - [ ] Import objects
  - [ ] Resolve import errors
  - [ ] Resolve object
  - [ ] Bulk resolve objects
  - [ ] Rotate encryption key
- [ ] Index Patterns
  - [ ] Get index pattern
  - [ ] Create index pattern
  - [ ] Update index pattern
  - [ ] Delete index pattern
  - [ ] Get default index pattern
  - [ ] Set default index pattern
  - [ ] Update index pattern fields metadata
  - [ ] Get runtime field
  - [ ] Create runtime field
  - [ ] Upsert runtime field
  - [ ] Update runtime field
  - [ ] Delete runtime field
- [ ] Alerting
  - [ ] Create rule
  - [ ] Update rule
  - [ ] Get rule
  - [ ] Delete rule
  - [ ] Find rules
  - [ ] List rule types
  - [ ] Enable rule
  - [ ] Disable rule
  - [ ] Mute all alerts
  - [ ] Mute alert
  - [ ] Unmute all alerts
  - [ ] Unmute alert
  - [ ] Get Alerting framework health
- [ ] Actions and Connectors
  - [ ] Get connector
  - [ ] Get all connectors
  - [ ] List all connector types
  - [ ] Create connector
  - [ ] Update connector
  - [ ] Execute connector
  - [ ] Delete connector
- [ ] Import & Export Dashboards
  - [ ] Import dashboard 
  - [ ] Export dashboard
- [ ] Logstash Configuration Management
  - [ ] Delete pipeline
  - [ ] List pipeline
  - [ ] Create Logstash pipeline
  - [ ] Retrieve pipeline
- [ ] Machine Learning
  - [ ] Sync machine learning saved objects
- [ ] Short URLs
  - [ ] Create short URL
  - [ ] Get short URL
  - [ ] Delete short URL
  - [ ] Resolve short URL
- [ ] Upgrade Assistant
  - [ ] Upgrade readiness status
  - [ ] Start or resume reindex
  - [ ] Batch start or resume reindex
  - [ ] Batch reindex queue
  - [ ] Add default field
  - [ ] Check reindex status
  - [ ] Cancel reindex

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
