package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestRolesService_Query(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/security/role/_query", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testJSONBody(t, r, `{"query":"probe-q","size":2,"sort":{"field":"name","direction":"asc"},"filters":{"showReservedRoles":true}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "roles/query.json"))
	})

	got, _, err := client.Roles.Query(context.Background(), &kibana.QueryRolesRequest{
		Query:   "probe-q",
		Size:    2,
		Sort:    &kibana.RoleSort{Field: "name", Direction: kibana.SortAsc},
		Filters: &kibana.QueryRolesFilters{ShowReservedRoles: true},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if got.Count != 2 || got.Total != 3 || len(got.Roles) != 2 {
		t.Fatalf("Query() = count %d, total %d, %d roles; want 2, 3, 2", got.Count, got.Total, len(got.Roles))
	}
	want := []kibana.KibanaPrivilege{{Base: []string{"read"}, Feature: map[string][]string{}, Spaces: []string{"default"}}}
	if got.Roles[0].Name != "probe-q-1" || !reflect.DeepEqual(got.Roles[0].Kibana, want) {
		t.Errorf("roles[0] = %+v", got.Roles[0])
	}
}

func TestRolesService_Query_nilRequest(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/security/role/_query", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{}`)
		writeJSON(t, w, http.StatusOK, []byte(`{"roles":[],"count":0,"total":0}`))
	})
	got, _, err := client.Roles.Query(context.Background(), nil)
	if err != nil || got.Total != 0 {
		t.Errorf("Query(nil) = %+v, %v", got, err)
	}
}

// roleQueryServer serves names as roles, paginated like Kibana.
func roleQueryServer(t *testing.T, mux *http.ServeMux, names []string, failAt int) *[]kibana.QueryRolesRequest {
	t.Helper()
	var requests []kibana.QueryRolesRequest
	mux.HandleFunc("POST /api/security/role/_query", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "X-Test", "yes")
		var req kibana.QueryRolesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		requests = append(requests, req)
		if req.From == failAt {
			writeJSON(t, w, http.StatusInternalServerError, []byte(`{"statusCode":500,"error":"Internal Server Error","message":"boom"}`))
			return
		}
		end := min(req.From+req.Size, len(names))
		var roles []string
		for _, n := range names[min(req.From, end):end] {
			roles = append(roles, fmt.Sprintf(`{"name":%q,"elasticsearch":{},"kibana":[]}`, n))
		}
		writeJSON(t, w, http.StatusOK, fmt.Appendf(nil, `{"roles":[%s],"count":%d,"total":%d}`, strings.Join(roles, ","), len(roles), len(names)))
	})
	return &requests
}

func TestRolesService_QueryAll(t *testing.T) {
	mux, client := setup(t)
	requests := roleQueryServer(t, mux, []string{"a", "b", "c", "d", "e"}, -1)

	req := &kibana.QueryRolesRequest{Query: "x", Size: 2}
	var names []string
	for role, err := range client.Roles.QueryAll(context.Background(), req, kibana.WithHeader("X-Test", "yes")) {
		if err != nil {
			t.Fatalf("QueryAll() error = %v", err)
		}
		names = append(names, role.Name)
	}
	if want := []string{"a", "b", "c", "d", "e"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if len(*requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(*requests))
	}
	for i, r := range *requests {
		if r.From != i*2 || r.Size != 2 || r.Query != "x" || r.Sort == nil || r.Sort.Field != "name" || r.Sort.Direction != kibana.SortAsc {
			t.Errorf("request %d = %+v (sort %+v), want from %d, size 2, query x, sorted by name", i, r, r.Sort, i*2)
		}
	}
	if req.Sort != nil || req.From != 0 {
		t.Error("QueryAll() modified the caller's request")
	}
}

func TestRolesService_QueryAll_error(t *testing.T) {
	mux, client := setup(t)
	roleQueryServer(t, mux, []string{"a", "b", "c"}, 2)

	var names []string
	var gotErr error
	for role, err := range client.Roles.QueryAll(context.Background(), &kibana.QueryRolesRequest{Size: 2}, kibana.WithHeader("X-Test", "yes")) {
		if err != nil {
			gotErr = err
			continue
		}
		names = append(names, role.Name)
	}
	var errResp *kibana.ErrorResponse
	if !errors.As(gotErr, &errResp) || errResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("error = %v, want a 500 ErrorResponse", gotErr)
	}
	if !reflect.DeepEqual(names, []string{"a", "b"}) {
		t.Errorf("names = %v, want [a b] before the error", names)
	}
}
