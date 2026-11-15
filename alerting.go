package kibana

import (
	"encoding/json"
	"time"
)

// AlertingService handles communication with the alerting rule and
// alerting framework methods of the Kibana API. Rules belong to a space:
// use [Client.InSpace] to manage the rules of another space.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-alerting
type AlertingService service

// RuleNotifyWhen is when the actions of a rule run.
type RuleNotifyWhen string

// When actions run.
const (
	// RuleNotifyWhenOnActionGroupChange runs the actions when the alert
	// status changes.
	RuleNotifyWhenOnActionGroupChange RuleNotifyWhen = "onActionGroupChange"

	// RuleNotifyWhenOnActiveAlert runs the actions at every check interval
	// while the alert is active.
	RuleNotifyWhenOnActiveAlert RuleNotifyWhen = "onActiveAlert"

	// RuleNotifyWhenOnThrottleInterval runs the actions when the alert
	// becomes active, then at most once per throttle interval while it is
	// active.
	RuleNotifyWhenOnThrottleInterval RuleNotifyWhen = "onThrottleInterval"
)

// Rule is an alerting rule. The same type is used for reads and writes:
// fields set by Kibana are ignored when writing, and so are RuleTypeID,
// Consumer and Enabled when updating.
type Rule struct {
	_ [0]func()

	ID   string `json:"id"`
	Name string `json:"name"`

	// RuleTypeID is the type of the rule, such as ".index-threshold" or
	// ".es-query". [AlertingService.ListRuleTypes] returns the types of a
	// Kibana.
	RuleTypeID string `json:"rule_type_id"`

	// Consumer is the Kibana feature that owns the rule, such as
	// "stackAlerts", "alerts" or "observability"; it decides who may
	// access it.
	Consumer string `json:"consumer"`

	// Enabled rules run on their schedule. Unlike Kibana, which enables new
	// rules by default, [AlertingService.CreateRule] creates a disabled rule
	// when Enabled is false. Use [AlertingService.EnableRule] and
	// [AlertingService.DisableRule] to change it later.
	Enabled bool `json:"enabled"`

	Schedule RuleSchedule `json:"schedule"`

	// Params are the parameters of the rule type, as a JSON object. Nil is
	// sent as {}.
	Params json.RawMessage `json:"params"`

	// Actions run when the rule's alerts match their frequency. Nil is
	// sent as [].
	Actions []RuleAction `json:"actions"`
	Tags    []string     `json:"tags"`

	// NotifyWhen and Throttle set the action frequency for all actions.
	// Prefer RuleAction.Frequency: Kibana rejects rules that set both, and
	// deprecated rule-level throttling in 8.13.
	NotifyWhen RuleNotifyWhen `json:"notify_when,omitempty"`
	Throttle   string         `json:"throttle,omitempty"`

	// AlertDelay, if set, raises alerts only after several consecutive
	// runs. It is kept when an update leaves it out.
	AlertDelay *RuleAlertDelay `json:"alert_delay,omitempty"`

	// Flapping overrides the space's flapping detection settings for the
	// rule. [AlertingService.UpdateRule] with a nil Flapping removes it.
	Flapping *RuleFlapping `json:"flapping,omitempty"`

	// Artifacts link dashboards and an investigation guide to the rule.
	Artifacts *RuleArtifacts `json:"artifacts,omitempty"`

	// The following fields are set by Kibana.

	CreatedBy string    `json:"created_by,omitempty"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`

	// APIKeyOwner is the user whose API key the rule runs with, set while
	// the rule is enabled. APIKeyCreatedByUser is set when the key was
	// created by the user rather than by Kibana, as when the rule was
	// enabled with API key authentication.
	APIKeyOwner         string `json:"api_key_owner,omitempty"`
	APIKeyCreatedByUser bool   `json:"api_key_created_by_user,omitempty"`

	// MuteAll silences all the rule's actions; MutedAlertIDs silences the
	// actions of these alerts.
	MuteAll       bool     `json:"mute_all"`
	MutedAlertIDs []string `json:"muted_alert_ids,omitempty"`

	ExecutionStatus RuleExecutionStatus `json:"execution_status"`

	// LastRun is the outcome of the last run, if the rule has run.
	LastRun *RuleLastRun `json:"last_run,omitempty"`

	// NextRun is when the rule runs next, if it is enabled.
	NextRun time.Time `json:"next_run,omitzero"`

	// Revision counts the updates that changed what the rule does.
	Revision int  `json:"revision"`
	Running  bool `json:"running"`

	// ScheduledTaskID is the ID of the Task Manager task that runs the
	// rule, if it is enabled.
	ScheduledTaskID string `json:"scheduled_task_id,omitempty"`
}

// RuleSchedule is how often a rule runs.
type RuleSchedule struct {
	_ [0]func()

	// Interval is a duration in seconds, minutes, hours or days, such as
	// "30s", "5m", "1h" or "1d".
	Interval string `json:"interval"`
}

// RuleAction is an action of a rule: a connector to run with parameters.
type RuleAction struct {
	_ [0]func()

	// ID is the ID of the connector.
	ID string `json:"id"`

	// Group is the action group that runs the action, such as
	// "threshold met" or "recovered"; [RuleType] lists them. System
	// actions have none.
	Group string `json:"group,omitempty"`

	// Params are the parameters of the connector, as a JSON object; strings
	// may use mustache variables such as "{{context.message}}". Nil is
	// sent as {}.
	Params json.RawMessage `json:"params"`

	Frequency    *RuleActionFrequency    `json:"frequency,omitempty"`
	AlertsFilter *RuleActionAlertsFilter `json:"alerts_filter,omitempty"`

	// UseAlertDataForTemplate makes the alert document available to the
	// mustache templates of Params.
	UseAlertDataForTemplate bool `json:"use_alert_data_for_template,omitempty"`

	// UUID identifies the action within the rule. Kibana generates it; keep
	// it when updating a rule.
	UUID string `json:"uuid,omitempty"`

	// ConnectorTypeID is set by Kibana.
	ConnectorTypeID ConnectorTypeID `json:"connector_type_id,omitempty"`
}

// RuleActionFrequency is when an action runs.
type RuleActionFrequency struct {
	_ [0]func()

	// Summary runs the action once for all alerts of a run, instead of
	// once per alert.
	Summary    bool           `json:"summary"`
	NotifyWhen RuleNotifyWhen `json:"notify_when"`

	// Throttle is the minimum time between runs of the action, such as
	// "1h", with [RuleNotifyWhenOnThrottleInterval].
	Throttle string `json:"throttle,omitempty"`
}

// RuleActionAlertsFilter limits an action to some alerts.
type RuleActionAlertsFilter struct {
	_ [0]func()

	Query     *RuleAlertsFilterQuery     `json:"query,omitempty"`
	Timeframe *RuleAlertsFilterTimeframe `json:"timeframe,omitempty"`
}

// RuleAlertsFilterQuery limits an action to the alerts that match a query.
type RuleAlertsFilterQuery struct {
	_ [0]func()

	// KQL is a KQL query on the alert documents, such as
	// `host.name: "web-1"`.
	KQL string `json:"kql"`

	// Filters are Kibana filters, as a JSON array, as saved by the Kibana
	// UI. Nil is sent as [].
	Filters json.RawMessage `json:"filters"`

	// DSL is set by Kibana: the KQL and filters as an Elasticsearch query.
	DSL string `json:"dsl,omitempty"`
}

// RuleAlertsFilterTimeframe limits an action to some days and hours.
type RuleAlertsFilterTimeframe struct {
	_ [0]func()

	// Days are ISO weekdays: 1 is Monday, 7 is Sunday.
	Days  []int                 `json:"days"`
	Hours RuleAlertsFilterHours `json:"hours"`

	// Timezone is an IANA time zone, such as "Europe/Tallinn".
	Timezone string `json:"timezone"`
}

// RuleAlertsFilterHours is a time range of a day.
type RuleAlertsFilterHours struct {
	_ [0]func()

	// Start and End are times such as "08:00" and "17:00".
	Start string `json:"start"`
	End   string `json:"end"`
}

// RuleAlertDelay delays alerts until enough consecutive runs matched.
type RuleAlertDelay struct {
	_ [0]func()

	// Active is the number of consecutive runs that must match.
	Active int `json:"active"`
}

// RuleFlapping are the flapping detection settings of a rule: an alert is
// flapping when its status changed at least StatusChangeThreshold times in
// its last LookBackWindow runs.
type RuleFlapping struct {
	_ [0]func()

	// Enabled turns flapping detection off for the rule when false. Only
	// Kibana 9.x accepts it; nil leaves it out.
	Enabled *bool `json:"enabled,omitempty"`

	// LookBackWindow and StatusChangeThreshold are between 2 and 20.
	LookBackWindow        int `json:"look_back_window"`
	StatusChangeThreshold int `json:"status_change_threshold"`
}

// RuleArtifacts are resources linked to a rule.
type RuleArtifacts struct {
	_ [0]func()

	// Dashboards are at most 10 dashboards.
	Dashboards         []RuleDashboard         `json:"dashboards,omitempty"`
	InvestigationGuide *RuleInvestigationGuide `json:"investigation_guide,omitempty"`
}

// RuleDashboard is a dashboard linked to a rule.
type RuleDashboard struct {
	_ [0]func()

	ID string `json:"id"`
}

// RuleInvestigationGuide is the investigation guide of a rule.
type RuleInvestigationGuide struct {
	_ [0]func()

	// Blob is the guide in Markdown, at most 10,000 characters.
	Blob string `json:"blob"`
}

// RuleExecutionState is the state of a rule after its last run.
type RuleExecutionState string

// Rule execution states.
const (
	RuleExecutionStateOK      RuleExecutionState = "ok"
	RuleExecutionStateActive  RuleExecutionState = "active"
	RuleExecutionStateError   RuleExecutionState = "error"
	RuleExecutionStateWarning RuleExecutionState = "warning"
	RuleExecutionStatePending RuleExecutionState = "pending"
	RuleExecutionStateUnknown RuleExecutionState = "unknown"
)

// RuleExecutionStatus is the status of a rule's last run.
type RuleExecutionStatus struct {
	_ [0]func()

	// Status is [RuleExecutionStatePending] until the rule first runs, and
	// [RuleExecutionStateActive] while it has active alerts.
	Status            RuleExecutionState `json:"status"`
	LastExecutionDate time.Time          `json:"last_execution_date,omitzero"`

	// LastDuration is the duration of the last run in milliseconds.
	LastDuration int `json:"last_duration,omitempty"`

	// Error and Warning describe a failed run or a run with a warning.
	Error   *RuleExecutionMessage `json:"error,omitempty"`
	Warning *RuleExecutionMessage `json:"warning,omitempty"`
}

// RuleExecutionMessage describes a problem of a rule run.
type RuleExecutionMessage struct {
	_ [0]func()

	// Reason is a category such as "execute", "decrypt", "license" or
	// "maxAlerts".
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// RuleLastRunOutcome is the outcome of a rule run.
type RuleLastRunOutcome string

// Rule run outcomes.
const (
	RuleLastRunOutcomeSucceeded RuleLastRunOutcome = "succeeded"
	RuleLastRunOutcomeWarning   RuleLastRunOutcome = "warning"
	RuleLastRunOutcomeFailed    RuleLastRunOutcome = "failed"
)

// RuleLastRun is the outcome of a rule's last run.
type RuleLastRun struct {
	_ [0]func()

	Outcome RuleLastRunOutcome `json:"outcome"`

	// OutcomeOrder sorts outcomes by severity, higher being worse: 0 for
	// success and 20 for a failure.
	OutcomeOrder int `json:"outcome_order"`

	// Warning is the reason of a warning or failure, such as "execute".
	Warning string `json:"warning,omitempty"`

	// OutcomeMsg holds the messages of a warning or failure.
	OutcomeMsg  []string        `json:"outcome_msg,omitempty"`
	AlertsCount RuleAlertsCount `json:"alerts_count"`
}

// RuleAlertsCount counts the alerts of a rule run.
type RuleAlertsCount struct {
	_ [0]func()

	New       int `json:"new"`
	Active    int `json:"active"`
	Recovered int `json:"recovered"`
	Ignored   int `json:"ignored"`
}
