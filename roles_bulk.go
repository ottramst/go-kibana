package kibana

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// BulkCreateOrUpdateRolesResult is the result of [RolesService.BulkCreateOrUpdate].
type BulkCreateOrUpdateRolesResult struct {
	_ [0]func()

	// Created lists the names of the roles that were created.
	Created []string `json:"created,omitempty"`

	// Updated lists the names of the roles that were changed.
	Updated []string `json:"updated,omitempty"`

	// Noop lists the names of the roles that already had the given
	// definition.
	Noop []string `json:"noop,omitempty"`

	// Errors maps the names of the roles that failed to their error.
	Errors map[string]BulkRoleError `json:"errors,omitempty"`
}

// BulkRoleError is the error of a role in [BulkCreateOrUpdateRolesResult].
type BulkRoleError struct {
	_ [0]func()

	// Type is the Elasticsearch error type, such as
	// "action_request_validation_exception".
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// Error returns the error type and reason.
func (e BulkRoleError) Error() string {
	if e.Type == "" {
		return "kibana: " + e.Reason
	}
	return "kibana: " + e.Type + ": " + e.Reason
}

// BulkCreateOrUpdateRolesOptions are the options of [RolesService.BulkCreateOrUpdate].
type BulkCreateOrUpdateRolesOptions struct {
	_ [0]func()
}

// BulkCreateOrUpdateRolesRequest is the request of
// [RolesService.BulkCreateOrUpdate].
type BulkCreateOrUpdateRolesRequest struct {
	_ [0]func()

	// Roles maps role names to their definitions. Required.
	Roles map[string]*Role
}

// BulkCreateOrUpdate creates or replaces several roles in one request. The
// read-only fields of the roles are ignored, and every Kibana privilege must
// list at least one space.
//
// Roles that fail, for example because of an unknown Elasticsearch
// privilege, are reported in [BulkCreateOrUpdateRolesResult.Errors]; the
// call itself still succeeds. Kibana does not validate Kibana feature names.
//
// Kibana API: POST /api/security/roles
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-security-roles
func (s *RolesService) BulkCreateOrUpdate(ctx context.Context, req *BulkCreateOrUpdateRolesRequest, opt *BulkCreateOrUpdateRolesOptions, opts ...RequestOption) (*BulkCreateOrUpdateRolesResult, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	roles := req.Roles
	if len(roles) == 0 {
		return nil, nil, errors.New("kibana: Roles must not be empty")
	}
	payloads := make(map[string]*rolePayload, len(roles))
	// Validate in a fixed order, so that the same input gives the same error.
	for _, name := range slices.Sorted(maps.Keys(roles)) {
		if err := requireID("role name", name); err != nil {
			return nil, nil, err
		}
		role := roles[name]
		if role == nil {
			return nil, nil, fmt.Errorf("kibana: role %q must not be nil", name)
		}
		payload, err := newRolePayload(role)
		if err != nil {
			return nil, nil, fmt.Errorf("kibana: role %q: %s", name, strings.TrimPrefix(err.Error(), "kibana: "))
		}
		payloads[name] = payload
	}
	body := struct {
		Roles map[string]*rolePayload `json:"roles"`
	}{payloads}
	result := new(BulkCreateOrUpdateRolesResult)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/security/roles", body, result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
