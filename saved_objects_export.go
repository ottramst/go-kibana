package kibana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ExportSavedObjectsRequest is the request of [SavedObjectsService.Export].
// Set either Types or Objects.
type ExportSavedObjectsRequest struct {
	_ [0]func()

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
	_ [0]func()

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
	_ [0]func()

	SavedObjectRef
	Reason string `json:"reason,omitempty"`
}

// ExportSavedObjectsOptions are the options of [SavedObjectsService.Export].
type ExportSavedObjectsOptions struct {
	_ [0]func()
}

// Export exports saved objects of the client's space as ndjson, one object
// per line, followed by an [ExportDetails] line, and writes them to w as
// Kibana sends them. Import them with [SavedObjectsService.Import], which
// ignores the details line.
//
// Export returns the details line decoded, or nil if
// req.ExcludeExportDetails is set. It keeps only the line being written in
// memory, so its memory use is bounded by the longest line.
//
// The export fails with [ErrBadRequest] if one of the Objects does not
// exist; the [ErrorResponse] attributes list the missing objects. Kibana
// limits the number of exported objects with the
// savedObjects.maxImportExportSize setting. If the export fails while it is
// streamed, w has received part of it.
//
// Kibana API: POST /api/saved_objects/_export
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-export
func (s *SavedObjectsService) Export(ctx context.Context, w io.Writer, req *ExportSavedObjectsRequest, opt *ExportSavedObjectsOptions, opts ...RequestOption) (*ExportDetails, *Response, error) {
	if w == nil {
		return nil, nil, errors.New("kibana: writer must not be nil")
	}
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	switch {
	case len(req.Types) == 0 && len(req.Objects) == 0:
		return nil, nil, errors.New("kibana: set Types or Objects")
	case len(req.Types) > 0 && len(req.Objects) > 0:
		return nil, nil, errors.New("kibana: Types and Objects cannot be combined")
	case len(req.Objects) > 0:
		if err := validateRefs(req.Objects); err != nil {
			return nil, nil, err
		}
	}
	if len(req.HasReference) > 0 {
		if err := validateRefs(req.HasReference); err != nil {
			return nil, nil, fmt.Errorf("kibana: HasReference: %s", strings.TrimPrefix(err.Error(), "kibana: "))
		}
	}
	lw := &lastLineWriter{w: w}
	resp, err := s.client.do(ctx, http.MethodPost, "/api/saved_objects/_export", req, lw, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	if req.ExcludeExportDetails {
		return nil, resp, nil
	}
	details, err := parseExportDetails(lw.lastLine())
	if err != nil {
		return nil, resp, err
	}
	return details, resp, nil
}

// parseExportDetails decodes the details line of an export. Saved object
// lines also decode into ExportDetails, so the line must have an
// exportedCount field.
func parseExportDetails(line []byte) (*ExportDetails, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(line, &fields); err != nil || fields["exportedCount"] == nil {
		return nil, errors.New("kibana: the export has no details line")
	}
	details := new(ExportDetails)
	if err := json.Unmarshal(line, details); err != nil {
		return nil, fmt.Errorf("kibana: decoding the export details: %w", err)
	}
	return details, nil
}

// lastLineWriter writes to w and keeps the last non-empty line written.
type lastLineWriter struct {
	w io.Writer

	// cur is the line being written, and last the last complete
	// non-empty line.
	cur, last []byte
}

func (l *lastLineWriter) Write(p []byte) (int, error) {
	n, err := l.w.Write(p)
	rest := p[:n]
	for len(rest) > 0 {
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			l.cur = append(l.cur, rest...)
			break
		}
		l.cur = append(l.cur, rest[:i]...)
		if len(bytes.TrimSpace(l.cur)) > 0 {
			l.last, l.cur = l.cur, l.last[:0]
		} else {
			l.cur = l.cur[:0]
		}
		rest = rest[i+1:]
	}
	return n, err
}

// lastLine returns the last non-empty line, including an unterminated one.
func (l *lastLineWriter) lastLine() []byte {
	if len(bytes.TrimSpace(l.cur)) > 0 {
		return l.cur
	}
	return l.last
}
