package kibana_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccAlertingHealth(t *testing.T) {
	client := accClient(t)
	h, _, err := client.Alerting.Health(t.Context(), nil)
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	// The statuses depend on the rules that ran, so only check that the
	// checks decoded.
	if h.AlertingFrameworkHealth.ReadHealth.Status == "" || h.AlertingFrameworkHealth.ExecutionHealth.Timestamp.IsZero() {
		t.Errorf("Health() = %+v", h)
	}
}

func TestAccAlertingListRuleTypes(t *testing.T) {
	client := accClient(t)
	types, _, err := client.Alerting.ListRuleTypes(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListRuleTypes() error = %v", err)
	}
	i := slices.IndexFunc(types, func(rt *kibana.RuleType) bool { return rt.ID == ".index-threshold" })
	if i < 0 {
		t.Fatal("ListRuleTypes() has no .index-threshold type")
	}
	rt := types[i]
	if rt.Category != kibana.RuleTypeCategoryManagement || rt.DefaultActionGroupID == "" || rt.RecoveryActionGroup.ID == "" || !rt.AuthorizedConsumers["stackAlerts"].All {
		t.Errorf("ListRuleTypes() %s = %+v", rt.ID, rt)
	}
}

func TestAccAlertingQueryInspector(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	// The custom threshold rule supports the query inspector; the index
	// need not exist.
	rule := createRule(t, client, &kibana.Rule{
		Name:       randName(t, "acc"),
		RuleTypeID: "observability.rules.custom_threshold",
		Consumer:   "alerts",
		Schedule:   kibana.RuleSchedule{Interval: "1m"},
		Params: json.RawMessage(`{"criteria":[{"comparator":">","metrics":[{"name":"A","aggType":"count"}],"threshold":[0],"timeSize":1,"timeUnit":"h"}],` +
			`"alertOnNoData":false,"alertOnGroupDisappear":false,` +
			`"searchConfiguration":{"query":{"query":"","language":"kuery"},"index":{"title":"go-kibana-acc-none","timeFieldName":"@timestamp"}}}`),
	})

	queries, _, err := client.Alerting.GetRuleQueryInspector(ctx, rule.ID, nil)
	if compareVersions(kibanaVersion(t, client), "9.5.0") < 0 {
		if !errors.Is(err, kibana.ErrNotFound) {
			t.Errorf("GetRuleQueryInspector() error = %v, want ErrNotFound before Kibana 9.5", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("GetRuleQueryInspector() error = %v", err)
	}
	if len(queries) != 1 || queries[0].Index != "go-kibana-acc-none" || len(queries[0].Request) == 0 || queries[0].Response != nil {
		t.Errorf("GetRuleQueryInspector(build) = %+v, want one query without a response", queries)
	}

	queries, _, err = client.Alerting.GetRuleQueryInspector(ctx, rule.ID, &kibana.GetRuleQueryInspectorOptions{Mode: kibana.QueryInspectorModeExecute})
	if err != nil {
		t.Fatalf("GetRuleQueryInspector(execute) error = %v", err)
	}
	if len(queries) != 1 || !strings.Contains(string(queries[0].Response), `"hits"`) {
		t.Errorf("GetRuleQueryInspector(execute) = %+v, want the query and its response", queries)
	}

	// Index threshold rules don't support the query inspector.
	threshold := createRule(t, client, &kibana.Rule{Name: randName(t, "acc"), RuleTypeID: ".index-threshold", Consumer: "stackAlerts",
		Schedule: kibana.RuleSchedule{Interval: "1m"}, Params: testThresholdParams})
	if _, _, err := client.Alerting.GetRuleQueryInspector(ctx, threshold.ID, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("GetRuleQueryInspector(index threshold) error = %v, want ErrBadRequest", err)
	}
}
