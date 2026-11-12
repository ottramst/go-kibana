package kibana

import (
	"context"
	"errors"
	"net/http"
)

// ShareableReference describes a saved object and its references, and the
// spaces they are in.
type ShareableReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`

	// Spaces lists the spaces the object is in. It is empty for objects that
	// cannot be shared, which exist in a single space.
	Spaces []string `json:"spaces"`

	// InboundReferences lists the objects that reference this object.
	InboundReferences []InboundReference `json:"inboundReferences"`

	// IsMissing is true if the object does not exist.
	IsMissing bool `json:"isMissing,omitempty"`

	// SpacesWithMatchingAliases lists spaces that have a legacy URL alias
	// for the object; sharing the object into them causes a conflict.
	SpacesWithMatchingAliases []string `json:"spacesWithMatchingAliases,omitempty"`

	// SpacesWithMatchingOrigins lists spaces that have a copy of the object,
	// so that sharing the object into them causes a conflict.
	SpacesWithMatchingOrigins []string `json:"spacesWithMatchingOrigins,omitempty"`
}

// InboundReference is a reference from another saved object.
type InboundReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetShareableReferences returns the given saved objects, the objects they
// reference, and the spaces they are in. The objects are looked up in the
// client's space; use [Client.InSpace] for another space.
//
// Kibana API: POST /api/spaces/_get_shareable_references
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-get-shareable-references
func (s *SpacesService) GetShareableReferences(ctx context.Context, objects []SavedObjectRef, opts ...RequestOption) ([]*ShareableReference, *Response, error) {
	if err := validateRefs(objects); err != nil {
		return nil, nil, err
	}
	body := struct {
		Objects []SavedObjectRef `json:"objects"`
	}{objects}
	var result struct {
		Objects []*ShareableReference `json:"objects"`
	}
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/_get_shareable_references", body, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result.Objects, resp, nil
}

// UpdateObjectsSpacesRequest is the request of [SpacesService.UpdateObjectsSpaces].
type UpdateObjectsSpacesRequest struct {
	Objects []SavedObjectRef `json:"objects"`

	// SpacesToAdd lists the spaces to share the objects into. "*" shares
	// them into all current and future spaces.
	SpacesToAdd []string `json:"spacesToAdd"`

	// SpacesToRemove lists the spaces to remove the objects from.
	SpacesToRemove []string `json:"spacesToRemove"`
}

// ObjectSpaces is the result of [SpacesService.UpdateObjectsSpaces] for one
// object.
type ObjectSpaces struct {
	Type string `json:"type"`
	ID   string `json:"id"`

	// Spaces lists the spaces the object is in after the update. It is empty
	// if the object was removed from all its spaces, which deletes it.
	Spaces []string `json:"spaces"`

	// Error is set if the object could not be updated, for example because
	// it does not exist (status code 404).
	Error *ObjectError `json:"error,omitempty"`
}

// UpdateObjectsSpaces shares saved objects into spaces and removes them from
// spaces. The objects are looked up in the client's space; use
// [Client.InSpace] for another space. Removing an object from all its spaces
// deletes it.
//
// Objects that fail, for example because they do not exist, are reported in
// [ObjectSpaces.Error]; the call itself still succeeds.
//
// Kibana API: POST /api/spaces/_update_objects_spaces
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-update-objects-spaces
func (s *SpacesService) UpdateObjectsSpaces(ctx context.Context, req *UpdateObjectsSpacesRequest, opts ...RequestOption) ([]*ObjectSpaces, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if err := validateRefs(req.Objects); err != nil {
		return nil, nil, err
	}
	if len(req.SpacesToAdd) == 0 && len(req.SpacesToRemove) == 0 {
		return nil, nil, errors.New("kibana: SpacesToAdd or SpacesToRemove must not be empty")
	}
	// Both lists are required: Kibana rejects null.
	body := *req
	if body.SpacesToAdd == nil {
		body.SpacesToAdd = []string{}
	}
	if body.SpacesToRemove == nil {
		body.SpacesToRemove = []string{}
	}
	var result struct {
		Objects []*ObjectSpaces `json:"objects"`
	}
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/_update_objects_spaces", &body, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result.Objects, resp, nil
}
