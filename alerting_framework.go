package kibana

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// AlertingHealthStatus is the status of an alerting framework health check.
type AlertingHealthStatus string

// Alerting framework health statuses.
const (
	AlertingHealthStatusOK      AlertingHealthStatus = "ok"
	AlertingHealthStatusWarning AlertingHealthStatus = "warn"
	AlertingHealthStatusError   AlertingHealthStatus = "error"
)

// AlertingHealth is the health of the alerting framework.
type AlertingHealth struct {
	_ [0]func()

	// IsSufficientlySecure is false when rules cannot get the API keys they
	// run with; Kibana documents this as security enabled without TLS.
	IsSufficientlySecure bool `json:"is_sufficiently_secure"`

	// HasPermanentEncryptionKey is set when Kibana has a permanent
	// encryption key (xpack.encryptedSavedObjects.encryptionKey), which
	// rules and connectors need.
	HasPermanentEncryptionKey bool `json:"has_permanent_encryption_key"`

	AlertingFrameworkHealth AlertingFrameworkHealth `json:"alerting_framework_health"`
}

// AlertingFrameworkHealth holds the health checks of the rules that ran
// recently.
type AlertingFrameworkHealth struct {
	_ [0]func()

	// DecryptionHealth fails when rules cannot be decrypted, ExecutionHealth
	// when rules fail to run, and ReadHealth when rules cannot be read.
	DecryptionHealth AlertingHealthCheck `json:"decryption_health"`
	ExecutionHealth  AlertingHealthCheck `json:"execution_health"`
	ReadHealth       AlertingHealthCheck `json:"read_health"`
}

// AlertingHealthCheck is the result of an alerting framework health check.
type AlertingHealthCheck struct {
	_ [0]func()

	Status    AlertingHealthStatus `json:"status"`
	Timestamp time.Time            `json:"timestamp,omitzero"`
}

// GetAlertingHealthOptions are the options of [AlertingService.Health].
type GetAlertingHealthOptions struct {
	_ [0]func()
}

// Health returns the health of the alerting framework of the Kibana.
// Kibana answers [ErrForbidden] to users who may not use any rule type in
// the space.
//
// Kibana API: GET /api/alerting/_health
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-getalertinghealth
func (s *AlertingService) Health(ctx context.Context, opt *GetAlertingHealthOptions, opts ...RequestOption) (*AlertingHealth, *Response, error) {
	h := new(AlertingHealth)
	resp, err := s.client.do(ctx, http.MethodGet, "/api/alerting/_health", nil, h, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return h, resp, nil
}

// RuleTypeCategory is the solution that a rule type belongs to.
type RuleTypeCategory string

// Rule type categories.
const (
	RuleTypeCategoryManagement       RuleTypeCategory = "management"
	RuleTypeCategoryObservability    RuleTypeCategory = "observability"
	RuleTypeCategorySecuritySolution RuleTypeCategory = "securitySolution"
)

// RuleType is a rule type, such as ".index-threshold".
type RuleType struct {
	_ [0]func()

	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Category RuleTypeCategory `json:"category"`

	// Producer is the Kibana feature that provides the rule type, such as
	// "stackAlerts" or "observability".
	Producer string `json:"producer"`

	// ActionGroups are the groups that actions of a rule can run on, such
	// as "threshold met" and "recovered".
	ActionGroups         []RuleActionGroup `json:"action_groups"`
	DefaultActionGroupID string            `json:"default_action_group_id"`
	RecoveryActionGroup  RuleActionGroup   `json:"recovery_action_group"`

	// ActionVariables are the variables that action parameters can use in
	// mustache templates, as JSON: {"context": [{"name", "description"}],
	// "state": [...], "params": [...]}.
	ActionVariables json.RawMessage `json:"action_variables,omitempty"`

	// Alerts describes the alert documents of the rule type, as JSON: the
	// index context and field mappings.
	Alerts json.RawMessage `json:"alerts,omitempty"`

	// AuthorizedConsumers are the consumers, such as "stackAlerts" or
	// "alerts", that the calling user may create rules of this type for,
	// with their privileges.
	AuthorizedConsumers map[string]RuleTypeConsumerPrivileges `json:"authorized_consumers"`

	EnabledInLicense       bool         `json:"enabled_in_license"`
	MinimumLicenseRequired LicenseLevel `json:"minimum_license_required"`

	// RuleTaskTimeout is how long a run may take, such as "5m".
	RuleTaskTimeout string `json:"rule_task_timeout,omitempty"`

	AutoRecoverAlerts      bool `json:"auto_recover_alerts"`
	DoesSetRecoveryContext bool `json:"does_set_recovery_context"`
	HasAlertsMappings      bool `json:"has_alerts_mappings"`
	HasFieldsForAAD        bool `json:"has_fields_for_a_a_d"`
	IsExportable           bool `json:"is_exportable"`

	// IsInternallyManaged rule types are managed by Kibana; Kibana 8.19 does
	// not return it.
	IsInternallyManaged bool `json:"is_internally_managed,omitempty"`
}

// RuleActionGroup is an action group of a rule type.
type RuleActionGroup struct {
	_ [0]func()

	ID   string `json:"id"`
	Name string `json:"name"`
}

// RuleTypeConsumerPrivileges are the privileges of the calling user on the
// rules of a type and consumer.
type RuleTypeConsumerPrivileges struct {
	_ [0]func()

	Read bool `json:"read"`
	All  bool `json:"all"`
}

// ListRuleTypesOptions are the options of [AlertingService.ListRuleTypes].
type ListRuleTypesOptions struct {
	_ [0]func()
}

// ListRuleTypes returns the rule types that the calling user may use, which
// depend on the Kibana version, license and plugins.
//
// Kibana API: GET /api/alerting/rule_types
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-getruletypes
func (s *AlertingService) ListRuleTypes(ctx context.Context, opt *ListRuleTypesOptions, opts ...RequestOption) ([]*RuleType, *Response, error) {
	var types []*RuleType
	resp, err := s.client.do(ctx, http.MethodGet, "/api/alerting/rule_types", nil, &types, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return types, resp, nil
}

// QueryInspectorMode is what [AlertingService.GetRuleQueryInspector]
// returns.
type QueryInspectorMode string

// Query inspector modes.
const (
	// QueryInspectorModeBuild returns the queries only.
	QueryInspectorModeBuild QueryInspectorMode = "build"

	// QueryInspectorModeExecute runs the queries and returns their
	// responses too.
	QueryInspectorModeExecute QueryInspectorMode = "execute"
)

// GetRuleQueryInspectorOptions are the options of
// [AlertingService.GetRuleQueryInspector].
type GetRuleQueryInspectorOptions struct {
	_ [0]func()

	// Mode is QueryInspectorModeBuild unless set.
	Mode QueryInspectorMode `url:"mode,omitempty"`

	// AlertID is the UUID of an alert of the rule (its kibana.alert.uuid
	// field, not the alert instance ID): the queries then use the rule
	// parameters and the time range with which the alert was evaluated, if
	// the alert recorded them, instead of the current ones.
	AlertID string `url:"alert_id,omitempty"`
}

// RuleQuery is an Elasticsearch query of a rule.
type RuleQuery struct {
	_ [0]func()

	// Index is the index pattern that the query searches.
	Index string `json:"index"`

	// Label describes the query, if the rule runs several.
	Label string `json:"label,omitempty"`

	// Request is the search request, and Response its response with
	// [QueryInspectorModeExecute], as JSON.
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response,omitempty"`
}

// GetRuleQueryInspector returns the Elasticsearch queries that a rule runs,
// and with [QueryInspectorModeExecute] their responses. It works on disabled
// rules. Kibana answers [ErrBadRequest] for rule types that don't support
// it, such as ".index-threshold" and ".es-query" (the observability custom
// threshold rule does). An opt.AlertID that Kibana does not find is
// [ErrNotFound], and one of another rule is rejected.
//
// The query inspector needs Kibana 9.5 or later: Kibana 8.19 answers
// [ErrNotFound].
//
// Kibana API: GET /api/alerting/rule/{id}/query_inspector
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-alerting-rule-id-query-inspector
func (s *AlertingService) GetRuleQueryInspector(ctx context.Context, ruleID string, opt *GetRuleQueryInspectorOptions, opts ...RequestOption) ([]*RuleQuery, *Response, error) {
	if err := requireID("rule ID", ruleID); err != nil {
		return nil, nil, err
	}
	var result struct {
		Queries []*RuleQuery `json:"queries"`
	}
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/alerting/rule/%s/query_inspector", ruleID), nil, &result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result.Queries, resp, nil
}
