package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// StatusService handles communication with the status method of the Kibana
// API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-system
type StatusService service

// StatusLevel is the availability level of Kibana or one of its services.
type StatusLevel string

// Status levels, from best to worst.
const (
	StatusAvailable   StatusLevel = "available"
	StatusDegraded    StatusLevel = "degraded"
	StatusUnavailable StatusLevel = "unavailable"
	StatusCritical    StatusLevel = "critical"
)

// Status is the status of a Kibana instance.
//
// Kibana redacts the status of unauthenticated callers and, since 9.5, of
// callers without the "monitor" cluster privilege: only Status.Overall.Level
// is set, and the other fields are empty, including Version.Number.
type Status struct {
	// Name is the Kibana server name.
	Name string `json:"name"`

	// UUID is the unique ID of the Kibana instance.
	UUID    string        `json:"uuid"`
	Version StatusVersion `json:"version"`
	Status  StatusInfo    `json:"status"`

	// Metrics are the process, OS and request metrics of the instance, as
	// returned by Kibana.
	Metrics json.RawMessage `json:"metrics,omitempty"`
}

// StatusVersion is the version and build of a Kibana instance.
type StatusVersion struct {
	// Number is the Kibana version, such as "9.5.6".
	Number        string    `json:"number"`
	BuildHash     string    `json:"build_hash"`
	BuildNumber   int       `json:"build_number"`
	BuildSnapshot bool      `json:"build_snapshot"`
	BuildFlavor   string    `json:"build_flavor,omitempty"`
	BuildDate     time.Time `json:"build_date,omitzero"`
}

// StatusInfo is the overall status of a Kibana instance and the status of
// its services.
type StatusInfo struct {
	Overall ServiceStatus `json:"overall"`

	// Core maps core services, such as "elasticsearch" and "savedObjects",
	// to their status.
	Core map[string]ServiceStatus `json:"core,omitempty"`

	// Plugins maps plugin IDs to their status.
	Plugins map[string]ServiceStatus `json:"plugins,omitempty"`
}

// ServiceStatus is the status of Kibana or one of its services.
type ServiceStatus struct {
	Level   StatusLevel `json:"level"`
	Summary string      `json:"summary,omitempty"`
	Detail  string      `json:"detail,omitempty"`

	// Reported is true for plugins that report their own status.
	Reported bool `json:"reported,omitempty"`

	// Meta holds service-specific details.
	Meta json.RawMessage `json:"meta,omitempty"`
}

// Get returns the status of Kibana.
//
// Kibana answers 503 Service Unavailable when a core service, such as
// Elasticsearch or saved objects, is unavailable or critical. Get then
// returns the decoded status together with an [*ErrorResponse] whose
// StatusCode is 503, so the caller can see which services are affected. The
// status is nil if the body is not a status, e.g. from a proxy.
//
// A degraded Kibana, or one with unavailable plugins, answers 200 OK, so a
// nil error does not mean that Kibana is healthy: check
// Status.Overall.Level.
//
// Kibana API: GET /api/status
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-status
func (s *StatusService) Get(ctx context.Context, opts ...RequestOption) (*Status, *Response, error) {
	status := new(Status)
	resp, err := s.client.do(ctx, http.MethodGet, "/api/status", nil, status, opts)
	if err != nil {
		var errResp *ErrorResponse
		if errors.As(err, &errResp) && errResp.StatusCode == http.StatusServiceUnavailable {
			status = new(Status)
			if json.Unmarshal(errResp.Body, status) == nil && status.Status.Overall.Level != "" {
				return status, resp, err
			}
		}
		return nil, resp, err
	}
	return status, resp, nil
}
