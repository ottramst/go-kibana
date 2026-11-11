package kibana_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"testing"

	"github.com/ottramst/go-kibana"
)

// Acceptance tests run against a real Kibana when KIBANA_ACC is set. Start
// one with `docker compose up -d --wait`, then run `make testacc`.
//
//	KIBANA_URL       Kibana URL (default http://localhost:5601)
//	KIBANA_API_KEY   API key; if empty, basic auth is used
//	KIBANA_USERNAME  basic auth user (default elastic)
//	KIBANA_PASSWORD  basic auth password (default changeme)

// accClient returns a client for the acceptance test Kibana, or skips the
// test unless KIBANA_ACC is set.
func accClient(t *testing.T) *kibana.Client {
	t.Helper()
	if os.Getenv("KIBANA_ACC") == "" {
		t.Skip("set KIBANA_ACC=1 to run acceptance tests")
	}
	auth := kibana.WithBasicAuth(envOr("KIBANA_USERNAME", "elastic"), envOr("KIBANA_PASSWORD", "changeme"))
	if key := os.Getenv("KIBANA_API_KEY"); key != "" {
		auth = kibana.WithAPIKey(key)
	}
	client, err := kibana.NewClient(envOr("KIBANA_URL", "http://localhost:5601"), auth)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

// localStack reports whether the tests run against the local compose stack
// (KIBANA_URL unset), where they may change shared state such as the default
// space.
func localStack() bool {
	return os.Getenv("KIBANA_URL") == ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// randName returns prefix followed by a random suffix, usable as a space ID
// or role name.
func randName(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// kibanaVersion returns the version of the acceptance test Kibana.
func kibanaVersion(t *testing.T, client *kibana.Client) string {
	t.Helper()
	req, err := client.NewRequest(t.Context(), http.MethodGet, "/api/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		Version struct {
			Number string `json:"number"`
		} `json:"version"`
	}
	if _, err := client.Do(req, &status); err != nil {
		t.Fatalf("GET /api/status error = %v", err)
	}
	return status.Version.Number
}

func TestAccStatus(t *testing.T) {
	client := accClient(t)
	v := kibanaVersion(t, client)
	if v == "" {
		t.Fatal("GET /api/status returned no version")
	}
	t.Logf("Kibana %s", v)
}

// cleanup runs fn when the test ends, with a context that is not canceled.
func cleanup(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	t.Cleanup(func() {
		if err := fn(context.WithoutCancel(t.Context())); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
}
