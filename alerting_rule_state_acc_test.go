package kibana_test

import (
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccAlertingRuleState(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	rule := createRule(t, client, &kibana.Rule{Name: randName(t, "acc"), RuleTypeID: ".index-threshold", Consumer: "stackAlerts",
		Schedule: kibana.RuleSchedule{Interval: "1m"}, Params: testThresholdParams})

	get := func() *kibana.Rule {
		t.Helper()
		r, _, err := client.Alerting.GetRule(ctx, rule.ID, nil)
		if err != nil {
			t.Fatalf("GetRule() error = %v", err)
		}
		return r
	}
	if r := get(); r.Enabled || r.APIKeyOwner != "" {
		t.Fatalf("new rule = %+v, want disabled without an API key", r)
	}

	// Enabling twice is not an error.
	for range 2 {
		if _, err := client.Alerting.EnableRule(ctx, rule.ID, nil); err != nil {
			t.Fatalf("EnableRule() error = %v", err)
		}
	}
	if r := get(); !r.Enabled || r.APIKeyOwner == "" || r.ScheduledTaskID == "" {
		t.Errorf("enabled rule = %+v, want enabled with an API key and a task", r)
	}

	if _, err := client.Alerting.MuteAllAlerts(ctx, rule.ID, nil); err != nil {
		t.Fatalf("MuteAllAlerts() error = %v", err)
	}
	if r := get(); !r.MuteAll {
		t.Error("MuteAll = false after MuteAllAlerts()")
	}
	if _, err := client.Alerting.UnmuteAllAlerts(ctx, rule.ID, nil); err != nil {
		t.Fatalf("UnmuteAllAlerts() error = %v", err)
	}
	if r := get(); r.MuteAll {
		t.Error("MuteAll = true after UnmuteAllAlerts()")
	}

	before := get()
	if _, err := client.Alerting.UpdateRuleAPIKey(ctx, rule.ID, nil); err != nil {
		t.Fatalf("UpdateRuleAPIKey() error = %v", err)
	}
	if r := get(); r.APIKeyOwner == "" || !r.UpdatedAt.After(before.UpdatedAt) {
		t.Errorf("rule after UpdateRuleAPIKey() = %+v, want an API key owner and a later update time than %v", r, before.UpdatedAt)
	}

	if _, err := client.Alerting.DisableRule(ctx, rule.ID, &kibana.DisableRuleOptions{Untrack: true}); err != nil {
		t.Fatalf("DisableRule() error = %v", err)
	}
	if _, err := client.Alerting.DisableRule(ctx, rule.ID, nil); err != nil {
		t.Fatalf("DisableRule(disabled) error = %v", err)
	}
	if r := get(); r.Enabled {
		t.Error("Enabled = true after DisableRule()")
	}
}
