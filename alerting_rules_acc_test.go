package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

// testThresholdParams are the params of an index threshold rule that counts
// the documents of an index that does not exist.
var testThresholdParams = json.RawMessage(`{"index":["go-kibana-acc-none"],"timeField":"@timestamp","aggType":"count","groupBy":"all",` +
	`"timeWindowSize":5,"timeWindowUnit":"m","thresholdComparator":">","threshold":[0]}`)

// createRule creates a rule and deletes it when the test ends.
func createRule(t *testing.T, client *kibana.Client, r *kibana.Rule) *kibana.Rule {
	t.Helper()
	created, _, err := client.Alerting.CreateRule(t.Context(), r, nil)
	if err != nil {
		t.Fatalf("creating rule %s: %v", r.Name, err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Alerting.DeleteRule(ctx, created.ID, nil)
		if errors.Is(err, kibana.ErrNotFound) {
			return nil
		}
		return err
	})
	return created
}

func TestAccAlertingRules(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	name := randName(t, "acc")
	connector := createConnector(t, client, &kibana.Connector{Name: name, ConnectorTypeID: kibana.ConnectorTypeIDServerLog})

	rule := &kibana.Rule{
		ID:         randName(t, "acc"),
		Name:       name,
		RuleTypeID: ".index-threshold",
		Consumer:   "stackAlerts",
		Schedule:   kibana.RuleSchedule{Interval: "1m"},
		Params:     testThresholdParams,
		Tags:       []string{"go-kibana"},
		Actions: []kibana.RuleAction{{
			ID:        connector.ID,
			Group:     "threshold met",
			Params:    json.RawMessage(`{"message":"{{context.message}}"}`),
			Frequency: &kibana.RuleActionFrequency{NotifyWhen: kibana.RuleNotifyWhenOnActionGroupChange},
			// Nil days, which the client sends as [], mean every day.
			AlertsFilter: &kibana.RuleActionAlertsFilter{Timeframe: &kibana.RuleAlertsFilterTimeframe{
				Hours:    kibana.RuleAlertsFilterHours{Start: "08:00", End: "17:00"},
				Timezone: "UTC",
			}},
		}},
		Artifacts:  &kibana.RuleArtifacts{InvestigationGuide: &kibana.RuleInvestigationGuide{Blob: "# Guide"}},
		AlertDelay: &kibana.RuleAlertDelay{Active: 2},
		Flapping:   &kibana.RuleFlapping{LookBackWindow: 10, StatusChangeThreshold: 3},
	}
	created := createRule(t, client, rule)
	if created.ID != rule.ID || created.Enabled || created.ExecutionStatus.Status != kibana.RuleExecutionStatePending || len(created.Actions) != 1 ||
		created.Actions[0].ConnectorTypeID != kibana.ConnectorTypeIDServerLog || created.Actions[0].UUID == "" || created.CreatedAt.IsZero() {
		t.Errorf("CreateRule() = %+v, want the new disabled rule", created)
	}
	if _, _, err := client.Alerting.CreateRule(ctx, rule, nil); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("CreateRule(same ID) error = %v, want ErrConflict", err)
	}
	generated := createRule(t, client, &kibana.Rule{Name: name + "-generated", RuleTypeID: ".index-threshold", Consumer: "stackAlerts",
		Schedule: kibana.RuleSchedule{Interval: "1m"}, Params: testThresholdParams})
	if generated.ID == "" || generated.Tags == nil || generated.Actions == nil {
		t.Errorf("CreateRule() = %+v, want a generated ID and empty tags and actions", generated)
	}

	got, _, err := client.Alerting.GetRule(ctx, rule.ID, nil)
	if err != nil {
		t.Fatalf("GetRule() error = %v", err)
	}
	if got.Name != name || got.Flapping == nil || got.AlertDelay == nil || !slices.Equal(got.Tags, rule.Tags) {
		t.Errorf("GetRule() = %+v, want %+v", got, created)
	}
	if len(got.Actions) != 1 {
		t.Fatalf("GetRule() actions = %+v, want 1", got.Actions)
	}
	if tf := got.Actions[0].AlertsFilter; tf == nil || tf.Timeframe == nil || tf.Timeframe.Days == nil || len(tf.Timeframe.Days) != 0 {
		t.Errorf("GetRule() alerts filter = %+v, want a timeframe with no days", tf)
	}
	// Kibana 8.19 accepts artifacts but does not store them; 9.5 stores
	// them. The versions in between are not tested.
	storesArtifacts := compareVersions(kibanaVersion(t, client), "9.5.0") >= 0
	if hasGuide := got.Artifacts != nil && got.Artifacts.InvestigationGuide != nil; hasGuide != storesArtifacts {
		t.Errorf("GetRule() artifacts = %+v, want stored: %v", got.Artifacts, storesArtifacts)
	}

	// Read, modify, write: the actions and settings of the rule that was
	// read are kept.
	got.Name = name + "-renamed"
	updated, _, err := client.Alerting.UpdateRule(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("UpdateRule() error = %v", err)
	}
	if updated.Name != got.Name || len(updated.Actions) != 1 || updated.Actions[0].UUID != got.Actions[0].UUID || updated.Flapping == nil ||
		updated.AlertDelay == nil || updated.Revision <= got.Revision {
		t.Errorf("UpdateRule() = %+v, want the renamed rule with its actions", updated)
	}

	if kept := updated.Artifacts != nil && updated.Artifacts.InvestigationGuide != nil; kept != storesArtifacts {
		t.Errorf("UpdateRule() artifacts = %+v, want them kept: %v", updated.Artifacts, storesArtifacts)
	}

	// A nil Flapping removes the flapping settings, and nil Artifacts
	// remove the artifacts; a nil AlertDelay keeps the delay.
	updated.Flapping, updated.AlertDelay, updated.Artifacts = nil, nil, nil
	updated, _, err = client.Alerting.UpdateRule(ctx, updated.ID, updated, nil)
	if err != nil {
		t.Fatalf("UpdateRule(no flapping) error = %v", err)
	}
	if updated.Flapping != nil || updated.AlertDelay == nil || updated.Artifacts != nil {
		t.Errorf("UpdateRule(no flapping) = flapping %+v, alert delay %+v, artifacts %+v, want only the delay", updated.Flapping, updated.AlertDelay, updated.Artifacts)
	}

	found, _, err := client.Alerting.FindRules(ctx, &kibana.FindRulesOptions{
		HasReference:    &kibana.SavedObjectRef{Type: "action", ID: connector.ID},
		FilterConsumers: []string{"stackAlerts"},
		Fields:          []string{"name"},
	})
	if err != nil {
		t.Fatalf("FindRules() error = %v", err)
	}
	if found.Total != 1 || len(found.Rules) != 1 || found.Rules[0].ID != rule.ID || found.Rules[0].Name != updated.Name {
		t.Errorf("FindRules(connector) = %+v, want the rule that uses the connector", found)
	}
	var ids []string
	for r, err := range client.Alerting.FindRulesAll(ctx, &kibana.FindRulesOptions{PerPage: 1, SortField: "name"}) {
		if err != nil {
			t.Fatalf("FindRulesAll() error = %v", err)
		}
		ids = append(ids, r.ID)
	}
	// "<name>-generated" sorts before "<name>-renamed".
	if want := []string{generated.ID, rule.ID}; !slices.Equal(ids, want) {
		t.Errorf("FindRulesAll() = %v, want %v", ids, want)
	}

	if _, err := client.Alerting.DeleteRule(ctx, rule.ID, nil); err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}
	if _, _, err := client.Alerting.GetRule(ctx, rule.ID, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("GetRule(deleted) error = %v, want ErrNotFound", err)
	}
}
