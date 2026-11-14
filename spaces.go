package kibana

import (
	"context"
	"errors"
	"net/http"
)

// SpacesService handles communication with the spaces related methods of the
// Kibana API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-spaces
type SpacesService service

// SpaceSolution is the solution view of a space, which determines the
// navigation it shows.
type SpaceSolution string

// Solution views of a space.
const (
	SpaceSolutionClassic       SpaceSolution = "classic"
	SpaceSolutionSecurity      SpaceSolution = "security"
	SpaceSolutionObservability SpaceSolution = "oblt"
	SpaceSolutionSearch        SpaceSolution = "es"
)

// Space is a Kibana space. The same type is used to read, create and update
// spaces, so a space can be read, changed and written back.
//
// Description, Initials and ImageURL are always sent, so setting them to ""
// clears them. Color cannot be cleared: Kibana requires a hex color when it
// is sent, so an empty Color is left out.
type Space struct {
	// ID is part of the Kibana URL inside the space. It is limited to
	// lowercase alphanumeric, underscore and hyphen characters, and cannot be
	// changed once the space is created.
	ID string `json:"id"`

	// Name is the display name of the space.
	Name string `json:"name"`

	Description string `json:"description"`

	// Color is the hexadecimal color code of the space avatar, such as
	// "#aabbcc". Kibana generates one from the name when it is empty.
	Color string `json:"color,omitempty"`

	// Initials are one or two characters shown in the space avatar. Kibana
	// generates them from the name when they are empty.
	Initials string `json:"initials"`

	// ImageURL is a data URL of the image shown in the space avatar.
	ImageURL string `json:"imageUrl"`

	// DisabledFeatures lists the IDs of the features turned off in the space.
	DisabledFeatures []string `json:"disabledFeatures,omitempty"`

	// Solution is the solution view of the space.
	Solution SpaceSolution `json:"solution,omitempty"`

	// ProjectRouting is the default cross-project search routing of the
	// space. Kibana 9.5 and later.
	ProjectRouting string `json:"projectRouting,omitempty"`

	// Reserved is true for spaces Kibana manages, such as the default space.
	// It is read-only and never sent.
	Reserved bool `json:"_reserved,omitempty"`

	// AuthorizedPurposes is returned by [SpacesService.List] when
	// ListSpacesOptions.IncludeAuthorizedPurposes is set. It is never sent.
	AuthorizedPurposes map[string]bool `json:"authorizedPurposes,omitempty"`
}

// writable returns a copy of the space without its read-only fields.
func (s *Space) writable() *Space {
	w := *s
	w.Reserved = false
	w.AuthorizedPurposes = nil
	return &w
}

// SpacePurpose is the authorization check applied when listing spaces.
type SpacePurpose string

// Purposes for [ListSpacesOptions].
const (
	SpacePurposeAny                        SpacePurpose = "any"
	SpacePurposeCopySavedObjectsIntoSpace  SpacePurpose = "copySavedObjectsIntoSpace"
	SpacePurposeShareSavedObjectsIntoSpace SpacePurpose = "shareSavedObjectsIntoSpace"
)

// ListSpacesOptions are the options for [SpacesService.List].
type ListSpacesOptions struct {
	// Purpose restricts the list to the spaces the user is authorized to use
	// for that purpose. Kibana defaults to SpacePurposeAny. Cannot be combined
	// with IncludeAuthorizedPurposes.
	Purpose SpacePurpose `url:"purpose,omitempty"`

	// IncludeAuthorizedPurposes returns every space the user can access in
	// any capacity, with the purposes the user is authorized for in
	// Space.AuthorizedPurposes.
	IncludeAuthorizedPurposes bool `url:"include_authorized_purposes,omitempty"`
}

// List returns the spaces the user is authorized to access.
//
// Kibana API: GET /api/spaces/space
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-spaces-space
func (s *SpacesService) List(ctx context.Context, opt *ListSpacesOptions, opts ...RequestOption) ([]*Space, *Response, error) {
	path, err := addQuery("/api/spaces/space", opt)
	if err != nil {
		return nil, nil, err
	}
	var spaces []*Space
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &spaces, opts)
	if err != nil {
		return nil, resp, err
	}
	return spaces, resp, nil
}

// Get returns the space with the given ID.
//
// Kibana API: GET /api/spaces/space/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-spaces-space-id
func (s *SpacesService) Get(ctx context.Context, id string, opts ...RequestOption) (*Space, *Response, error) {
	if err := requireID("space ID", id); err != nil {
		return nil, nil, err
	}
	space := new(Space)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/spaces/space/%s", id), nil, space, opts)
	if err != nil {
		return nil, resp, err
	}
	return space, resp, nil
}

// Create creates a space. ID and Name are required.
//
// Kibana API: POST /api/spaces/space
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-space
func (s *SpacesService) Create(ctx context.Context, space *Space, opts ...RequestOption) (*Space, *Response, error) {
	if space == nil {
		return nil, nil, errors.New("kibana: space must not be nil")
	}
	created := new(Space)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/space", space.writable(), created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the space with the given ID: an empty Description,
// Initials or ImageURL clears the setting, and DisabledFeatures replaces the
// disabled features. An empty Color, Solution or ProjectRouting is not sent,
// so Kibana keeps the current value. Read the space with [SpacesService.Get] and
// change it to keep its other settings. The ID of space is ignored; the space
// is not modified.
//
// Kibana API: PUT /api/spaces/space/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-put-spaces-space-id
func (s *SpacesService) Update(ctx context.Context, id string, space *Space, opts ...RequestOption) (*Space, *Response, error) {
	if err := requireID("space ID", id); err != nil {
		return nil, nil, err
	}
	if space == nil {
		return nil, nil, errors.New("kibana: space must not be nil")
	}
	body := space.writable()
	body.ID = id
	updated := new(Space)
	resp, err := s.client.do(ctx, http.MethodPut, pathf("/api/spaces/space/%s", id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes the space with the given ID, and permanently deletes all
// saved objects in it.
//
// Kibana API: DELETE /api/spaces/space/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-spaces-space-id
func (s *SpacesService) Delete(ctx context.Context, id string, opts ...RequestOption) (*Response, error) {
	if err := requireID("space ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/spaces/space/%s", id), nil, nil, opts)
}
