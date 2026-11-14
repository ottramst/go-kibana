package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// RolesService handles communication with the role management methods of
// the Kibana API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-roles
type RolesService service

// Role is a Kibana role: Elasticsearch cluster and index privileges plus
// Kibana privileges per space. The same type is used to read and write roles,
// so a role can be read, changed and written back.
type Role struct {
	_ [0]func()

	// Name is the role name. It is read-only: [RolesService.CreateOrUpdate]
	// takes the name as an argument.
	Name string `json:"name,omitempty"`

	Description string `json:"description,omitempty"`

	// Metadata holds arbitrary metadata. Keys starting with "_" are reserved
	// for system use.
	Metadata map[string]any `json:"metadata,omitempty"`

	// TransientMetadata is read-only. Its "enabled" key is false when the role
	// is disabled, for example because it needs a higher license level.
	TransientMetadata map[string]any `json:"transient_metadata,omitempty"`

	// Elasticsearch holds the cluster, index and run-as privileges.
	Elasticsearch ElasticsearchPrivileges `json:"elasticsearch"`

	// Kibana lists the Kibana privileges, each granted in some spaces.
	Kibana []KibanaPrivilege `json:"kibana,omitempty"`

	// UnrecognizedApplications is read-only and lists application privileges
	// of the role that Kibana does not manage.
	UnrecognizedApplications []string `json:"_unrecognized_applications,omitempty"`

	// TransformErrors is read-only and lists the application privileges
	// Kibana could not convert to Kibana privileges.
	TransformErrors []RoleTransformError `json:"_transform_error,omitempty"`
}

// RoleTransformError describes application privileges of a role that
// Kibana could not convert to Kibana privileges.
type RoleTransformError struct {
	_ [0]func()

	// Reason says why the privileges could not be converted, and State
	// holds them as Kibana returned them.
	Reason string          `json:"reason"`
	State  json.RawMessage `json:"state,omitempty"`
}

// ElasticsearchPrivileges are the Elasticsearch privileges of a role.
type ElasticsearchPrivileges struct {
	_ [0]func()

	// Cluster lists cluster privileges, such as "monitor" or "manage".
	Cluster []string `json:"cluster,omitempty"`

	Indices []IndexPrivilege `json:"indices,omitempty"`

	RemoteIndices []RemoteIndexPrivilege `json:"remote_indices,omitempty"`

	RemoteCluster []RemoteClusterPrivilege `json:"remote_cluster,omitempty"`

	// RunAs lists the users the role can impersonate.
	RunAs []string `json:"run_as,omitempty"`
}

// IndexPrivilege grants privileges on a set of Elasticsearch indices.
type IndexPrivilege struct {
	_ [0]func()

	// Names lists index names or patterns. Required.
	Names []string `json:"names"`

	// Privileges lists index privileges, such as "read". Required.
	Privileges []string `json:"privileges"`

	// FieldSecurity restricts the fields the role can read, with "grant" and
	// "except" lists of field names.
	FieldSecurity map[string][]string `json:"field_security,omitempty"`

	// Query is a document level security query, as a JSON string.
	Query string `json:"query,omitempty"`

	// AllowRestrictedIndices lets Names match restricted indices, such as
	// .security.
	AllowRestrictedIndices bool `json:"allow_restricted_indices,omitempty"`
}

// RemoteIndexPrivilege grants privileges on indices of remote clusters.
type RemoteIndexPrivilege struct {
	_ [0]func()

	// Clusters lists remote cluster aliases or patterns. Required.
	Clusters []string `json:"clusters"`

	// Names lists index names or patterns. Required.
	Names []string `json:"names"`

	// Privileges lists index privileges, such as "read". Required.
	Privileges []string `json:"privileges"`

	// FieldSecurity, Query and AllowRestrictedIndices are as in
	// [IndexPrivilege].
	FieldSecurity map[string][]string `json:"field_security,omitempty"`

	Query string `json:"query,omitempty"`

	AllowRestrictedIndices bool `json:"allow_restricted_indices,omitempty"`
}

// RemoteClusterPrivilege grants cluster privileges on remote clusters.
type RemoteClusterPrivilege struct {
	_ [0]func()

	// Clusters lists remote cluster aliases or patterns. Required.
	Clusters []string `json:"clusters"`

	// Privileges lists remote cluster privileges, such as "monitor_enrich".
	// Required.
	Privileges []string `json:"privileges"`
}

// KibanaPrivilege grants Kibana privileges in a set of spaces. Set either
// Base or Feature.
type KibanaPrivilege struct {
	_ [0]func()

	// Base lists base privileges, "all" or "read", that apply to every
	// feature.
	Base []string `json:"base"`

	// Feature maps feature IDs to feature privileges, for example
	// {"discover": {"read"}, "dashboard": {"all"}}. The available features
	// depend on the Kibana version and license.
	Feature map[string][]string `json:"feature,omitempty"`

	// Spaces lists the space IDs the privileges apply to, or "*" for all
	// spaces. Required: [RolesService.CreateOrUpdate] rejects an empty list,
	// because Kibana would grant the privileges in all spaces.
	Spaces []string `json:"spaces"`

	// Reserved is read-only and lists reserved privileges.
	Reserved []string `json:"_reserved,omitempty"`
}

// rolePayload is the request body of the create or update role endpoint,
// which rejects the read-only fields of [Role].
type rolePayload struct {
	Description   string                  `json:"description,omitempty"`
	Metadata      map[string]any          `json:"metadata,omitempty"`
	Elasticsearch ElasticsearchPrivileges `json:"elasticsearch"`
	Kibana        []KibanaPrivilege       `json:"kibana,omitempty"`
}

func newRolePayload(r *Role) (*rolePayload, error) {
	p := &rolePayload{
		Description:   r.Description,
		Metadata:      r.Metadata,
		Elasticsearch: r.Elasticsearch,
	}
	if r.Kibana != nil {
		p.Kibana = make([]KibanaPrivilege, len(r.Kibana))
		for i, k := range r.Kibana {
			if len(k.Spaces) == 0 {
				return nil, fmt.Errorf("kibana: Kibana privilege %d has no spaces; use \"*\" for all spaces", i)
			}
			k.Reserved = nil
			// base is required, and must be an empty list for feature privileges.
			if k.Base == nil {
				k.Base = []string{}
			}
			p.Kibana[i] = k
		}
	}
	return p, nil
}

// ListRolesOptions are the options for [RolesService.List].
type ListRolesOptions struct {
	_ [0]func()

	// ReplaceDeprecatedPrivileges replaces privileges of deprecated features
	// with the privileges of the features that replace them.
	ReplaceDeprecatedPrivileges bool `url:"replaceDeprecatedPrivileges,omitempty"`
}

// GetRoleOptions are the options for [RolesService.Get].
type GetRoleOptions struct {
	_ [0]func()

	// ReplaceDeprecatedPrivileges replaces privileges of deprecated features
	// with the privileges of the features that replace them.
	ReplaceDeprecatedPrivileges bool `url:"replaceDeprecatedPrivileges,omitempty"`
}

// List returns all roles.
//
// Kibana API: GET /api/security/role
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-security-role
func (s *RolesService) List(ctx context.Context, opt *ListRolesOptions, opts ...RequestOption) ([]*Role, *Response, error) {
	path, err := addQuery("/api/security/role", opt)
	if err != nil {
		return nil, nil, err
	}
	var roles []*Role
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &roles, nil, opts)
	if err != nil {
		return nil, resp, err
	}
	return roles, resp, nil
}

// Get returns the role with the given name.
//
// Kibana API: GET /api/security/role/{name}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-security-role-name
func (s *RolesService) Get(ctx context.Context, name string, opt *GetRoleOptions, opts ...RequestOption) (*Role, *Response, error) {
	if err := requireID("role name", name); err != nil {
		return nil, nil, err
	}
	role := new(Role)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/security/role/%s", name), nil, role, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return role, resp, nil
}

// CreateOrUpdateRoleOptions are the options for [RolesService.CreateOrUpdate].
type CreateOrUpdateRoleOptions struct {
	_ [0]func()

	// CreateOnly makes the request fail with a conflict instead of
	// overwriting an existing role.
	CreateOnly bool `url:"createOnly,omitempty"`
}

// CreateOrUpdate creates the role with the given name, or replaces it if it
// exists. The read-only fields of role are ignored. Every Kibana privilege
// must list at least one space.
//
// Kibana API: PUT /api/security/role/{name}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-put-security-role-name
func (s *RolesService) CreateOrUpdate(ctx context.Context, name string, role *Role, opt *CreateOrUpdateRoleOptions, opts ...RequestOption) (*Response, error) {
	if err := requireID("role name", name); err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.New("kibana: role must not be nil")
	}
	payload, err := newRolePayload(role)
	if err != nil {
		return nil, err
	}
	path, err := addQuery(pathf("/api/security/role/%s", name), opt)
	if err != nil {
		return nil, err
	}
	// Kibana answers 204 No Content.
	return s.client.do(ctx, http.MethodPut, path, payload, nil, nil, opts)
}

// DeleteRoleOptions are the options of [RolesService.Delete].
type DeleteRoleOptions struct {
	_ [0]func()
}

// Delete deletes the role with the given name.
//
// Kibana API: DELETE /api/security/role/{name}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-security-role-name
func (s *RolesService) Delete(ctx context.Context, name string, opt *DeleteRoleOptions, opts ...RequestOption) (*Response, error) {
	if err := requireID("role name", name); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/security/role/%s", name), nil, nil, opt, opts)
}
