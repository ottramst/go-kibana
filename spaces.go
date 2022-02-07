package kibana

import (
	"fmt"
	"net/http"
)

// SpacesService handles communication with the runner related methods of the
// Kibana API
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api.html
type SpacesService struct {
	client *Client
}

// Space represents a Kibana Space
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api.html
type Space struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Color            string   `json:"color"`
	Initials         string   `json:"initials"`
	DisabledFeatures []string `json:"disabledFeatures"`
	ImageURL         string   `json:"imageUrl"`
}

// CreateSpaceOptions represents the available CreateSpace() options
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-post.html
type CreateSpaceOptions struct {
	ID               *string   `url:"id" json:"id"`
	Name             *string   `url:"name" json:"name"`
	Description      *string   `url:"description,omitempty" json:"description,omitempty"`
	Color            *string   `url:"color,omitempty" json:"color,omitempty"`
	Initials         *string   `url:"initials,omitempty" json:"initials,omitempty"`
	DisabledFeatures *[]string `url:"disabledFeatures,omitempty" json:"disabledFeatures,omitempty"`
	ImageURL         *string   `url:"imageUrl,omitempty" json:"imageUrl,omitempty"`
}

// CreateSpace creates a Kibana space
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-post.html
func (s SpacesService) CreateSpace(opt *CreateSpaceOptions, options ...RequestOptionFunc) (*Space, *Response, error) {

	req, err := s.client.NewRequest(http.MethodPost, "spaces/space", opt, options)
	if err != nil {
		return nil, nil, err
	}

	sp := new(Space)
	resp, err := s.client.Do(req, sp)
	if err != nil {
		return nil, resp, err
	}

	return sp, resp, err
}

// UpdateSpaceOptions represents the available UpdateSpace() options
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-put.html
type UpdateSpaceOptions struct {
	ID               *string   `url:"id" json:"id"`
	Name             *string   `url:"name" json:"name"`
	Description      *string   `url:"description,omitempty" json:"description,omitempty"`
	Color            *string   `url:"color,omitempty" json:"color,omitempty"`
	Initials         *string   `url:"initials,omitempty" json:"initials,omitempty"`
	DisabledFeatures *[]string `url:"disabledFeatures,omitempty" json:"disabledFeatures,omitempty"`
	ImageURL         *string   `url:"imageUrl,omitempty" json:"imageUrl,omitempty"`
}

// UpdateSpace updates a Kibana space
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-put.html
func (s SpacesService) UpdateSpace(sid string, opt *UpdateSpaceOptions, options ...RequestOptionFunc) (*Space, *Response, error) {

	u := fmt.Sprintf("spaces/space/%s", sid)
	opt.ID = String(sid)

	req, err := s.client.NewRequest(http.MethodPut, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	sp := new(Space)
	resp, err := s.client.Do(req, sp)
	if err != nil {
		return nil, resp, err
	}

	return sp, resp, err
}

// GetSpace lists a single Kibana space
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-get.html
func (s SpacesService) GetSpace(sid string, options ...RequestOptionFunc) (*Space, *Response, error) {

	u := fmt.Sprintf("spaces/space/%s", sid)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	sp := new(Space)
	resp, err := s.client.Do(req, sp)
	if err != nil {
		return nil, resp, err
	}

	return sp, resp, err
}

// GetAllSpacesOptions represents the available ListAllSpaces() options
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-get-all.html
type GetAllSpacesOptions struct {
	Purpose                  *string `url:"purpose,omitempty" json:"purpose,omitempty"`
	IncludeAuthorizedPurpose *bool   `url:"include_authorized_purpose,omitempty" json:"include_authorized_purpose,omitempty"`
}

// GetAllSpaces lists all the available Kibana spaces
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-get-all.html
func (s SpacesService) GetAllSpaces(opt *GetAllSpacesOptions, options ...RequestOptionFunc) ([]*Space, *Response, error) {

	req, err := s.client.NewRequest(http.MethodGet, "spaces/space", opt, options)
	if err != nil {
		return nil, nil, err
	}

	var ss []*Space
	resp, err := s.client.Do(req, &ss)
	if err != nil {
		return nil, resp, err
	}

	return ss, resp, err
}

// DeleteSpace deletes a single Kibana space
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/spaces-api-delete.html
func (s SpacesService) DeleteSpace(sid string, options ...RequestOptionFunc) (*Response, error) {

	u := fmt.Sprintf("spaces/space/%s", sid)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
