package kibana_test

import (
	"slices"
	"testing"
)

func TestAccStatus(t *testing.T) {
	client := accClient(t)
	status, _, err := client.Status.Get(t.Context(), nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if status.Version.Number == "" {
		t.Fatal("Get() returned no version")
	}
	// Kibana answers 200 when it is degraded too.
	if status.Summary.Overall.Level == "" {
		t.Error("Overall.Level is empty")
	}
	if _, ok := status.Summary.Core["elasticsearch"]; !ok {
		t.Error("Core has no elasticsearch status")
	}
	if len(status.Summary.Plugins) == 0 {
		t.Error("Plugins is empty")
	}
	t.Logf("Kibana %s, %s", status.Version.Number, status.Summary.Overall.Level)
}

func TestAccFeatures(t *testing.T) {
	client := accClient(t)
	features, _, err := client.Features.List(t.Context(), nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	ids := make([]string, 0, len(features))
	for _, f := range features {
		ids = append(ids, f.ID)
	}
	// Most feature IDs differ between the versions (dashboard and
	// dashboard_v2); advancedSettings is in both.
	i := slices.Index(ids, "advancedSettings")
	if i < 0 {
		t.Fatalf("List() = %v, want advancedSettings", ids)
	}
	f := features[i]
	if _, ok := f.Privileges["all"]; !ok {
		t.Errorf("advancedSettings privileges = %v, want all and read", f.Privileges)
	}
	if _, ok := f.Privileges["read"]; !ok {
		t.Errorf("advancedSettings privileges = %v, want all and read", f.Privileges)
	}
}

func TestAccTaskManagerHealth(t *testing.T) {
	client := accClient(t)
	health, _, err := client.TaskManager.Health(t.Context(), nil)
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
