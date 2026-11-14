package kibana

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Sentinel errors for common API failures. An [*ErrorResponse] matches the
// sentinel for its status code with [errors.Is]:
//
//	if errors.Is(err, kibana.ErrNotFound) {
//		// the space does not exist
//	}
var (
	ErrBadRequest   = errors.New("kibana: bad request")
	ErrUnauthorized = errors.New("kibana: unauthorized")
	ErrForbidden    = errors.New("kibana: forbidden")
	ErrNotFound     = errors.New("kibana: not found")
	ErrConflict     = errors.New("kibana: conflict")

	// ErrTooManyRequests is returned for 429 Too Many Requests, e.g. while
	// an encryption key rotation is already running.
	ErrTooManyRequests = errors.New("kibana: too many requests")

	// ErrServiceUnavailable is returned for 503 Service Unavailable, e.g.
	// while Kibana starts, or when a core service is unavailable. In the
	// second case, [StatusFromError] returns the status Kibana sends with
	// it.
	ErrServiceUnavailable = errors.New("kibana: service unavailable")
)

const (
	// maxErrorBodySize limits how much of an error response body is read.
	maxErrorBodySize = 1 << 20

	// maxTextMessageLength limits the message taken from a body that is not
	// a Kibana error, such as the HTML error page of a proxy.
	maxTextMessageLength = 512
)

// ErrorResponse is returned for API responses with a non-2xx status code.
// Kibana error bodies look like:
//
//	{"statusCode": 404, "error": "Not Found", "message": "Saved object [space/foo] not found"}
type ErrorResponse struct {
	// Method and URL identify the request. The URL has no credentials.
	Method string `json:"-"`
	URL    string `json:"-"`

	// Header holds the response headers.
	Header http.Header `json:"-"`

	// StatusCode is the HTTP status code of the response.
	StatusCode int `json:"statusCode"`

	// StatusText is the "error" field of the body, usually the HTTP status text.
	StatusText string `json:"error"`

	// Message describes the error.
	Message string `json:"message"`

	// Attributes holds additional error details some endpoints return.
	Attributes json.RawMessage `json:"attributes,omitempty"`

	// Body is the raw response body (up to 1 MiB).
	Body []byte `json:"-"`
}

// Error returns a description of the error, with the request method and URL.
// Its format is not part of the API and may change.
func (e *ErrorResponse) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.StatusText
	}
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.Method != "" && e.URL != "" {
		return fmt.Sprintf("kibana: %s %s: %d %s", e.Method, e.URL, e.StatusCode, msg)
	}
	return fmt.Sprintf("kibana: %d %s", e.StatusCode, msg)
}

// Is reports whether target is the sentinel error for the response's status
// code, such as [ErrNotFound] for 404 Not Found.
func (e *ErrorResponse) Is(target error) bool {
	return e != nil && statusIs(e.StatusCode, target)
}

// statusIs reports whether target is the sentinel error for the HTTP status
// code. It compares the target by identity, like errors.Is does, so that
// targets of any type, including ones that cannot be hashed, are safe.
//
//nolint:errorlint // Is methods compare targets by identity.
func statusIs(code int, target error) bool {
	switch target {
	case ErrBadRequest:
		return code == http.StatusBadRequest
	case ErrUnauthorized:
		return code == http.StatusUnauthorized
	case ErrForbidden:
		return code == http.StatusForbidden
	case ErrNotFound:
		return code == http.StatusNotFound
	case ErrConflict:
		return code == http.StatusConflict
	case ErrTooManyRequests:
		return code == http.StatusTooManyRequests
	case ErrServiceUnavailable:
		return code == http.StatusServiceUnavailable
	}
	return false
}

// checkResponse returns an [*ErrorResponse] if the response has a non-2xx
// status code, and nil otherwise. It reads the body of error responses.
func checkResponse(r *http.Response) error {
	if r.StatusCode >= 200 && r.StatusCode <= 299 {
		return nil
	}

	errResp := &ErrorResponse{Header: r.Header}
	if req := r.Request; req != nil && req.URL != nil {
		u := *req.URL
		u.User = nil
		errResp.Method, errResp.URL = req.Method, u.Redacted()
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySize))
	if err == nil && len(data) > 0 {
		errResp.Body = data
		parseErrorBody(errResp, data)
	}
	// The body's statusCode is informational; the HTTP status is authoritative.
	errResp.StatusCode = r.StatusCode
	return errResp
}

// parseErrorBody sets the fields of e from a Kibana error body. Fields of an
// unexpected type are skipped. A body that is not a JSON object, such as the
// error page of a proxy, becomes the message, shortened.
func parseErrorBody(e *ErrorResponse, data []byte) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		e.Message = truncate(strings.TrimSpace(string(data)), maxTextMessageLength)
		return
	}
	if v, ok := fields["message"]; ok {
		_ = json.Unmarshal(v, &e.Message)
	}
	if v, ok := fields["error"]; ok {
		_ = json.Unmarshal(v, &e.StatusText)
	}
	if v, ok := fields["attributes"]; ok {
		e.Attributes = v
	}
}

// truncate shortens s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
