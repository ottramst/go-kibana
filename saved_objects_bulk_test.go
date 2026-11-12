package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestSavedObjectsService_BulkCreate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/saved_objects/_bulk_create", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "overwrite=true")
		testJSONBody(t, r, `[{"id":"probe-b1","type":"index-pattern","attributes":{"title":"b1-*"}},{"id":"probe-ip","type":"index-pattern","attributes":{"title":"dup"}}]`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/bulk_create.json"))
	})
	got, _, err := client.InSpace("x").SavedObjects.BulkCreate(context.Background(), []*kibana.SavedObject{
		{ID: "probe-b1", Type: "index-pattern", Attributes: json.RawMessage(`{"title":"b1-*"}`), Version: "ignored"},
		{ID: "probe-ip", Type: "index-pattern", Attributes: json.RawMessage(`{"title":"dup"}`)},
	}, &kibana.BulkCreateSavedObjectsOptions{Overwrite: true})
	if err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}
	if len(got) != 2 || got[0].Error != nil || got[0].Version == "" {
		t.Fatalf("BulkCreate() = %+v, want 2 objects, the first created", got)
	}
	// Failed objects are reported in the result.
	if got[1].ID != "probe-ip" || !errors.Is(got[1].Error, kibana.ErrConflict) {
		t.Errorf("BulkCreate()[1] = %+v (%v), want a conflict", got[1], got[1].Error)
	}
}

func TestSavedObjectsService_BulkGet(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_bulk_get", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `[{"type":"index-pattern","id":"probe-b1"},{"type":"index-pattern","id":"probe-nope"},{"type":"index-pattern","id":"probe-ip","fields":["title"],"namespaces":["default"]}]`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/bulk_get.json"))
	})
	got, _, err := client.SavedObjects.BulkGet(context.Background(), []kibana.BulkGetSavedObjectsRequest{
		{Type: "index-pattern", ID: "probe-b1"},
		{Type: "index-pattern", ID: "probe-nope"},
		{Type: "index-pattern", ID: "probe-ip", Fields: []string{"title"}, Namespaces: []string{"default"}},
	})
	if err != nil {
		t.Fatalf("BulkGet() error = %v", err)
	}
	if len(got) != 3 || got[0].Error != nil || !errors.Is(got[1].Error, kibana.ErrNotFound) || got[2].ID != "probe-ip" {
		t.Errorf("BulkGet() = %+v, want 3 objects with the second missing", got)
	}
}

func TestSavedObjectsService_BulkUpdate(t *testing.T) {
	mux, client := setup(t)
	// Kibana 8.19 and 9.5 only accept PUT.
	mux.HandleFunc("PUT /api/saved_objects/_bulk_update", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `[{"id":"probe-b1","type":"index-pattern","attributes":{"title":"b1u-*"},"version":"v1"},{"id":"probe-nope","type":"index-pattern","attributes":{"title":"x"},"references":[]}]`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/bulk_update_put.json"))
	})
	got, _, err := client.SavedObjects.BulkUpdate(context.Background(), []*kibana.SavedObject{
		{ID: "probe-b1", Type: "index-pattern", Attributes: json.RawMessage(`{"title":"b1u-*"}`), Version: "v1", TypeMigrationVersion: "8.0.0"},
		{ID: "probe-nope", Type: "index-pattern", Attributes: json.RawMessage(`{"title":"x"}`), References: []kibana.SavedObjectReference{}},
	})
	if err != nil {
		t.Fatalf("BulkUpdate() error = %v", err)
	}
	if len(got) != 2 || got[0].Version != "WzE0LDFd" || !errors.Is(got[1].Error, kibana.ErrNotFound) {
		t.Errorf("BulkUpdate() = %+v, want the update and a missing object", got)
	}
}

func TestSavedObjectsService_BulkDelete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_bulk_delete", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "force=true")
		testJSONBody(t, r, `[{"type":"index-pattern","id":"probe-b1"},{"type":"index-pattern","id":"probe-nope"}]`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/bulk_delete.json"))
	})
	got, _, err := client.SavedObjects.BulkDelete(context.Background(), []kibana.SavedObjectRef{
		{Type: "index-pattern", ID: "probe-b1"}, {Type: "index-pattern", ID: "probe-nope"},
	}, &kibana.DeleteSavedObjectOptions{Force: true})
	if err != nil {
		t.Fatalf("BulkDelete() error = %v", err)
	}
	if len(got) != 2 || !got[0].Success || got[0].Error != nil || got[1].Success || !errors.Is(got[1].Error, kibana.ErrNotFound) {
		t.Errorf("BulkDelete() = %+v, %+v; want the first deleted and the second missing", got[0], got[1])
	}
}

func TestSavedObjectsService_BulkResolve(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_bulk_resolve", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `[{"type":"index-pattern","id":"probe-b1"},{"type":"index-pattern","id":"probe-nope"}]`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/bulk_resolve.json"))
	})
	got, _, err := client.SavedObjects.BulkResolve(context.Background(), []kibana.SavedObjectRef{
		{Type: "index-pattern", ID: "probe-b1"}, {Type: "index-pattern", ID: "probe-nope"},
	})
	if err != nil {
		t.Fatalf("BulkResolve() error = %v", err)
	}
	if len(got) != 2 || got[0].Outcome != kibana.ResolveExactMatch || got[0].SavedObject.ID != "probe-b1" {
		t.Fatalf("BulkResolve() = %+v, want 2 results, the first an exact match", got)
	}
	// Missing objects have an error, with the outcome exactMatch.
	if !errors.Is(got[1].SavedObject.Error, kibana.ErrNotFound) {
		t.Errorf("BulkResolve()[1].SavedObject.Error = %v, want ErrNotFound", got[1].SavedObject.Error)
	}
}

func TestSavedObjectsService_bulkErrors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/api/saved_objects/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, []byte(`{"statusCode":403,"error":"Forbidden","message":"Unable to bulk_create index-pattern"}`))
	})
	ctx := context.Background()
	obj := []*kibana.SavedObject{{ID: "i", Type: "t", Attributes: json.RawMessage(`{}`)}}
	ref := []kibana.SavedObjectRef{{Type: "t", ID: "i"}}
	if _, _, err := client.SavedObjects.BulkCreate(ctx, obj, nil); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("BulkCreate() error = %v, want ErrForbidden", err)
	}
	if _, _, err := client.SavedObjects.BulkUpdate(ctx, obj); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("BulkUpdate() error = %v, want ErrForbidden", err)
	}
	if _, _, err := client.SavedObjects.BulkGet(ctx, []kibana.BulkGetSavedObjectsRequest{{Type: "t", ID: "i"}}); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("BulkGet() error = %v, want ErrForbidden", err)
	}
	if _, _, err := client.SavedObjects.BulkDelete(ctx, ref, nil); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("BulkDelete() error = %v, want ErrForbidden", err)
	}
	if _, _, err := client.SavedObjects.BulkResolve(ctx, ref); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("BulkResolve() error = %v, want ErrForbidden", err)
	}
}

func TestSavedObjectsService_bulkInvalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	attrs := json.RawMessage(`{}`)
	for name, objs := range map[string][]*kibana.SavedObject{
		"no objects":    nil,
		"nil object":    {nil},
		"no type":       {{ID: "i", Attributes: attrs}},
		"no attributes": {{ID: "i", Type: "t"}},
	} {
		if _, _, err := client.SavedObjects.BulkCreate(ctx, objs, nil); err == nil {
			t.Errorf("BulkCreate(%s) error = nil, want error", name)
		}
		if _, _, err := client.SavedObjects.BulkUpdate(ctx, objs); err == nil {
			t.Errorf("BulkUpdate(%s) error = nil, want error", name)
		}
	}
	if _, _, err := client.SavedObjects.BulkUpdate(ctx, []*kibana.SavedObject{{Type: "t", Attributes: attrs}}); err == nil {
		t.Error("BulkUpdate(no ID) error = nil, want error")
	}
	for name, reqs := range map[string][]kibana.BulkGetSavedObjectsRequest{"no objects": nil, "no ID": {{Type: "t"}}} {
		if _, _, err := client.SavedObjects.BulkGet(ctx, reqs); err == nil {
			t.Errorf("BulkGet(%s) error = nil, want error", name)
		}
	}
	for name, refs := range map[string][]kibana.SavedObjectRef{"no objects": nil, "no type": {{ID: "i"}}} {
		if _, _, err := client.SavedObjects.BulkDelete(ctx, refs, nil); err == nil {
			t.Errorf("BulkDelete(%s) error = nil, want error", name)
		}
		if _, _, err := client.SavedObjects.BulkResolve(ctx, refs); err == nil {
			t.Errorf("BulkResolve(%s) error = nil, want error", name)
		}
	}
}
