package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestStatusService_Get(t *testing.T) {
	for _, name := range []string{"status/get.json", "status/get_8.19.json"} {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
				testQuery(t, r, "")
				writeJSON(t, w, http.StatusOK, fixture(t, name))
			})

			got, _, err := client.Status.Get(context.Background())
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if got.Version.Number == "" || got.Version.BuildHash == "" || got.Version.BuildDate.IsZero() {
				t.Errorf("Version = %+v, want number, build hash and date", got.Version)
			}
			if got.Status.Overall.Level != kibana.StatusAvailable {
				t.Errorf("Overall.Level = %q, want %q", got.Status.Overall.Level, kibana.StatusAvailable)
			}
			if es := got.Status.Core["elasticsearch"]; es.Level != kibana.StatusAvailable || len(es.Meta) == 0 {
				t.Errorf("Core[elasticsearch] = %+v, want available with meta", es)
			}
			if tm := got.Status.Plugins["taskManager"]; tm.Level != kibana.StatusAvailable || !tm.Reported {
				t.Errorf("Plugins[taskManager] = %+v, want available and reported", tm)
			}
			if len(got.Metrics) == 0 {
				t.Error("Metrics is empty")
			}
		})
	}
}

func TestStatusService_Get_version(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "status/get.json"))
	})
	got, _, err := client.Status.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := kibana.StatusVersion{
		Number:        "9.5.6",
		BuildHash:     "6b06ce57188008d15d48928b2c828ecceaa7e8fd",
		BuildNumber:   108460,
		BuildFlavor:   "traditional",
		BuildDate:     time.Date(2026, 10, 29, 0, 9, 11, 136e6, time.UTC),
		BuildSnapshot: false,
	}
	if !got.Version.BuildDate.Equal(want.BuildDate) {
		t.Errorf("BuildDate = %v, want %v", got.Version.BuildDate, want.BuildDate)
	}
	got.Version.BuildDate = want.BuildDate
	if got.Version != want {
		t.Errorf("Version = %+v, want %+v", got.Version, want)
	}
}

func TestStatusService_Get_redacted(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "status/get_redacted.json"))
	})
	got, _, err := client.Status.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Version.Number != "" || got.Status.Overall.Level != kibana.StatusAvailable {
		t.Errorf("Get() = %+v, want only the overall level", got)
	}
}

func TestStatusService_Get_unavailable(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/status", func(w http.ResponseWriter, _ *http.Request) {
		// Kibana answers 503 with the full status when it is not available.
		writeJSON(t, w, http.StatusServiceUnavailable, []byte(`{"name":"k","version":{"number":"9.5.6"},"status":{"overall":{"level":"degraded","summary":"1 service is degraded"},"core":{"elasticsearch":{"level":"degraded","summary":"slow"}}}}`))
	})
	got, resp, err := client.InSpace("x").Status.Get(context.Background())
	var errResp *kibana.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Get() error = %v, want a 503 *ErrorResponse", err)
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Response = %v, want the 503 response", resp)
	}
	if got == nil {
		t.Fatal("Get() status = nil, want the decoded status")
	}
	if got.Status.Overall.Level != kibana.StatusDegraded || got.Status.Core["elasticsearch"].Summary != "slow" {
		t.Errorf("Get() status = %+v, want degraded with the core status", got.Status)
	}
}

func TestStatusService_Get_errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"unauthorized", http.StatusUnauthorized, `{"statusCode":401,"error":"Unauthorized","message":"Unauthorized"}`, kibana.ErrUnauthorized},
		// A 503 from a proxy is not a Kibana status.
		{"proxy 503", http.StatusServiceUnavailable, `<html>Service Unavailable</html>`, nil},
		{"503 without level", http.StatusServiceUnavailable, `{"statusCode":503,"error":"Service Unavailable","message":"Kibana server is not ready yet"}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, []byte(tt.body))
			})
			got, _, err := client.Status.Get(context.Background())
			if err == nil || got != nil {
				t.Fatalf("Get() = %+v, %v; want nil status and an error", got, err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Get() error = %v, want %v", err, tt.want)
			}
		})
	}
}
