package kibana_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestAlertingService_CreateSnoozeSchedule(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/alerting/rule/{id}/snooze_schedule", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/alerting/rule/a%2Fb%3F/snooze_schedule"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		// The ID is not sent, and times are UTC with milliseconds, the only
		// format Kibana accepts.
		testJSONBody(t, r, `{"schedule":{"custom":{"start":"2030-01-01T00:00:00.000Z","duration":"2h","timezone":"Europe/Tallinn",`+
			`"recurring":{"every":"1M","end":"2031-01-01T00:00:00.000Z","onMonthDay":[1,15]}}}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "alerting/snooze_schedule_create.json"))
	})
	tallinn := time.FixedZone("EET", 2*60*60)
	req := &kibana.CreateSnoozeScheduleRequest{Schedule: kibana.SnoozeSchedule{
		ID: "ignored",
		Custom: kibana.CustomSchedule{
			Start:    time.Date(2030, 1, 1, 2, 0, 0, 123, tallinn),
			Duration: "2h",
			Timezone: "Europe/Tallinn",
			Recurring: &kibana.ScheduleRecurrence{
				Every:      "1M",
				End:        time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC),
				OnMonthDay: []int{1, 15},
			},
		},
	}}
	got, _, err := client.Alerting.CreateSnoozeSchedule(context.Background(), "a/b?", req, nil)
	if err != nil {
		t.Fatalf("CreateSnoozeSchedule() error = %v", err)
	}
	c := got.Custom
	if got.ID != "6a45bc34-f63a-4498-a921-086fa6542b5e" || !c.Start.Equal(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) || c.Duration != "2h" ||
		c.Timezone != "Europe/Tallinn" || c.Recurring == nil || c.Recurring.Every != "1M" || len(c.Recurring.OnMonthDay) != 2 ||
		!c.Recurring.End.Equal(time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("CreateSnoozeSchedule() = %+v, recurring %+v", got, c.Recurring)
	}
	if req.Schedule.ID != "ignored" {
		t.Error("CreateSnoozeSchedule() modified the request")
	}
}

func TestAlertingService_CreateSnoozeSchedule_invalid(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	valid := kibana.CustomSchedule{Start: time.Now(), Duration: "1h"}
	for name, tt := range map[string]struct {
		id  string
		req *kibana.CreateSnoozeScheduleRequest
	}{
		"nil":         {"r", nil},
		"no start":    {"r", &kibana.CreateSnoozeScheduleRequest{Schedule: kibana.SnoozeSchedule{Custom: kibana.CustomSchedule{Duration: "1h"}}}},
		"no duration": {"r", &kibana.CreateSnoozeScheduleRequest{Schedule: kibana.SnoozeSchedule{Custom: kibana.CustomSchedule{Start: time.Now()}}}},
		"no rule ID":  {"", &kibana.CreateSnoozeScheduleRequest{Schedule: kibana.SnoozeSchedule{Custom: valid}}},
	} {
		if _, _, err := client.Alerting.CreateSnoozeSchedule(ctx, tt.id, tt.req, nil); err == nil {
			t.Errorf("CreateSnoozeSchedule(%s) error = nil, want an error", name)
		}
	}
}

func TestAlertingService_DeleteSnoozeSchedule(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/alerting/rule/{rule}/snooze_schedule/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/alerting/rule/a%2Fb%3F/snooze_schedule/s%2F1"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/alerting/rule/r/snooze_schedule/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "none" {
			writeJSON(t, w, http.StatusBadRequest, fixture(t, "alerting/snooze_schedule_none_8.19.json"))
			return
		}
		writeJSON(t, w, http.StatusNotFound, fixture(t, "alerting/snooze_schedule_notfound.json"))
	})
	ctx := context.Background()
	if resp, err := client.Alerting.DeleteSnoozeSchedule(ctx, "a/b?", "s/1", nil); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DeleteSnoozeSchedule() = %v, %v, want 204", resp, err)
	}
	if _, err := client.Alerting.DeleteSnoozeSchedule(ctx, "r", "bogus", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("DeleteSnoozeSchedule(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := client.Alerting.DeleteSnoozeSchedule(ctx, "r", "none", nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("DeleteSnoozeSchedule(8.19, no schedules) error = %v, want ErrBadRequest", err)
	}
	for _, ids := range [][2]string{{"", "s"}, {"r", ""}} {
		if _, err := client.Alerting.DeleteSnoozeSchedule(ctx, ids[0], ids[1], nil); err == nil {
			t.Errorf("DeleteSnoozeSchedule(%q, %q) error = nil, want an error", ids[0], ids[1])
		}
	}
}

func TestAlertingService_alertActions(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		action string
		query  string
		body   string
		call   func(c *kibana.Client, rule, alert string) (*kibana.Response, error)
	}{
		{"mute", "_mute", "", "", func(c *kibana.Client, rule, alert string) (*kibana.Response, error) {
			return c.Alerting.MuteAlert(ctx, rule, alert, nil)
		}},
		{"mute without validation", "_mute", "validate_alerts_existence=false", "", func(c *kibana.Client, rule, alert string) (*kibana.Response, error) {
			return c.Alerting.MuteAlert(ctx, rule, alert, &kibana.MuteAlertOptions{ValidateAlertsExistence: new(false)})
		}},
		{"unmute", "_unmute", "", "", func(c *kibana.Client, rule, alert string) (*kibana.Response, error) {
			return c.Alerting.UnmuteAlert(ctx, rule, alert, nil)
		}},
		{"snooze", "_snooze", "", `{"expires_at":"2030-01-01T00:00:00.000Z","conditions":[{"type":"severity_equals","value":"critical"},{"type":"field_change","field":"host.name"}],"condition_operator":"any"}`,
			func(c *kibana.Client, rule, alert string) (*kibana.Response, error) {
				return c.Alerting.SnoozeAlert(ctx, rule, alert, &kibana.SnoozeAlertRequest{
					// Times are sent in UTC with milliseconds, in at most 30
					// characters.
					ExpiresAt: time.Date(2030, 1, 1, 2, 0, 0, 999999, time.FixedZone("EET", 2*60*60)),
					Conditions: []kibana.AlertSnoozeCondition{
						{Type: kibana.AlertSnoozeConditionTypeSeverityEquals, Value: kibana.AlertSeverityCritical},
						{Type: kibana.AlertSnoozeConditionTypeFieldChange, Field: "host.name"},
					},
					ConditionOperator: kibana.AlertSnoozeConditionOperatorAny,
				}, nil)
			}},
		{"snooze with conditions only", "_snooze", "", `{"conditions":[{"type":"severity_change"}]}`, func(c *kibana.Client, rule, alert string) (*kibana.Response, error) {
			return c.Alerting.SnoozeAlert(ctx, rule, alert, &kibana.SnoozeAlertRequest{
				Conditions: []kibana.AlertSnoozeCondition{{Type: kibana.AlertSnoozeConditionTypeSeverityChange}},
			}, nil)
		}},
		{"unsnooze", "_unsnooze", "", "", func(c *kibana.Client, rule, alert string) (*kibana.Response, error) {
			return c.Alerting.UnsnoozeAlert(ctx, rule, alert, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/alerting/rule/{rule}/alert/{alert}/"+tt.action, func(w http.ResponseWriter, r *http.Request) {
				testHeader(t, r, "kbn-xsrf", "true")
				if r.PathValue("alert") == "nope" {
					writeJSON(t, w, http.StatusNotFound, fixture(t, "alerting/alert_notfound.json"))
					return
				}
				if got, want := r.URL.EscapedPath(), "/api/alerting/rule/a%2Fb%3F/alert/all%20documents/"+tt.action; got != want {
					t.Errorf("path = %q, want %q", got, want)
				}
				testQuery(t, r, tt.query)
				if tt.body != "" {
					testJSONBody(t, r, tt.body)
				} else if b, _ := io.ReadAll(r.Body); len(b) != 0 {
					t.Errorf("body = %s, want none", b)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			resp, err := tt.call(client, "a/b?", "all documents")
			if err != nil || resp.StatusCode != http.StatusNoContent {
				t.Fatalf("%s = %v, %v, want 204", tt.name, resp, err)
			}
			if _, err := tt.call(client, "r", "nope"); !errors.Is(err, kibana.ErrNotFound) {
				t.Errorf("%s(missing alert) error = %v, want ErrNotFound", tt.name, err)
			}
			for _, ids := range [][2]string{{"", "a"}, {"r", ""}} {
				if _, err := tt.call(client, ids[0], ids[1]); err == nil {
					t.Errorf("%s(%q, %q) error = nil, want an error", tt.name, ids[0], ids[1])
				}
			}
		})
	}
}

func TestAlertingService_SnoozeAlert_invalid(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	// Kibana requires an expiry or conditions.
	for name, req := range map[string]*kibana.SnoozeAlertRequest{
		"nil":   nil,
		"empty": {ConditionOperator: kibana.AlertSnoozeConditionOperatorAll},
	} {
		if _, err := client.Alerting.SnoozeAlert(ctx, "r", "a", req, nil); err == nil {
			t.Errorf("SnoozeAlert(%s) error = nil, want an error", name)
		}
	}
}
