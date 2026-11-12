package kibana

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// BulkCreateSavedObjectsOptions are the options of
// [SavedObjectsService.BulkCreate].
type BulkCreateSavedObjectsOptions struct {
	// Overwrite replaces objects with the same ID instead of reporting
	// conflicts.
	Overwrite bool `url:"overwrite,omitempty"`
}

// BulkCreate creates saved objects in the client's space. Kibana generates
// the IDs that are empty.
//
// The result has an object for each of objs, in the same order. Objects
// that could not be created, for example because of a conflict, have
// [SavedObject.Error] set; the call itself still succeeds.
//
// Deprecated: Kibana deprecated the saved objects bulk API in 8.7. Use
// [SavedObjectsService.Import].
//
// Kibana API: POST /api/saved_objects/_bulk_create
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-bulk-create
func (s *SavedObjectsService) BulkCreate(ctx context.Context, objs []*SavedObject, opt *BulkCreateSavedObjectsOptions, opts ...RequestOption) ([]*SavedObject, *Response, error) {
	body, err := bulkWrites(objs, false)
	if err != nil {
		return nil, nil, err
	}
	path, err := addQuery("/api/saved_objects/_bulk_create", opt)
	if err != nil {
		return nil, nil, err
	}
	return s.bulkObjects(ctx, http.MethodPost, path, body, opts)
}

// BulkGetSavedObjectsRequest is an object to get with
// [SavedObjectsService.BulkGet].
type BulkGetSavedObjectsRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`

	// Fields limits the returned attributes to these fields.
	Fields []string `json:"fields,omitempty"`

	// Namespaces gets the object from these spaces instead of the
	// client's space.
	Namespaces []string `json:"namespaces,omitempty"`
}

// BulkGet returns saved objects of the client's space.
//
// The result has an object for each request, in the same order. Objects
// that could not be read, such as missing objects, have
// [SavedObject.Error] set; the call itself still succeeds.
//
// Deprecated: Kibana deprecated the saved objects bulk API in 8.7. Use
// [SavedObjectsService.Export].
//
// Kibana API: POST /api/saved_objects/_bulk_get
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-bulk-get
func (s *SavedObjectsService) BulkGet(ctx context.Context, reqs []BulkGetSavedObjectsRequest, opts ...RequestOption) ([]*SavedObject, *Response, error) {
	if len(reqs) == 0 {
		return nil, nil, errors.New("kibana: objects must not be empty")
	}
	for i, r := range reqs {
		if r.Type == "" || r.ID == "" {
			return nil, nil, fmt.Errorf("kibana: object %d must have a type and an ID", i)
		}
	}
	return s.bulkObjects(ctx, http.MethodPost, "/api/saved_objects/_bulk_get", reqs, opts)
}

// BulkUpdate updates saved objects of the client's space, like
// [SavedObjectsService.Update] does for one object. Every object needs an
// ID.
//
// The result has an object for each of objs, in the same order, with the
// fields Kibana changed. Objects that could not be updated, such as
// missing objects, have [SavedObject.Error] set; the call itself still
// succeeds.
//
// Kibana 8.19 and 9.5 only accept PUT, although the Kibana 8.19 API
// specification documents POST.
//
// Deprecated: Kibana deprecated the saved objects bulk API in 8.7. Use
// [SavedObjectsService.Import].
//
// Kibana API: PUT /api/saved_objects/_bulk_update
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-put-saved-objects-bulk-update
func (s *SavedObjectsService) BulkUpdate(ctx context.Context, objs []*SavedObject, opts ...RequestOption) ([]*SavedObject, *Response, error) {
	body, err := bulkWrites(objs, true)
	if err != nil {
		return nil, nil, err
	}
	return s.bulkObjects(ctx, http.MethodPut, "/api/saved_objects/_bulk_update", body, opts)
}

// bulkWrites validates objs and returns their writable fields. Updates need
// IDs, and send the versions instead of the migration versions.
func bulkWrites(objs []*SavedObject, update bool) ([]*savedObjectWrite, error) {
	if len(objs) == 0 {
		return nil, errors.New("kibana: objects must not be empty")
	}
	body := make([]*savedObjectWrite, len(objs))
	for i, obj := range objs {
		w, err := newSavedObjectWrite(obj)
		if err != nil {
			return nil, fmt.Errorf("kibana: object %d: %s", i, strings.TrimPrefix(err.Error(), "kibana: "))
		}
		if update && obj.ID == "" {
			return nil, fmt.Errorf("kibana: object %d must have an ID", i)
		}
		w.ID, w.Type = obj.ID, obj.Type
		if update {
			w.CoreMigrationVersion, w.TypeMigrationVersion = "", ""
			w.Version = obj.Version
		}
		body[i] = w
	}
	return body, nil
}

// bulkObjects sends a bulk request whose response is {"saved_objects": [...]}.
func (s *SavedObjectsService) bulkObjects(ctx context.Context, method, path string, body any, opts []RequestOption) ([]*SavedObject, *Response, error) {
	var result struct {
		SavedObjects []*SavedObject `json:"saved_objects"`
	}
	resp, err := s.client.do(ctx, method, path, body, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result.SavedObjects, resp, nil
}

// BulkDeleteStatus is the result of deleting one object with
// [SavedObjectsService.BulkDelete].
type BulkDeleteStatus struct {
	Type    string       `json:"type"`
	ID      string       `json:"id"`
	Success bool         `json:"success"`
	Error   *ObjectError `json:"error,omitempty"`
}

// BulkDelete deletes saved objects of the client's space.
//
// The result has a status for each of refs, in the same order. Objects
// that could not be deleted, such as missing objects, have Success false
// and Error set; the call itself still succeeds.
//
// Deprecated: Kibana deprecated the saved objects bulk API in 8.7. Use the
// API of the object type where there is one, such as data views.
//
// Kibana API: POST /api/saved_objects/_bulk_delete
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-bulk-delete
func (s *SavedObjectsService) BulkDelete(ctx context.Context, refs []SavedObjectRef, opt *DeleteSavedObjectOptions, opts ...RequestOption) ([]*BulkDeleteStatus, *Response, error) {
	if err := validateRefs(refs); err != nil {
		return nil, nil, err
	}
	path, err := addQuery("/api/saved_objects/_bulk_delete", opt)
	if err != nil {
		return nil, nil, err
	}
	var result struct {
		Statuses []*BulkDeleteStatus `json:"statuses"`
	}
	resp, err := s.client.do(ctx, http.MethodPost, path, refs, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result.Statuses, resp, nil
}

// ResolveOutcome is how Kibana found a resolved saved object.
type ResolveOutcome string

// Resolve outcomes.
const (
	// ResolveExactMatch: the object has the requested ID.
	ResolveExactMatch ResolveOutcome = "exactMatch"

	// ResolveAliasMatch: the requested ID is a legacy URL alias of the
	// object, which got a new ID when it was converted or imported. Use
	// ResolvedSavedObject.AliasTargetID from now on.
	ResolveAliasMatch ResolveOutcome = "aliasMatch"

	// ResolveConflict: both an object with the requested ID and a legacy
	// URL alias with it exist. The result is the object with the ID.
	ResolveConflict ResolveOutcome = "conflict"
)

// ResolvedSavedObject is a saved object found by its current ID or by a
// legacy URL alias.
type ResolvedSavedObject struct {
	SavedObject *SavedObject   `json:"saved_object"`
	Outcome     ResolveOutcome `json:"outcome"`

	// AliasTargetID is the current ID of the object, for alias matches and
	// conflicts.
	AliasTargetID string `json:"alias_target_id,omitempty"`

	// AliasPurpose is why the alias exists: "savedObjectConversion" or
	// "savedObjectImport".
	AliasPurpose string `json:"alias_purpose,omitempty"`
}

// BulkResolve returns saved objects of the client's space by their ID or
// a legacy URL alias.
//
// The result has an object for each of refs, in the same order. Missing
// objects have SavedObject.Error set; the call itself still succeeds.
//
// Deprecated: Kibana deprecated the saved objects bulk API in 8.7.
//
// Kibana API: POST /api/saved_objects/_bulk_resolve
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-saved-objects-bulk-resolve
func (s *SavedObjectsService) BulkResolve(ctx context.Context, refs []SavedObjectRef, opts ...RequestOption) ([]*ResolvedSavedObject, *Response, error) {
	if err := validateRefs(refs); err != nil {
		return nil, nil, err
	}
	var result struct {
		ResolvedObjects []*ResolvedSavedObject `json:"resolved_objects"`
	}
	resp, err := s.client.do(ctx, http.MethodPost, "/api/saved_objects/_bulk_resolve", refs, &result, opts)
	if err != nil {
		return nil, resp, err
	}
	return result.ResolvedObjects, resp, nil
}
