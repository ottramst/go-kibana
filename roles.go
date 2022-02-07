package kibana

import (
	"fmt"
	"net/http"
)

// RolesService handles communication with the roles related methods of the
// Kibana API
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-api.html
type RolesService struct {
	client *Client
}

// Feature represents a single Kibana feature privileges
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/kibana-privileges.html
type Feature struct {
	Discover         []string `json:"discover,omitempty"`
	Visualize        []string `json:"visualize,omitempty"`
	Dashboard        []string `json:"dashboard,omitempty"`
	DevTools         []string `json:"dev_tools,omitempty"`
	AdvancedSettings []string `json:"advancedSettings,omitempty"`
	IndexPatterns    []string `json:"indexPatterns,omitempty"`
	Graph            []string `json:"graph,omitempty"`
	APM              []string `json:"apm,omitempty"`
	Maps             []string `json:"maps,omitempty"`
	Canvas           []string `json:"canvas,omitempty"`
	Infrastructure   []string `json:"infrastructure,omitempty"`
	Logs             []string `json:"logs,omitempty"`
	Uptime           []string `json:"uptime,omitempty"`
}

// Privilege represents a Kibana feature privilege
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/kibana-privileges.html
type Privilege struct {
	Base    []string `json:"base"`
	Feature *Feature `json:"feature"`
	Spaces  []string `json:"spaces"`
}

// ElasticsearchIndex represents an Elasticsearch index privilege
//
// Elasticsearch API docs: https://www.elastic.co/guide/en/elasticsearch/reference/7.17/defining-roles.html
type ElasticsearchIndex struct {
	Names         []string `json:"names"`
	Privileges    []string `json:"privileges"`
	FieldSecurity struct {
		Grant  []string `json:"grant"`
		Except []string `json:"except"`
	} `json:"field_security"`
	// TODO: Query
	AllowRestrictedIndices bool `json:"allow_restricted_indices"`
}

// Role represents a Kibana Role
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-api.html
type Role struct {
	Name     string `json:"name"`
	Metadata struct {
		Version int `json:"version"`
	} `json:"metadata"`
	TransientMetadata struct {
		Enabled bool `json:"enabled"`
	} `json:"transient_metadata"`
	Elasticsearch struct {
		Cluster []string              `json:"cluster"`
		Indices *[]ElasticsearchIndex `json:"indices"`
		RunAs   []string              `json:"run_as"`
	} `json:"elasticsearch"`
	Kibana *[]Privilege `json:"kibana"`
}

// CreateOrUpdateRoleOptions represents the available CreateOrUpdateRole() options
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-api-put.html
type CreateOrUpdateRoleOptions struct {
	Metadata *struct {
		Version *int `url:"version,omitempty" json:"version,omitempty"`
	} `url:"metadata,omitempty" json:"metadata,omitempty"`
	Elasticsearch *struct {
		Cluster *[]string             `url:"cluster,omitempty" json:"cluster,omitempty"`
		Indices *[]ElasticsearchIndex `url:"indices,omitempty" json:"indices,omitempty"`
		RunAs   *[]string             `url:"run_as,omitempty" json:"run_as,omitempty"`
	} `url:"elasticsearch,omitempty" json:"elasticsearch,omitempty"`
	Kibana *[]Privilege `url:"kibana" json:"kibana"`
}

// CreateOrUpdateRole creates or updates a Kibana role
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-api-put.html
func (s RolesService) CreateOrUpdateRole(rName string, opt *CreateOrUpdateRoleOptions, options ...RequestOptionFunc) (*Role, *Response, error) {
	u := fmt.Sprintf("security/role/%s", rName)

	req, err := s.client.NewRequest(http.MethodPut, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	r := new(Role)
	resp, err := s.client.Do(req, r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, err
}

// GetRole lists a single Kibana role
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-specific-api-get.html
func (s RolesService) GetRole(rName string, options ...RequestOptionFunc) (*Role, *Response, error) {
	u := fmt.Sprintf("security/role/%s", rName)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	r := new(Role)
	resp, err := s.client.Do(req, r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, err
}

// GetAllRoles lists all Kibana roles
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-api-get.html
func (s RolesService) GetAllRoles(options ...RequestOptionFunc) ([]*Role, *Response, error) {
	req, err := s.client.NewRequest(http.MethodGet, "security/role", nil, options)
	if err != nil {
		return nil, nil, err
	}

	var rs []*Role
	resp, err := s.client.Do(req, &rs)
	if err != nil {
		return nil, resp, err
	}

	return rs, resp, err
}

// DeleteRole deletes a single Kibana role
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/role-management-api-delete.html
func (s RolesService) DeleteRole(rName string, options ...RequestOptionFunc) (*Response, error) {
	u := fmt.Sprintf("security/role/%s", rName)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
