package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// ImportSavedObjectsResult is the result of an import, and of a copy of
// saved objects into a space.
type ImportSavedObjectsResult struct {
	// Success is true if all objects were imported.
	Success bool `json:"success"`

	SuccessCount   int             `json:"successCount"`
	SuccessResults []ImportSuccess `json:"successResults,omitempty"`
	Errors         []ImportError   `json:"errors,omitempty"`
	Warnings       []ImportWarning `json:"warnings,omitempty"`
}

// ImportSuccess is an object that was imported or copied.
type ImportSuccess struct {
	Type string          `json:"type"`
	ID   string          `json:"id"`
	Meta SavedObjectMeta `json:"meta"`

	// DestinationID is the ID of the new object, if it differs from ID.
	// Objects of types that can be shared between spaces, such as
	// index-pattern, get a new ID when they are imported into, or copied
	// to, another space, even without CreateNewCopies.
	DestinationID string `json:"destinationId,omitempty"`

	// Overwrite is true if the object overwrote an existing object.
	Overwrite bool `json:"overwrite,omitempty"`

	Managed bool `json:"managed,omitempty"`
}

// ImportErrorType is the type of an [ImportErrorDetail].
type ImportErrorType string

// Types of import errors.
const (
	// ImportErrorConflict: an object with the same ID or origin exists.
	// Retry with Overwrite or CreateNewCopy.
	ImportErrorConflict ImportErrorType = "conflict"

	// ImportErrorAmbiguousConflict: several existing objects conflict with
	// the object. Retry with Overwrite and the DestinationID of one of
	// ImportErrorDetail.Destinations.
	ImportErrorAmbiguousConflict ImportErrorType = "ambiguous_conflict"

	// ImportErrorMissingReferences: objects the object references do not
	// exist. Retry with IgnoreMissingReferences, or with
	// ImportRetry.ReplaceReferences.
	ImportErrorMissingReferences ImportErrorType = "missing_references"

	// ImportErrorUnsupportedType: objects of the type cannot be imported.
	ImportErrorUnsupportedType ImportErrorType = "unsupported_type"

	// ImportErrorUnknown: any other error.
	ImportErrorUnknown ImportErrorType = "unknown"
)

// ImportError is an object that was not imported or copied, or an error
// that stopped the copy into a space.
//
// For an object, Type, ID, Meta and Error are set. When the copy into a
// whole space failed, only StatusCode, StatusText and Message are set.
type ImportError struct {
	Type  string          `json:"type,omitempty"`
	ID    string          `json:"id,omitempty"`
	Title string          `json:"title,omitempty"`
	Meta  SavedObjectMeta `json:"meta"`
	Error *ImportErrorDetail

	// Overwrite is true if the object would have overwritten an existing
	// object.
	Overwrite bool `json:"overwrite,omitempty"`

	StatusCode int    `json:"statusCode,omitempty"`
	StatusText string `json:"-"`
	Message    string `json:"message,omitempty"`
}

// UnmarshalJSON decodes both shapes of import errors: the "error" field is
// an object for an object's error, and a string (the HTTP status text) for a
// failure of the copy into a whole space.
func (e *ImportError) UnmarshalJSON(data []byte) error {
	type plain ImportError
	var raw struct {
		plain
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = ImportError(raw.plain)
	switch {
	case len(raw.Error) == 0 || string(raw.Error) == "null":
	case raw.Error[0] == '"':
		return json.Unmarshal(raw.Error, &e.StatusText)
	default:
		e.Error = new(ImportErrorDetail)
		return json.Unmarshal(raw.Error, e.Error)
	}
	return nil
}

// ImportErrorDetail describes why an object was not imported or copied.
type ImportErrorDetail struct {
	Type ImportErrorType `json:"type"`

	// DestinationID is the ID of the conflicting object, for conflicts.
	DestinationID string `json:"destinationId,omitempty"`

	// Destinations lists the conflicting objects, for ambiguous conflicts.
	Destinations []ImportErrorDestination `json:"destinations,omitempty"`

	// References lists the missing objects, for missing references.
	References []SavedObjectRef `json:"references,omitempty"`

	// StatusCode and Message describe unknown errors.
	StatusCode int    `json:"statusCode,omitempty"`
	Message    string `json:"message,omitempty"`
}

// ImportErrorDestination is one of the objects of an ambiguous conflict.
type ImportErrorDestination struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ImportWarning is a warning about an import that succeeded.
type ImportWarning struct {
	// Type is "simple" or "action_required".
	Type    string `json:"type"`
	Message string `json:"message"`

	// ActionPath and ButtonLabel link to a page that resolves an
	// action_required warning.
	ActionPath  string `json:"actionPath,omitempty"`
	ButtonLabel string `json:"buttonLabel,omitempty"`
}

// ImportSavedObjectsOptions are the options of [SavedObjectsService.Import].
type ImportSavedObjectsOptions struct {
	// Overwrite overwrites existing objects instead of reporting
	// conflicts. It cannot be combined with CreateNewCopies.
	Overwrite bool `url:"overwrite,omitempty"`

	// CreateNewCopies imports the objects with new IDs, so that they never
	// conflict.
	CreateNewCopies bool `url:"createNewCopies,omitempty"`

	// CompatibilityMode applies adjustments to avoid compatibility problems
	// with objects created before Kibana 8.0. It cannot be combined with
	// CreateNewCopies.
	CompatibilityMode bool `url:"compatibilityMode,omitempty"`
}

// Import imports saved objects into the client's space from r, an ndjson
// export such as the output of [SavedObjectsService.Export]. The export
// details line is ignored.
//
// r is streamed to Kibana with chunked transfer encoding, without a
// Content-Length. Import does not read r after it returns: it waits for a
// pending Read of r, unless ctx is done first, in which case that Read
// can complete after Import returned.
//
// Objects that could not be imported, for example because of conflicts,
// are reported in [ImportSavedObjectsResult.Errors]; the call itself still
// succeeds. Retry them with [SavedObjectsService.ResolveImportErrors].
//
// Kibana API: POST /api/saved_objects/_import
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-import
func (s *SavedObjectsService) Import(ctx context.Context, r io.Reader, opt *ImportSavedObjectsOptions, opts ...RequestOption) (*ImportSavedObjectsResult, *Response, error) {
	if r == nil {
		return nil, nil, errors.New("kibana: reader must not be nil")
	}
	if opt != nil && opt.CreateNewCopies && (opt.Overwrite || opt.CompatibilityMode) {
		return nil, nil, errors.New("kibana: CreateNewCopies cannot be combined with Overwrite or CompatibilityMode")
	}
	path, err := addQuery("/api/saved_objects/_import", opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(ImportSavedObjectsResult)
	resp, err := s.uploadNDJSON(ctx, path, r, nil, result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// ImportRetry is an object to import again with
// [SavedObjectsService.ResolveImportErrors].
type ImportRetry struct {
	Type string `json:"type"`
	ID   string `json:"id"`

	// Overwrite overwrites the conflicting object.
	Overwrite bool `json:"overwrite,omitempty"`

	// DestinationID is the ID of the object to overwrite, or the ID to
	// import the object with.
	DestinationID string `json:"destinationId,omitempty"`

	// CreateNewCopy imports the object with a new ID.
	CreateNewCopy bool `json:"createNewCopy,omitempty"`

	// IgnoreMissingReferences imports the object although objects it
	// references are missing.
	IgnoreMissingReferences bool `json:"ignoreMissingReferences,omitempty"`

	// ReplaceReferences changes the object's references before importing
	// it, e.g. to point a visualization to another data view.
	ReplaceReferences []ImportReferenceReplacement `json:"replaceReferences,omitempty"`
}

// ImportReferenceReplacement replaces the references of an object to the
// object of type Type with ID From by references to the object with ID To.
type ImportReferenceReplacement struct {
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ResolveImportErrorsRequest is the request of
// [SavedObjectsService.ResolveImportErrors].
type ResolveImportErrorsRequest struct {
	// Retries lists the objects of the file to import. Objects of the file
	// that are not listed are not imported. Required.
	Retries []ImportRetry `url:"-"`

	// CreateNewCopies and CompatibilityMode must have the values of the
	// import that is retried.
	CreateNewCopies   bool `url:"createNewCopies,omitempty"`
	CompatibilityMode bool `url:"compatibilityMode,omitempty"`
}

// ResolveImportErrors imports saved objects from r into the client's space
// again, resolving the errors of a previous [SavedObjectsService.Import],
// for example by overwriting conflicting objects. r must be the same ndjson
// export. It is streamed like the file of Import.
//
// Kibana API: POST /api/saved_objects/_resolve_import_errors
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-resolve-import-errors
func (s *SavedObjectsService) ResolveImportErrors(ctx context.Context, r io.Reader, req *ResolveImportErrorsRequest, opts ...RequestOption) (*ImportSavedObjectsResult, *Response, error) {
	if r == nil {
		return nil, nil, errors.New("kibana: reader must not be nil")
	}
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if len(req.Retries) == 0 {
		return nil, nil, errors.New("kibana: Retries must not be empty")
	}
	if req.CreateNewCopies && req.CompatibilityMode {
		return nil, nil, errors.New("kibana: CreateNewCopies cannot be combined with CompatibilityMode")
	}
	for i, retry := range req.Retries {
		if retry.Type == "" || retry.ID == "" {
			return nil, nil, fmt.Errorf("kibana: retry %d must have a type and an ID", i)
		}
	}
	retries, err := json.Marshal(req.Retries)
	if err != nil {
		return nil, nil, fmt.Errorf("kibana: encoding retries: %w", err)
	}
	path, err := addQuery("/api/saved_objects/_resolve_import_errors", req)
	if err != nil {
		return nil, nil, err
	}
	result := new(ImportSavedObjectsResult)
	resp, err := s.uploadNDJSON(ctx, path, r, map[string]string{"retries": string(retries)}, result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// uploadNDJSON posts r as the ndjson "file" of a multipart form, with the
// given extra fields, and decodes the response into v. The form is streamed,
// not buffered.
func (s *SavedObjectsService) uploadNDJSON(ctx context.Context, path string, r io.Reader, fields map[string]string, v any, opts []RequestOption) (*Response, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	// The Content-Type comes first, so that the caller's options can
	// override it.
	opts = append([]RequestOption{WithHeader("Content-Type", mw.FormDataContentType())}, opts...)
	req, err := s.client.NewRequest(ctx, http.MethodPost, path, pr, opts...)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		pw.CloseWithError(writeNDJSONForm(mw, r, fields))
	}()
	// The transport waits for its body read to end before returning, also
	// when ctx is done; a blocked Read of r would block it.
	stop := context.AfterFunc(ctx, func() { _ = pr.CloseWithError(ctx.Err()) })
	defer stop()
	resp, err := s.client.Do(req, v)
	// Kibana can answer before it read the whole form, e.g. with a 400.
	// Stop the writer, and wait for it, so that r is not read after
	// returning, unless ctx is done: a blocked Read of r cannot be
	// interrupted.
	_ = pr.Close()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return resp, err
}

// writeNDJSONForm writes the multipart form of uploadNDJSON.
func writeNDJSONForm(mw *multipart.Writer, r io.Reader, fields map[string]string) error {
	for name, value := range fields {
		if err := mw.WriteField(name, value); err != nil {
			return err
		}
	}
	// Kibana rejects files whose name does not end in .ndjson.
	part, err := mw.CreateFormFile("file", "import.ndjson")
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, r); err != nil {
		return err
	}
	return mw.Close()
}
