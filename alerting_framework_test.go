package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAlertingService_Health(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/_health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/health.json"))
	})
	got, _, err := client.Alerting.Health(context.Background(), nil)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	fh := got.AlertingFrameworkHealth
	if !got.IsSufficientlySecure || !got.HasPermanentEncryptionKey || fh.DecryptionHealth.Status != kibana.AlertingHealthStatusOK ||
		fh.ExecutionHealth.Status != kibana.AlertingHealthStatusOK || fh.ReadHealth.Status != kibana.AlertingHealthStatusOK || fh.ReadHealth.Timestamp.IsZero() {
		t.Errorf("Health() = %+v", got)
	}
}

func TestAlertingService_ListRuleTypes(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/alerting/rule_types", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_types.json"))
	})
	got, _, err := client.InSpace("x").Alerting.ListRuleTypes(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListRuleTypes() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListRuleTypes() = %d types, want 2", len(got))
	}
	rt := got[0]
	if rt.ID != ".index-threshold" || rt.Category != kibana.RuleTypeCategoryManagement || rt.Producer != "stackAlerts" || rt.DefaultActionGroupID != "threshold met" ||
		len(rt.ActionGroups) != 2 || rt.RecoveryActionGroup.ID != "recovered" || rt.MinimumLicenseRequired != kibana.LicenseLevelBasic ||
		rt.RuleTaskTimeout != "5m" || !rt.AuthorizedConsumers["stackAlerts"].All || !rt.EnabledInLicense || !rt.AutoRecoverAlerts ||
		len(rt.ActionVariables) == 0 || len(rt.Alerts) == 0 {
		t.Errorf("ListRuleTypes()[0] = %+v", rt)
	}
	if got[1].Category != kibana.RuleTypeCategoryObservability {
		t.Errorf("ListRuleTypes()[1] = %+v", got[1])
	}
}

func TestAlertingService_GetRuleQueryInspector(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rule/{id}/query_inspector", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/alerting/rule/a%2Fb%3F/query_inspector"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		testQuery(t, r, "alert_id=doc-1&mode=execute")
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/query_inspector.json"))
	})
	got, _, err := client.Alerting.GetRuleQueryInspector(context.Background(), "a/b?", &kibana.GetRuleQueryInspectorOptions{
		Mode:    kibana.QueryInspectorModeExecute,
		AlertID: "doc-1",
	})
	if err != nil {
		t.Fatalf("GetRuleQueryInspector() error = %v", err)
	}
	if len(got) != 1 || got[0].Index != "probe-live" || got[0].Label != "" || len(got[0].Request) == 0 || len(got[0].Response) == 0 {
		t.Errorf("GetRuleQueryInspector() = %+v", got)
	}
}

func TestAlertingService_GetRuleQueryInspector_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rule/{id}/query_inspector", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alert_id") != "" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "alerting/query_inspector_alert_notfound.json"))
			return
		}
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "alerting/query_inspector_unsupported.json"))
	})
	ctx := context.Background()
	if _, _, err := client.Alerting.GetRuleQueryInspector(ctx, "r", nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("GetRuleQueryInspector(unsupported type) error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.Alerting.GetRuleQueryInspector(ctx, "r", &kibana.GetRuleQueryInspectorOptions{AlertID: "bogus"}); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("GetRuleQueryInspector(unknown alert) error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.Alerting.GetRuleQueryInspector(ctx, "", nil); err == nil {
		t.Error("GetRuleQueryInspector(empty ID) error = nil, want an error")
	}
}
