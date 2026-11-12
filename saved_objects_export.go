package kibana

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// ExportSavedObjectsRequest is the request of [SavedObjectsService.Export].
// Set either Types or Objects.
type ExportSavedObjectsRequest struct {
	// Types exports all objects of these types, such as "dashboard" or
	// "index-pattern".
	Types []string `json:"type,omitempty"`

	// Objects exports these objects.
	Objects []SavedObjectRef `json:"objects,omitempty"`

	// Search limits Types to the objects that match this Elasticsearch
	// simple_query_string query.
	Search string `json:"search,omitempty"`

	// HasReference limits Types to the objects that reference one of these
	// objects.
	HasReference []SavedObjectRef `json:"hasReference,omitempty"`

	// IncludeReferencesDeep also exports the objects the objects reference,
	// recursively.
	IncludeReferencesDeep bool `json:"includeReferencesDeep,omitempty"`

	// ExcludeExportDetails leaves out the export details line.
	ExcludeExportDetails bool `json:"excludeExportDetails,omitempty"`
}

// ExportDetails is the last line of an export, unless
// ExportSavedObjectsRequest.ExcludeExportDetails is set.
type ExportDetails struct {
	ExportedCount int `json:"exportedCount"`

	// MissingReferences lists the referenced objects that were not
	// exported because they do not exist.
	MissingRefCount   int              `json:"missingRefCount"`
	MissingReferences []SavedObjectRef `json:"missingReferences"`

	// ExcludedObjects lists the objects that were left out, e.g. by their
	// type's export rules.
	ExcludedObjectsCount int                    `json:"excludedObjectsCount"`
	ExcludedObjects      []ExportExcludedObject `json:"excludedObjects"`
}

// ExportExcludedObject is an object that was left out of an export.
type ExportExcludedObject struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Reason string `json:"reason,omitempty"`
}

// Export exports saved objects of the client's space as ndjson, one object
// per line, followed by an [ExportDetails] line, and writes them to w as
// Kibana sends them. Import them with [SavedObjectsService.Import].
//
// The export fails with [ErrBadRequest] if one of the Objects does not
// exist; the [ErrorResponse] attributes list the missing objects. Kibana
// limits the number of exported objects with the
// savedObjects.maxImportExportSize setting.
//
// Kibana API: POST /api/saved_objects/_export
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-export
func (s *SavedObjectsService) Export(ctx context.Context, req *ExportSavedObjectsRequest, w io.Writer, opts ...RequestOption) (*Response, error) {
	if req == nil {
		return nil, errors.New("kibana: request must not be nil")
	}
	if w == nil {
		return nil, errors.New("kibana: writer must not be nil")
	}
	switch {
	case len(req.Types) == 0 && len(req.Objects) == 0:
		return nil, errors.New("kibana: set Types or Objects")
	case len(req.Types) > 0 && len(req.Objects) > 0:
		return nil, errors.New("kibana: Types and Objects cannot be combined")
	case len(req.Objects) > 0:
		if err := validateRefs(req.Objects); err != nil {
			return nil, err
		}
	}
	return s.client.do(ctx, http.MethodPost, "/api/saved_objects/_export", req, w, opts)
}
