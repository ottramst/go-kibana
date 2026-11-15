package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// ConnectorsService handles communication with the connector methods of the
// Kibana API. Connectors belong to a space: use [Client.InSpace] to manage
// the connectors of another space.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-connectors
type ConnectorsService service

// ConnectorTypeID identifies a connector type, such as ".webhook". The
// constants are not exhaustive: [ConnectorsService.ListTypes] returns the
// types of a Kibana.
type ConnectorTypeID string

// Some connector types.
const (
	ConnectorTypeIDEmail      ConnectorTypeID = ".email"
	ConnectorTypeIDIndex      ConnectorTypeID = ".index"
	ConnectorTypeIDJira       ConnectorTypeID = ".jira"
	ConnectorTypeIDOpsgenie   ConnectorTypeID = ".opsgenie"
	ConnectorTypeIDPagerDuty  ConnectorTypeID = ".pagerduty"
	ConnectorTypeIDServerLog  ConnectorTypeID = ".server-log"
	ConnectorTypeIDServiceNow ConnectorTypeID = ".servicenow"
	ConnectorTypeIDSlack      ConnectorTypeID = ".slack"
	ConnectorTypeIDSlackAPI   ConnectorTypeID = ".slack_api"
	ConnectorTypeIDTeams      ConnectorTypeID = ".teams"
	ConnectorTypeIDWebhook    ConnectorTypeID = ".webhook"
)

// ConnectorAuthMode is how a connector authenticates to its service.
type ConnectorAuthMode string

// Connector authentication modes.
const (
	ConnectorAuthModeShared  ConnectorAuthMode = "shared"
	ConnectorAuthModePerUser ConnectorAuthMode = "per-user"
)

// Connector is a connector, which actions use to call an external service
// or Kibana itself. The same type is used for reads and writes: only Name,
// Config and Secrets are written, plus ConnectorTypeID when creating.
type Connector struct {
	_ [0]func()

	ID              string          `json:"id"`
	Name            string          `json:"name"`
	ConnectorTypeID ConnectorTypeID `json:"connector_type_id"`

	// Config is the configuration of the connector, as a JSON object whose
	// fields depend on the connector type, such as {"url": "...",
	// "method": "post"} for a webhook. Left out when nil or empty: Kibana
	// then uses {}.
	Config json.RawMessage `json:"config,omitempty"`

	// Secrets are the credentials of the connector, as a JSON object whose
	// fields depend on the connector type, such as {"user": "...",
	// "password": "..."}. Left out when nil or empty. Kibana stores them
	// encrypted and never returns them, so Secrets is always nil in
	// results.
	Secrets json.RawMessage `json:"secrets,omitempty"`

	// The following fields are set by Kibana.

	// IsPreconfigured connectors are defined in kibana.yml and cannot be
	// changed through the API.
	IsPreconfigured bool `json:"is_preconfigured"`
	IsDeprecated    bool `json:"is_deprecated"`
	IsSystemAction  bool `json:"is_system_action"`

	// IsMissingSecrets is set when the secrets could not be decrypted, for
	// example after the encryption key changed.
	IsMissingSecrets bool `json:"is_missing_secrets"`

	// IsConnectorTypeDeprecated and AuthMode are only returned by Kibana
	// 9.x; AuthMode only by get and list.
	IsConnectorTypeDeprecated bool              `json:"is_connector_type_deprecated"`
	AuthMode                  ConnectorAuthMode `json:"auth_mode,omitempty"`

	// ReferencedByCount is the number of rules and other saved objects that
	// use the connector, returned only by [ConnectorsService.List].
	ReferencedByCount int `json:"referenced_by_count,omitempty"`
}

// connectorPayload is the body of connector create and update requests.
type connectorPayload struct {
	Name string `json:"name"`

	// ConnectorTypeID is only sent on create: updates reject it.
	ConnectorTypeID ConnectorTypeID `json:"connector_type_id,omitempty"`

	// Config and Secrets are left out when nil or empty: Kibana rejects
	// null.
	Config  json.RawMessage `json:"config,omitempty"`
	Secrets json.RawMessage `json:"secrets,omitempty"`
}

// newConnectorPayload validates a connector and returns its writable
// fields, with the type ID if create is set.
func newConnectorPayload(c *Connector, create bool) (*connectorPayload, error) {
	if c == nil {
		return nil, errors.New("kibana: connector must not be nil")
	}
	if c.Name == "" {
		return nil, errors.New("kibana: connector Name must not be empty")
	}
	p := &connectorPayload{Name: c.Name, Config: c.Config, Secrets: c.Secrets}
	if create {
		if c.ConnectorTypeID == "" {
			return nil, errors.New("kibana: connector ConnectorTypeID must not be empty")
		}
		p.ConnectorTypeID = c.ConnectorTypeID
	}
	return p, nil
}

// ListConnectorsOptions are the options of [ConnectorsService.List].
type ListConnectorsOptions struct {
	_ [0]func()
}

// List returns the connectors of the client's space, including the
// preconfigured ones, sorted by name.
//
// Kibana API: GET /api/actions/connectors
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-actions-connectors
func (s *ConnectorsService) List(ctx context.Context, opt *ListConnectorsOptions, opts ...RequestOption) ([]*Connector, *Response, error) {
	var connectors []*Connector
	resp, err := s.client.do(ctx, http.MethodGet, "/api/actions/connectors", nil, &connectors, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return connectors, resp, nil
}

// GetConnectorOptions are the options of [ConnectorsService.Get].
type GetConnectorOptions struct {
	_ [0]func()
}

// Get returns a connector by its ID.
//
// Kibana API: GET /api/actions/connector/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-actions-connector-id
func (s *ConnectorsService) Get(ctx context.Context, id string, opt *GetConnectorOptions, opts ...RequestOption) (*Connector, *Response, error) {
	if err := requireID("connector ID", id); err != nil {
		return nil, nil, err
	}
	c := new(Connector)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/actions/connector/%s", id), nil, c, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return c, resp, nil
}

// CreateConnectorOptions are the options of [ConnectorsService.Create].
type CreateConnectorOptions struct {
	_ [0]func()
}

// Create creates a connector. Kibana generates its ID if c.ID is empty.
// IDs are unique across all spaces, at most 36 characters long, and must be
// UUIDs on Kibana 8.19. Kibana answers [ErrConflict] for an ID that is
// taken, and [ErrBadRequest] for invalid config or secrets.
//
// Kibana API: POST /api/actions/connector/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-actions-connector-id
func (s *ConnectorsService) Create(ctx context.Context, c *Connector, opt *CreateConnectorOptions, opts ...RequestOption) (*Connector, *Response, error) {
	payload, err := newConnectorPayload(c, true)
	if err != nil {
		return nil, nil, err
	}
	path := "/api/actions/connector"
	if c.ID != "" {
		if err := requireID("connector ID", c.ID); err != nil {
			return nil, nil, err
		}
		path = pathf("/api/actions/connector/%s", c.ID)
	}
	created := new(Connector)
	resp, err := s.client.do(ctx, http.MethodPost, path, payload, created, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateConnectorOptions are the options of [ConnectorsService.Update].
type UpdateConnectorOptions struct {
	_ [0]func()
}

// Update replaces the name, config and secrets of the connector with the
// given ID. The connector's ID and type are ignored: the type of a
// connector cannot change.
//
// Kibana never returns secrets, and Update replaces them too: set Secrets
// on every update of a connector that has secrets. Without them, Kibana
// answers [ErrBadRequest] for connector types that require secrets, and
// silently removes optional ones, such as the credentials of a webhook.
//
// Kibana API: PUT /api/actions/connector/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-put-actions-connector-id
func (s *ConnectorsService) Update(ctx context.Context, id string, c *Connector, opt *UpdateConnectorOptions, opts ...RequestOption) (*Connector, *Response, error) {
	if err := requireID("connector ID", id); err != nil {
		return nil, nil, err
	}
	payload, err := newConnectorPayload(c, false)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Connector)
	resp, err := s.client.do(ctx, http.MethodPut, pathf("/api/actions/connector/%s", id), payload, updated, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteConnectorOptions are the options of [ConnectorsService.Delete].
type DeleteConnectorOptions struct {
	_ [0]func()
}

// Delete deletes a connector by its ID.
//
// Kibana API: DELETE /api/actions/connector/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-actions-connector-id
func (s *ConnectorsService) Delete(ctx context.Context, id string, opt *DeleteConnectorOptions, opts ...RequestOption) (*Response, error) {
	if err := requireID("connector ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/actions/connector/%s", id), nil, nil, opt, opts)
}

// ExecuteConnectorRequest is the body of [ConnectorsService.Execute].
type ExecuteConnectorRequest struct {
	_ [0]func()

	// Params are the parameters of the action, as a JSON object whose
	// fields depend on the connector type, such as {"message": "..."} for
	// a server log connector. Nil or empty sends {}.
	Params json.RawMessage `json:"params"`
}

// ExecuteConnectorStatus is the outcome of a connector execution.
type ExecuteConnectorStatus string

// Connector execution outcomes.
const (
	ExecuteConnectorStatusOK    ExecuteConnectorStatus = "ok"
	ExecuteConnectorStatusError ExecuteConnectorStatus = "error"
)

// ExecuteConnectorErrorSource tells who caused a failed connector
// execution.
type ExecuteConnectorErrorSource string

// Sources of connector execution errors.
const (
	// ExecuteConnectorErrorSourceUser is an error in the connector's
	// configuration or parameters, or of the called service.
	ExecuteConnectorErrorSourceUser ExecuteConnectorErrorSource = "user"

	// ExecuteConnectorErrorSourceFramework is an error in Kibana.
	ExecuteConnectorErrorSourceFramework ExecuteConnectorErrorSource = "framework"
)

// ExecuteConnectorResult is the result of [ConnectorsService.Execute].
type ExecuteConnectorResult struct {
	_ [0]func()

	ConnectorID string                 `json:"connector_id"`
	Status      ExecuteConnectorStatus `json:"status"`

	// Message describes a failure; ServiceMessage holds the error of the
	// called service, if any.
	Message        string                      `json:"message,omitempty"`
	ServiceMessage string                      `json:"service_message,omitempty"`
	ErrorSource    ExecuteConnectorErrorSource `json:"errorSource,omitempty"`

	// Data is the response of the called service, as JSON, if the
	// connector type returns one.
	Data json.RawMessage `json:"data,omitempty"`

	// Retry tells whether the execution can be retried, as JSON: false,
	// true, or the time after which to retry, such as after a 429 response
	// with a Retry-After header.
	Retry json.RawMessage `json:"retry,omitempty"`
}

// ExecuteConnectorOptions are the options of [ConnectorsService.Execute].
type ExecuteConnectorOptions struct {
	_ [0]func()
}

// Execute runs a connector once with the given parameters (none if req is
// nil). A failed execution is not an error: Kibana answers 200, and the
// result's Status is [ExecuteConnectorStatusError], with the reason in
// Message and ServiceMessage.
//
// Kibana API: POST /api/actions/connector/{id}/_execute
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-actions-connector-id-execute
func (s *ConnectorsService) Execute(ctx context.Context, id string, req *ExecuteConnectorRequest, opt *ExecuteConnectorOptions, opts ...RequestOption) (*ExecuteConnectorResult, *Response, error) {
	if err := requireID("connector ID", id); err != nil {
		return nil, nil, err
	}
	body := struct {
		Params json.RawMessage `json:"params"`
	}{Params: json.RawMessage("{}")}
	if req != nil && len(req.Params) > 0 {
		body.Params = req.Params
	}
	result := new(ExecuteConnectorResult)
	resp, err := s.client.do(ctx, http.MethodPost, pathf("/api/actions/connector/%s/_execute", id), body, result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// LicenseLevel is an Elastic license level.
type LicenseLevel string

// License levels.
const (
	LicenseLevelBasic      LicenseLevel = "basic"
	LicenseLevelStandard   LicenseLevel = "standard"
	LicenseLevelGold       LicenseLevel = "gold"
	LicenseLevelPlatinum   LicenseLevel = "platinum"
	LicenseLevelEnterprise LicenseLevel = "enterprise"
	LicenseLevelTrial      LicenseLevel = "trial"
)

// ConnectorTypeSource is where a connector type is defined.
type ConnectorTypeSource string

// Connector type sources.
const (
	ConnectorTypeSourceStack ConnectorTypeSource = "stack"
	ConnectorTypeSourceSpec  ConnectorTypeSource = "spec"
	ConnectorTypeSourceYML   ConnectorTypeSource = "yml"
)

// ConnectorType is a connector type.
type ConnectorType struct {
	_ [0]func()

	ID   ConnectorTypeID `json:"id"`
	Name string          `json:"name"`

	// Enabled is set when the type is enabled in kibana.yml
	// (EnabledInConfig) and by the license (EnabledInLicense).
	Enabled                bool         `json:"enabled"`
	EnabledInConfig        bool         `json:"enabled_in_config"`
	EnabledInLicense       bool         `json:"enabled_in_license"`
	MinimumLicenseRequired LicenseLevel `json:"minimum_license_required"`

	// SupportedFeatureIDs are the Kibana features that can use the type,
	// such as "alerting", "cases" or "siem".
	SupportedFeatureIDs []string `json:"supported_feature_ids"`

	IsSystemActionType         bool `json:"is_system_action_type"`
	AllowMultipleSystemActions bool `json:"allow_multiple_system_actions,omitempty"`

	// SubFeature is the sub-feature that groups the type, such as
	// "endpointSecurity", if any.
	SubFeature string `json:"sub_feature,omitempty"`

	// Description, IsDeprecated, IsExperimental and Source are only
	// returned by Kibana 9.x.
	Description    string              `json:"description,omitempty"`
	IsDeprecated   bool                `json:"is_deprecated"`
	IsExperimental bool                `json:"is_experimental,omitempty"`
	Source         ConnectorTypeSource `json:"source,omitempty"`
}

// ListConnectorTypesOptions are the options of [ConnectorsService.ListTypes].
type ListConnectorTypesOptions struct {
	_ [0]func()

	// FeatureID limits the types to those that support this Kibana
	// feature, such as "alerting" or "cases".
	FeatureID string `url:"feature_id,omitempty"`
}

// ListTypes returns the connector types of the Kibana, which depend on its
// version, configuration and license.
//
// Kibana API: GET /api/actions/connector_types
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-actions-connector-types
func (s *ConnectorsService) ListTypes(ctx context.Context, opt *ListConnectorTypesOptions, opts ...RequestOption) ([]*ConnectorType, *Response, error) {
	var types []*ConnectorType
	resp, err := s.client.do(ctx, http.MethodGet, "/api/actions/connector_types", nil, &types, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return types, resp, nil
}
