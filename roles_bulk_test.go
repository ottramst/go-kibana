package kibana_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestRolesService_BulkCreateOrUpdate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/security/roles", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		// Read-only fields are not sent, and base is [] for feature privileges.
		testJSONBody(t, r, `{"roles":{
			"probe-b-1":{"elasticsearch":{"cluster":["monitor"]}},
			"probe-q-2":{"description":"d","elasticsearch":{},"kibana":[{"base":[],"feature":{"discover":["read"]},"spaces":["default"]}]}
		}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "roles/bulk.json"))
	})

	roles := map[string]*kibana.Role{
		"probe-b-1": {Name: "ignored", Elasticsearch: kibana.RoleElasticsearch{Cluster: []string{"monitor"}}},
		"probe-q-2": {
			Description:       "d",
			TransientMetadata: map[string]any{"enabled": true},
			Kibana:            []kibana.KibanaPrivilege{{Feature: map[string][]string{"discover": {"read"}}, Spaces: []string{"default"}, Reserved: []string{"x"}}},
		},
	}
	got, _, err := client.Roles.BulkCreateOrUpdate(context.Background(), roles)
	if err != nil {
		t.Fatalf("BulkCreateOrUpdate() error = %v", err)
	}
	if !reflect.DeepEqual(got.Created, []string{"probe-b-1"}) || !reflect.DeepEqual(got.Noop, []string{"probe-q-1"}) || !reflect.DeepEqual(got.Updated, []string{"probe-q-2"}) {
		t.Errorf("result = %+v", got)
	}
	bad, ok := got.Errors["probe-b-bad"]
	if !ok || bad.Type != "action_request_validation_exception" || !strings.Contains(bad.Reason, "unknown cluster privilege [not_a_privilege]") {
		t.Errorf("Errors = %+v", got.Errors)
	}
	if roles["probe-q-2"].Kibana[0].Base != nil || roles["probe-q-2"].Kibana[0].Reserved == nil {
		t.Error("BulkCreateOrUpdate() modified the caller's roles")
	}
}

func TestRolesService_BulkCreateOrUpdate_invalidArguments(t *testing.T) {
	_, client := setup(t)
	cases := map[string]map[string]*kibana.Role{
		"nil":        nil,
		"empty name": {"": {}},
		"dot name":   {"..": {}},
		"nil role":   {"r": nil},
		"no spaces":  {"r": {Kibana: []kibana.KibanaPrivilege{{Base: []string{"read"}}}}},
	}
	for name, roles := range cases {
		_, _, err := client.Roles.BulkCreateOrUpdate(context.Background(), roles)
		if err == nil {
			t.Errorf("BulkCreateOrUpdate(%s): error = nil, want error", name)
		}
	}
	// The error names the role.
	_, _, err := client.Roles.BulkCreateOrUpdate(context.Background(), map[string]*kibana.Role{
		"good": {},
		"bad":  {Kibana: []kibana.KibanaPrivilege{{Base: []string{"read"}}}},
	})
	if err == nil || !strings.HasPrefix(err.Error(), `kibana: role "bad": `) {
		t.Errorf("error = %v, want it to name the role", err)
	}
}
