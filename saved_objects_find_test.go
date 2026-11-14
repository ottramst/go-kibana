package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestSavedObjectsService_Find(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/saved_objects/_find", func(w http.ResponseWriter, r *http.Request) {
		// Types are repeated, and references are JSON.
		testQuery(t, r, "default_search_operator=AND&fields=title&filter=dashboard.attributes.title%3Afoo"+
			"&has_no_reference_operator=OR&has_reference_operator=AND&namespaces=default&page=2&per_page=2&search=probe%2A"+
			"&search_fields=title&sort_field=updated_at&aggs=%7B%7D"+
			"&has_no_reference=%5B%7B%22type%22%3A%22visualization%22%2C%22id%22%3A%22v%22%7D%5D"+
			"&has_reference=%5B%7B%22type%22%3A%22index-pattern%22%2C%22id%22%3A%22probe-ip%22%7D%5D"+
			"&type=index-pattern&type=dashboard")
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/find_type.json"))
	})
	got, _, err := client.InSpace("x").SavedObjects.Find(context.Background(), []string{"index-pattern", "dashboard"}, &kibana.FindSavedObjectsOptions{
		Search:                 "probe*",
		SearchFields:           []string{"title"},
		DefaultSearchOperator:  kibana.SearchOperatorAnd,
		Filter:                 "dashboard.attributes.title:foo",
		HasReference:           []kibana.SavedObjectRef{{Type: "index-pattern", ID: "probe-ip"}},
		HasReferenceOperator:   kibana.SearchOperatorAnd,
		HasNoReference:         []kibana.SavedObjectRef{{Type: "visualization", ID: "v"}},
		HasNoReferenceOperator: kibana.SearchOperatorOr,
		Fields:                 []string{"title"},
		SortField:              "updated_at",
		Page:                   2,
		PerPage:                2,
		Aggs:                   json.RawMessage(`{}`),
		Namespaces:             []string{"default"},
	})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got.Page != 1 || got.PerPage != 2 || got.Total != 5 || len(got.SavedObjects) != 2 || got.SavedObjects[1].ID != "probe-dash" {
		t.Errorf("Find() = %+v, want page 1 of 5 objects", got)
	}
}

func TestSavedObjectsService_Find_errors(t *testing.T) {
	tests := []string{"savedobjects/find_notype.json", "savedobjects/find_deep.json"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/saved_objects/_find", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusBadRequest, fixture(t, name))
			})
			_, _, err := client.SavedObjects.Find(context.Background(), []string{"index-pattern"}, nil)
			if !errors.Is(err, kibana.ErrBadRequest) {
				t.Errorf("Find() error = %v, want ErrBadRequest", err)
			}
		})
	}
}

func TestSavedObjectsService_Find_invalidArguments(t *testing.T) {
	_, client := setup(t)
	for name, tt := range map[string]struct {
		types []string
		opt   *kibana.FindSavedObjectsOptions
	}{
		"no types":             {nil, nil},
		"empty type":           {[]string{""}, nil},
		"reference without ID": {[]string{"dashboard"}, &kibana.FindSavedObjectsOptions{HasReference: []kibana.SavedObjectRef{{Type: "index-pattern"}}}},
		"no reference type":    {[]string{"dashboard"}, &kibana.FindSavedObjectsOptions{HasNoReference: []kibana.SavedObjectRef{{ID: "i"}}}},
	} {
		if _, _, err := client.SavedObjects.Find(context.Background(), tt.types, tt.opt); err == nil {
			t.Errorf("Find(%s) error = nil, want error", name)
		}
	}
}

// findPages serves total index patterns named o-0 to o-<total-1>, paginated.
func findPages(t *testing.T, mux *http.ServeMux, total int, requests *[]string) {
	t.Helper()
	mux.HandleFunc("GET /api/saved_objects/_find", func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.RawQuery)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		objs := []json.RawMessage{}
		for i := (page - 1) * perPage; i < min(page*perPage, total); i++ {
			objs = append(objs, json.RawMessage(fmt.Sprintf(`{"type":"index-pattern","id":"o-%d","attributes":{}}`, i)))
		}
		body, _ := json.Marshal(map[string]any{"page": page, "per_page": perPage, "total": total, "saved_objects": objs})
		writeJSON(t, w, http.StatusOK, body)
	})
}

func TestSavedObjectsService_FindAll(t *testing.T) {
	mux, client := setup(t)
	var requests []string
	findPages(t, mux, 5, &requests)

	var ids []string
	opt := &kibana.FindSavedObjectsOptions{PerPage: 2, Aggs: json.RawMessage(`{}`)}
	for obj, err := range client.SavedObjects.FindAll(context.Background(), []string{"index-pattern"}, opt) {
		if err != nil {
			t.Fatalf("FindAll() error = %v", err)
		}
		ids = append(ids, obj.ID)
	}
	if want := []string{"o-0", "o-1", "o-2", "o-3", "o-4"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("FindAll() = %v, want %v", ids, want)
	}
	want := []string{"page=1&per_page=2&type=index-pattern", "page=2&per_page=2&type=index-pattern", "page=3&per_page=2&type=index-pattern"}
	if !reflect.DeepEqual(requests, want) {
		t.Errorf("requests = %v, want %v", requests, want)
	}
	if opt.Page != 0 {
		t.Error("FindAll() modified the options")
	}
}

func TestSavedObjectsService_FindAll_startPageAndBreak(t *testing.T) {
	mux, client := setup(t)
	var requests []string
	findPages(t, mux, 250, &requests)
	n := 0
	for obj, err := range client.SavedObjects.FindAll(context.Background(), []string{"index-pattern"}, &kibana.FindSavedObjectsOptions{Page: 2}) {
		if err != nil {
			t.Fatalf("FindAll() error = %v", err)
		}
		if n == 0 && obj.ID != "o-100" {
			t.Errorf("first object = %s, want o-100", obj.ID)
		}
		if n++; n == 101 {
			break
		}
	}
	if want := []string{"page=2&per_page=100&type=index-pattern", "page=3&per_page=100&type=index-pattern"}; !reflect.DeepEqual(requests, want) {
		t.Errorf("requests = %v, want %v", requests, want)
	}
}

func TestSavedObjectsService_FindAll_error(t *testing.T) {
	_, client := setup(t)
	errs := 0
	// Invalid arguments fail on the first page.
	for obj, err := range client.SavedObjects.FindAll(context.Background(), nil, nil) {
		if obj != nil || err == nil {
			t.Errorf("FindAll() = %v, %v; want nil and an error", obj, err)
		}
		errs++
	}
	if errs != 1 {
		t.Errorf("FindAll() yielded %d errors, want 1", errs)
	}
}

func TestSavedObjectsService_Resolve(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/saved_objects/resolve/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/saved_objects/resolve/index-pattern/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_resolve.json"))
	})
	got, _, err := client.SavedObjects.Resolve(context.Background(), "index-pattern", "a/b?", nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Outcome != kibana.ResolveOutcomeExactMatch || got.SavedObject == nil || got.SavedObject.ID != "probe-ip" || got.AliasTargetID != "" {
		t.Errorf("Resolve() = %+v, want an exact match of probe-ip", got)
	}
}

func TestSavedObjectsService_Resolve_aliasMatch(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/saved_objects/resolve/{type}/{id}", func(w http.ResponseWriter, _ *http.Request) {
		// Legacy URL aliases can't be created on demand, so this body is
		// written by hand, in the format of Kibana's resolve response.
		writeJSON(t, w, http.StatusOK, []byte(`{"saved_object":{"id":"new","type":"dashboard","attributes":{}},"outcome":"aliasMatch","alias_target_id":"new","alias_purpose":"savedObjectConversion"}`))
	})
	got, _, err := client.SavedObjects.Resolve(context.Background(), "dashboard", "old", nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Outcome != kibana.ResolveOutcomeAliasMatch || got.AliasTargetID != "new" || got.AliasPurpose != "savedObjectConversion" {
		t.Errorf("Resolve() = %+v, want an alias match", got)
	}
}

func TestSavedObjectsService_Resolve_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/saved_objects/resolve/{type}/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "savedobjects/so_resolve_missing.json"))
	})
	ctx := context.Background()
	if _, _, err := client.SavedObjects.Resolve(ctx, "index-pattern", "probe-nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Resolve() error = %v, want ErrNotFound", err)
	}
	for _, args := range [][2]string{{"", "i"}, {"t", ""}, {"..", "i"}, {"t", "."}} {
		if _, _, err := client.SavedObjects.Resolve(ctx, args[0], args[1], nil); err == nil {
			t.Errorf("Resolve(%q, %q) error = nil, want error", args[0], args[1])
		}
	}
}
