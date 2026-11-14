package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestSavedObjectsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/saved_objects/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		if got, want := r.URL.EscapedPath(), "/s/x/api/saved_objects/index-pattern/probe-ip"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		testQuery(t, r, "overwrite=true")
		// Read-only fields are not sent.
		testJSONBody(t, r, `{"attributes":{"title":"probe-*","timeFieldName":"@timestamp"},"references":[],"initialNamespaces":["x","y"]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_create_ip.json"))
	})

	obj := &kibana.SavedObject{
		ID: "probe-ip", Type: "index-pattern",
		Attributes: json.RawMessage(`{"title":"probe-*","timeFieldName":"@timestamp"}`),
		References: []kibana.SavedObjectReference{},
		Version:    "ignored", Namespaces: []string{"ignored"}, Managed: true, CreatedAt: time.Now(),
	}
	before := *obj
	got, _, err := client.InSpace("x").SavedObjects.Create(context.Background(), obj, &kibana.CreateSavedObjectOptions{Overwrite: true, InitialNamespaces: []string{"x", "y"}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !reflect.DeepEqual(*obj, before) {
		t.Error("Create() modified the object")
	}

	at := time.Date(2026, 11, 12, 18, 9, 55, 860e6, time.UTC)
	want := &kibana.SavedObject{
		ID: "probe-ip", Type: "index-pattern",
		Attributes:           json.RawMessage(`{"title":"probe-*","timeFieldName":"@timestamp"}`),
		References:           []kibana.SavedObjectReference{},
		Version:              "WzUsMV0=",
		Namespaces:           []string{"default"},
		CreatedAt:            at,
		UpdatedAt:            at,
		CoreMigrationVersion: "8.8.0",
		TypeMigrationVersion: "8.0.0",
		MigrationVersion:     map[string]string{"index-pattern": "8.0.0"},
	}
	var gotAttrs, wantAttrs any
	_ = json.Unmarshal(got.Attributes, &gotAttrs)
	_ = json.Unmarshal(want.Attributes, &wantAttrs)
	if !reflect.DeepEqual(gotAttrs, wantAttrs) {
		t.Errorf("Attributes = %s, want %s", got.Attributes, want.Attributes)
	}
	got.Attributes, want.Attributes = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Create() = %+v, want %+v", got, want)
	}
}

func TestSavedObjectsService_Create_generatedID(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/{type}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("type"); got != "index-pattern" {
			t.Errorf("type = %q, want index-pattern", got)
		}
		testQuery(t, r, "")
		// The migration versions of an object that was read are kept.
		testJSONBody(t, r, `{"attributes":{"title":"noid-*"},"coreMigrationVersion":"8.8.0","typeMigrationVersion":"8.0.0"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_create_ip.json"))
	})
	_, _, err := client.SavedObjects.Create(context.Background(), &kibana.SavedObject{
		Type: "index-pattern", Attributes: json.RawMessage(`{"title":"noid-*"}`),
		CoreMigrationVersion: "8.8.0", TypeMigrationVersion: "8.0.0",
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestSavedObjectsService_Create_errors(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		want    error
	}{
		{"savedobjects/so_create_ip_dup.json", http.StatusConflict, kibana.ErrConflict},
		{"savedobjects/so_create_badtype.json", http.StatusBadRequest, kibana.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/saved_objects/{type}/{id}", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			_, _, err := client.SavedObjects.Create(context.Background(), &kibana.SavedObject{Type: "t", ID: "i", Attributes: json.RawMessage(`{}`)}, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("Create() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSavedObjectsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/saved_objects/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/saved_objects/a%2Fb/c%3Fd"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_get_dash.json"))
	})
	got, _, err := client.SavedObjects.Get(context.Background(), "a/b", "c?d", nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	wantRefs := []kibana.SavedObjectReference{{Name: "panel_0", Type: "visualization", ID: "probe-vis"}}
	if got.ID != "probe-dash" || !reflect.DeepEqual(got.References, wantRefs) || got.Ref() != (kibana.SavedObjectRef{Type: "dashboard", ID: "probe-dash"}) {
		t.Errorf("Get() = %+v, want probe-dash with its references", got)
	}
	var attrs struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(got.Attributes, &attrs); err != nil || attrs.Title != "probe dash" {
		t.Errorf("Attributes = %s, want the title", got.Attributes)
	}
}

func TestSavedObjectsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/saved_objects/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		// The version is sent, but not the migration versions or the
		// references, which stay unchanged when nil.
		testJSONBody(t, r, `{"attributes":{"title":"probe2-*"},"version":"WzUsMV0="}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_update.json"))
	})
	got, _, err := client.SavedObjects.Update(context.Background(), &kibana.SavedObject{
		ID: "probe-ip", Type: "index-pattern", Attributes: json.RawMessage(`{"title":"probe2-*"}`),
		Version: "WzUsMV0=", CoreMigrationVersion: "8.8.0", TypeMigrationVersion: "8.0.0",
	}, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.Version != "WzExLDFd" || got.UpdatedAt.IsZero() {
		t.Errorf("Update() = %+v, want the new version", got)
	}
}

func TestSavedObjectsService_Update_upsertAndReferences(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/saved_objects/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Empty references are sent, to remove them.
		testJSONBody(t, r, `{"attributes":{"title":"u-*"},"references":[],"upsert":{"title":"u-*"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_update_upsert.json"))
	})
	_, _, err := client.SavedObjects.Update(context.Background(), &kibana.SavedObject{
		ID: "probe-upsert", Type: "index-pattern", Attributes: json.RawMessage(`{"title":"u-*"}`),
		References: []kibana.SavedObjectReference{},
	}, &kibana.UpdateSavedObjectOptions{Upsert: json.RawMessage(`{"title":"u-*"}`)})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestSavedObjectsService_Update_errors(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		want    error
	}{
		{"savedobjects/so_update_stale.json", http.StatusConflict, kibana.ErrConflict},
		{"savedobjects/so_get_missing.json", http.StatusNotFound, kibana.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PUT /api/saved_objects/{type}/{id}", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			_, _, err := client.SavedObjects.Update(context.Background(), &kibana.SavedObject{Type: "t", ID: "i", Attributes: json.RawMessage(`{}`)}, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("Update() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSavedObjectsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/saved_objects/{type}/{id}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testQuery(t, r, "force=true")
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/so_delete.json"))
	})
	if _, err := client.SavedObjects.Delete(context.Background(), "index-pattern", "probe-ip", &kibana.DeleteSavedObjectOptions{Force: true}); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestSavedObjectsService_notFound(t *testing.T) {
	mux, client := setup(t)
	missing := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "savedobjects/so_get_missing.json"))
	}
	mux.HandleFunc("GET /api/saved_objects/{type}/{id}", missing)
	mux.HandleFunc("DELETE /api/saved_objects/{type}/{id}", missing)
	ctx := context.Background()
	if _, _, err := client.SavedObjects.Get(ctx, "index-pattern", "probe-nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if _, err := client.SavedObjects.Delete(ctx, "index-pattern", "probe-nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestSavedObjectsService_crudInvalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	attrs := json.RawMessage(`{}`)
	for name, obj := range map[string]*kibana.SavedObject{
		"nil object":    nil,
		"no type":       {ID: "i", Attributes: attrs},
		"no attributes": {Type: "t", ID: "i"},
		"dot ID":        {Type: "t", ID: "..", Attributes: attrs},
	} {
		if _, _, err := client.SavedObjects.Create(ctx, obj, nil); err == nil {
			t.Errorf("Create(%s) error = nil, want error", name)
		}
		if _, _, err := client.SavedObjects.Update(ctx, obj, nil); err == nil {
			t.Errorf("Update(%s) error = nil, want error", name)
		}
	}
	if _, _, err := client.SavedObjects.Update(ctx, &kibana.SavedObject{Type: "t", Attributes: attrs}, nil); err == nil {
		t.Error("Update(no ID) error = nil, want error")
	}
	for _, args := range [][2]string{{"", "i"}, {"t", ""}, {".", "i"}, {"t", ".."}} {
		if _, _, err := client.SavedObjects.Get(ctx, args[0], args[1], nil); err == nil {
			t.Errorf("Get(%q, %q) error = nil, want error", args[0], args[1])
		}
		if _, err := client.SavedObjects.Delete(ctx, args[0], args[1], nil); err == nil {
			t.Errorf("Delete(%q, %q) error = nil, want error", args[0], args[1])
		}
	}
}
