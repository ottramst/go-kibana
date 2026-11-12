package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestTaskManagerService_Health(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/task_manager/_health", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		writeJSON(t, w, http.StatusOK, fixture(t, "taskmanager/health.json"))
	})

	got, _, err := client.TaskManager.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if got.ID != "bbd371be-a630-404f-9152-945ef42c7c65" || got.Status != kibana.TaskManagerHealthOK {
		t.Errorf("Health() = %s, %s; want the instance ID and OK", got.ID, got.Status)
	}
	if want := time.Date(2026, 11, 12, 16, 43, 27, 296e6, time.UTC); !got.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, want)
	}
	if got.LastUpdate.IsZero() {
		t.Error("LastUpdate is zero")
	}
	for _, section := range []string{"configuration", "runtime", "workload", "capacity_estimation"} {
		stat, ok := got.Stats[section]
		if !ok || stat.Status != kibana.TaskManagerHealthOK || stat.Timestamp.IsZero() || len(stat.Value) == 0 {
			t.Errorf("Stats[%s] = %+v, want OK with a timestamp and value", section, stat)
		}
	}
	if got.Stats["capacity_estimation"].Reason == "" {
		t.Error("Stats[capacity_estimation].Reason is empty")
	}

	// The sections are kept raw, to be decoded as needed.
	var config struct {
		PollInterval int `json:"poll_interval"`
	}
	if err := json.Unmarshal(got.Stats["configuration"].Value, &config); err != nil || config.PollInterval != 500 {
		t.Errorf("configuration poll_interval = %d, %v; want 500", config.PollInterval, err)
	}
}

func TestTaskManagerService_Health_spaceAndErrors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/task_manager/_health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, []byte(`{"statusCode":401,"error":"Unauthorized","message":"Unauthorized"}`))
	})
	got, _, err := client.InSpace("x").TaskManager.Health(context.Background())
	if !errors.Is(err, kibana.ErrUnauthorized) || got != nil {
		t.Errorf("Health() = %v, %v; want nil, ErrUnauthorized", got, err)
	}
}
