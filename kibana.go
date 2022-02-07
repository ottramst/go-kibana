package kibana

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
)

const (
	apiPath = "api/"

	userAgent = "go-kibana"
)

// AuthType represents an authentication type within Kibana
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/api.html
type AuthType int

// List of available authentication types
//
// Kibana API docs: https://www.elastic.co/guide/en/kibana/current/api.html
const (
	BasicAuth AuthType = iota
	APIKey
)

// A Client manages communication with the Kibana API
type Client struct {
	// HTTP client used to communicate with the API
	client *http.Client

	// Base URL for API requests, baseURL
	// should always be specified with a trailing slash.
	baseURL *url.URL

	// Token type used to make authenticated API calls.
	authType AuthType

	// Username and password used for basic authentication.
	username, password string

	// API Key used for authentication
	apiKey string

	// User agent used when communicating with the Kibana API
	UserAgent string

	// Services used for talking to different parts of the Kibana API
	Spaces *SpacesService
}

// Response is a Kibana API response. This wraps the standard http.Response
// returned from Kibana
type Response struct {
	*http.Response
}

// newResponse creates a new Response for the provided http.Response.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	return response
}

// BaseURL return a copy of the baseURL.
func (c *Client) BaseURL() *url.URL {
	u := *c.baseURL
	return &u
}

func newClient(options ...ClientOptionFunc) (*Client, error) {
	c := &Client{UserAgent: userAgent}

	// Configure the HTTP client.
	c.client = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Apply any given client options.
	for _, fn := range options {
		if fn == nil {
			continue
		}
		if err := fn(c); err != nil {
			return nil, err
		}
	}

	// Create all the public services
	c.Spaces = &SpacesService{client: c}

	return c, nil
}

// NewClient returns a new Kibana API client. To use API methods which require
// authentication, provide a valid API key
func NewClient(apiKey string, options ...ClientOptionFunc) (*Client, error) {
	client, err := newClient(options...)
	if err != nil {
		return nil, err
	}
	client.authType = APIKey
	client.apiKey = apiKey
	return client, nil
}

// NewBasicAuthClient returns a new Kibana API client. To use API methods which
// require authentication, provide a valid username and password.
func NewBasicAuthClient(username, password string, options ...ClientOptionFunc) (*Client, error) {
	client, err := newClient(options...)
	if err != nil {
		return nil, err
	}

	client.authType = BasicAuth
	client.username = username
	client.password = password

	return client, nil
}

// setBaseURL sets the base URL for API requests to a custom endpoint.
func (c *Client) setBaseURL(urlStr string) error {
	// Make sure the given URL end with a slash
	if !strings.HasSuffix(urlStr, "/") {
		urlStr += "/"
	}

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(baseURL.Path, apiPath) {
		baseURL.Path += apiPath
	}

	// Update the base URL of the client.
	c.baseURL = baseURL

	return nil
}

// NewRequest creates a new API request. The method expects a relative URL
// path that will be resolved relative to the base URL of the Client.
// Relative URL paths should always be specified without a preceding slash.
// If specified, the value pointed to by body is JSON encoded and included
// as the request body.
func (c *Client) NewRequest(method, path string, opt interface{}, options []RequestOptionFunc) (*http.Request, error) {
	u := *c.baseURL
	unescaped, err := url.PathUnescape(path)
	if err != nil {
		return nil, err
	}

	// Set the encoded path data
	u.RawPath = c.baseURL.Path + path
	u.Path = c.baseURL.Path + unescaped

	// Create a request specific headers map.
	reqHeaders := make(http.Header)
	reqHeaders.Set("Accept", "application/json")

	if c.UserAgent != "" {
		reqHeaders.Set("User-Agent", c.UserAgent)
	}

	var body []byte
	switch {
	case method == http.MethodPost || method == http.MethodPut:
		reqHeaders.Set("kbn-xsrf", "true")
		reqHeaders.Set("Content-Type", "application/json")

		if opt != nil {
			body, err = json.Marshal(opt)
			if err != nil {
				return nil, err
			}
		}
	case method == http.MethodDelete:
		reqHeaders.Set("kbn-xsrf", "true")
	case opt != nil:
		q, err := query.Values(opt)
		if err != nil {
			return nil, err
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequest(method, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	for _, fn := range options {
		if fn == nil {
			continue
		}
		if err := fn(req); err != nil {
			return nil, err
		}
	}

	// Set the request specific headers.
	for k, v := range reqHeaders {
		req.Header[k] = v
	}

	return req, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred. If v implements the io.Writer
// interface, the raw response body will be written to v, without attempting to
// first decode it.
func (c Client) Do(req *http.Request, v interface{}) (*Response, error) {
	// Set the correct authentication header. If using basic auth, then check
	// if we already have a token and if not first authenticate and get one.
	switch c.authType {
	case BasicAuth:
		auth := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
		if values := req.Header.Values("Authorization"); len(values) == 0 {
			req.Header.Set("Authorization", "Basic "+auth)
		}
	case APIKey:
		if values := req.Header.Values("Authorization"); len(values) == 0 {
			req.Header.Set("Authorization", "ApiKey "+c.apiKey)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := newResponse(resp)

	err = CheckResponse(resp)
	if err != nil {
		return response, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			_, err = io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
		}
	}

	return response, err
}

// An ErrorResponse reports one or more errors caused by an API request
type ErrorResponse struct {
	Body     []byte
	Response *http.Response
	Message  string
}

func (e *ErrorResponse) Error() string {
	path, _ := url.QueryUnescape(e.Response.Request.URL.Path)
	u := fmt.Sprintf("%s://%s%s", e.Response.Request.URL.Scheme, e.Response.Request.URL.Host, path)
	return fmt.Sprintf("%s %s: %d %s", e.Response.Request.Method, u, e.Response.StatusCode, e.Message)
}

// CheckResponse checks the API response for errors, and returns them if present
func CheckResponse(r *http.Response) error {
	switch r.StatusCode {
	case 200, 201, 202, 204, 304:
		return nil
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	if err == nil && data != nil {
		errorResponse.Body = data

		var raw interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			errorResponse.Message = "failed to parse unknown error format"
		} else {
			errorResponse.Message = parseError(raw)
		}
	}

	return errorResponse
}

// Format:
// {
//	 "statusCode": 401,
//   "error": "Unauthorized",
//   "message": "[security_exception: [security_exception] Reason: unable to authenticate user [elastic] for REST request [/_security/_authenticate]]: unable to authenticate user [elastic] for REST request [/_security/_authenticate]"
// }

func parseError(raw interface{}) string {
	switch raw := raw.(type) {
	case string:
		return raw

	case []interface{}:
		var errs []string
		for _, v := range raw {
			errs = append(errs, parseError(v))
		}
		return fmt.Sprintf("[%s]", strings.Join(errs, ", "))

	case map[string]interface{}:
		var errs []string
		for k, v := range raw {
			errs = append(errs, fmt.Sprintf("{%s: %s}", k, parseError(v)))
		}
		sort.Strings(errs)
		return strings.Join(errs, ", ")

	default:
		return fmt.Sprintf("failed to parse unexpected error type: %T", raw)
	}
}
