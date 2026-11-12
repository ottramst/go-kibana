package kibana_test

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
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

// kibanaVersion returns the version of the acceptance test Kibana, such as
// "9.5.6".
func kibanaVersion(t *testing.T, client *kibana.Client) string {
	t.Helper()
	status, _, err := client.Status.Get(t.Context())
	if err != nil {
		t.Fatalf("Status.Get() error = %v", err)
	}
	// Kibana 9.5+ hides the version from users without the monitor cluster
	// privilege.
	if status.Version.Number == "" {
		t.Fatal("Status.Get() returned no version; the test user needs the monitor cluster privilege")
	}
	return status.Version.Number
}

// skipBelow skips the test if the acceptance test Kibana is older than
// version minVersion, such as "9.5.0".
func skipBelow(t *testing.T, client *kibana.Client, minVersion string) {
	t.Helper()
	if v := kibanaVersion(t, client); compareVersions(v, minVersion) < 0 {
		t.Skipf("needs Kibana %s or later, got %s", minVersion, v)
	}
}

// compareVersions compares two dotted versions such as "9.5.6", ignoring
// suffixes such as "-SNAPSHOT". Missing parts count as 0.
func compareVersions(a, b string) int {
	parse := func(v string) []int {
		v, _, _ = strings.Cut(v, "-")
		var parts []int
		for p := range strings.SplitSeq(v, ".") {
			n, _ := strconv.Atoi(p)
			parts = append(parts, n)
		}
		return parts
	}
	pa, pb := parse(a), parse(b)
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if c := cmp.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"9.5.6", "9.5.0", 1},
		{"8.19.25", "9.5.0", -1},
		{"9.5.0", "9.5.0", 0},
		{"9.5", "9.5.0", 0},
		{"9.10.0", "9.5.0", 1},
		{"9.5.0-SNAPSHOT", "9.5.0", 0},
		{"", "9.5.0", -1},
	}
	for _, tt := range tests {
		if got := compareVersions(tt.a, tt.b); got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
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

// createIndexPattern creates an index pattern (data view) saved object in the
// client's space. Objects in a space are deleted with the space.
func createIndexPattern(t *testing.T, client *kibana.Client, id, title string) {
	t.Helper()
	body := map[string]any{"attributes": map[string]string{"title": title}}
	req, err := client.NewRequest(t.Context(), http.MethodPost, "/api/saved_objects/index-pattern/"+url.PathEscape(id), body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, nil); err != nil {
		t.Fatalf("creating index pattern %s: %v", id, err)
	}
}

// createSpace creates a space and deletes it, with its objects, when the
// test ends.
func createSpace(t *testing.T, client *kibana.Client) string {
	t.Helper()
	id := randName(t, "acc")
	if _, _, err := client.Spaces.Create(t.Context(), &kibana.Space{ID: id, Name: "Acceptance " + id}); err != nil {
		t.Fatalf("creating space %s: %v", id, err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Spaces.Delete(ctx, id)
		return err
	})
	return id
}
