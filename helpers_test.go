package kibana_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

// setup starts a test server and returns its request multiplexer and a
// client that sends requests to it with basic auth (user "u", password "p").
func setup(t *testing.T) (*http.ServeMux, *kibana.Client) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := kibana.NewClient(server.URL, kibana.WithBasicAuth("u", "p"))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return mux, client
}

func testHeader(t *testing.T, r *http.Request, key, want string) {
	t.Helper()
	if got := r.Header.Get(key); got != want {
		t.Errorf("header %s = %q, want %q", key, got, want)
	}
}

// testJSONBody compares the request body with want as JSON values, ignoring
// formatting and key order.
func testJSONBody(t *testing.T, r *http.Request, want string) {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	var gotV, wantV any
	if err := json.Unmarshal(data, &gotV); err != nil {
		t.Fatalf("request body is not JSON: %v\n%s", err, data)
	}
	if err := json.Unmarshal([]byte(want), &wantV); err != nil {
		t.Fatalf("want is not JSON: %v", err)
	}
	if !reflect.DeepEqual(gotV, wantV) {
		t.Errorf("request body = %s, want %s", data, want)
	}
}

func testQuery(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.URL.RawQuery; got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

// fixture returns the contents of testdata/name.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return data
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, body []byte) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		t.Errorf("writing response: %v", err)
	}
}

// notFound is the body Kibana returns for missing objects.
const notFound = `{"statusCode":404,"error":"Not Found","message":"Not Found"}`
