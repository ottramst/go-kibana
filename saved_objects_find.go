package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strings"
)

// SearchOperator combines search terms or references.
type SearchOperator string

// Search operators.
const (
	SearchOperatorOr  SearchOperator = "OR"
	SearchOperatorAnd SearchOperator = "AND"
)

// FindSavedObjectsOptions are the options of [SavedObjectsService.Find] and
// [SavedObjectsService.FindAll].
type FindSavedObjectsOptions struct {
	_ [0]func()

	// Types lists the types of the objects to find. Required.
	Types []string `url:"type"`

	// Search is an Elasticsearch simple_query_string query, such as
	// "my-dash*", on SearchFields (the type's default search fields if
	// empty).
	Search                string         `url:"search,omitempty"`
	SearchFields          []string       `url:"search_fields,omitempty"`
	DefaultSearchOperator SearchOperator `url:"default_search_operator,omitempty"`

	// Filter is a KQL filter on the object fields, such as
	// "dashboard.attributes.title:foo" or "dashboard.updated_at > 2026".
	Filter string `url:"filter,omitempty"`

	// HasReference limits the results to objects that reference these
	// objects: any of them, or all with SearchOperatorAnd.
	HasReference         []SavedObjectRef `url:"-"`
	HasReferenceOperator SearchOperator   `url:"has_reference_operator,omitempty"`

	// HasNoReference limits the results to objects that do not reference
	// these objects.
	HasNoReference         []SavedObjectRef `url:"-"`
	HasNoReferenceOperator SearchOperator   `url:"has_no_reference_operator,omitempty"`

	// Fields limits the returned attributes to these fields.
	Fields []string `url:"fields,omitempty"`

	// SortField sorts the results by this field, such as "updated_at" or
	// "title".
	SortField string `url:"sort_field,omitempty"`

	// Page is the page to return, starting at 1 (the default).
	Page int `url:"page,omitempty"`

	// PerPage is the number of objects per page. Kibana's default is 20,
	// which 0 also selects; FindAll's is 100. Set it to 1 for queries that
	// only need Total or Aggregations.
	PerPage int `url:"per_page,omitempty"`

	// Aggs are Elasticsearch aggregations on the object fields, as JSON,
	// such as {"by_x": {"terms": {"field": "<type>.attributes.<field>"}}}.
	// Aggregations on text fields, such as titles, fail with
	// [ErrBadRequest]: use keyword fields. FindAll does not send them.
	Aggs string `url:"aggs,omitempty"`

	// Namespaces searches these spaces instead of the client's space.
	Namespaces []string `url:"namespaces,omitempty"`
}

// FindSavedObjectsResult is a page of saved objects.
type FindSavedObjectsResult struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`

	// Total is the number of objects that match.
	Total        int            `json:"total"`
	SavedObjects []*SavedObject `json:"saved_objects"`

	// Aggregations holds the results of FindSavedObjectsOptions.Aggs.
	Aggregations json.RawMessage `json:"aggregations,omitempty"`
}

// findPath returns the path and query of a find request.
func findPath(opt *FindSavedObjectsOptions) (string, error) {
	if opt == nil || len(opt.Types) == 0 {
		return "", errors.New("kibana: Types must not be empty")
	}
	path, err := addQuery("/api/saved_objects/_find", opt)
	if err != nil {
		return "", err
	}
	// Kibana takes references as JSON in the query string.
	for _, p := range []struct {
		key  string
		refs []SavedObjectRef
	}{{"has_reference", opt.HasReference}, {"has_no_reference", opt.HasNoReference}} {
		if len(p.refs) == 0 {
			continue
		}
		if err := validateRefs(p.refs); err != nil {
			return "", fmt.Errorf("kibana: %s: %s", p.key, strings.TrimPrefix(err.Error(), "kibana: "))
		}
		data, err := json.Marshal(p.refs)
		if err != nil {
			return "", fmt.Errorf("kibana: encoding %s: %w", p.key, err)
		}
		path += "&" + p.key + "=" + url.QueryEscape(string(data))
	}
	return path, nil
}

// Find returns a page of the saved objects of the client's space that match
// opt. Use [SavedObjectsService.FindAll] to iterate over all of them.
//
// Kibana answers [ErrBadRequest] when Page × PerPage exceeds 10,000.
//
// Deprecated: Kibana deprecated the saved objects find API in 8.7. Use the
// API of the object type where there is one, such as data views or tags,
// or [SavedObjectsService.Export].
//
// Kibana API: GET /api/saved_objects/_find
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-saved-objects-find
func (s *SavedObjectsService) Find(ctx context.Context, opt *FindSavedObjectsOptions, opts ...RequestOption) (*FindSavedObjectsResult, *Response, error) {
	path, err := findPath(opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(FindSavedObjectsResult)
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, result, nil, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// FindAll returns an iterator over the saved objects of the client's space
// that match opt, starting at opt.Page. It fetches opt.PerPage objects per
// request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops. Kibana only returns the first 10,000 matches:
// the iterator fails with [ErrBadRequest] at the first page that ends
// beyond them (use a PerPage that divides 10,000 to get all of them), so
// narrow the query, or use [SavedObjectsService.Export]. Objects created or deleted while
// iterating can be skipped or returned twice; set opt.SortField for a
// stable order.
//
// Deprecated: Kibana deprecated the saved objects find API in 8.7. Use the
// API of the object type where there is one, such as data views or tags,
// or [SavedObjectsService.Export].
func (s *SavedObjectsService) FindAll(ctx context.Context, opt *FindSavedObjectsOptions, opts ...RequestOption) iter.Seq2[*SavedObject, error] {
	base := FindSavedObjectsOptions{}
	if opt != nil {
		base = *opt
	}
	// The iterator has no use for aggregations.
	base.Aggs = ""
	return paginatePages(ctx, base.Page, base.PerPage, func(ctx context.Context, page, perPage int) ([]*SavedObject, int, error) {
		req := base
		req.Page, req.PerPage = page, perPage
		result, _, err := s.Find(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.SavedObjects, result.Total, nil
	})
}

// ResolveSavedObjectOptions are the options of [SavedObjectsService.Resolve].
type ResolveSavedObjectOptions struct {
	_ [0]func()
}

// Resolve returns a saved object of the client's space by its ID, or by a
// legacy URL alias of it: objects that got a new ID when Kibana 8.0 made
// object IDs unique across spaces, or when they were imported, can still be
// found by their old ID.
//
// Deprecated: Kibana deprecated the saved objects resolve API in 8.7.
//
// Kibana API: GET /api/saved_objects/resolve/{type}/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-saved-objects-resolve-type-id
func (s *SavedObjectsService) Resolve(ctx context.Context, objType, id string, opt *ResolveSavedObjectOptions, opts ...RequestOption) (*ResolvedSavedObject, *Response, error) {
	if err := requireID("saved object type", objType); err != nil {
		return nil, nil, err
	}
	if err := requireID("saved object ID", id); err != nil {
		return nil, nil, err
	}
	result := new(ResolvedSavedObject)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/saved_objects/resolve/%s/%s", objType, id), nil, result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
