package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// ruleCreatePayload is the body of rule create requests.
type ruleCreatePayload struct {
	Name       string              `json:"name"`
	RuleTypeID string              `json:"rule_type_id"`
	Consumer   string              `json:"consumer"`
	Enabled    bool                `json:"enabled"`
	Schedule   ruleSchedulePayload `json:"schedule"`
	ruleCommonPayload

	// Flapping is left out when nil.
	Flapping *RuleFlapping `json:"flapping,omitempty"`
}

// ruleUpdatePayload is the body of rule update requests.
type ruleUpdatePayload struct {
	Name     string              `json:"name"`
	Schedule ruleSchedulePayload `json:"schedule"`
	ruleCommonPayload

	// Flapping is sent as null when nil: Kibana keeps the settings when
	// the field is left out.
	Flapping *RuleFlapping `json:"flapping"`
}

// ruleCommonPayload holds the fields of rule create and update requests
// other than the name, schedule and flapping settings.
type ruleCommonPayload struct {
	Params  json.RawMessage     `json:"params"`
	Actions []ruleActionPayload `json:"actions"`
	Tags    []string            `json:"tags"`

	// NotifyWhen and Throttle are left out when empty: Kibana rejects "".
	NotifyWhen RuleNotifyWhen `json:"notify_when,omitempty"`
	Throttle   string         `json:"throttle,omitempty"`

	AlertDelay *RuleAlertDelay `json:"alert_delay,omitempty"`
	Artifacts  *RuleArtifacts  `json:"artifacts,omitempty"`
}

// ruleSchedulePayload is the schedule of a rule write.
type ruleSchedulePayload struct {
	Interval string `json:"interval"`
}

// ruleActionPayload is an action of a rule write, without the read-only
// connector type, which Kibana rejects.
type ruleActionPayload struct {
	ID                      string                  `json:"id"`
	Group                   string                  `json:"group,omitempty"`
	Params                  json.RawMessage         `json:"params"`
	Frequency               *RuleActionFrequency    `json:"frequency,omitempty"`
	AlertsFilter            *RuleActionAlertsFilter `json:"alerts_filter,omitempty"`
	UseAlertDataForTemplate bool                    `json:"use_alert_data_for_template,omitempty"`
	UUID                    string                  `json:"uuid,omitempty"`
}

// rawOr returns raw, or def if raw is empty.
func rawOr(raw json.RawMessage, def string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(def)
	}
	return raw
}

// validateRule checks the fields that every rule write needs.
func validateRule(r *Rule) error {
	if r == nil {
		return errors.New("kibana: rule must not be nil")
	}
	if r.Name == "" {
		return errors.New("kibana: rule Name must not be empty")
	}
	if r.Schedule.Interval == "" {
		return errors.New("kibana: rule Schedule.Interval must not be empty")
	}
	for i, a := range r.Actions {
		if a.ID == "" {
			return fmt.Errorf("kibana: rule action %d: ID must not be empty", i)
		}
	}
	return nil
}

// commonPayload returns the writable fields that rule create and update
// share. Nil collections become empty ones, because Kibana rejects null.
func (r *Rule) commonPayload() ruleCommonPayload {
	p := ruleCommonPayload{
		Params:     rawOr(r.Params, "{}"),
		Actions:    make([]ruleActionPayload, len(r.Actions)),
		Tags:       r.Tags,
		NotifyWhen: r.NotifyWhen,
		Throttle:   r.Throttle,
		AlertDelay: r.AlertDelay,
		Artifacts:  r.Artifacts,
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	for i, a := range r.Actions {
		p.Actions[i] = ruleActionPayload{
			ID:                      a.ID,
			Group:                   a.Group,
			Params:                  rawOr(a.Params, "{}"),
			Frequency:               a.Frequency,
			AlertsFilter:            a.AlertsFilter,
			UseAlertDataForTemplate: a.UseAlertDataForTemplate,
			UUID:                    a.UUID,
		}
		if f := a.AlertsFilter; f != nil && f.Query != nil && len(f.Query.Filters) == 0 {
			// Kibana requires the filters; copy, so as not to modify the
			// caller's value.
			q := *f.Query
			q.Filters = json.RawMessage("[]")
			p.Actions[i].AlertsFilter = &RuleActionAlertsFilter{Query: &q, Timeframe: f.Timeframe}
		}
	}
	return p
}

// newRuleCreatePayload validates a rule and returns its writable fields.
func newRuleCreatePayload(r *Rule) (*ruleCreatePayload, error) {
	if err := validateRule(r); err != nil {
		return nil, err
	}
	if r.RuleTypeID == "" {
		return nil, errors.New("kibana: rule RuleTypeID must not be empty")
	}
	if r.Consumer == "" {
		return nil, errors.New("kibana: rule Consumer must not be empty")
	}
	return &ruleCreatePayload{
		Name:              r.Name,
		RuleTypeID:        r.RuleTypeID,
		Consumer:          r.Consumer,
		Enabled:           r.Enabled,
		Schedule:          ruleSchedulePayload{Interval: r.Schedule.Interval},
		ruleCommonPayload: r.commonPayload(),
		Flapping:          r.Flapping,
	}, nil
}

// newRuleUpdatePayload validates a rule and returns its writable fields.
func newRuleUpdatePayload(r *Rule) (*ruleUpdatePayload, error) {
	if err := validateRule(r); err != nil {
		return nil, err
	}
	return &ruleUpdatePayload{
		Name:              r.Name,
		Schedule:          ruleSchedulePayload{Interval: r.Schedule.Interval},
		ruleCommonPayload: r.commonPayload(),
		Flapping:          r.Flapping,
	}, nil
}

// CreateRuleOptions are the options of [AlertingService.CreateRule].
type CreateRuleOptions struct {
	_ [0]func()
}

// CreateRule creates a rule. Kibana generates its ID if r.ID is empty. IDs
// are unique across all spaces: Kibana answers [ErrConflict] for an ID that
// is taken. Invalid params, unknown rule types and missing connectors are
// [ErrBadRequest].
//
// The rule is disabled unless r.Enabled is set. An enabled rule runs with
// an API key that Kibana creates for the user who created it.
//
// Kibana API: POST /api/alerting/rule/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-alerting-rule-id
func (s *AlertingService) CreateRule(ctx context.Context, r *Rule, opt *CreateRuleOptions, opts ...RequestOption) (*Rule, *Response, error) {
	payload, err := newRuleCreatePayload(r)
	if err != nil {
		return nil, nil, err
	}
	path := "/api/alerting/rule"
	if r.ID != "" {
		if err := requireID("rule ID", r.ID); err != nil {
			return nil, nil, err
		}
		path = pathf("/api/alerting/rule/%s", r.ID)
	}
	created := new(Rule)
	resp, err := s.client.do(ctx, http.MethodPost, path, payload, created, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// GetRuleOptions are the options of [AlertingService.GetRule].
type GetRuleOptions struct {
	_ [0]func()
}

// GetRule returns a rule by its ID.
//
// Kibana API: GET /api/alerting/rule/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-alerting-rule-id
func (s *AlertingService) GetRule(ctx context.Context, id string, opt *GetRuleOptions, opts ...RequestOption) (*Rule, *Response, error) {
	if err := requireID("rule ID", id); err != nil {
		return nil, nil, err
	}
	r := new(Rule)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/alerting/rule/%s", id), nil, r, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return r, resp, nil
}

// UpdateRuleOptions are the options of [AlertingService.UpdateRule].
type UpdateRuleOptions struct {
	_ [0]func()
}

// UpdateRule replaces the rule with the given ID. The rule's ID, type,
// consumer, enabled state and the fields set by Kibana are ignored.
//
// Tags and actions that are left out are removed, and so are the flapping
// settings when r.Flapping is nil; AlertDelay is kept when nil. Pass a rule
// returned by [AlertingService.GetRule] to keep its other fields, but not
// one returned by [AlertingService.FindRules] with Fields, which is
// partial. Actions whose connector was deleted fail the update with
// [ErrBadRequest].
//
// Kibana API: PUT /api/alerting/rule/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-put-alerting-rule-id
func (s *AlertingService) UpdateRule(ctx context.Context, id string, r *Rule, opt *UpdateRuleOptions, opts ...RequestOption) (*Rule, *Response, error) {
	if err := requireID("rule ID", id); err != nil {
		return nil, nil, err
	}
	payload, err := newRuleUpdatePayload(r)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Rule)
	resp, err := s.client.do(ctx, http.MethodPut, pathf("/api/alerting/rule/%s", id), payload, updated, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteRuleOptions are the options of [AlertingService.DeleteRule].
type DeleteRuleOptions struct {
	_ [0]func()
}

// DeleteRule deletes a rule by its ID.
//
// Kibana API: DELETE /api/alerting/rule/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-alerting-rule-id
func (s *AlertingService) DeleteRule(ctx context.Context, id string, opt *DeleteRuleOptions, opts ...RequestOption) (*Response, error) {
	if err := requireID("rule ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/alerting/rule/%s", id), nil, nil, opt, opts)
}

// FindRulesOptions are the options of [AlertingService.FindRules] and
// [AlertingService.FindRulesAll].
type FindRulesOptions struct {
	_ [0]func()

	// Search is an Elasticsearch simple_query_string query, such as
	// "cpu*", on SearchFields (such as "name" or "tags"; all fields if
	// empty). DefaultSearchOperator combines its terms: OR unless set.
	Search                string         `url:"search,omitempty"`
	SearchFields          []string       `url:"search_fields,omitempty"`
	DefaultSearchOperator SearchOperator `url:"default_search_operator,omitempty"`

	// Filter is a KQL filter on the rule attributes, such as
	// "alert.attributes.tags:prod" or "alert.attributes.enabled:true".
	Filter string `url:"filter,omitempty"`

	// FilterConsumers limits the results to rules of these consumers, such
	// as "stackAlerts".
	FilterConsumers []string `url:"-"`

	// HasReference limits the results to rules that reference this saved
	// object, such as {Type: "action", ID: connectorID} for the rules that
	// use a connector.
	HasReference *SavedObjectRef `url:"-"`

	// Fields limits the returned rule fields to these, such as "name" and
	// "tags". The rules are then partial: don't pass them to
	// [AlertingService.UpdateRule].
	Fields []string `url:"fields,omitempty"`

	// SortField sorts the results by this field, such as "name" or
	// "updatedAt".
	SortField string        `url:"sort_field,omitempty"`
	SortOrder SortDirection `url:"sort_order,omitempty"`

	// Page is the page to return, starting at 1 (the default).
	Page int `url:"page,omitempty"`

	// PerPage is the number of rules per page. Kibana's default is 10;
	// FindRulesAll's is 100.
	PerPage int `url:"per_page,omitempty"`
}

// FindRulesResult is a page of rules.
type FindRulesResult struct {
	_ [0]func()

	Rules   []Rule `json:"data"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`

	// Total is the number of rules that match.
	Total int `json:"total"`
}

// findRulesPath returns the path and query of a find request.
func findRulesPath(opt *FindRulesOptions) (string, error) {
	path, err := addQuery("/api/alerting/rules/_find", opt)
	if err != nil || opt == nil {
		return path, err
	}
	q := url.Values{}
	if len(opt.FilterConsumers) > 0 {
		// Kibana rejects a single value that is not a JSON array.
		data, err := json.Marshal(opt.FilterConsumers)
		if err != nil {
			return "", fmt.Errorf("kibana: encoding filter_consumers: %w", err)
		}
		q.Set("filter_consumers", string(data))
	}
	if ref := opt.HasReference; ref != nil {
		if err := validateRefs([]SavedObjectRef{*ref}); err != nil {
			return "", fmt.Errorf("kibana: has_reference: %s", strings.TrimPrefix(err.Error(), "kibana: "))
		}
		data, err := json.Marshal(ref)
		if err != nil {
			return "", fmt.Errorf("kibana: encoding has_reference: %w", err)
		}
		q.Set("has_reference", string(data))
	}
	if len(q) == 0 {
		return path, nil
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + q.Encode(), nil
}

// FindRules returns a page of the rules of the client's space that match
// opt. Use [AlertingService.FindRulesAll] to iterate over all of them.
//
// Kibana answers [ErrBadRequest] when Page × PerPage exceeds 10,000.
//
// Kibana API: GET /api/alerting/rules/_find
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-alerting-rules-find
func (s *AlertingService) FindRules(ctx context.Context, opt *FindRulesOptions, opts ...RequestOption) (*FindRulesResult, *Response, error) {
	path, err := findRulesPath(opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(FindRulesResult)
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, result, nil, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// FindRulesAll returns an iterator over the rules of the client's space
// that match opt, starting at opt.Page. It fetches opt.PerPage rules per
// request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops. Kibana only returns the first 10,000 matches:
// the iterator fails with [ErrBadRequest] at the first page that ends
// beyond them, so narrow the query. Rules created or deleted while
// iterating can be skipped or returned twice; set opt.SortField for a
// stable order.
func (s *AlertingService) FindRulesAll(ctx context.Context, opt *FindRulesOptions, opts ...RequestOption) iter.Seq2[*Rule, error] {
	base := FindRulesOptions{}
	if opt != nil {
		base = *opt
	}
	base.SearchFields = slices.Clone(base.SearchFields)
	base.FilterConsumers = slices.Clone(base.FilterConsumers)
	base.Fields = slices.Clone(base.Fields)
	return paginatePages(ctx, base.Page, base.PerPage, func(ctx context.Context, page, perPage int) ([]*Rule, int, error) {
		req := base
		req.Page, req.PerPage = page, perPage
		result, _, err := s.FindRules(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Rules), result.Total, nil
	})
}
