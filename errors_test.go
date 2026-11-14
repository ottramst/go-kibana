package kibana_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ottramst/go-kibana"
)

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		sentinel    error
		wantMessage string
		wantText    string
	}{
		{name: "bad request", status: 400, body: `{"statusCode":400,"error":"Bad Request","message":"[request body.name]: expected value of type [string]"}`,
			sentinel: kibana.ErrBadRequest, wantMessage: "[request body.name]: expected value of type [string]", wantText: "Bad Request"},
		{name: "unauthorized", status: 401, body: `{"statusCode":401,"error":"Unauthorized","message":"Unauthorized"}`,
			sentinel: kibana.ErrUnauthorized, wantMessage: "Unauthorized", wantText: "Unauthorized"},
		{name: "forbidden", status: 403, body: `{"statusCode":403,"error":"Forbidden","message":"forbidden"}`,
			sentinel: kibana.ErrForbidden, wantMessage: "forbidden", wantText: "Forbidden"},
		{name: "not found", status: 404, body: notFound, sentinel: kibana.ErrNotFound, wantMessage: "Not Found", wantText: "Not Found"},
		{name: "conflict", status: 409, body: `{"statusCode":409,"error":"Conflict","message":"A space with the identifier x already exists."}`,
			sentinel: kibana.ErrConflict, wantMessage: "A space with the identifier x already exists.", wantText: "Conflict"},
		{name: "too many requests", status: 429, body: `{"statusCode":429,"error":"Too Many Requests","message":"Encryption key rotation is in progress already."}`,
			sentinel: kibana.ErrTooManyRequests, wantMessage: "Encryption key rotation is in progress already.", wantText: "Too Many Requests"},
		{name: "plain text body", status: 502, body: "Bad Gateway\n", wantMessage: "Bad Gateway"},
		{name: "unexpected field type", status: 400, body: `{"statusCode":400,"error":{"type":"x"},"message":"bad input"}`,
			sentinel: kibana.ErrBadRequest, wantMessage: "bad input"},
		{name: "JSON array body", status: 500, body: `["x"]`, wantMessage: `["x"]`},
		{name: "empty body", status: 503, sentinel: kibana.ErrServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/spaces/space/x", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, []byte(tt.body))
			})

			_, resp, err := client.Spaces.Get(context.Background(), "x", nil)
			var errResp *kibana.ErrorResponse
			if !errors.As(err, &errResp) {
				t.Fatalf("error = %v (%T), want *kibana.ErrorResponse", err, err)
			}
			if resp == nil || resp.StatusCode != tt.status {
				t.Errorf("Response = %v, want status %d", resp, tt.status)
			}
			if errResp.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", errResp.StatusCode, tt.status)
			}
			if errResp.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", errResp.Message, tt.wantMessage)
			}
			if errResp.StatusText != tt.wantText {
				t.Errorf("StatusText = %q, want %q", errResp.StatusText, tt.wantText)
			}
			if string(errResp.Body) != tt.body {
				t.Errorf("Body = %q, want %q", errResp.Body, tt.body)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("errors.Is(err, %v) = false, want true", tt.sentinel)
			}
			for _, other := range []error{kibana.ErrBadRequest, kibana.ErrUnauthorized, kibana.ErrForbidden, kibana.ErrNotFound, kibana.ErrConflict, kibana.ErrTooManyRequests, kibana.ErrServiceUnavailable} {
				if errors.Is(other, tt.sentinel) {
					continue
				}
				if errors.Is(err, other) {
					t.Errorf("errors.Is(err, %v) = true, want false", other)
				}
			}
			if msg := err.Error(); !strings.Contains(msg, "GET ") || !strings.Contains(msg, "/api/spaces/space/x") {
				t.Errorf("Error() = %q, want method and URL", msg)
			}
		})
	}
}

func TestErrorResponse_withoutRequest(t *testing.T) {
	err := &kibana.ErrorResponse{StatusCode: 404}
	if got, want := err.Error(), "kibana: 404 Not Found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestCheckResponse_success(t *testing.T) {
	for _, code := range []int{200, 201, 204, 299} {
		if err := kibana.CheckResponse(&http.Response{StatusCode: code}); err != nil {
			t.Errorf("CheckResponse(%d) = %v, want nil", code, err)
		}
	}
}

func TestErrorResponse_longTextBody(t *testing.T) {
	mux, client := setup(t)
	body := "<html>" + strings.Repeat("é", 2000) + "</html>"
	mux.HandleFunc("GET /api/spaces/space/x", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, body)
	})

	_, _, err := client.Spaces.Get(context.Background(), "x", nil)
	var errResp *kibana.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("error = %v, want *kibana.ErrorResponse", err)
	}
	if len(errResp.Message) > 520 || !strings.HasPrefix(errResp.Message, "<html>") {
		t.Errorf("Message is %d bytes, want a truncated prefix of the body", len(errResp.Message))
	}
	if !utf8.ValidString(errResp.Message) {
		t.Error("Message is not valid UTF-8 after truncation")
	}
	if string(errResp.Body) != body {
		t.Error("Body was truncated, want the full body")
	}
}

// sliceErr is an error type that cannot be hashed or compared.
type sliceErr []string

func (e sliceErr) Error() string { return "slice error" }

func TestErrorResponse_isUnhashableTarget(t *testing.T) {
	err := error(&kibana.ErrorResponse{StatusCode: 404})
	if errors.Is(err, sliceErr{"a"}) {
		t.Error("errors.Is(err, sliceErr) = true, want false")
	}
	objErr := error(&kibana.ObjectError{StatusCode: 404})
	if errors.Is(objErr, sliceErr{"a"}) || !errors.Is(objErr, kibana.ErrNotFound) {
		t.Error("errors.Is(ObjectError) gives the wrong result")
	}
	var nilObjErr *kibana.ObjectError
	if errors.Is(nilObjErr, kibana.ErrNotFound) {
		t.Error("errors.Is(nil *ObjectError, ErrNotFound) = true, want false")
	}
	var nilErrResp *kibana.ErrorResponse
	if errors.Is(nilErrResp, kibana.ErrNotFound) || errors.Is(nilErrResp, sliceErr{"a"}) {
		t.Error("errors.Is(nil *ErrorResponse, ...) = true, want false")
	}
}

func TestErrorResponse_request(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/spaces/space/x", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "r1")
		writeJSON(t, w, http.StatusNotFound, []byte(notFound))
	})
	_, resp, err := client.InSpace("x").Spaces.Get(context.Background(), "x", nil)
	var errResp *kibana.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("error = %v, want *kibana.ErrorResponse", err)
	}
	if errResp.Method != http.MethodGet || !strings.HasSuffix(errResp.URL, "/s/x/api/spaces/space/x") {
		t.Errorf("Method, URL = %q, %q; want the request", errResp.Method, errResp.URL)
	}
	if errResp.Header.Get("X-Request-Id") != "r1" || resp.Header.Get("X-Request-Id") != "r1" {
		t.Errorf("headers = %v, %v; want X-Request-Id", errResp.Header, resp.Header)
	}
	if resp.StatusCode != http.StatusNotFound || resp.Status != "404 Not Found" {
		t.Errorf("Response = %d %q, want 404 Not Found", resp.StatusCode, resp.Status)
	}
}

func TestErrorResponse_urlWithoutCredentials(t *testing.T) {
	r := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Request:    &http.Request{Method: http.MethodGet, URL: mustParseURL(t, "https://user:secret@kibana.example.com/api/x?token=1")},
	}
	var errResp *kibana.ErrorResponse
	if !errors.As(kibana.CheckResponse(r), &errResp) {
		t.Fatal("CheckResponse() is not an *ErrorResponse")
	}
	if strings.Contains(errResp.URL, "secret") || strings.Contains(errResp.URL, "user") || strings.Contains(errResp.Error(), "secret") {
		t.Errorf("URL = %q, Error() = %q; want no credentials", errResp.URL, errResp.Error())
	}
}

func mustParseURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
