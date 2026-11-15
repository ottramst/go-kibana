package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestAlertingService_GetRule(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rule/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/alerting/rule/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_create.json"))
	})
	got, _, err := client.Alerting.GetRule(context.Background(), "a/b?", nil)
	if err != nil {
		t.Fatalf("GetRule() error = %v", err)
	}
	at := time.Date(2026, 11, 15, 17, 42, 47, 693e6, time.UTC)
	if got.ID != "probe-rule-3" || got.RuleTypeID != ".index-threshold" || got.Consumer != "stackAlerts" || got.Enabled ||
		got.Schedule.Interval != "1m" || !reflect.DeepEqual(got.Tags, []string{"probe"}) || !got.CreatedAt.Equal(at) ||
		got.CreatedBy != "elastic" || got.APIKeyOwner != "" || got.NotifyWhen != "" || got.Throttle != "" {
		t.Errorf("GetRule() = %+v", got)
	}
	if got.ExecutionStatus.Status != kibana.RuleExecutionStatePending || !got.ExecutionStatus.LastExecutionDate.Equal(at) || got.LastRun != nil || !got.NextRun.IsZero() {
		t.Errorf("GetRule() status = %+v, last run %+v, next run %v", got.ExecutionStatus, got.LastRun, got.NextRun)
	}
	if len(got.Actions) != 1 {
		t.Fatalf("GetRule() actions = %+v, want 1", got.Actions)
	}
	a := got.Actions[0]
	if a.ID != "0b8d1a4e-0000-4000-8000-000000000002" || a.Group != "threshold met" || a.ConnectorTypeID != kibana.ConnectorTypeIDServerLog ||
		a.UUID == "" || a.Frequency == nil || a.Frequency.NotifyWhen != kibana.RuleNotifyWhenOnActionGroupChange || a.Frequency.Throttle != "" ||
		compactJSON(t, a.Params) != `{"message":"{{context.message}}"}` {
		t.Errorf("GetRule() action = %+v", a)
	}
}

func TestAlertingService_GetRule_status(t *testing.T) {
	tests := []struct {
		fixture string
		check   func(t *testing.T, r *kibana.Rule)
	}{
		{"alerting/rule_get_active.json", func(t *testing.T, r *kibana.Rule) {
			if r.ExecutionStatus.Status != kibana.RuleExecutionStateActive || r.ExecutionStatus.LastDuration == 0 || r.NextRun.IsZero() ||
				r.LastRun == nil || r.LastRun.Outcome != kibana.RuleLastRunOutcomeSucceeded || r.LastRun.AlertsCount.Active != 1 ||
				r.AlertDelay == nil || r.AlertDelay.Active != 2 || r.Flapping != nil || r.APIKeyOwner != "elastic" || r.ScheduledTaskID != r.ID {
				t.Errorf("rule = %+v, last run %+v", r, r.LastRun)
			}
		}},
		{"alerting/rule_get_error.json", func(t *testing.T, r *kibana.Rule) {
			es := r.ExecutionStatus
			if es.Status != kibana.RuleExecutionStateError || es.Error == nil || es.Error.Reason != kibana.RuleExecutionReasonExecute || es.Error.Message == "" {
				t.Errorf("execution status = %+v", es)
			}
			if lr := r.LastRun; lr == nil || lr.Outcome != kibana.RuleLastRunOutcomeFailed || lr.Warning != kibana.RuleExecutionReasonExecute || len(lr.OutcomeMsg) != 1 {
				t.Errorf("last run = %+v", lr)
			}
		}},
		// Kibana 8.19 returns the same shape.
		{"alerting/rule_get_8.19.json", func(t *testing.T, r *kibana.Rule) {
			if r.ID != "8c1f0000-0000-4000-8000-000000000001" || len(r.Actions) != 1 || r.Actions[0].ConnectorTypeID != kibana.ConnectorTypeIDServerLog {
				t.Errorf("rule = %+v", r)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/alerting/rule/{id}", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusOK, fixture(t, tt.fixture))
			})
			got, _, err := client.Alerting.GetRule(context.Background(), "x", nil)
			if err != nil {
				t.Fatalf("GetRule() error = %v", err)
			}
			tt.check(t, got)
		})
	}
}

func TestAlertingService_GetRule_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rule/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "alerting/rule_get_notfound.json"))
	})
	ctx := context.Background()
	if _, _, err := client.Alerting.GetRule(ctx, "nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("GetRule() error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.Alerting.GetRule(ctx, "", nil); err == nil {
		t.Error("GetRule(empty ID) error = nil, want an error")
	}
}

// newTestRule returns a rule with every writable field set, plus read-only
// fields that must not be sent.
func newTestRule() *kibana.Rule {
	return &kibana.Rule{
		ID:         "ignored-on-update",
		Name:       "r",
		RuleTypeID: ".index-threshold",
		Consumer:   "stackAlerts",
		Enabled:    true,
		Schedule:   kibana.RuleSchedule{Interval: "1m"},
		Params:     json.RawMessage(`{"index":["i"]}`),
		Actions: []kibana.RuleAction{{
			ID:        "c1",
			Group:     "threshold met",
			Params:    json.RawMessage(`{"message":"m"}`),
			Frequency: &kibana.RuleActionFrequency{NotifyWhen: kibana.RuleNotifyWhenOnActiveAlert},
			AlertsFilter: &kibana.RuleActionAlertsFilter{
				Query:     &kibana.RuleAlertsFilterQuery{KQL: "host.name:a"},
				Timeframe: &kibana.RuleAlertsFilterTimeframe{Hours: kibana.RuleAlertsFilterHours{Start: "08:00", End: "17:00"}, Timezone: "UTC"},
			},
			UUID:            "u1",
			ConnectorTypeID: kibana.ConnectorTypeIDServerLog,
		}},
		Tags:            []string{"t"},
		AlertDelay:      &kibana.RuleAlertDelay{Active: 2},
		Flapping:        &kibana.RuleFlapping{LookBackWindow: 10, StatusChangeThreshold: 3},
		Artifacts:       &kibana.RuleArtifacts{Dashboards: []kibana.RuleDashboard{{ID: "d1"}}},
		CreatedBy:       "elastic",
		CreatedAt:       time.Now(),
		APIKeyOwner:     "elastic",
		MuteAll:         true,
		MutedAlertIDs:   []string{"a"},
		ExecutionStatus: kibana.RuleExecutionStatus{Status: kibana.RuleExecutionStateOK},
		LastRun:         &kibana.RuleLastRun{Outcome: kibana.RuleLastRunOutcomeSucceeded},
		NextRun:         time.Now(),
		Revision:        3,
		ScheduledTaskID: "task",
	}
}

const testRuleBody = `"params":{"index":["i"]},` +
	`"actions":[{"id":"c1","group":"threshold met","params":{"message":"m"},` +
	`"frequency":{"summary":false,"notify_when":"onActiveAlert"},` +
	`"alerts_filter":{"query":{"kql":"host.name:a","filters":[]},` +
	`"timeframe":{"days":[],"hours":{"start":"08:00","end":"17:00"},"timezone":"UTC"}},"uuid":"u1"}],` +
	`"tags":["t"],"alert_delay":{"active":2},"artifacts":{"dashboards":[{"id":"d1"}]},` +
	`"flapping":{"look_back_window":10,"status_change_threshold":3}`

func TestAlertingService_CreateRule(t *testing.T) {
	tests := []struct {
		name string
		id   string
		path string
	}{
		{"generated ID", "", "/api/alerting/rule"},
		{"given ID", "a/b?", "/api/alerting/rule/a%2Fb%3F"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.EscapedPath(); got != tt.path {
					t.Errorf("path = %q, want %q", got, tt.path)
				}
				testHeader(t, r, "kbn-xsrf", "true")
				// Only the writable fields are sent: no ID, no connector type
				// in the actions.
				testJSONBody(t, r, `{"name":"r","rule_type_id":".index-threshold","consumer":"stackAlerts","enabled":true,"schedule":{"interval":"1m"},`+testRuleBody+`}`)
				writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_create.json"))
			})
			rule := newTestRule()
			rule.ID = tt.id
			before := *rule
			beforeFilter := *rule.Actions[0].AlertsFilter.Query
			beforeTimeframe := *rule.Actions[0].AlertsFilter.Timeframe
			got, _, err := client.Alerting.CreateRule(context.Background(), rule, nil)
			if err != nil {
				t.Fatalf("CreateRule() error = %v", err)
			}
			if got.ID != "probe-rule-3" {
				t.Errorf("CreateRule() = %+v, want the created rule", got)
			}
			if !reflect.DeepEqual(*rule, before) || !reflect.DeepEqual(*rule.Actions[0].AlertsFilter.Query, beforeFilter) ||
				!reflect.DeepEqual(*rule.Actions[0].AlertsFilter.Timeframe, beforeTimeframe) {
				t.Error("CreateRule() modified the rule")
			}
		})
	}
}

func TestAlertingService_CreateRule_minimal(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/alerting/rule", func(w http.ResponseWriter, r *http.Request) {
		// Kibana rejects null params, actions and tags, and "" for
		// notify_when and throttle. A zero Enabled creates a disabled rule.
		testJSONBody(t, r, `{"name":"r","rule_type_id":".index-threshold","consumer":"stackAlerts","enabled":false,"schedule":{"interval":"1m"},"params":{},"actions":[],"tags":[]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_create.json"))
	})
	rule := &kibana.Rule{Name: "r", RuleTypeID: ".index-threshold", Consumer: "stackAlerts", Schedule: kibana.RuleSchedule{Interval: "1m"}}
	if _, _, err := client.Alerting.CreateRule(context.Background(), rule, nil); err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}
}

func TestAlertingService_CreateRule_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusConflict, fixture(t, "alerting/rule_create_dup.json"))
	})
	ctx := context.Background()
	if _, _, err := client.Alerting.CreateRule(ctx, newTestRule(), nil); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("CreateRule() error = %v, want ErrConflict", err)
	}
	invalid := map[string]func(r *kibana.Rule){
		"no name":      func(r *kibana.Rule) { r.Name = "" },
		"no type":      func(r *kibana.Rule) { r.RuleTypeID = "" },
		"no consumer":  func(r *kibana.Rule) { r.Consumer = "" },
		"no interval":  func(r *kibana.Rule) { r.Schedule.Interval = "" },
		"no action ID": func(r *kibana.Rule) { r.Actions[0].ID = "" },
		"no frequency notify when": func(r *kibana.Rule) {
			r.Actions[0].Frequency = &kibana.RuleActionFrequency{Summary: true}
		},
		"bad ID": func(r *kibana.Rule) { r.ID = ".." },
	}
	for name, modify := range invalid {
		rule := newTestRule()
		modify(rule)
		if _, _, err := client.Alerting.CreateRule(ctx, rule, nil); err == nil || errors.Is(err, kibana.ErrConflict) {
			t.Errorf("CreateRule(%s) error = %v, want a validation error", name, err)
		}
	}
	if _, _, err := client.Alerting.CreateRule(ctx, nil, nil); err == nil {
		t.Error("CreateRule(nil) error = nil, want an error")
	}
}

func TestAlertingService_UpdateRule(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/alerting/rule/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("id"); got != "probe-rule-3" {
			t.Errorf("id = %q", got)
		}
		// No ID, type, consumer, enabled state or read-only fields.
		testJSONBody(t, r, `{"name":"r","schedule":{"interval":"1m"},`+testRuleBody+`}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_create.json"))
	})
	rule := newTestRule()
	before := *rule
	beforeFilter := *rule.Actions[0].AlertsFilter.Query
	beforeTimeframe := *rule.Actions[0].AlertsFilter.Timeframe
	if _, _, err := client.Alerting.UpdateRule(context.Background(), "probe-rule-3", rule, nil); err != nil {
		t.Fatalf("UpdateRule() error = %v", err)
	}
	if !reflect.DeepEqual(*rule, before) || !reflect.DeepEqual(*rule.Actions[0].AlertsFilter.Query, beforeFilter) ||
		!reflect.DeepEqual(*rule.Actions[0].AlertsFilter.Timeframe, beforeTimeframe) {
		t.Error("UpdateRule() modified the rule")
	}
}

func TestAlertingService_UpdateRule_readModifyWrite(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rule/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_create.json"))
	})
	mux.HandleFunc("PUT /api/alerting/rule/{id}", func(w http.ResponseWriter, r *http.Request) {
		// The rule that was read is written back without its read-only
		// fields; a nil Flapping is sent as null, which removes it.
		testJSONBody(t, r, `{"name":"renamed","schedule":{"interval":"1m"},`+
			`"params":{"index":["probe-idx"],"timeField":"@timestamp","aggType":"count","groupBy":"all","timeWindowSize":5,"timeWindowUnit":"m","thresholdComparator":">","threshold":[0]},`+
			`"actions":[{"id":"0b8d1a4e-0000-4000-8000-000000000002","group":"threshold met","params":{"message":"{{context.message}}"},`+
			`"frequency":{"summary":false,"notify_when":"onActionGroupChange"},"uuid":"07ae050f-b216-4ebb-a404-c92eb2115e86"}],`+
			`"tags":["probe"],"flapping":null}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rule_create.json"))
	})
	ctx := context.Background()
	rule, _, err := client.Alerting.GetRule(ctx, "probe-rule-3", nil)
	if err != nil {
		t.Fatalf("GetRule() error = %v", err)
	}
	rule.Name = "renamed"
	if _, _, err := client.Alerting.UpdateRule(ctx, rule.ID, rule, nil); err != nil {
		t.Fatalf("UpdateRule() error = %v", err)
	}
}

func TestAlertingService_UpdateRule_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/alerting/rule/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "alerting/rule_update_frequency.json"))
	})
	ctx := context.Background()
	if _, _, err := client.Alerting.UpdateRule(ctx, "x", newTestRule(), nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("UpdateRule() error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.Alerting.UpdateRule(ctx, "", newTestRule(), nil); err == nil {
		t.Error("UpdateRule(empty ID) error = nil, want an error")
	}
	if _, _, err := client.Alerting.UpdateRule(ctx, "x", nil, nil); err == nil {
		t.Error("UpdateRule(nil) error = nil, want an error")
	}
	// Type and consumer are not needed to update.
	rule := newTestRule()
	rule.RuleTypeID, rule.Consumer = "", ""
	if _, _, err := client.Alerting.UpdateRule(ctx, "x", rule, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("UpdateRule(no type) error = %v, want the server's ErrBadRequest", err)
	}
}

func TestAlertingService_DeleteRule(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/alerting/rule/{id}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		if got, want := r.URL.EscapedPath(), "/api/alerting/rule/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := context.Background()
	resp, err := client.Alerting.DeleteRule(ctx, "a/b?", nil)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteRule() = %v, %v, want 204", resp, err)
	}
	if _, err := client.Alerting.DeleteRule(ctx, "", nil); err == nil {
		t.Error("DeleteRule(empty ID) error = nil, want an error")
	}
}

func TestAlertingService_FindRules(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/alerting/rules/_find", func(w http.ResponseWriter, r *http.Request) {
		want := url.Values{
			"search":                  {"probe*"},
			"search_fields":           {"name"},
			"default_search_operator": {"AND"},
			"filter":                  {"alert.attributes.enabled:false"},
			// A single consumer must be a JSON array.
			"filter_consumers": {`["stackAlerts"]`},
			"has_reference":    {`{"type":"action","id":"c1"}`},
			"fields":           {"name", "tags"},
			"sort_field":       {"name"},
			"sort_order":       {"asc"},
			"page":             {"1"},
			"per_page":         {"2"},
		}
		if got := r.URL.Query(); !reflect.DeepEqual(got, want) {
			t.Errorf("query = %v, want %v", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rules_find.json"))
	})
	got, _, err := client.InSpace("x").Alerting.FindRules(context.Background(), &kibana.FindRulesOptions{
		Search:                "probe*",
		SearchFields:          []string{"name"},
		DefaultSearchOperator: kibana.SearchOperatorAnd,
		Filter:                "alert.attributes.enabled:false",
		FilterConsumers:       []string{"stackAlerts"},
		HasReference:          &kibana.SavedObjectRef{Type: "action", ID: "c1"},
		Fields:                []string{"name", "tags"},
		SortField:             "name",
		SortOrder:             kibana.SortDirectionAsc,
		Page:                  1,
		PerPage:               2,
	})
	if err != nil {
		t.Fatalf("FindRules() error = %v", err)
	}
	if got.Page != 1 || got.PerPage != 2 || got.Total != 5 || len(got.Rules) != 2 || got.Rules[0].ID != "probe-rule-3" {
		t.Errorf("FindRules() = %+v", got)
	}
}

func TestAlertingService_FindRules_noOptions(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rules/_find", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/rules_find.json"))
	})
	if _, _, err := client.Alerting.FindRules(context.Background(), nil); err != nil {
		t.Fatalf("FindRules() error = %v", err)
	}
	if _, _, err := client.Alerting.FindRules(context.Background(), &kibana.FindRulesOptions{HasReference: &kibana.SavedObjectRef{Type: "action"}}); err == nil {
		t.Error("FindRules(reference without ID) error = nil, want an error")
	}
}

func TestAlertingService_FindRules_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/alerting/rules/_find", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "alerting/rules_find_window.json"))
	})
	_, _, err := client.Alerting.FindRules(context.Background(), &kibana.FindRulesOptions{Page: 101, PerPage: 100})
	if !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("FindRules() error = %v, want ErrBadRequest", err)
	}
}

func TestAlertingService_FindRulesAll(t *testing.T) {
	mux, client := setup(t)
	pages := map[string]string{
		"1": `{"page":1,"per_page":2,"total":3,"data":[{"id":"a"},{"id":"b"}]}`,
		"2": `{"page":2,"per_page":2,"total":3,"data":[{"id":"c"}]}`,
	}
	mux.HandleFunc("GET /api/alerting/rules/_find", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("per_page") != "2" || q.Get("search") != "x" {
			t.Errorf("query = %v", q)
		}
		writeJSON(t, w, http.StatusOK, []byte(pages[q.Get("page")]))
	})
	var ids []string
	for rule, err := range client.Alerting.FindRulesAll(context.Background(), &kibana.FindRulesOptions{Search: "x", PerPage: 2}) {
		if err != nil {
			t.Fatalf("FindRulesAll() error = %v", err)
		}
		ids = append(ids, rule.ID)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("FindRulesAll() = %v, want %v", ids, want)
	}
}
