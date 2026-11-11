package kibana

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-querystring/query"
)

// A RequestOption customizes a single API request. Options are applied after
// the client has set its headers, so they can override them.
type RequestOption func(*http.Request) error

// WithHeader sets a request header, replacing any value the client set.
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) error {
		req.Header.Set(key, value)
		return nil
	}
}

// WithQueryParam adds a query parameter to the request URL.
func WithQueryParam(key, value string) RequestOption {
	return func(req *http.Request) error {
		q := req.URL.Query()
		q.Add(key, value)
		req.URL.RawQuery = q.Encode()
		return nil
	}
}

// pathf formats an API path, escaping each segment for use in a URL path.
func pathf(format string, segments ...string) string {
	args := make([]any, len(segments))
	for i, s := range segments {
		args[i] = url.PathEscape(s)
	}
	return fmt.Sprintf(format, args...)
}

// addQuery appends the query parameters encoded from the url struct tags of
// opt to path. A nil opt leaves path unchanged.
func addQuery(path string, opt any) (string, error) {
	if opt == nil {
		return path, nil
	}
	v, err := query.Values(opt)
	if err != nil {
		return "", fmt.Errorf("kibana: encoding query parameters: %w", err)
	}
	if len(v) == 0 {
		return path, nil
	}
	return path + "?" + v.Encode(), nil
}

// requireID returns an error if a required path parameter is empty, so that
// no request is sent to a different endpoint than intended.
func requireID(name, value string) error {
	if value == "" {
		return fmt.Errorf("kibana: %s must not be empty", name)
	}
	return nil
}
