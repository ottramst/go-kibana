package kibana

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// ShortURLsService handles communication with the short URL methods of the
// Kibana API. Short URLs belong to a space: use [Client.InSpace] to manage
// the short URLs of another space.
//
// The short URL API is in technical preview.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-short-url
type ShortURLsService service

// LegacyShortURLLocator is the ID of the locator that redirects to a Kibana
// URL path, given as the "url" parameter, e.g.
// {"url": "/app/dashboards#/view/<id>"}.
const LegacyShortURLLocator = "LEGACY_SHORT_URL_LOCATOR"

// ShortURL is a Kibana short URL. It is opened at /goto/{Slug}.
type ShortURL struct {
	ID   string
	Slug string

	// Locator is the Kibana location the short URL points to.
	Locator ShortURLLocator

	// AccessCount and AccessDate track the use of the short URL. Kibana
	// updates them when the short URL is resolved, including by
	// [ShortURLsService.Resolve], but not by [ShortURLsService.Get].
	AccessCount int64
	AccessDate  time.Time
	CreateDate  time.Time
}

// ShortURLLocator is a locator and the parameters it resolves to a Kibana
// location.
type ShortURLLocator struct {
	ID string `json:"id"`

	// Version is the Kibana version that created the short URL.
	Version string `json:"version"`

	// State holds the locator parameters.
	State map[string]any `json:"state"`
}

// shortURLJSON is the JSON form of a [ShortURL]: Kibana sends the dates as
// milliseconds since the epoch.
type shortURLJSON struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug"`
	Locator     ShortURLLocator `json:"locator"`
	AccessCount int64           `json:"accessCount"`
	AccessDate  int64           `json:"accessDate"`
	CreateDate  int64           `json:"createDate"`
}

// MarshalJSON encodes a short URL in Kibana's format.
func (u ShortURL) MarshalJSON() ([]byte, error) {
	return json.Marshal(shortURLJSON{
		ID:          u.ID,
		Slug:        u.Slug,
		Locator:     u.Locator,
		AccessCount: u.AccessCount,
		AccessDate:  toEpochMillis(u.AccessDate),
		CreateDate:  toEpochMillis(u.CreateDate),
	})
}

// UnmarshalJSON decodes a short URL in Kibana's format.
func (u *ShortURL) UnmarshalJSON(data []byte) error {
	var raw shortURLJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*u = ShortURL{
		ID:          raw.ID,
		Slug:        raw.Slug,
		Locator:     raw.Locator,
		AccessCount: raw.AccessCount,
		AccessDate:  epochMillis(raw.AccessDate),
		CreateDate:  epochMillis(raw.CreateDate),
	}
	return nil
}

// epochMillis converts milliseconds since the epoch to a UTC time, and 0 to
// the zero time.
func epochMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

// toEpochMillis is the inverse of epochMillis.
func toEpochMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// CreateShortURLRequest is the request of [ShortURLsService.Create].
type CreateShortURLRequest struct {
	// LocatorID is the ID of the locator, such as LegacyShortURLLocator or
	// "DASHBOARD_APP_LOCATOR". Required. Kibana answers 409 Conflict for an
	// unknown locator.
	LocatorID string `json:"locatorId"`

	// Params are the locator parameters. Kibana does not validate them,
	// except that the "url" of LegacyShortURLLocator must be a Kibana path:
	// external URLs are a 400 Bad Request.
	Params map[string]any `json:"params"`

	// Slug is a custom slug of letters, digits, "-", "." or "_". Kibana
	// generates a random one if it is empty, and answers 409 Conflict if it
	// is taken. Create accepts 3 to 255 characters, but Resolve only 4 to
	// 128, so use a slug of that length.
	Slug string `json:"slug,omitempty"`
}

// Create creates a short URL in the client's space.
//
// Kibana API: POST /api/short_url
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-url
func (s *ShortURLsService) Create(ctx context.Context, req *CreateShortURLRequest, opts ...RequestOption) (*ShortURL, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	if req.LocatorID == "" {
		return nil, nil, errors.New("kibana: LocatorID must not be empty")
	}
	body := *req
	if body.Params == nil {
		// Kibana requires params.
		body.Params = map[string]any{}
	}
	u := new(ShortURL)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/short_url", &body, u, opts)
	if err != nil {
		return nil, resp, err
	}
	return u, resp, nil
}

// Get returns a short URL by its ID. Unlike [ShortURLsService.Resolve], it
// does not change the access fields.
//
// Kibana API: GET /api/short_url/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-get-url
func (s *ShortURLsService) Get(ctx context.Context, id string, opts ...RequestOption) (*ShortURL, *Response, error) {
	if err := requireID("short URL ID", id); err != nil {
		return nil, nil, err
	}
	u := new(ShortURL)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/short_url/%s", id), nil, u, opts)
	if err != nil {
		return nil, resp, err
	}
	return u, resp, nil
}

// Resolve returns a short URL by its slug, which must have 4 to 128
// characters.
//
// Kibana counts this as a use of the short URL: it increments AccessCount
// and sets AccessDate in the background, after returning the short URL as it
// was before.
//
// Kibana API: GET /api/short_url/_slug/{slug}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-resolve-url
func (s *ShortURLsService) Resolve(ctx context.Context, slug string, opts ...RequestOption) (*ShortURL, *Response, error) {
	if err := requireID("short URL slug", slug); err != nil {
		return nil, nil, err
	}
	u := new(ShortURL)
	resp, err := s.client.do(ctx, http.MethodGet, pathf("/api/short_url/_slug/%s", slug), nil, u, opts)
	if err != nil {
		return nil, resp, err
	}
	return u, resp, nil
}

// Delete deletes a short URL by its ID.
//
// Kibana API: DELETE /api/short_url/{id}
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-delete-url
func (s *ShortURLsService) Delete(ctx context.Context, id string, opts ...RequestOption) (*Response, error) {
	if err := requireID("short URL ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, pathf("/api/short_url/%s", id), nil, nil, opts)
}
