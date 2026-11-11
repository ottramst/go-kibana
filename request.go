package kibana

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// WithQueryParam adds a query parameter to the request URL. Existing query
// parameters are kept as they are.
func WithQueryParam(key, value string) RequestOption {
	return func(req *http.Request) error {
		param := url.QueryEscape(key) + "=" + url.QueryEscape(value)
		if req.URL.RawQuery == "" {
			req.URL.RawQuery = param
		} else {
			req.URL.RawQuery += "&" + param
		}
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

// requireID returns an error if a required path parameter is empty, "." or
// "..", so that no request is sent to a different endpoint than intended.
func requireID(name, value string) error {
	switch value {
	case "":
		return fmt.Errorf("kibana: %s must not be empty", name)
	case ".", "..":
		return fmt.Errorf("kibana: invalid %s %q", name, value)
	}
	return nil
}

// validEscapedQuery reports whether q contains no characters that must be
// escaped in a URL query: spaces, control characters and non-ASCII.
func validEscapedQuery(q string) bool {
	for i := 0; i < len(q); i++ {
		if q[i] <= ' ' || q[i] >= 0x7f {
			return false
		}
	}
	return true
}

// validEscapedPath reports whether p contains only characters that may
// appear in an escaped URL path, so that escaped segments such as %2F are
// kept as they are.
func validEscapedPath(p string) bool {
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case strings.IndexByte("-._~!$&'()*+,;=:@/%", c) >= 0:
		default:
			return false
		}
	}
	return true
}
