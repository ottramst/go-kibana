package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// CopySavedObjectsRequest is the request of [SpacesService.CopySavedObjects].
type CopySavedObjectsRequest struct {
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
	// Objects lists the objects that were copied.
	Objects []SavedObjectRef `json:"objects"`

	// Retries maps target space IDs to the objects to copy into them. List
	// every object to copy into a space, including the objects that were
	// copied successfully before: objects that are not listed are not
	// copied.
	Retries map[string][]CopyRetry `json:"retries"`

	// IncludeReferences also copies the objects the objects reference.
	IncludeReferences bool `json:"includeReferences,omitempty"`

	// CreateNewCopies creates the copies with new IDs. Kibana defaults to
	// true when it is nil, and then ignores CopyRetry.Overwrite. Set it to
	// new(false) to overwrite objects.
	CreateNewCopies *bool `json:"createNewCopies,omitempty"`

	// CompatibilityMode applies adjustments to avoid compatibility problems
	// with copies of objects created before Kibana 8.0. It requires
	// CreateNewCopies to be false.
	CompatibilityMode bool `json:"compatibilityMode,omitempty"`
}

// CopyRetry is an object to copy again with [SpacesService.ResolveCopySavedObjectsErrors].
type CopyRetry struct {
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

// CopySavedObjectsResult is the result of copying saved objects into one
// space.
type CopySavedObjectsResult struct {
	// Success is true if all objects were copied.
	Success bool `json:"success"`

	SuccessCount   int                       `json:"successCount"`
	SuccessResults []CopySavedObjectsSuccess `json:"successResults,omitempty"`
	Errors         []CopySavedObjectsError   `json:"errors,omitempty"`
}

// CopySavedObjectsSuccess is an object that was copied.
type CopySavedObjectsSuccess struct {
	Type string          `json:"type"`
	ID   string          `json:"id"`
	Meta SavedObjectMeta `json:"meta"`

	// DestinationID is the ID of the copy. Copies of objects that can be
	// shared between spaces get a new ID even without CreateNewCopies.
	DestinationID string `json:"destinationId,omitempty"`

	// Overwrite is true if the copy overwrote an existing object.
	Overwrite bool `json:"overwrite,omitempty"`

	Managed bool `json:"managed,omitempty"`
}

// CopyErrorType is the type of a [CopyErrorDetail].
type CopyErrorType string

// Types of copy errors.
const (
	// CopyErrorConflict: an object with the same ID or origin exists in the
	// target space. Retry with CopyRetry.Overwrite, or with
	// CopyRetry.CreateNewCopy.
	CopyErrorConflict CopyErrorType = "conflict"

	// CopyErrorAmbiguousConflict: several objects in the target space
	// conflict with the object. Retry with CopyRetry.Overwrite and the
	// DestinationID of one of CopyErrorDetail.Destinations.
	CopyErrorAmbiguousConflict CopyErrorType = "ambiguous_conflict"

	// CopyErrorMissingReferences: objects the object references do not
	// exist. Retry with CopyRetry.IgnoreMissingReferences.
	CopyErrorMissingReferences CopyErrorType = "missing_references"

	// CopyErrorUnsupportedType: objects of the type cannot be copied.
	CopyErrorUnsupportedType CopyErrorType = "unsupported_type"

	// CopyErrorUnknown: any other error.
	CopyErrorUnknown CopyErrorType = "unknown"
)

// CopySavedObjectsError is an object that was not copied, or an error that
// stopped the copy into a space.
//
// For an object, Type, ID, Meta and Error are set. When the copy into the
// whole space failed, only StatusCode, StatusText and Message are set.
type CopySavedObjectsError struct {
	Type  string          `json:"type,omitempty"`
	ID    string          `json:"id,omitempty"`
	Title string          `json:"title,omitempty"`
	Meta  SavedObjectMeta `json:"meta"`
	Error *CopyErrorDetail

	StatusCode int    `json:"statusCode,omitempty"`
	StatusText string `json:"-"`
	Message    string `json:"message,omitempty"`
}

// UnmarshalJSON decodes both shapes of copy errors: the "error" field is an
// object for an object's error, and a string (the HTTP status text) for a
// failure of the whole space.
func (e *CopySavedObjectsError) UnmarshalJSON(data []byte) error {
	type plain CopySavedObjectsError
	var raw struct {
		plain
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = CopySavedObjectsError(raw.plain)
	switch {
	case len(raw.Error) == 0 || string(raw.Error) == "null":
	case raw.Error[0] == '"':
		return json.Unmarshal(raw.Error, &e.StatusText)
	default:
		e.Error = new(CopyErrorDetail)
		return json.Unmarshal(raw.Error, e.Error)
	}
	return nil
}

// CopyErrorDetail describes why an object was not copied.
type CopyErrorDetail struct {
	Type CopyErrorType `json:"type"`

	// DestinationID is the ID of the conflicting object, for conflicts.
	DestinationID string `json:"destinationId,omitempty"`

	// Destinations lists the conflicting objects, for ambiguous conflicts.
	Destinations []CopyErrorDestination `json:"destinations,omitempty"`

	// References lists the missing objects, for missing references.
	References []SavedObjectRef `json:"references,omitempty"`
}

// CopyErrorDestination is one of the objects of an ambiguous conflict.
type CopyErrorDestination struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// validateCopyModes returns an error for option combinations Kibana rejects
// or silently ignores.
func validateCopyModes(overwrite, compatibilityMode bool, createNewCopies *bool) error {
	if (overwrite || compatibilityMode) && (createNewCopies == nil || *createNewCopies) {
		return errors.New("kibana: Overwrite and CompatibilityMode require CreateNewCopies to be new(false); Kibana creates new copies by default")
	}
	return nil
}

// CopySavedObjects copies saved objects from the client's space into other
// spaces. Use [Client.InSpace] to copy from another space.
//
// The result maps each target space to the outcome of the copy into it.
// Objects that could not be copied, for example because of conflicts, are
// reported in [CopySavedObjectsResult.Errors]; the call itself still
// succeeds. Retry them with [SpacesService.ResolveCopySavedObjectsErrors].
// The call fails with [ErrNotFound] if one of the objects does not exist.
//
// Kibana API: POST /api/spaces/_copy_saved_objects
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-copy-saved-objects
func (s *SpacesService) CopySavedObjects(ctx context.Context, req *CopySavedObjectsRequest, opts ...RequestOption) (map[string]*CopySavedObjectsResult, *Response, error) {
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
	var result map[string]*CopySavedObjectsResult
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/_copy_saved_objects", req, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// ResolveCopySavedObjectsErrors copies saved objects from the client's space
// into other spaces again, resolving the errors of a previous
// [SpacesService.CopySavedObjects] call, for example by overwriting
// conflicting objects. Use [Client.InSpace] to copy from another space.
//
// The result has the same form as the result of CopySavedObjects.
//
// Kibana API: POST /api/spaces/_resolve_copy_saved_objects_errors
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-resolve-copy-saved-objects-errors
func (s *SpacesService) ResolveCopySavedObjectsErrors(ctx context.Context, req *ResolveCopySavedObjectsErrorsRequest, opts ...RequestOption) (map[string]*CopySavedObjectsResult, *Response, error) {
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
	var result map[string]*CopySavedObjectsResult
	resp, err := s.client.do(ctx, http.MethodPost, "/api/spaces/_resolve_copy_saved_objects_errors", req, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
