package kibana

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// CopySavedObjectsRequest is the request of [SpacesService.CopySavedObjects].
type CopySavedObjectsRequest struct {
	_ [0]func()

	// Spaces lists the IDs of the spaces to copy the objects into. Kibana
	// does not check that they exist: copying into a space that does not
	// exist succeeds and creates objects that no space shows.
	Spaces []string `json:"spaces"`

	// Objects lists the objects to copy.
	Objects []SavedObjectRef `json:"objects"`

	// IncludeReferences also copies the objects the objects reference.
	IncludeReferences bool `json:"includeReferences,omitempty"`

	// Overwrite overwrites objects that already exist in a target space.
	// It requires CreateNewCopies to be false.
	Overwrite bool `json:"overwrite,omitempty"`

	// CreateNewCopies creates the copies with new IDs, so that they never
	// conflict. Kibana defaults to true when it is nil. Set it to new(false)
	// to detect conflicts, or to use Overwrite or CompatibilityMode.
	CreateNewCopies *bool `json:"createNewCopies,omitempty"`

	// CompatibilityMode applies adjustments to avoid compatibility problems
	// with copies of objects created before Kibana 8.0. It requires
	// CreateNewCopies to be false.
	CompatibilityMode bool `json:"compatibilityMode,omitempty"`
}

// ResolveCopySavedObjectsErrorsRequest is the request of
// [SpacesService.ResolveCopySavedObjectsErrors].
type ResolveCopySavedObjectsErrorsRequest struct {
	_ [0]func()

	// Objects lists the objects that were copied.
	Objects []SavedObjectRef `json:"objects"`

	// Retries maps target space IDs to the objects to copy into them. List
	// every object to copy into a space, including the objects that were
	// copied successfully before: objects that are not listed are not
	// copied.
	Retries map[string][]CopySavedObjectsRetry `json:"retries"`

	// IncludeReferences also copies the objects the objects reference.
	IncludeReferences bool `json:"includeReferences,omitempty"`

	// CreateNewCopies creates the copies with new IDs. Kibana defaults to
	// true when it is nil, and then ignores CopySavedObjectsRetry.Overwrite. Set it to
	// new(false) to overwrite objects.
	CreateNewCopies *bool `json:"createNewCopies,omitempty"`

	// CompatibilityMode applies adjustments to avoid compatibility problems
	// with copies of objects created before Kibana 8.0. It requires
	// CreateNewCopies to be false.
	CompatibilityMode bool `json:"compatibilityMode,omitempty"`
}

// CopySavedObjectsRetry is an object to copy again with [SpacesService.ResolveCopySavedObjectsErrors].
type CopySavedObjectsRetry struct {
	Type string `json:"type"`
	ID   string `json:"id"`

	// Overwrite overwrites the conflicting object in the target space.
	Overwrite bool `json:"overwrite,omitempty"`

	// DestinationID is the ID of the object to overwrite, for conflicts
	// that report a destination ID.
	DestinationID string `json:"destinationId,omitempty"`

	// CreateNewCopy copies the object with a new ID.
	CreateNewCopy bool `json:"createNewCopy,omitempty"`

	// IgnoreMissingReferences copies the object although objects it
	// references are missing.
	IgnoreMissingReferences bool `json:"ignoreMissingReferences,omitempty"`
}

// validateCopyModes returns an error for option combinations Kibana rejects
// or silently ignores.
func validateCopyModes(overwrite, compatibilityMode bool, createNewCopies *bool) error {
	if (overwrite || compatibilityMode) && (createNewCopies == nil || *createNewCopies) {
		return errors.New("kibana: Overwrite and CompatibilityMode require CreateNewCopies to be new(false); Kibana creates new copies by default")
	}
	return nil
}

// CopySavedObjectsOptions are the options of [SpacesService.CopySavedObjects].
type CopySavedObjectsOptions struct {
	_ [0]func()
}

// CopySavedObjects copies saved objects from the client's space into other
// spaces. Use [Client.InSpace] to copy from another space.
//
// The result maps each target space to the outcome of the copy into it.
// Objects that could not be copied, for example because of conflicts, are
// reported in [ImportSavedObjectsResult.Errors]; the call itself still
// succeeds. Retry them with [SpacesService.ResolveCopySavedObjectsErrors].
// The call fails with [ErrNotFound] if one of the objects does not exist.
//
// Kibana API: POST /api/spaces/_copy_saved_objects
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-copy-saved-objects
func (s *SpacesService) CopySavedObjects(ctx context.Context, req *CopySavedObjectsRequest, opt *CopySavedObjectsOptions, opts ...RequestOption) (map[string]*ImportSavedObjectsResult, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if len(req.Spaces) == 0 {
		return nil, nil, errors.New("kibana: Spaces must not be empty")
	}
	for i, id := range req.Spaces {
		if id == "" {
			return nil, nil, fmt.Errorf("kibana: space %d must not be empty", i)
		}
	}
	if err := validateRefs(req.Objects); err != nil {
		return nil, nil, err
	}
	if err := validateCopyModes(req.Overwrite, req.CompatibilityMode, req.CreateNewCopies); err != nil {
		return nil, nil, err
	}
	var result map[string]*ImportSavedObjectsResult
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/_copy_saved_objects", req, &result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// ResolveCopySavedObjectsErrorsOptions are the options of [SpacesService.ResolveCopySavedObjectsErrors].
type ResolveCopySavedObjectsErrorsOptions struct {
	_ [0]func()
}

// ResolveCopySavedObjectsErrors copies saved objects from the client's space
// into other spaces again, resolving the errors of a previous
// [SpacesService.CopySavedObjects] call, for example by overwriting
// conflicting objects. Use [Client.InSpace] to copy from another space.
//
// The result has the same form as the result of CopySavedObjects.
//
// Kibana API: POST /api/spaces/_resolve_copy_saved_objects_errors
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-resolve-copy-saved-objects-errors
func (s *SpacesService) ResolveCopySavedObjectsErrors(ctx context.Context, req *ResolveCopySavedObjectsErrorsRequest, opt *ResolveCopySavedObjectsErrorsOptions, opts ...RequestOption) (map[string]*ImportSavedObjectsResult, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if err := validateRefs(req.Objects); err != nil {
		return nil, nil, err
	}
	if len(req.Retries) == 0 {
		return nil, nil, errors.New("kibana: Retries must not be empty")
	}
	overwrite := false
	for space, retries := range req.Retries {
		if space == "" {
			return nil, nil, errors.New("kibana: Retries must not have an empty space ID")
		}
		for i, r := range retries {
			if r.Type == "" || r.ID == "" {
				return nil, nil, fmt.Errorf("kibana: retry %d for space %q must have a type and an ID", i, space)
			}
			overwrite = overwrite || r.Overwrite
		}
	}
	// Kibana ignores Overwrite while CreateNewCopies is true, and creates
	// new copies instead.
	if err := validateCopyModes(overwrite, req.CompatibilityMode, req.CreateNewCopies); err != nil {
		return nil, nil, err
	}
	var result map[string]*ImportSavedObjectsResult
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/_resolve_copy_saved_objects_errors", req, &result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
