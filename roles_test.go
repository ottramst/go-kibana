package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestRolesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/security/role", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "replaceDeprecatedPrivileges=true")
		writeJSON(t, w, http.StatusOK, fixture(t, "roles/list.json"))
	})

	roles, _, err := client.Roles.List(context.Background(), &kibana.ListRolesOptions{ReplaceDeprecatedPrivileges: true})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
	}
	if want := []string{"editor", "my_kibana_role", "viewer"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("role names = %v, want %v", names, want)
	}
	editor := roles[0]
	if editor.Metadata["_reserved"] != true {
		t.Errorf("editor Metadata = %v, want _reserved=true", editor.Metadata)
	}
	wantKibana := []kibana.KibanaPrivilege{{Base: []string{"all"}, Feature: map[string][]string{}, Spaces: []string{"*"}}}
	if !reflect.DeepEqual(editor.Kibana, wantKibana) {
		t.Errorf("editor Kibana = %+v, want %+v", editor.Kibana, wantKibana)
	}
	if got := roles[1].Kibana[0].Feature["discover"]; !reflect.DeepEqual(got, []string{"all"}) {
		t.Errorf("my_kibana_role discover privileges = %v, want [all]", got)
	}
}

func TestRolesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/security/role/{name}", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		if r.PathValue("name") != "my_kibana_role" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "roles/get.json"))
	})

	role, _, err := client.Roles.Get(context.Background(), "my_kibana_role", nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if role.Name != "my_kibana_role" || role.Description != "Read logs and use Discover" {
		t.Errorf("Get() = %+v", role)
	}
	wantES := kibana.ElasticsearchPrivileges{
		Cluster: []string{"monitor"},
		Indices: []kibana.IndexPrivilege{{
			Names:         []string{"logs-*"},
			Privileges:    []string{"read", "view_index_metadata"},
			FieldSecurity: map[string][]string{"grant": {"*"}, "except": {"secret"}},
		}},
		RunAs: []string{},
	}
	if !reflect.DeepEqual(role.Elasticsearch, wantES) {
		t.Errorf("Elasticsearch = %+v, want %+v", role.Elasticsearch, wantES)
	}
	wantKibana := []kibana.KibanaPrivilege{
		{Base: []string{}, Feature: map[string][]string{"discover": {"all"}, "dashboard": {"read"}}, Spaces: []string{"marketing"}},
		{Base: []string{"read"}, Feature: map[string][]string{}, Spaces: []string{"default"}},
	}
	if !reflect.DeepEqual(role.Kibana, wantKibana) {
		t.Errorf("Kibana = %+v, want %+v", role.Kibana, wantKibana)
	}
	if role.TransientMetadata["enabled"] != true {
		t.Errorf("TransientMetadata = %v, want enabled=true", role.TransientMetadata)
	}

	if _, _, err := client.Roles.Get(context.Background(), "missing", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestRolesService_CreateOrUpdate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/security/role/{name}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("name"); got != "my role" {
			t.Errorf("name = %q, want %q", got, "my role")
		}
		testQuery(t, r, "createOnly=true")
		testHeader(t, r, "kbn-xsrf", "true")
		// Read-only fields (name, transient_metadata, _reserved,
		// _unrecognized_applications) must not be sent, and base must be an
		// empty list for feature privileges.
		testJSONBody(t, r, `{
			"description": "d",
			"metadata": {"version": 1},
			"elasticsearch": {"cluster": ["monitor"], "indices": [{"names": ["logs-*"], "privileges": ["read"]}]},
			"kibana": [
				{"base": [], "feature": {"discover": ["read"]}, "spaces": ["default"]},
				{"base": ["all"], "spaces": ["*"]}
			]
		}`)
		w.WriteHeader(http.StatusNoContent)
	})

	role := &kibana.Role{
		Name:              "ignored",
		Description:       "d",
		Metadata:          map[string]any{"version": 1},
		TransientMetadata: map[string]any{"enabled": true},
		Elasticsearch: kibana.ElasticsearchPrivileges{
			Cluster: []string{"monitor"},
			Indices: []kibana.IndexPrivilege{{Names: []string{"logs-*"}, Privileges: []string{"read"}}},
		},
		Kibana: []kibana.KibanaPrivilege{
			{Feature: map[string][]string{"discover": {"read"}}, Spaces: []string{"default"}, Reserved: []string{"x"}},
			{Base: []string{"all"}, Spaces: []string{"*"}},
		},
		UnrecognizedApplications: []string{"app"},
		TransformErrors:          []kibana.RoleTransformError{{Reason: "r"}},
	}
	resp, err := client.Roles.CreateOrUpdate(context.Background(), "my role", role, &kibana.CreateOrUpdateRoleOptions{CreateOnly: true})
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if role.Kibana[0].Base != nil || role.Kibana[0].Reserved == nil {
		t.Error("CreateOrUpdate() modified the caller's role")
	}
}

func TestRolesService_CreateOrUpdate_minimal(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/security/role/r", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		// elasticsearch is required by Kibana, even when empty.
		testJSONBody(t, r, `{"elasticsearch": {}}`)
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := client.Roles.CreateOrUpdate(context.Background(), "r", &kibana.Role{}, nil); err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
}

func TestRolesService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/security/role/r", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := client.Roles.Delete(context.Background(), "r", nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestRolesService_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	checks := map[string]error{}
	_, _, checks["Get empty name"] = client.Roles.Get(ctx, "", nil)
	_, checks["CreateOrUpdate empty name"] = client.Roles.CreateOrUpdate(ctx, "", &kibana.Role{}, nil)
	_, checks["CreateOrUpdate nil"] = client.Roles.CreateOrUpdate(ctx, "r", nil, nil)
	_, checks["Delete empty name"] = client.Roles.Delete(ctx, "", nil)
	_, checks["Delete dot-dot name"] = client.Roles.Delete(ctx, "..", nil)
	_, _, checks["Get dot name"] = client.Roles.Get(ctx, ".", nil)
	// An empty Spaces list would make Kibana grant the privilege in every space.
	_, checks["CreateOrUpdate privilege without spaces"] = client.Roles.CreateOrUpdate(ctx, "r", &kibana.Role{
		Kibana: []kibana.KibanaPrivilege{{Base: []string{"read"}, Spaces: []string{}}},
	}, nil)
	for name, err := range checks {
		if err == nil {
			t.Errorf("%s: error = nil, want error", name)
		}
		var errResp *kibana.ErrorResponse
		if errors.As(err, &errResp) {
			t.Errorf("%s: request was sent: %v", name, err)
		}
	}
}
