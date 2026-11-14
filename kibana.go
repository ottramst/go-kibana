package kibana

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

const (
	modulePath       = "github.com/ottramst/go-kibana"
	defaultUserAgent = "go-kibana"

	// DefaultSpace is the ID of the space Kibana creates on startup. Requests
	// in the default space are sent without a /s/{space} path prefix.
	DefaultSpace = "default"
)

// A Client manages communication with the Kibana API. It is safe for
// concurrent use by multiple goroutines.
type Client struct {
	httpClient *http.Client

	// baseURL is the Kibana URL, including any server.basePath, without a
	// trailing slash.
	baseURL *url.URL

	// space is the space requests are scoped to; empty means the default space.
	space string

	// authorization is the value of the Authorization header, if any.
	authorization string

	userAgent string

	// retry is the policy of WithRetry, if any, and retrySleep the function
	// its transport waits with (replaced in tests).
	retry      *RetryPolicy
	retrySleep func(ctx context.Context, d time.Duration) error

	// Reuse a single struct instead of allocating one for each service.
	common service

	// Services used for talking to different parts of the Kibana API.
	DataViews             *DataViewsService
	EncryptedSavedObjects *EncryptedSavedObjectsService
	Features              *FeaturesService
	Roles                 *RolesService
	SavedObjects          *SavedObjectsService
	ShortURLs             *ShortURLsService
	Spaces                *SpacesService
	Status                *StatusService
	Tags                  *TagsService
	TaskManager           *TaskManagerService
	UserSessions          *UserSessionsService
}

type service struct {
	client *Client
}

// A ClientOption configures a [Client] created with [NewClient].
type ClientOption func(*Client) error

// WithAPIKey authenticates requests with an Elasticsearch API key. The key
// is the base64-encoded "id:api_key" value (the "encoded" field returned when
// the key is created).
func WithAPIKey(encoded string) ClientOption {
	return func(c *Client) error {
		if encoded == "" {
			return errors.New("kibana: API key must not be empty")
		}
		c.authorization = "ApiKey " + encoded
		return nil
	}
}

// WithBasicAuth authenticates requests with a username and password.
func WithBasicAuth(username, password string) ClientOption {
	return func(c *Client) error {
		if username == "" {
			return errors.New("kibana: username must not be empty")
		}
		c.authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		return nil
	}
}

// WithHTTPClient sets the HTTP client used for requests. Use it to configure
// timeouts, TLS or proxies. The default client has no timeout; pass a
// context with a deadline to bound individual calls. To retry failed
// requests, use [WithRetry] rather than a retrying transport.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) error {
		if httpClient == nil {
			return errors.New("kibana: HTTP client must not be nil")
		}
		c.httpClient = httpClient
		return nil
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) error {
		c.userAgent = userAgent
		return nil
	}
}

// WithSpace scopes the client to a space: space-aware endpoints act on that
// space instead of the default space. See also [Client.InSpace].
func WithSpace(spaceID string) ClientOption {
	return func(c *Client) error {
		c.space = spaceID
		return nil
	}
}

// NewClient returns a new Kibana API client for the Kibana instance at
// baseURL, for example "https://kibana.example.com:5601". If Kibana is served
// under a base path (server.basePath), include it in the URL.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("kibana: invalid base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("kibana: invalid base URL %q: must be an absolute http or https URL", baseURL)
	}
	// Keep escaped characters of the base path, such as %2F, as they are.
	escaped := strings.TrimSuffix(u.EscapedPath(), "/")
	unescaped, err := url.PathUnescape(escaped)
	if err != nil {
		return nil, fmt.Errorf("kibana: invalid base URL: %w", err)
	}
	u.Path = unescaped
	u.RawPath = escaped
	u.RawQuery = ""
	u.Fragment = ""

	c := &Client{
		httpClient: &http.Client{},
		baseURL:    u,
		userAgent:  userAgent(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.retry != nil {
		// Wrap a copy, so that the caller's client is not changed.
		hc := *c.httpClient
		base := hc.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		sleep := c.retrySleep
		if sleep == nil {
			sleep = sleepContext
		}
		hc.Transport = &retryTransport{base: base, policy: c.retry, sleep: sleep}
		c.httpClient = &hc
	}
	c.initServices()
	return c, nil
}

func (c *Client) initServices() {
	c.common.client = c
	c.DataViews = (*DataViewsService)(&c.common)
	c.EncryptedSavedObjects = (*EncryptedSavedObjectsService)(&c.common)
	c.Features = (*FeaturesService)(&c.common)
	c.Roles = (*RolesService)(&c.common)
	c.SavedObjects = (*SavedObjectsService)(&c.common)
	c.ShortURLs = (*ShortURLsService)(&c.common)
	c.Spaces = (*SpacesService)(&c.common)
	c.Status = (*StatusService)(&c.common)
	c.Tags = (*TagsService)(&c.common)
	c.TaskManager = (*TaskManagerService)(&c.common)
	c.UserSessions = (*UserSessionsService)(&c.common)
}

// InSpace returns a copy of the client that is scoped to the given space.
// The copy shares the underlying HTTP client. Pass [DefaultSpace] to get a
// client for the default space.
func (c *Client) InSpace(spaceID string) *Client {
	clone := *c
	clone.space = spaceID
	clone.initServices()
	return &clone
}

// Space returns the ID of the space the client is scoped to.
func (c *Client) Space() string {
	if c.space == "" {
		return DefaultSpace
	}
	return c.space
}

// BaseURL returns a copy of the Kibana URL the client sends requests to.
func (c *Client) BaseURL() *url.URL {
	u := *c.baseURL
	return &u
}

// userAgent returns "go-kibana/<version>" when the module version is known
// from the build information, and "go-kibana" otherwise.
func userAgent() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return defaultUserAgent
	}
	for _, dep := range bi.Deps {
		if dep.Path == modulePath && dep.Version != "" && dep.Version != "(devel)" {
			return defaultUserAgent + "/" + dep.Version
		}
	}
	return defaultUserAgent
}

// NewRequest creates an API request. path is the API path as it appears in
// the Kibana API reference, such as "/api/spaces/space", and may include a
// query string. Path segments taken from user input must already be escaped.
// The request is scoped to the client's space.
//
// If body is an [io.Reader], it is sent as is; otherwise a non-nil body is
// JSON encoded. Request options are applied last, so they can override any
// header the client sets.
//
// NewRequest and [Client.Do] can be used to call endpoints the client has no
// method for yet.
func (c *Client) NewRequest(ctx context.Context, method, path string, body any, opts ...RequestOption) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("kibana: path %q must start with /", path)
	}
	rawPath, rawQuery, _ := strings.Cut(path, "?")
	if strings.Contains(path, "#") || !validEscapedPath(rawPath) || !validEscapedQuery(rawQuery) {
		return nil, fmt.Errorf("kibana: path %q must be escaped", path)
	}

	prefix := ""
	if c.space != "" && c.space != DefaultSpace {
		prefix = "/s/" + url.PathEscape(c.space)
	}

	u := *c.baseURL
	u.RawPath = u.EscapedPath() + prefix + rawPath
	unescaped, err := url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, fmt.Errorf("kibana: invalid path %q: %w", path, err)
	}
	u.Path = unescaped
	u.RawQuery = rawQuery

	var (
		bodyReader  io.Reader
		contentType string
	)
	switch b := body.(type) {
	case nil:
	case io.Reader:
		bodyReader = b
	default:
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("kibana: encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	// Kibana rejects requests that change state without this header; it is
	// harmless on reads, so it is always sent.
	req.Header.Set("kbn-xsrf", "true")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.authorization != "" {
		req.Header.Set("Authorization", c.authorization)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(req); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// Response describes the HTTP response of an API call. The body has already
// been read: it is decoded into the method's result, or reported in an
// [*ErrorResponse].
type Response struct {
	// StatusCode is the HTTP status code, such as 200.
	StatusCode int

	// Status is the HTTP status line, such as "200 OK".
	Status string

	// Header holds the response headers.
	Header http.Header
}

// Do sends an API request and returns the API response.
//
// If the response status is not 2xx, Do returns an [*ErrorResponse]. Otherwise
// the response body is handled according to v: a nil v discards it, an
// [io.Writer] receives the raw body, and any other value is decoded from JSON.
// An empty body, such as a 204 No Content response, leaves v unchanged.
func (c *Client) Do(req *http.Request, v any) (*Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Drain the body so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	response := &Response{StatusCode: resp.StatusCode, Status: resp.Status, Header: resp.Header}
	if err := checkResponse(resp); err != nil {
		return response, err
	}

	switch v := v.(type) {
	case nil:
	case io.Writer:
		if _, err := io.Copy(v, resp.Body); err != nil {
			return response, fmt.Errorf("kibana: reading response body: %w", err)
		}
	default:
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(v); err != nil {
			if errors.Is(err, io.EOF) {
				return response, nil
			}
			return response, fmt.Errorf("kibana: decoding response body: %w", err)
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return response, errors.New("kibana: decoding response body: unexpected data after the JSON value")
		}
	}
	return response, nil
}

// do creates and sends a request in one step, for service methods. query is
// the method's options; their url tags are added to the query string.
func (c *Client) do(ctx context.Context, method, path string, body, v, query any, opts []RequestOption) (*Response, error) {
	path, err := addQuery(path, query)
	if err != nil {
		return nil, err
	}
	req, err := c.NewRequest(ctx, method, path, body, opts...)
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}
