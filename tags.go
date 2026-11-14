package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net/http"
	"time"
)

// TagsService handles communication with the tag methods of the Kibana API.
// Tags belong to a space: use [Client.InSpace] to manage the tags of another
// space.
//
// The tags API is experimental and was added in Kibana 9.5. Kibana 8.19
// answers every tag request with [ErrNotFound].
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-tags
type TagsService service

// Tag is a saved object tag. The same type is used for reads and writes: ID
// and Meta are ignored when writing.
type Tag struct {
	ID          string
	Name        string
	Description string

	// Color is a hex color such as "#FF0000". Kibana picks a random color
	// when it is empty, also when updating a tag.
	Color string

	// Meta is set by Kibana.
	Meta TagMeta
}

// TagMeta holds the metadata of a [Tag].
type TagMeta struct {
	// CreatedAt is zero in the result of [TagsService.Update] when the tag
	// already existed.
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`

	// CreatedBy and UpdatedBy are user profile IDs, set only when the user
	// who wrote the tag has a user profile.
	CreatedBy string `json:"created_by,omitempty"`
	UpdatedBy string `json:"updated_by,omitempty"`

	// Owner identifies the plugin or team that owns the tag, if any.
	Owner string `json:"owner,omitempty"`

	// Managed tags are managed by Kibana or an integration.
	Managed bool `json:"managed"`

	// Version is the version of the saved object.
	Version string `json:"version,omitempty"`
}

// tagJSON is the JSON form of a [Tag].
type tagJSON struct {
	ID   string     `json:"id"`
	Data tagPayload `json:"data"`
	Meta TagMeta    `json:"meta"`
}

// tagPayload is the body of tag create and update requests.
type tagPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// Color is left out when empty: Kibana rejects "".
	Color string `json:"color,omitempty"`
}

// MarshalJSON encodes a tag in Kibana's format.
func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(tagJSON{ID: t.ID, Data: t.payload(), Meta: t.Meta})
}

// UnmarshalJSON decodes a tag in Kibana's format.
func (t *Tag) UnmarshalJSON(data []byte) error {
	var raw tagJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = Tag{
		ID:          raw.ID,
		Name:        raw.Data.Name,
		Description: raw.Data.Description,
		Color:       raw.Data.Color,
		Meta:        raw.Meta,
	}
	return nil
}

// payload returns the writable fields of the tag.
func (t *Tag) payload() tagPayload {
	return tagPayload{Name: t.Name, Description: t.Description, Color: t.Color}
}

// newTagPayload validates a tag and returns its writable fields.
func newTagPayload(tag *Tag) (*tagPayload, error) {
	if tag == nil {
		return nil, errors.New("kibana: tag must not be nil")
	}
	if tag.Name == "" {
		return nil, errors.New("kibana: tag Name must not be empty")
	}
	p := tag.payload()
	return &p, nil
}

// ListTagsOptions are the options of [TagsService.List] and
// [TagsService.ListAll].
type ListTagsOptions struct {
	// Query filters the tags by name and description, in Elasticsearch
	// simple_query_string syntax.
	Query string `url:"query,omitempty"`

	// Page is the page to return, starting at 1 (the default).
	Page int `url:"page,omitempty"`

	// PerPage is the number of tags per page, at most 1000. Kibana's
	// default is 20; ListAll's is 100.
	PerPage int `url:"per_page,omitempty"`
}

// TagList is a page of tags.
type TagList struct {
	Tags    []*Tag
	Page    int
	PerPage int

	// Total is the number of tags that match the query.
	Total int
}

// List returns a page of the tags of the client's space.
//
// Kibana API: GET /api/tags
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-tags
func (s *TagsService) List(ctx context.Context, opt *ListTagsOptions, opts ...RequestOption) (*TagList, *Response, error) {
	path, err := addQuery("/api/tags", opt)
	if err != nil {
		return nil, nil, err
	}
	var page struct {
		Data []*Tag `json:"data"`
		Meta struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"meta"`
	}
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &TagList{Tags: page.Data, Page: page.Meta.Page, PerPage: page.Meta.PerPage, Total: page.Meta.Total}, resp, nil
}

// ListAll returns an iterator over the tags of the client's space, starting
// at opt.Page. It fetches opt.PerPage tags per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops. Tags created or deleted while iterating can be
// skipped or returned twice; a page shorter than expected ends the
// iteration.
func (s *TagsService) ListAll(ctx context.Context, opt *ListTagsOptions, opts ...RequestOption) iter.Seq2[*Tag, error] {
	base := ListTagsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginatePages(ctx, base.Page, base.PerPage, func(ctx context.Context, page, perPage int) ([]*Tag, int, error) {
		req := base
		req.Page, req.PerPage = page, perPage
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.Tags, result.Total, nil
	})
}

// Get returns a tag by its ID.
//
// Kibana API: GET /api/tags/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-tags-id
func (s *TagsService) Get(ctx context.Context, id string, opts ...RequestOption) (*Tag, *Response, error) {
	if err := requireID("tag ID", id); err != nil {
		return nil, nil, err
	}
	tag := new(Tag)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/tags/%s", id), nil, tag, opts)
	if err != nil {
		return nil, resp, err
	}
	return tag, resp, nil
}

// Create creates a tag with a generated ID. Its ID and Meta are ignored.
// Tag names are unique in a space: Kibana answers [ErrConflict] for a name
// that is taken.
//
// Kibana API: POST /api/tags
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-tags
func (s *TagsService) Create(ctx context.Context, tag *Tag, opts ...RequestOption) (*Tag, *Response, error) {
	payload, err := newTagPayload(tag)
	if err != nil {
		return nil, nil, err
	}
	created := new(Tag)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/tags", payload, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the tag with the given ID, or creates it with that ID if
// it does not exist (the response status is then 201 Created). The tag's ID
// and Meta are ignored.
//
// Every field is replaced: an empty Description clears it, and an empty
// Color gets a new random color. Update a tag returned by
// [TagsService.Get] to keep its other fields.
//
// Kibana API: PUT /api/tags/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-put-tags-id
func (s *TagsService) Update(ctx context.Context, id string, tag *Tag, opts ...RequestOption) (*Tag, *Response, error) {
	if err := requireID("tag ID", id); err != nil {
		return nil, nil, err
	}
	payload, err := newTagPayload(tag)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Tag)
	resp, err := s.client.do(ctx, http.MethodPut, pathf("/api/tags/%s", id), payload, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes a tag by its ID.
//
// Kibana API: DELETE /api/tags/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-tags-id
func (s *TagsService) Delete(ctx context.Context, id string, opts ...RequestOption) (*Response, error) {
	if err := requireID("tag ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/tags/%s", id), nil, nil, opts)
}
