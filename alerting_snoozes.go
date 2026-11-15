package kibana

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// kibanaTimeLayout is the only date format that Kibana accepts for snooze
// times: UTC with milliseconds.
const kibanaTimeLayout = "2006-01-02T15:04:05.000Z"

// formatKibanaTime formats t in kibanaTimeLayout.
func formatKibanaTime(t time.Time) string {
	return t.UTC().Format(kibanaTimeLayout)
}

// SnoozeSchedule is a time during which the actions of a rule don't run.
type SnoozeSchedule struct {
	_ [0]func()

	// ID is set by Kibana.
	ID     string         `json:"id,omitempty"`
	Custom CustomSchedule `json:"custom"`
}

// CustomSchedule is a time range, optionally recurring.
type CustomSchedule struct {
	_ [0]func()

	// Start is when the first occurrence begins. Kibana stores it with
	// millisecond precision.
	Start time.Time `json:"start"`

	// Duration is the length of each occurrence, such as "30m", "2h" or
	// "1d".
	Duration string `json:"duration"`

	// Timezone is the IANA time zone in which the recurrence is computed,
	// such as "Europe/Tallinn"; UTC if empty.
	Timezone  string              `json:"timezone,omitempty"`
	Recurring *ScheduleRecurrence `json:"recurring,omitempty"`
}

// ScheduleRecurrence is how a [CustomSchedule] repeats.
type ScheduleRecurrence struct {
	_ [0]func()

	// Every is the interval between occurrences, such as "1d", "2w", "1M"
	// or "1y".
	Every string `json:"every,omitempty"`

	// End and Occurrences limit the recurrence; it repeats forever if both
	// are zero.
	End         time.Time `json:"end,omitzero"`
	Occurrences int       `json:"occurrences,omitempty"`

	// OnWeekDay, OnMonthDay and OnMonth limit the occurrences to these
	// weekdays ("MO" to "SU", or with a position, such as "+1MO" for the
	// first Monday), days of the month (1 to 31) and months (1 to 12).
	OnWeekDay  []string `json:"onWeekDay,omitempty"`
	OnMonthDay []int    `json:"onMonthDay,omitempty"`
	OnMonth    []int    `json:"onMonth,omitempty"`
}

// CreateSnoozeScheduleRequest is the body of
// [AlertingService.CreateSnoozeSchedule].
type CreateSnoozeScheduleRequest struct {
	_ [0]func()

	// Schedule is the schedule to add. Its ID is ignored.
	Schedule SnoozeSchedule
}

// snoozeScheduleRequestPayload is the body of snooze schedule requests.
type snoozeScheduleRequestPayload struct {
	Schedule struct {
		Custom customSchedulePayload `json:"custom"`
	} `json:"schedule"`
}

// customSchedulePayload is a CustomSchedule with the times as Kibana
// accepts them.
type customSchedulePayload struct {
	Start     string                     `json:"start"`
	Duration  string                     `json:"duration"`
	Timezone  string                     `json:"timezone,omitempty"`
	Recurring *scheduleRecurrencePayload `json:"recurring,omitempty"`
}

// scheduleRecurrencePayload is a ScheduleRecurrence with the end as Kibana
// accepts it.
type scheduleRecurrencePayload struct {
	Every       string   `json:"every,omitempty"`
	End         string   `json:"end,omitempty"`
	Occurrences int      `json:"occurrences,omitempty"`
	OnWeekDay   []string `json:"onWeekDay,omitempty"`
	OnMonthDay  []int    `json:"onMonthDay,omitempty"`
	OnMonth     []int    `json:"onMonth,omitempty"`
}

// newSnoozeScheduleRequestPayload validates a request and returns its body.
func newSnoozeScheduleRequestPayload(req *CreateSnoozeScheduleRequest) (*snoozeScheduleRequestPayload, error) {
	if req == nil {
		return nil, errors.New("kibana: snooze schedule request must not be nil")
	}
	c := req.Schedule.Custom
	if c.Start.IsZero() || c.Duration == "" {
		return nil, errors.New("kibana: snooze schedule Start and Duration must be set")
	}
	p := new(snoozeScheduleRequestPayload)
	p.Schedule.Custom = customSchedulePayload{Start: formatKibanaTime(c.Start), Duration: c.Duration, Timezone: c.Timezone}
	if r := c.Recurring; r != nil {
		rp := &scheduleRecurrencePayload{Every: r.Every, Occurrences: r.Occurrences, OnWeekDay: r.OnWeekDay, OnMonthDay: r.OnMonthDay, OnMonth: r.OnMonth}
		if !r.End.IsZero() {
			rp.End = formatKibanaTime(r.End)
		}
		p.Schedule.Custom.Recurring = rp
	}
	return p, nil
}

// CreateSnoozeScheduleOptions are the options of
// [AlertingService.CreateSnoozeSchedule].
type CreateSnoozeScheduleOptions struct {
	_ [0]func()
}

// CreateSnoozeSchedule adds a snooze schedule to a rule: the rule keeps
// running, but its actions don't during the schedule. It returns the schedule
// with its ID, which [AlertingService.DeleteSnoozeSchedule] takes. Kibana
// does not return a rule's snooze schedules otherwise.
//
// Kibana API: POST /api/alerting/rule/{id}/snooze_schedule
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id-snooze-schedule
func (s *AlertingService) CreateSnoozeSchedule(ctx context.Context, ruleID string, req *CreateSnoozeScheduleRequest, opt *CreateSnoozeScheduleOptions, opts ...RequestOption) (*SnoozeSchedule, *Response, error) {
	if err := requireID("rule ID", ruleID); err != nil {
		return nil, nil, err
	}
	payload, err := newSnoozeScheduleRequestPayload(req)
	if err != nil {
		return nil, nil, err
	}
	var result struct {
		Schedule SnoozeSchedule `json:"schedule"`
	}
	resp, err := s.client.do(ctx, http.MethodPost, pathf("/api/alerting/rule/%s/snooze_schedule", ruleID), payload, &result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return &result.Schedule, resp, nil
}

// DeleteSnoozeScheduleOptions are the options of
// [AlertingService.DeleteSnoozeSchedule].
type DeleteSnoozeScheduleOptions struct {
	_ [0]func()
}

// DeleteSnoozeSchedule deletes a snooze schedule of a rule. For a schedule
// that does not exist, Kibana answers [ErrNotFound], or, on Kibana 8.19
// when the rule has no schedules left, [ErrBadRequest].
//
// Kibana API: DELETE /api/alerting/rule/{ruleId}/snooze_schedule/{scheduleId}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-alerting-rule-ruleid-snooze-schedule-scheduleid
func (s *AlertingService) DeleteSnoozeSchedule(ctx context.Context, ruleID, scheduleID string, opt *DeleteSnoozeScheduleOptions, opts ...RequestOption) (*Response, error) {
	if err := requireID("rule ID", ruleID); err != nil {
		return nil, err
	}
	if err := requireID("snooze schedule ID", scheduleID); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/alerting/rule/%s/snooze_schedule/%s", ruleID, scheduleID), nil, nil, opt, opts)
}

// alertAction posts to an action endpoint of an alert of a rule, such as
// _mute.
func (s *AlertingService) alertAction(ctx context.Context, ruleID, alertID, action string, body, opt any, opts []RequestOption) (*Response, error) {
	if err := requireID("rule ID", ruleID); err != nil {
		return nil, err
	}
	if err := requireID("alert ID", alertID); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodPost, pathf("/api/alerting/rule/%s/alert/%s/", ruleID, alertID)+action, body, nil, opt, opts)
}

// MuteAlertOptions are the options of [AlertingService.MuteAlert].
type MuteAlertOptions struct {
	_ [0]func()

	// ValidateAlertsExistence makes Kibana answer [ErrNotFound] for an
	// alert that does not exist. Kibana 9.5 validates by default; set it
	// to false to mute an alert before it exists. Kibana 8.19 never
	// validates.
	ValidateAlertsExistence *bool `url:"validate_alerts_existence,omitempty"`
}

// MuteAlert mutes an alert of a rule, by its alert (instance) ID, such as
// a group name or "all documents" for an ungrouped index threshold rule:
// the actions of the alert no longer run. Its ID is then in
// Rule.MutedAlertIDs. Muting a muted alert does nothing, and so does muting
// an alert while all alerts are muted ([AlertingService.MuteAllAlerts]).
//
// Kibana API: POST /api/alerting/rule/{rule_id}/alert/{alert_id}/_mute
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-rule-id-alert-alert-id-mute
func (s *AlertingService) MuteAlert(ctx context.Context, ruleID, alertID string, opt *MuteAlertOptions, opts ...RequestOption) (*Response, error) {
	return s.alertAction(ctx, ruleID, alertID, "_mute", nil, opt, opts)
}

// UnmuteAlertOptions are the options of [AlertingService.UnmuteAlert].
type UnmuteAlertOptions struct {
	_ [0]func()
}

// UnmuteAlert undoes [AlertingService.MuteAlert]. Unmuting an alert that is
// not muted does nothing.
//
// Kibana API: POST /api/alerting/rule/{rule_id}/alert/{alert_id}/_unmute
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-rule-id-alert-alert-id-unmute
func (s *AlertingService) UnmuteAlert(ctx context.Context, ruleID, alertID string, opt *UnmuteAlertOptions, opts ...RequestOption) (*Response, error) {
	return s.alertAction(ctx, ruleID, alertID, "_unmute", nil, opt, opts)
}

// AlertSnoozeConditionType is the kind of an [AlertSnoozeCondition].
type AlertSnoozeConditionType string

// Kinds of alert snooze conditions.
const (
	// AlertSnoozeConditionTypeFieldChange ends the snooze when Field of
	// the alert changes.
	AlertSnoozeConditionTypeFieldChange AlertSnoozeConditionType = "field_change"

	// AlertSnoozeConditionTypeSeverityChange ends the snooze when the
	// severity of the alert changes.
	AlertSnoozeConditionTypeSeverityChange AlertSnoozeConditionType = "severity_change"

	// AlertSnoozeConditionTypeSeverityEquals ends the snooze when the
	// severity of the alert becomes Value.
	AlertSnoozeConditionTypeSeverityEquals AlertSnoozeConditionType = "severity_equals"
)

// AlertSeverity is the severity of an alert.
type AlertSeverity string

// Alert severities.
const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityMajor    AlertSeverity = "major"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityMinor    AlertSeverity = "minor"
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

// AlertSnoozeCondition ends an alert snooze early.
type AlertSnoozeCondition struct {
	_ [0]func()

	Type AlertSnoozeConditionType `json:"type"`

	// Field is the alert field of [AlertSnoozeConditionTypeFieldChange],
	// such as "host.name".
	Field string `json:"field,omitempty"`

	// Value is the severity of [AlertSnoozeConditionTypeSeverityEquals].
	Value AlertSeverity `json:"value,omitempty"`
}

// AlertSnoozeConditionOperator combines the conditions of an alert snooze.
type AlertSnoozeConditionOperator string

// Alert snooze condition operators.
const (
	AlertSnoozeConditionOperatorAny AlertSnoozeConditionOperator = "any"
	AlertSnoozeConditionOperatorAll AlertSnoozeConditionOperator = "all"
)

// SnoozeAlertRequest is the body of [AlertingService.SnoozeAlert]. At least
// one of ExpiresAt and Conditions must be set.
type SnoozeAlertRequest struct {
	_ [0]func()

	// ExpiresAt is when the snooze ends, in the future. Kibana stores it
	// with millisecond precision.
	ExpiresAt time.Time

	// Conditions end the snooze early, at most 10; ConditionOperator
	// combines them.
	Conditions        []AlertSnoozeCondition
	ConditionOperator AlertSnoozeConditionOperator
}

// snoozeAlertPayload is the body of alert snooze requests.
type snoozeAlertPayload struct {
	ExpiresAt         string                       `json:"expires_at,omitempty"`
	Conditions        []AlertSnoozeCondition       `json:"conditions,omitempty"`
	ConditionOperator AlertSnoozeConditionOperator `json:"condition_operator,omitempty"`
}

// SnoozeAlertOptions are the options of [AlertingService.SnoozeAlert].
type SnoozeAlertOptions struct {
	_ [0]func()
}

// SnoozeAlert snoozes an alert of a rule, by its alert (instance) ID: its
// actions don't run until req.ExpiresAt or until one of req.Conditions
// matches. The alert must exist: Kibana answers [ErrNotFound] otherwise.
// Kibana returns alert snoozes in the alert documents only, not in rules.
//
// Snoozing alerts needs Kibana 9.5 or later: Kibana 8.19 answers
// [ErrNotFound].
//
// Kibana API: POST /api/alerting/rule/{rule_id}/alert/{alert_id}/_snooze
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-rule-id-alert-alert-id-snooze
func (s *AlertingService) SnoozeAlert(ctx context.Context, ruleID, alertID string, req *SnoozeAlertRequest, opt *SnoozeAlertOptions, opts ...RequestOption) (*Response, error) {
	if req == nil || (req.ExpiresAt.IsZero() && len(req.Conditions) == 0) {
		return nil, errors.New("kibana: snooze alert request must have ExpiresAt or Conditions")
	}
	payload := snoozeAlertPayload{Conditions: req.Conditions, ConditionOperator: req.ConditionOperator}
	if !req.ExpiresAt.IsZero() {
		payload.ExpiresAt = formatKibanaTime(req.ExpiresAt)
	}
	return s.alertAction(ctx, ruleID, alertID, "_snooze", payload, opt, opts)
}

// UnsnoozeAlertOptions are the options of [AlertingService.UnsnoozeAlert].
type UnsnoozeAlertOptions struct {
	_ [0]func()
}

// UnsnoozeAlert undoes [AlertingService.SnoozeAlert]. Unsnoozing an alert
// that is not snoozed does nothing. It needs Kibana 9.5 or later, like
// SnoozeAlert.
//
// Kibana API: POST /api/alerting/rule/{rule_id}/alert/{alert_id}/_unsnooze
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-rule-id-alert-alert-id-unsnooze
func (s *AlertingService) UnsnoozeAlert(ctx context.Context, ruleID, alertID string, opt *UnsnoozeAlertOptions, opts ...RequestOption) (*Response, error) {
	return s.alertAction(ctx, ruleID, alertID, "_unsnooze", nil, opt, opts)
}
