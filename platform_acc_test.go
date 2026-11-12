package kibana_test

import (
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccStatus(t *testing.T) {
	client := accClient(t)
	status, _, err := client.Status.Get(t.Context())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if status.Version.Number == "" {
		t.Fatal("Get() returned no version")
	}
	if status.Status.Overall.Level != kibana.StatusAvailable {
		t.Errorf("Overall.Level = %q, want %q", status.Status.Overall.Level, kibana.StatusAvailable)
	}
	if _, ok := status.Status.Core["elasticsearch"]; !ok {
		t.Error("Core has no elasticsearch status")
	}
	if len(status.Status.Plugins) == 0 {
		t.Error("Plugins is empty")
	}
	t.Logf("Kibana %s", status.Version.Number)

	// The test Kibana is never older than the oldest supported version.
	skipBelow(t, client, "8.19.0")
}

func TestAccFeatures(t *testing.T) {
	client := accClient(t)
	features, _, err := client.Features.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	ids := make([]string, 0, len(features))
	for _, f := range features {
		ids = append(ids, f.ID)
	}
	// dashboard_v2 replaced dashboard in 8.18, and both supported versions
	// have it.
	i := slices.Index(ids, "dashboard_v2")
	if i < 0 {
		t.Fatalf("List() = %v, want dashboard_v2", ids)
	}
	if f := features[i]; f.Privileges["all"] == nil || f.Privileges["read"] == nil {
		t.Errorf("dashboard_v2 privileges = %v, want all and read", f.Privileges)
	}
}

func TestAccTaskManagerHealth(t *testing.T) {
	client := accClient(t)
	health, _, err := client.TaskManager.Health(t.Context())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.Status == "" || health.Timestamp.IsZero() {
		t.Errorf("Health() = %+v, want a status and timestamp", health)
	}
	if _, ok := health.Stats["configuration"]; !ok {
		t.Errorf("Stats = %v, want a configuration section", health.Stats)
	}
}
