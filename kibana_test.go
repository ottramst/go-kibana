package kibana_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
		wantErr bool
	}{
		{baseURL: "http://localhost:5601", want: "http://localhost:5601"},
		{baseURL: "https://kibana.example.com/", want: "https://kibana.example.com"},
		{baseURL: "https://example.com/kibana/", want: "https://example.com/kibana"},
		{baseURL: "https://example.com/kibana?x=1#y", want: "https://example.com/kibana"},
		{baseURL: "localhost:5601", wantErr: true},
		{baseURL: "/relative", wantErr: true},
		{baseURL: "ftp://example.com", wantErr: true},
		{baseURL: "http://[::1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			c, err := kibana.NewClient(tt.baseURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewClient(%q) error = nil, want error", tt.baseURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient(%q) error = %v", tt.baseURL, err)
			}
			if got := c.BaseURL().String(); got != tt.want {
				t.Errorf("BaseURL() = %q, want %q", got, tt.want)
			}
			if got := c.Space(); got != kibana.DefaultSpace {
				t.Errorf("Space() = %q, want %q", got, kibana.DefaultSpace)
			}
		})
	}
}

func TestNewClient_optionErrors(t *testing.T) {
	opts := map[string]kibana.ClientOption{
		"empty API key":   kibana.WithAPIKey(""),
		"empty username":  kibana.WithBasicAuth("", "p"),
		"nil HTTP client": kibana.WithHTTPClient(nil),
	}
	for name, opt := range opts {
		t.Run(name, func(t *testing.T) {
			if _, err := kibana.NewClient("http://localhost:5601", opt); err == nil {
				t.Error("NewClient() error = nil, want error")
			}
		})
	}
}

func TestClient_BaseURLIsACopy(t *testing.T) {
	c, err := kibana.NewClient("http://localhost:5601")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL().Host = "changed"
	if got := c.BaseURL().Host; got != "localhost:5601" {
		t.Errorf("BaseURL().Host = %q after modifying a copy, want %q", got, "localhost:5601")
	}
}

func TestNewRequest_headers(t *testing.T) {
	tests := []struct {
		name     string
		opts     []kibana.ClientOption
		wantAuth string
	}{
		{name: "API key", opts: []kibana.ClientOption{kibana.WithAPIKey("abc==")}, wantAuth: "ApiKey abc=="},
		{name: "basic auth", opts: []kibana.ClientOption{kibana.WithBasicAuth("elastic", "changeme")}, wantAuth: "Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="},
		{name: "no auth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := kibana.NewClient("http://localhost:5601", tt.opts...)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
				req, err := c.NewRequest(context.Background(), method, "/api/status", nil)
				if err != nil {
					t.Fatalf("NewRequest(%s) error = %v", method, err)
				}
				testHeader(t, req, "Authorization", tt.wantAuth)
				testHeader(t, req, "kbn-xsrf", "true")
				testHeader(t, req, "Accept", "application/json")
				testHeader(t, req, "Content-Type", "")
				if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "go-kibana") {
					t.Errorf("User-Agent = %q, want go-kibana prefix", ua)
				}
			}
		})
	}
}

func TestNewRequest_optionsOverrideHeaders(t *testing.T) {
	c, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey("abc"), kibana.WithUserAgent("my-app/1.0"))
	if err != nil {
		t.Fatal(err)
	}
	req, err := c.NewRequest(context.Background(), http.MethodGet, "/api/status", nil,
		kibana.WithHeader("Authorization", "Bearer token"),
		kibana.WithHeader("es-security-runas-user", "jdoe"),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	testHeader(t, req, "Authorization", "Bearer token")
	testHeader(t, req, "es-security-runas-user", "jdoe")
	testHeader(t, req, "User-Agent", "my-app/1.0")
}

func TestNewRequest_optionError(t *testing.T) {
	c, err := kibana.NewClient("http://localhost:5601")
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("boom")
	_, err = c.NewRequest(context.Background(), http.MethodGet, "/api/status", nil,
		func(*http.Request) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Errorf("NewRequest() error = %v, want %v", err, wantErr)
	}
}

func TestNewRequest_url(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		space   string
		path    string
		opts    []kibana.RequestOption
		want    string
	}{
		{name: "plain", baseURL: "http://k:5601", path: "/api/status", want: "http://k:5601/api/status"},
		{name: "base path", baseURL: "http://k:5601/kibana/", path: "/api/status", want: "http://k:5601/kibana/api/status"},
		{name: "space", baseURL: "http://k:5601", space: "marketing", path: "/api/status", want: "http://k:5601/s/marketing/api/status"},
		{name: "default space has no prefix", baseURL: "http://k:5601", space: "default", path: "/api/status", want: "http://k:5601/api/status"},
		{name: "base path and space", baseURL: "http://k:5601/kibana", space: "a-b", path: "/api/status", want: "http://k:5601/kibana/s/a-b/api/status"},
		{name: "escaped segment", baseURL: "http://k:5601", path: "/api/security/role/a%2Fb%20c", want: "http://k:5601/api/security/role/a%2Fb%20c"},
		{name: "query string", baseURL: "http://k:5601", path: "/api/spaces/space?purpose=any", want: "http://k:5601/api/spaces/space?purpose=any"},
		{
			name: "query param option", baseURL: "http://k:5601", path: "/api/spaces/space?purpose=any",
			opts: []kibana.RequestOption{kibana.WithQueryParam("x", "1 2")},
			want: "http://k:5601/api/spaces/space?purpose=any&x=1+2",
		},
		{
			name: "query param option keeps existing query", baseURL: "http://k:5601", path: "/api/x?b=2&a=1;c",
			opts: []kibana.RequestOption{kibana.WithQueryParam("d", "&")},
			want: "http://k:5601/api/x?b=2&a=1;c&d=%26",
		},
		{name: "escaped base path", baseURL: "http://k:5601/a%2Fb/", space: "s", path: "/api/x", want: "http://k:5601/a%2Fb/s/s/api/x"},
		{name: "base path ending in an escaped slash", baseURL: "http://k:5601/a%2F", path: "/api/x", want: "http://k:5601/a%2F/api/x"},
		{name: "escaped query", baseURL: "http://k:5601", path: "/api/x?q=a+b&f=[1]&h=%7B%7D", want: "http://k:5601/api/x?q=a+b&f=[1]&h=%7B%7D"},
		{name: "escaped space", baseURL: "http://k:5601", space: "a/b", path: "/api/x", want: "http://k:5601/s/a%2Fb/api/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := kibana.NewClient(tt.baseURL, kibana.WithSpace(tt.space))
			if err != nil {
				t.Fatal(err)
			}
			req, err := c.NewRequest(context.Background(), http.MethodGet, tt.path, nil, tt.opts...)
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			if got := req.URL.String(); got != tt.want {
				t.Errorf("URL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewRequest_invalidPath(t *testing.T) {
	c, err := kibana.NewClient("http://localhost:5601")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"api/status", "", "/api/%zz", "/api/a#frag", "/api/a b", "/api/é", "/api/a%2Fb/é", "/api/x?q=a b", "/api/x?q=é"} {
		if _, err := c.NewRequest(context.Background(), http.MethodGet, path, nil); err == nil {
			t.Errorf("NewRequest(%q) error = nil, want error", path)
		}
	}
}

func TestNewRequest_body(t *testing.T) {
	c, err := kibana.NewClient("http://localhost:5601")
	if err != nil {
		t.Fatal(err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodPost, "/api/x", map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	testHeader(t, req, "Content-Type", "application/json")
	testJSONBody(t, req, `{"a":"b"}`)

	req, err = c.NewRequest(context.Background(), http.MethodPost, "/api/x", strings.NewReader("raw"),
		kibana.WithHeader("Content-Type", "text/plain"))
	if err != nil {
		t.Fatal(err)
	}
	testHeader(t, req, "Content-Type", "text/plain")
	if data, _ := io.ReadAll(req.Body); string(data) != "raw" {
		t.Errorf("body = %q, want %q", data, "raw")
	}

	if _, err := c.NewRequest(context.Background(), http.MethodPost, "/api/x", make(chan int)); err == nil {
		t.Error("NewRequest() with unencodable body: error = nil, want error")
	}
}

func TestClient_InSpace(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/marketing/api/spaces/space/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"id":"`+r.PathValue("id")+`","name":"x"}`))
	})

	scoped := client.InSpace("marketing")
	if got := scoped.Space(); got != "marketing" {
		t.Errorf("Space() = %q, want %q", got, "marketing")
	}
	if got := client.Space(); got != kibana.DefaultSpace {
		t.Errorf("original Space() = %q, want %q", got, kibana.DefaultSpace)
	}
	space, _, err := scoped.Spaces.Get(context.Background(), "marketing", nil)
	if err != nil {
		t.Fatalf("Spaces.Get() error = %v", err)
	}
	if space.ID != "marketing" {
		t.Errorf("ID = %q, want %q", space.ID, "marketing")
	}
	if got := scoped.InSpace(kibana.DefaultSpace).Space(); got != kibana.DefaultSpace {
		t.Errorf("InSpace(default).Space() = %q, want %q", got, kibana.DefaultSpace)
	}
}

func TestDo(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /json", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"a":"b"}`))
	})
	mux.HandleFunc("PUT /no-content", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /empty", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /bad-json", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"a":`))
	})
	mux.HandleFunc("GET /trailing-data", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"a":"b"} junk`))
	})
	mux.HandleFunc("GET /trailing-space", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte("{\"a\":\"b\"}\n\n"))
	})
	mux.HandleFunc("GET /ndjson", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "{\"a\":1}\n{\"b\":2}\n")
	})
	ctx := context.Background()

	do := func(method, path string, v any) (*kibana.Response, error) {
		t.Helper()
		req, err := client.NewRequest(ctx, method, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		return client.Do(req, v)
	}

	var got map[string]string
	resp, err := do(http.MethodGet, "/json", &got)
	if err != nil {
		t.Fatalf("Do(json) error = %v", err)
	}
	if resp.StatusCode != http.StatusOK || got["a"] != "b" {
		t.Errorf("Do(json) = %d %v, want 200 map[a:b]", resp.StatusCode, got)
	}

	var untouched map[string]string
	if resp, err := do(http.MethodPut, "/no-content", &untouched); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Do(204) = %v, %v; want 204, nil", resp, err)
	}
	if untouched != nil {
		t.Errorf("Do(204) decoded %v, want nil", untouched)
	}
	if _, err := do(http.MethodGet, "/empty", &untouched); err != nil {
		t.Errorf("Do(empty body) error = %v", err)
	}
	if _, err := do(http.MethodGet, "/json", nil); err != nil {
		t.Errorf("Do(nil v) error = %v", err)
	}
	if _, err := do(http.MethodGet, "/bad-json", &got); err == nil {
		t.Error("Do(bad json) error = nil, want error")
	}
	if _, err := do(http.MethodGet, "/trailing-data", &got); err == nil {
		t.Error("Do(trailing data) error = nil, want error")
	}
	if _, err := do(http.MethodGet, "/trailing-space", &got); err != nil {
		t.Errorf("Do(trailing whitespace) error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := do(http.MethodGet, "/ndjson", &buf); err != nil {
		t.Fatalf("Do(writer) error = %v", err)
	}
	if want := "{\"a\":1}\n{\"b\":2}\n"; buf.String() != want {
		t.Errorf("Do(writer) wrote %q, want %q", buf.String(), want)
	}
}

func TestDo_contextCanceled(t *testing.T) {
	_, client := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := client.NewRequest(ctx, http.MethodGet, "/api/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Do() error = %v, want context.Canceled", err)
	}
}

func TestWithHTTPClient(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return http.DefaultTransport.RoundTrip(r)
	})}

	client, err := kibana.NewClient(server.URL, kibana.WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Spaces.Delete(context.Background(), "x", nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !called {
		t.Error("custom HTTP client was not used")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAddQuery(t *testing.T) {
	opt := &kibana.GetRoleOptions{ReplaceDeprecatedPrivileges: true}
	tests := []struct {
		path string
		opt  any
		want string
	}{
		{"/api/x", opt, "/api/x?replaceDeprecatedPrivileges=true"},
		// A path that has a query gets the options appended to it.
		{"/api/x?a=1", opt, "/api/x?a=1&replaceDeprecatedPrivileges=true"},
		{"/api/x?a=1", (*kibana.GetRoleOptions)(nil), "/api/x?a=1"},
		{"/api/x", nil, "/api/x"},
		{"/api/x", &kibana.GetRoleOptions{}, "/api/x"},
	}
	for _, tt := range tests {
		got, err := kibana.AddQuery(tt.path, tt.opt)
		if err != nil || got != tt.want {
			t.Errorf("addQuery(%q, %+v) = %q, %v; want %q", tt.path, tt.opt, got, err, tt.want)
		}
	}
}
