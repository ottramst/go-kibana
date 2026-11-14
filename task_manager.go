package kibana

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// TaskManagerService handles communication with the Task Manager methods of
// the Kibana API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-task-manager
type TaskManagerService service

// TaskManagerHealthStatus is the health of Task Manager or one of its
// statistics sections.
type TaskManagerHealthStatus string

// Task Manager health statuses.
const (
	TaskManagerHealthStatusOK            TaskManagerHealthStatus = "OK"
	TaskManagerHealthStatusWarning       TaskManagerHealthStatus = "warn"
	TaskManagerHealthStatusError         TaskManagerHealthStatus = "error"
	TaskManagerHealthStatusUninitialized TaskManagerHealthStatus = "uninitialized"
)

// TaskManagerHealth is the health of the Kibana Task Manager.
type TaskManagerHealth struct {
	_ [0]func()

	// ID is the UUID of the Kibana instance.
	ID     string                  `json:"id"`
	Status TaskManagerHealthStatus `json:"status"`

	// Timestamp is when Kibana computed the health, and LastUpdate when
	// it last updated the statistics.
	Timestamp  time.Time `json:"timestamp"`
	LastUpdate time.Time `json:"last_update"`

	// Reason explains a status other than TaskManagerHealthStatusOK.
	Reason string `json:"reason,omitempty"`

	// Stats maps statistics sections, such as "configuration", "runtime",
	// "workload" and "capacity_estimation", to their health.
	Stats map[string]TaskManagerHealthStat `json:"stats"`
}

// TaskManagerHealthStat is a statistics section of [TaskManagerHealth].
type TaskManagerHealthStat struct {
	_ [0]func()

	Status    TaskManagerHealthStatus `json:"status"`
	Timestamp time.Time               `json:"timestamp"`

	// Reason explains the status, if Kibana gives one.
	Reason string `json:"reason,omitempty"`

	// Value holds the statistics, whose fields depend on the section and
	// the Kibana version.
	Value json.RawMessage `json:"value,omitempty"`
}

// GetTaskManagerHealthOptions are the options of [TaskManagerService.Health].
type GetTaskManagerHealthOptions struct {
	_ [0]func()
}

// Health returns the health of the Kibana Task Manager, which runs
// background tasks such as alerting rules and reporting.
//
// Kibana API: GET /api/task_manager/_health
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-task-manager-health
func (s *TaskManagerService) Health(ctx context.Context, opt *GetTaskManagerHealthOptions, opts ...RequestOption) (*TaskManagerHealth, *Response, error) {
	health := new(TaskManagerHealth)
	resp, err := s.client.do(ctx, http.MethodGet, "/api/task_manager/_health", nil, health, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return health, resp, nil
}
