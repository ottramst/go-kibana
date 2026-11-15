package kibana_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestAccAlertingSnoozeSchedules(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	rule := createRule(t, client, &kibana.Rule{Name: randName(t, "acc"), RuleTypeID: ".index-threshold", Consumer: "stackAlerts",
		Schedule: kibana.RuleSchedule{Interval: "1m"}, Params: testThresholdParams})

	start := time.Now().Add(24 * time.Hour).Truncate(time.Millisecond)
	schedule, _, err := client.Alerting.CreateSnoozeSchedule(ctx, rule.ID, &kibana.SnoozeSchedule{
		Custom: kibana.CustomSchedule{
			Start:     start,
			Duration:  "2h",
			Timezone:  "Europe/Tallinn",
			Recurring: &kibana.ScheduleRecurrence{Every: "1w", OnWeekDay: []string{"MO", "FR"}, Occurrences: 3},
		},
	}, nil)
	if err != nil {
		t.Fatalf("CreateSnoozeSchedule() error = %v", err)
	}
	c := schedule.Custom
	if schedule.ID == "" || !c.Start.Equal(start) || c.Duration != "2h" || c.Timezone != "Europe/Tallinn" || c.Recurring == nil ||
		c.Recurring.Occurrences != 3 || !slices.Equal(c.Recurring.OnWeekDay, []string{"MO", "FR"}) {
		t.Errorf("CreateSnoozeSchedule() = %+v, recurring %+v", schedule, c.Recurring)
	}

	if _, err := client.Alerting.DeleteSnoozeSchedule(ctx, rule.ID, schedule.ID, nil); err != nil {
		t.Fatalf("DeleteSnoozeSchedule() error = %v", err)
	}
	// Kibana 8.19 answers 400 when the rule has no schedules left.
	if _, err := client.Alerting.DeleteSnoozeSchedule(ctx, rule.ID, schedule.ID, nil); !errors.Is(err, kibana.ErrNotFound) && !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("DeleteSnoozeSchedule(deleted) error = %v, want ErrNotFound or ErrBadRequest", err)
	}
}

func TestAccAlertingMuteAlert(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	rule := createRule(t, client, &kibana.Rule{Name: randName(t, "acc"), RuleTypeID: ".index-threshold", Consumer: "stackAlerts",
		Schedule: kibana.RuleSchedule{Interval: "1m"}, Params: testThresholdParams})
	mutedIDs := func() []string {
		t.Helper()
		r, _, err := client.Alerting.GetRule(ctx, rule.ID, nil)
		if err != nil {
			t.Fatalf("GetRule() error = %v", err)
		}
		return r.MutedAlertIDs
	}

	// Kibana 9.5 checks that the alert exists unless told not to; 8.19
	// never does.
	_, err := client.Alerting.MuteAlert(ctx, rule.ID, "a/1", nil)
	if compareVersions(kibanaVersion(t, client), "9.5.0") >= 0 {
		if !errors.Is(err, kibana.ErrNotFound) {
			t.Errorf("MuteAlert(missing alert) error = %v, want ErrNotFound", err)
		}
	} else if err != nil {
		t.Errorf("MuteAlert(missing alert) error = %v, want none before Kibana 9.5", err)
	}
	noValidation := &kibana.MuteAlertOptions{ValidateAlertsExistence: new(false)}
	for _, id := range []string{"a/1", "a 2"} {
		if _, err := client.Alerting.MuteAlert(ctx, rule.ID, id, noValidation); err != nil {
			t.Fatalf("MuteAlert(%q) error = %v", id, err)
		}
	}
	if got := mutedIDs(); !slices.Equal(got, []string{"a/1", "a 2"}) {
		t.Errorf("MutedAlertIDs = %q, want both alerts", got)
	}

	if _, err := client.Alerting.UnmuteAlert(ctx, rule.ID, "a/1", nil); err != nil {
		t.Fatalf("UnmuteAlert() error = %v", err)
	}
	if got := mutedIDs(); !slices.Equal(got, []string{"a 2"}) {
		t.Errorf("MutedAlertIDs = %q, want the alert that is still muted", got)
	}

	// Muting all alerts clears the alerts muted one by one.
	if _, err := client.Alerting.MuteAllAlerts(ctx, rule.ID, nil); err != nil {
		t.Fatalf("MuteAllAlerts() error = %v", err)
	}
	if got := mutedIDs(); len(got) != 0 {
		t.Errorf("MutedAlertIDs after MuteAllAlerts() = %q, want none", got)
	}
}

func TestAccAlertingSnoozeAlert_unsupported(t *testing.T) {
	client := accClient(t)
	if compareVersions(kibanaVersion(t, client), "9.5.0") >= 0 {
		t.Skip("Kibana 9.5+ can snooze alerts")
	}
	client = client.InSpace(createSpace(t, client))
	rule := createRule(t, client, &kibana.Rule{Name: randName(t, "acc"), RuleTypeID: ".index-threshold", Consumer: "stackAlerts",
		Schedule: kibana.RuleSchedule{Interval: "1m"}, Params: testThresholdParams})
	req := &kibana.SnoozeAlertRequest{ExpiresAt: time.Now().Add(time.Hour)}
	if _, err := client.Alerting.SnoozeAlert(t.Context(), rule.ID, "a", req, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("SnoozeAlert() error = %v, want ErrNotFound before Kibana 9.5", err)
	}
}

// TestAccAlertingSnoozeAlert snoozes a real alert: an index threshold rule
// over an index with one document.
func TestAccAlertingSnoozeAlert(t *testing.T) {
	client := accClient(t)
	skipBelow(t, client, "9.5.0")
	if !localStack() {
		t.Skip("needs the compose stack, to write the documents the rule alerts on")
	}
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	index := randName(t, "go-kibana-acc")
	if err := esRequest(t, "POST", "/"+index+"/_doc?refresh=true", fmt.Sprintf(`{"@timestamp":%q}`, time.Now().UTC().Format(time.RFC3339))); err != nil {
		t.Fatal(err)
	}
	cleanup(t, func(context.Context) error {
		return esRequest(t, "DELETE", "/"+index, "")
	})
	rule := createRule(t, client, &kibana.Rule{
		Name:       randName(t, "acc"),
		RuleTypeID: ".index-threshold",
		Consumer:   "stackAlerts",
		Enabled:    true,
		Schedule:   kibana.RuleSchedule{Interval: "5s"},
		Params: fmt.Appendf(nil, `{"index":[%q],"timeField":"@timestamp","aggType":"count","groupBy":"all",`+
			`"timeWindowSize":1,"timeWindowUnit":"h","thresholdComparator":">","threshold":[0]}`, index),
	})

	// An ungrouped index threshold rule has one alert, "all documents".
	const alertID = "all documents"
	deadline := time.Now().Add(60 * time.Second)
	for {
		r, _, err := client.Alerting.GetRule(ctx, rule.ID, nil)
		if err != nil {
			t.Fatalf("GetRule() error = %v", err)
		}
		if r.ExecutionStatus.Status == kibana.RuleExecutionStateActive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("rule status = %+v, want active within 60s", r.ExecutionStatus)
		}
		time.Sleep(2 * time.Second)
	}

	if _, err := client.Alerting.SnoozeAlert(ctx, rule.ID, "no such alert", &kibana.SnoozeAlertRequest{ExpiresAt: time.Now().Add(time.Hour)}, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("SnoozeAlert(missing alert) error = %v, want ErrNotFound", err)
	}
	if _, err := client.Alerting.SnoozeAlert(ctx, rule.ID, alertID, &kibana.SnoozeAlertRequest{
		ExpiresAt:         time.Now().Add(time.Hour),
		Conditions:        []kibana.AlertSnoozeCondition{{Type: kibana.AlertSnoozeConditionTypeSeverityChange}},
		ConditionOperator: kibana.AlertSnoozeConditionOperatorAny,
	}, nil); err != nil {
		t.Fatalf("SnoozeAlert() error = %v", err)
	}
	// Unsnoozing twice is not an error.
	for range 2 {
		if _, err := client.Alerting.UnsnoozeAlert(ctx, rule.ID, alertID, nil); err != nil {
			t.Fatalf("UnsnoozeAlert() error = %v", err)
		}
	}
	// The alert exists, so muting it needs no option.
	if _, err := client.Alerting.MuteAlert(ctx, rule.ID, alertID, nil); err != nil {
		t.Fatalf("MuteAlert(existing alert) error = %v", err)
	}
}
