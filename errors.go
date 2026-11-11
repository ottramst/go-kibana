package kibana

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
)

// maxErrorBodySize limits how much of an error response body is read.
const maxErrorBodySize = 1 << 20

// ErrorResponse is returned for API responses with a non-2xx status code.
// Kibana error bodies look like:
//
//	{"statusCode": 404, "error": "Not Found", "message": "Saved object [space/foo] not found"}
type ErrorResponse struct {
	// Response is the HTTP response. Its body has already been read into Body.
	Response *http.Response `json:"-"`

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

func (e *ErrorResponse) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.StatusText
	}
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.Response != nil && e.Response.Request != nil {
		req := e.Response.Request
		u := *req.URL
		u.User = nil
		return fmt.Sprintf("kibana: %s %s: %d %s", req.Method, u.Redacted(), e.StatusCode, msg)
	}
	return fmt.Sprintf("kibana: %d %s", e.StatusCode, msg)
}

// Is reports whether target is the sentinel error for the response's status
// code, such as [ErrNotFound] for 404 Not Found.
func (e *ErrorResponse) Is(target error) bool {
	switch target {
	case ErrBadRequest:
		return e.StatusCode == http.StatusBadRequest
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrConflict:
		return e.StatusCode == http.StatusConflict
	}
	return false
}

// CheckResponse returns an [*ErrorResponse] if the response has a non-2xx
// status code, and nil otherwise. It reads the body of error responses.
func CheckResponse(r *http.Response) error {
	if r.StatusCode >= 200 && r.StatusCode <= 299 {
		return nil
	}

	errResp := &ErrorResponse{Response: r}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySize))
	if err == nil && len(data) > 0 {
		errResp.Body = data
		if json.Unmarshal(data, errResp) != nil {
			// Not a JSON error body, e.g. a proxy error page.
			errResp.Message = strings.TrimSpace(string(data))
		}
	}
	// The body's statusCode is informational; the HTTP status is authoritative.
	errResp.StatusCode = r.StatusCode
	return errResp
}
