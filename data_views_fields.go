package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// DataViewFieldMetadata changes the presentation of a field of a data view
// with [DataViewsService.UpdateFieldsMetadata]. A nil field is left
// unchanged, and a pointer to the zero value ("", 0, or a format with an
// empty ID) removes the value.
type DataViewFieldMetadata struct {
	CustomLabel *string

	// CustomDescription has at most 300 characters.
	CustomDescription *string

	// Count is the popularity of the field.
	Count *int

	// Format is the display format of the field. It is stored in
	// DataView.FieldFormats.
	Format *DataViewFieldFormat
}

// MarshalJSON encodes the metadata as Kibana expects it: null removes a
// value, and fields that are not set are left out.
func (m DataViewFieldMetadata) MarshalJSON() ([]byte, error) {
	fields := map[string]any{}
	if m.CustomLabel != nil {
		fields["customLabel"] = emptyAsNull(*m.CustomLabel, "")
	}
	if m.CustomDescription != nil {
		fields["customDescription"] = emptyAsNull(*m.CustomDescription, "")
	}
	if m.Count != nil {
		fields["count"] = emptyAsNull(*m.Count, 0)
	}
	if m.Format != nil {
		if m.Format.ID == "" {
			fields["format"] = nil
		} else {
			fields["format"] = m.Format
		}
	}
	return json.Marshal(fields)
}

// emptyAsNull returns nil if v is the zero value, and v otherwise.
func emptyAsNull[T comparable](v, zero T) any {
	if v == zero {
		return nil
	}
	return v
}

// UpdateFieldsMetadataOptions are the options of [DataViewsService.UpdateFieldsMetadata].
type UpdateFieldsMetadataOptions struct {
	_ [0]func()
}

// UpdateFieldsMetadataRequest is the request of
// [DataViewsService.UpdateFieldsMetadata].
type UpdateFieldsMetadataRequest struct {
	_ [0]func()

	// Fields maps field names to the changes. The fields do not need to
	// exist in the mappings. Required.
	Fields map[string]DataViewFieldMetadata `json:"fields"`
}

// UpdateFieldsMetadata changes the custom labels, descriptions, popularity
// and formats of fields of a data view of the client's space. It returns the
// data view without its fields.
//
// Kibana API: POST /api/data_views/data_view/{viewId}/fields
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-updatefieldsmetadatadefault
func (s *DataViewsService) UpdateFieldsMetadata(ctx context.Context, viewID string, req *UpdateFieldsMetadataRequest, opt *UpdateFieldsMetadataOptions, opts ...RequestOption) (*DataView, *Response, error) {
	if err := requireID("data view ID", viewID); err != nil {
		return nil, nil, err
	}
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if len(req.Fields) == 0 {
		return nil, nil, errors.New("kibana: Fields must not be empty")
	}
	for name, m := range req.Fields {
		if name == "" {
			return nil, nil, errors.New("kibana: field names must not be empty")
		}
		if m.CustomDescription != nil && len([]rune(*m.CustomDescription)) > 300 {
			return nil, nil, fmt.Errorf("kibana: field %q: CustomDescription must have at most 300 characters", name)
		}
	}
	return s.dataView(ctx, http.MethodPost, pathf("/api/data_views/data_view/%s/fields", viewID), req, opt, opts)
}

// RuntimeFieldResult is the result of the runtime field methods of
// [DataViewsService]: the data view, without its fields, and the runtime
// field. A composite runtime field has an entry for each of its subfields,
// named "<field>.<subfield>".
type RuntimeFieldResult struct {
	DataView *DataView       `json:"data_view"`
	Fields   []DataViewField `json:"fields"`
}

// runtimeFieldRequest is the body of the runtime field create and
// create-or-update requests.
type runtimeFieldRequest struct {
	Name         string        `json:"name,omitempty"`
	RuntimeField *RuntimeField `json:"runtimeField"`
}

// CreateRuntimeFieldOptions are the options of [DataViewsService.CreateRuntimeField].
type CreateRuntimeFieldOptions struct {
	_ [0]func()
}

// CreateRuntimeField adds a runtime field to a data view of the client's
// space. It fails with [ErrBadRequest] if the data view already has a field
// with that name, including a mapped field.
//
// rf.CustomLabel, rf.CustomDescription, rf.Popularity and rf.Format are
// stored as the field's metadata.
//
// Kibana API: POST /api/data_views/data_view/{viewId}/runtime_field
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-createruntimefielddefault
func (s *DataViewsService) CreateRuntimeField(ctx context.Context, viewID, name string, rf *RuntimeField, opt *CreateRuntimeFieldOptions, opts ...RequestOption) (*RuntimeFieldResult, *Response, error) {
	return s.putRuntimeField(ctx, http.MethodPost, viewID, name, rf, opt, opts)
}

// CreateOrUpdateRuntimeFieldOptions are the options of [DataViewsService.CreateOrUpdateRuntimeField].
type CreateOrUpdateRuntimeFieldOptions struct {
	_ [0]func()
}

// CreateOrUpdateRuntimeField adds a runtime field to a data view of the
// client's space, or replaces the runtime field with that name.
//
// Kibana API: PUT /api/data_views/data_view/{viewId}/runtime_field
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-createupdateruntimefielddefault
func (s *DataViewsService) CreateOrUpdateRuntimeField(ctx context.Context, viewID, name string, rf *RuntimeField, opt *CreateOrUpdateRuntimeFieldOptions, opts ...RequestOption) (*RuntimeFieldResult, *Response, error) {
	return s.putRuntimeField(ctx, http.MethodPut, viewID, name, rf, opt, opts)
}

func (s *DataViewsService) putRuntimeField(ctx context.Context, method, viewID, name string, rf *RuntimeField, query any, opts []RequestOption) (*RuntimeFieldResult, *Response, error) {
	if err := requireID("data view ID", viewID); err != nil {
		return nil, nil, err
	}
	if err := requireID("runtime field name", name); err != nil {
		return nil, nil, err
	}
	if rf == nil || rf.Type == "" {
		return nil, nil, errors.New("kibana: runtime field must have a Type")
	}
	return s.runtimeField(ctx, method, pathf("/api/data_views/data_view/%s/runtime_field", viewID), &runtimeFieldRequest{Name: name, RuntimeField: rf}, query, opts)
}

// GetRuntimeFieldOptions are the options of [DataViewsService.GetRuntimeField].
type GetRuntimeFieldOptions struct {
	_ [0]func()
}

// GetRuntimeField returns a runtime field of a data view of the client's
// space. Kibana fails to return composite runtime fields: it answers
// [ErrBadRequest] for them, and [ErrNotFound] for their subfields.
//
// Kibana API: GET /api/data_views/data_view/{viewId}/runtime_field/{fieldName}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-getruntimefielddefault
func (s *DataViewsService) GetRuntimeField(ctx context.Context, viewID, name string, opt *GetRuntimeFieldOptions, opts ...RequestOption) (*RuntimeFieldResult, *Response, error) {
	path, err := runtimeFieldPath(viewID, name)
	if err != nil {
		return nil, nil, err
	}
	return s.runtimeField(ctx, http.MethodGet, path, nil, opt, opts)
}

// UpdateRuntimeFieldOptions are the options of [DataViewsService.UpdateRuntimeField].
type UpdateRuntimeFieldOptions struct {
	_ [0]func()
}

// UpdateRuntimeField changes a runtime field of a data view of the client's
// space: its type, script or metadata. Fields of rf that are not set are
// kept.
//
// Kibana API: POST /api/data_views/data_view/{viewId}/runtime_field/{fieldName}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-updateruntimefielddefault
func (s *DataViewsService) UpdateRuntimeField(ctx context.Context, viewID, name string, rf *RuntimeField, opt *UpdateRuntimeFieldOptions, opts ...RequestOption) (*RuntimeFieldResult, *Response, error) {
	path, err := runtimeFieldPath(viewID, name)
	if err != nil {
		return nil, nil, err
	}
	if rf == nil {
		return nil, nil, errors.New("kibana: runtime field must not be nil")
	}
	return s.runtimeField(ctx, http.MethodPost, path, &runtimeFieldRequest{RuntimeField: rf}, opt, opts)
}

// DeleteRuntimeFieldOptions are the options of [DataViewsService.DeleteRuntimeField].
type DeleteRuntimeFieldOptions struct {
	_ [0]func()
}

// DeleteRuntimeField deletes a runtime field of a data view of the client's
// space.
//
// Kibana API: DELETE /api/data_views/data_view/{viewId}/runtime_field/{fieldName}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-deleteruntimefielddefault
func (s *DataViewsService) DeleteRuntimeField(ctx context.Context, viewID, name string, opt *DeleteRuntimeFieldOptions, opts ...RequestOption) (*Response, error) {
	path, err := runtimeFieldPath(viewID, name)
	if err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, path, nil, nil, opt, opts)
}

// runtimeFieldPath returns the path of a runtime field.
func runtimeFieldPath(viewID, name string) (string, error) {
	if err := requireID("data view ID", viewID); err != nil {
		return "", err
	}
	if err := requireID("runtime field name", name); err != nil {
		return "", err
	}
	return pathf("/api/data_views/data_view/%s/runtime_field/%s", viewID, name), nil
}

// runtimeField sends a request whose response is a [RuntimeFieldResult].
func (s *DataViewsService) runtimeField(ctx context.Context, method, path string, body, query any, opts []RequestOption) (*RuntimeFieldResult, *Response, error) {
	result := new(RuntimeFieldResult)
	resp, err := s.client.do(ctx, method, path, body, result, query, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// SwapReferencesRequest is the request of [DataViewsService.SwapReferences]
// and [DataViewsService.PreviewSwapReferences].
type SwapReferencesRequest struct {
	_ [0]func()

	// FromID is the ID of the object the saved objects reference now.
	// Required.
	FromID string `json:"fromId"`

	// FromType is the type of that object, "index-pattern" if empty.
	FromType string `json:"fromType,omitempty"`

	// ToID is the ID of the object to reference instead, of the same type.
	// It must exist. Required.
	ToID string `json:"toId"`

	// ForID limits the change to the saved objects with these IDs. Nil
	// means all saved objects; an empty, non-nil slice is rejected, so that
	// an empty list of IDs does not change every saved object.
	ForID []string `json:"forId,omitempty"`

	// ForType limits the change to the saved objects of this type.
	ForType string `json:"forType,omitempty"`

	// Delete deletes the FromID object if no saved object references it
	// after the swap.
	Delete bool `json:"delete,omitempty"`
}

// SwapReferencesResult is the result of [DataViewsService.SwapReferences]
// and [DataViewsService.PreviewSwapReferences].
type SwapReferencesResult struct {
	// Result lists the saved objects whose references are, or would be,
	// changed.
	Result []SavedObjectRef `json:"result"`

	// DeleteStatus is set when SwapReferencesRequest.Delete is.
	DeleteStatus *SwapReferencesDeleteStatus `json:"deleteStatus,omitempty"`
}

// SwapReferencesDeleteStatus reports whether a swap deleted the object
// whose references it changed.
type SwapReferencesDeleteStatus struct {
	// DeletePerformed reports whether the object whose references were
	// swapped was deleted.
	DeletePerformed bool `json:"deletePerformed"`

	// RemainingRefs is the number of saved objects that still reference
	// the object, which is not deleted if it is not 0.
	RemainingRefs int `json:"remainingRefs"`
}

// SwapReferencesOptions are the options of [DataViewsService.SwapReferences].
type SwapReferencesOptions struct {
	_ [0]func()
}

// PreviewSwapReferencesOptions are the options of
// [DataViewsService.PreviewSwapReferences].
type PreviewSwapReferencesOptions struct {
	_ [0]func()
}

// SwapReferences changes the references of the saved objects of the
// client's space from one data view, or other object, to another, e.g.
// to point the visualizations of a data view to its replacement.
//
// Kibana API: POST /api/data_views/swap_references
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-swapdataviewsdefault
func (s *DataViewsService) SwapReferences(ctx context.Context, req *SwapReferencesRequest, opt *SwapReferencesOptions, opts ...RequestOption) (*SwapReferencesResult, *Response, error) {
	return s.swapReferences(ctx, "/api/data_views/swap_references", req, opt, opts)
}

// PreviewSwapReferences returns the saved objects whose references
// [DataViewsService.SwapReferences] would change, without changing them or
// deleting anything.
//
// Kibana API: POST /api/data_views/swap_references/_preview
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-previewswapdataviewsdefault
func (s *DataViewsService) PreviewSwapReferences(ctx context.Context, req *SwapReferencesRequest, opt *PreviewSwapReferencesOptions, opts ...RequestOption) (*SwapReferencesResult, *Response, error) {
	return s.swapReferences(ctx, "/api/data_views/swap_references/_preview", req, opt, opts)
}

func (s *DataViewsService) swapReferences(ctx context.Context, path string, req *SwapReferencesRequest, query any, opts []RequestOption) (*SwapReferencesResult, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if req.FromID == "" || req.ToID == "" {
		return nil, nil, errors.New("kibana: FromID and ToID must not be empty")
	}
	if req.ForID != nil && len(req.ForID) == 0 {
		return nil, nil, errors.New("kibana: ForID must not be empty; use nil for all saved objects")
	}
	for i, id := range req.ForID {
		if id == "" {
			return nil, nil, fmt.Errorf("kibana: ForID %d must not be empty", i)
		}
	}
	result := new(SwapReferencesResult)
	resp, err := s.client.do(ctx, http.MethodPost, path, req, result, query, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
