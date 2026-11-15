package kibana

import (
	"context"
	"net/http"
)

// ruleAction posts to an action endpoint of a rule, such as _enable.
func (s *AlertingService) ruleAction(ctx context.Context, id, action string, body, opt any, opts []RequestOption) (*Response, error) {
	if err := requireID("rule ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodPost, pathf("/api/alerting/rule/%s/", id)+action, body, nil, opt, opts)
}

// EnableRuleOptions are the options of [AlertingService.EnableRule].
type EnableRuleOptions struct {
	_ [0]func()
}

// EnableRule enables a rule, so that it runs on its schedule. If the rule
// has no API key, Kibana creates one as the calling user, or, with API key
// authentication, stores the caller's key. Enabling an enabled rule does
// nothing.
//
// Kibana API: POST /api/alerting/rule/{id}/_enable
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id-enable
func (s *AlertingService) EnableRule(ctx context.Context, id string, opt *EnableRuleOptions, opts ...RequestOption) (*Response, error) {
	return s.ruleAction(ctx, id, "_enable", nil, opt, opts)
}

// DisableRuleOptions are the options of [AlertingService.DisableRule].
type DisableRuleOptions struct {
	_ [0]func()

	// Untrack marks the rule's active alerts as untracked.
	Untrack bool `url:"-"`
}

// DisableRule disables a rule, so that it no longer runs. Disabling a
// disabled rule does nothing.
//
// Kibana API: POST /api/alerting/rule/{id}/_disable
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id-disable
func (s *AlertingService) DisableRule(ctx context.Context, id string, opt *DisableRuleOptions, opts ...RequestOption) (*Response, error) {
	var body any
	if opt != nil && opt.Untrack {
		body = struct {
			Untrack bool `json:"untrack"`
		}{Untrack: true}
	}
	return s.ruleAction(ctx, id, "_disable", body, opt, opts)
}

// MuteAllAlertsOptions are the options of [AlertingService.MuteAllAlerts].
type MuteAllAlertsOptions struct {
	_ [0]func()
}

// MuteAllAlerts mutes all the alerts of a rule: the rule keeps running, but
// its actions don't. Rule.MuteAll is then set, and the alerts muted one by
// one (Rule.MutedAlertIDs) are cleared.
//
// Kibana API: POST /api/alerting/rule/{id}/_mute_all
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id-mute-all
func (s *AlertingService) MuteAllAlerts(ctx context.Context, id string, opt *MuteAllAlertsOptions, opts ...RequestOption) (*Response, error) {
	return s.ruleAction(ctx, id, "_mute_all", nil, opt, opts)
}

// UnmuteAllAlertsOptions are the options of [AlertingService.UnmuteAllAlerts].
type UnmuteAllAlertsOptions struct {
	_ [0]func()
}

// UnmuteAllAlerts undoes [AlertingService.MuteAllAlerts]. Like it, it
// clears the alerts muted one by one (Rule.MutedAlertIDs).
//
// Kibana API: POST /api/alerting/rule/{id}/_unmute_all
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id-unmute-all
func (s *AlertingService) UnmuteAllAlerts(ctx context.Context, id string, opt *UnmuteAllAlertsOptions, opts ...RequestOption) (*Response, error) {
	return s.ruleAction(ctx, id, "_unmute_all", nil, opt, opts)
}

// UpdateRuleAPIKeyOptions are the options of [AlertingService.UpdateRuleAPIKey].
type UpdateRuleAPIKeyOptions struct {
	_ [0]func()
}

// UpdateRuleAPIKey replaces the API key that a rule runs with by a new one
// of the calling user, who becomes Rule.APIKeyOwner. Use it when the owner
// of the key lost access, or to rotate the key. With API key
// authentication, Kibana 8.19 stores the caller's key, while 9.x creates a
// new one.
//
// Kibana API: POST /api/alerting/rule/{id}/_update_api_key
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id-update-api-key
func (s *AlertingService) UpdateRuleAPIKey(ctx context.Context, id string, opt *UpdateRuleAPIKeyOptions, opts ...RequestOption) (*Response, error) {
	return s.ruleAction(ctx, id, "_update_api_key", nil, opt, opts)
}
