package kibana_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

// compactJSON returns raw without insignificant whitespace.
func compactJSON(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatalf("compact %q: %v", raw, err)
	}
	return b.String()
}

func TestConnectorsService_Get(t *testing.T) {
	tests := []struct {
		fixture  string
		authMode kibana.ConnectorAuthMode
	}{
		{"connectors/get.json", kibana.ConnectorAuthModeShared},
		// Kibana 8.19 returns neither auth_mode nor is_connector_type_deprecated.
		{"connectors/get_8.19.json", ""},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/actions/connector/{id}", func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.EscapedPath(), "/api/actions/connector/a%2Fb%3F"; got != want {
					t.Errorf("path = %q, want %q", got, want)
				}
				writeJSON(t, w, http.StatusOK, fixture(t, tt.fixture))
			})
			got, _, err := client.Connectors.Get(context.Background(), "a/b?", nil)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if got.ConnectorTypeID != kibana.ConnectorTypeIDWebhook || got.Name == "" || got.AuthMode != tt.authMode || got.Secrets != nil {
				t.Errorf("Get() = %+v", got)
			}
			if got, want := compactJSON(t, got.Config), `"hasAuth":true`; !bytes.Contains([]byte(got), []byte(want)) {
				t.Errorf("Config = %s, want it to contain %s", got, want)
			}
		})
	}
}

func TestConnectorsService_Get_notFound(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/actions/connector/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "connectors/get_notfound.json"))
	})
	if _, _, err := client.Connectors.Get(context.Background(), "nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestConnectorsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/actions/connectors", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "connectors/list.json"))
	})
	got, _, err := client.InSpace("x").Connectors.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != "probe-es" || got[1].ConnectorTypeID != kibana.ConnectorTypeIDServerLog {
		t.Errorf("List() = %+v, want the 2 connectors", got)
	}
}

func TestConnectorsService_Create(t *testing.T) {
	tests := []struct {
		name string
		id   string
		path string
	}{
		{"generated ID", "", "/api/actions/connector"},
		{"given ID", "a/b?", "/api/actions/connector/a%2Fb%3F"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.EscapedPath(); got != tt.path {
					t.Errorf("path = %q, want %q", got, tt.path)
				}
				testHeader(t, r, "kbn-xsrf", "true")
				// Only the writable fields are sent.
				testJSONBody(t, r, `{"name":"probe-log","connector_type_id":".server-log","config":{"a":1},"secrets":{"b":2}}`)
				writeJSON(t, w, http.StatusOK, fixture(t, "connectors/create.json"))
			})
			c := &kibana.Connector{
				ID:                tt.id,
				Name:              "probe-log",
				ConnectorTypeID:   kibana.ConnectorTypeIDServerLog,
				Config:            json.RawMessage(`{"a":1}`),
				Secrets:           json.RawMessage(`{"b":2}`),
				IsPreconfigured:   true,
				IsMissingSecrets:  true,
				AuthMode:          kibana.ConnectorAuthModeShared,
				ReferencedByCount: 3,
			}
			before := *c
			got, _, err := client.Connectors.Create(context.Background(), c, nil)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if got.ID != "5558f519-84e2-4d96-a7d6-18a5a93f4248" {
				t.Errorf("Create() = %+v, want the created connector", got)
			}
			if !reflect.DeepEqual(*c, before) {
				t.Error("Create() modified the connector")
			}
		})
	}
}

func TestConnectorsService_Create_nilConfig(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/actions/connector", func(w http.ResponseWriter, r *http.Request) {
		// Kibana rejects null config and secrets.
		testJSONBody(t, r, `{"name":"n","connector_type_id":".server-log"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "connectors/create.json"))
	})
	if _, _, err := client.Connectors.Create(context.Background(), &kibana.Connector{Name: "n", ConnectorTypeID: kibana.ConnectorTypeIDServerLog}, nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestConnectorsService_Create_errors(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		want    error
	}{
		{"connectors/create_dup.json", http.StatusConflict, kibana.ErrConflict},
		{"connectors/create_badtype.json", http.StatusBadRequest, kibana.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			_, _, err := client.Connectors.Create(context.Background(), &kibana.Connector{ID: "x", Name: "n", ConnectorTypeID: ".nope"}, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("Create() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestConnectorsService_Create_invalid(t *testing.T) {
	_, client := setup(t)
	for name, c := range map[string]*kibana.Connector{
		"nil":     nil,
		"no name": {ConnectorTypeID: kibana.ConnectorTypeIDServerLog},
		"no type": {Name: "n"},
		"bad ID":  {ID: "..", Name: "n", ConnectorTypeID: kibana.ConnectorTypeIDServerLog},
	} {
		if _, _, err := client.Connectors.Create(context.Background(), c, nil); err == nil {
			t.Errorf("Create(%s) error = nil, want an error", name)
		}
	}
}

func TestConnectorsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/actions/connector/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "connectors/get.json"))
	})
	mux.HandleFunc("PUT /api/actions/connector/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("id"); got != "probe-es" {
			t.Errorf("id = %q", got)
		}
		// Read, modify, write: the read-only fields and the type are not
		// sent, and the config is kept.
		testJSONBody(t, r, `{"name":"renamed","config":{"url":"http://elasticsearch:9200/_count","method":"post","headers":{"Content-Type":"application/json"},"hasAuth":true,"authType":"webhook-authentication-basic"},"secrets":{"user":"u","password":"p"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "connectors/get.json"))
	})
	ctx := context.Background()
	c, _, err := client.Connectors.Get(ctx, "probe-es", nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	c.Name = "renamed"
	c.Secrets = json.RawMessage(`{"user":"u","password":"p"}`)
	if _, _, err := client.Connectors.Update(ctx, c.ID, c, nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestConnectorsService_Update_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/actions/connector/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "connectors/update_nosecrets.json"))
	})
	ctx := context.Background()
	if _, _, err := client.Connectors.Update(ctx, "x", &kibana.Connector{Name: "n"}, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("Update() error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.Connectors.Update(ctx, "", &kibana.Connector{Name: "n"}, nil); err == nil {
		t.Error("Update(empty ID) error = nil, want an error")
	}
	if _, _, err := client.Connectors.Update(ctx, "x", nil, nil); err == nil {
		t.Error("Update(nil) error = nil, want an error")
	}
}

func TestConnectorsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/actions/connector/{id}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		if got, want := r.URL.EscapedPath(), "/api/actions/connector/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := context.Background()
	resp, err := client.Connectors.Delete(ctx, "a/b?", nil)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Delete() = %v, %v, want 204", resp, err)
	}
	if _, err := client.Connectors.Delete(ctx, "", nil); err == nil {
		t.Error("Delete(empty ID) error = nil, want an error")
	}
}

func TestConnectorsService_Execute(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/actions/connector/{id}/_execute", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"params":{"body":"{\"query\":{\"match_all\":{}}}"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "connectors/execute.json"))
	})
	got, _, err := client.Connectors.Execute(context.Background(), "probe-es", &kibana.ExecuteConnectorRequest{
		Params: json.RawMessage(`{"body":"{\"query\":{\"match_all\":{}}}"}`),
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Status != kibana.ExecuteConnectorStatusOK || got.ConnectorID != "probe-es" || compactJSON(t, got.Data) != `"{\"query\":{\"match_all\":{}}}"` {
		t.Errorf("Execute() = %+v", got)
	}
}

func TestConnectorsService_Execute_failed(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/actions/connector/{id}/_execute", func(w http.ResponseWriter, r *http.Request) {
		// Empty params are sent as {}: Kibana requires the field.
		testJSONBody(t, r, `{"params":{}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "connectors/execute_error.json"))
	})
	// A failed execution is a result, not an error.
	got, _, err := client.Connectors.Execute(context.Background(), "probe-log-1", &kibana.ExecuteConnectorRequest{Params: json.RawMessage{}}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Status != kibana.ExecuteConnectorStatusError || got.ErrorSource != kibana.ExecuteConnectorErrorSourceUser || got.Message == "" || string(got.Retry) != "false" {
		t.Errorf("Execute() = %+v, want a user error", got)
	}
	if _, _, err := client.Connectors.Execute(context.Background(), "probe-log-1", nil, nil); err != nil {
		t.Errorf("Execute(nil) error = %v", err)
	}
	if _, _, err := client.Connectors.Execute(context.Background(), "", nil, nil); err == nil {
		t.Error("Execute(empty ID) error = nil, want an error")
	}
}

func TestConnectorsService_ListTypes(t *testing.T) {
	tests := []struct {
		fixture string
		source  kibana.ConnectorTypeSource
	}{
		{"connectors/types.json", kibana.ConnectorTypeSourceStack},
		// Kibana 8.19 returns no source.
		{"connectors/types_8.19.json", ""},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/actions/connector_types", func(w http.ResponseWriter, r *http.Request) {
				testQuery(t, r, "feature_id=alerting")
				writeJSON(t, w, http.StatusOK, fixture(t, tt.fixture))
			})
			got, _, err := client.Connectors.ListTypes(context.Background(), &kibana.ListConnectorTypesOptions{FeatureID: "alerting"})
			if err != nil {
				t.Fatalf("ListTypes() error = %v", err)
			}
			want := &kibana.ConnectorType{
				ID:                     kibana.ConnectorTypeIDServerLog,
				Name:                   "Server log",
				Enabled:                true,
				EnabledInConfig:        true,
				EnabledInLicense:       true,
				MinimumLicenseRequired: kibana.LicenseLevelBasic,
				SupportedFeatureIDs:    []string{"alerting", "uptime", "siem"},
				Source:                 tt.source,
			}
			if len(got) < 2 || !reflect.DeepEqual(got[0], want) {
				t.Errorf("ListTypes()[0] = %+v, want %+v", got[0], want)
			}
			if got[1].SubFeature != "endpointSecurity" || got[1].MinimumLicenseRequired != kibana.LicenseLevelEnterprise {
				t.Errorf("ListTypes()[1] = %+v", got[1])
			}
		})
	}
}
