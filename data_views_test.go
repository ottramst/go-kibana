package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestDataViewsService_List(t *testing.T) {
	for _, name := range []string{"dataviews/list.json", "dataviews/list_8.19.json"} {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /s/x/api/data_views", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusOK, fixture(t, name))
			})
			got, _, err := client.InSpace("x").DataViews.List(context.Background(), nil)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(got) == 0 || got[0].ID == "" || got[0].Title != "probe-logs-*" || !reflect.DeepEqual(got[0].Namespaces, []string{"probe-dv"}) {
				t.Errorf("List() = %+v, want the data views of the space", got)
			}
		})
	}
}

func TestDataViewsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/data_views/data_view/{viewId}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/data_views/data_view/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/get.json"))
	})
	got, _, err := client.DataViews.Get(context.Background(), "a/b?", nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != "probe-dv-2" || got.Title != "probe-logs-1" || got.Name != "Probe logs" || got.TimeFieldName != "@timestamp" || got.Version == "" {
		t.Errorf("Get() = %+v, want probe-dv-2", got)
	}
	if !reflect.DeepEqual(got.SourceFilters, []kibana.DataViewSourceFilter{{Value: "msg"}}) {
		t.Errorf("SourceFilters = %+v", got.SourceFilters)
	}
	var params map[string]string
	if f := got.FieldFormats["status"]; f.ID != "number" || json.Unmarshal(f.Params, &params) != nil || params["pattern"] != "0,0" {
		t.Errorf("FieldFormats[status] = %+v (%s)", f, f.Params)
	}
	wantRT := kibana.RuntimeField{Type: "keyword", Script: &kibana.RuntimeFieldScript{Source: `emit("x")`}}
	if !reflect.DeepEqual(got.RuntimeFieldMap["rt1"], wantRT) {
		t.Errorf("RuntimeFieldMap[rt1] = %+v", got.RuntimeFieldMap["rt1"])
	}
	if a := got.FieldAttrs["host"]; a.CustomLabel != "Host" || a.Count != 3 {
		t.Errorf("FieldAttrs[host] = %+v", a)
	}
	// Kibana leaves out typeMeta for views that are not rollups.
	if got.TypeMeta != nil {
		t.Errorf("TypeMeta = %+v, want nil", got.TypeMeta)
	}
	host, ok := got.Fields["host"]
	if !ok || host.Type != "string" || !reflect.DeepEqual(host.ESTypes, []string{"keyword"}) || !host.Aggregatable || host.CustomLabel != "Host" || host.Count != 3 {
		t.Errorf("Fields[host] = %+v", host)
	}
	if rt, ok := got.Fields["rt1"]; !ok || rt.RuntimeField == nil || rt.RuntimeField.Type != "keyword" {
		t.Errorf("Fields[rt1] = %+v, want a runtime field", rt)
	}
}

func TestDataViewsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/data_views/data_view", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		// Override goes in the body, not the query.
		testQuery(t, r, "")
		// Fields and Version are not sent; empty fields are left out,
		// because Kibana rejects null.
		testJSONBody(t, r, `{"data_view":{"id":"probe-dv-2","title":"probe-logs-1","name":"Probe logs","timeFieldName":"@timestamp",
			"sourceFilters":[{"value":"msg"}],"fieldFormats":{"status":{"id":"number","params":{"pattern":"0,0"}}},
			"runtimeFieldMap":{"rt1":{"type":"keyword","script":{"source":"emit(\"x\")"}}},
			"fieldAttrs":{"host":{"customLabel":"Host","count":3}},"namespaces":["x"],"allowHidden":true},"override":true}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/get.json"))
	})
	dv := &kibana.DataView{
		ID: "probe-dv-2", Title: "probe-logs-1", Name: "Probe logs", TimeFieldName: "@timestamp",
		SourceFilters:   []kibana.DataViewSourceFilter{{Value: "msg"}},
		FieldFormats:    map[string]kibana.DataViewFieldFormat{"status": {ID: "number", Params: json.RawMessage(`{"pattern":"0,0"}`)}},
		RuntimeFieldMap: map[string]kibana.RuntimeField{"rt1": {Type: "keyword", Script: &kibana.RuntimeFieldScript{Source: `emit("x")`}}},
		FieldAttrs:      map[string]kibana.DataViewFieldAttrs{"host": {CustomLabel: "Host", Count: 3}},
		Namespaces:      []string{"x"},
		Fields:          map[string]kibana.DataViewField{"host": {Name: "host"}},
		AllowHidden:     true,
		Version:         "ignored",
	}
	before := *dv
	got, _, err := client.InSpace("x").DataViews.Create(context.Background(), dv, &kibana.CreateDataViewOptions{Override: true})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID != "probe-dv-2" {
		t.Errorf("Create().ID = %q", got.ID)
	}
	if !reflect.DeepEqual(*dv, before) {
		t.Error("Create() modified the data view")
	}
}

func TestDataViewsService_Create_minimal(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/data_view", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"data_view":{"title":"probe-logs-*"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/create_min.json"))
	})
	got, _, err := client.DataViews.Create(context.Background(), &kibana.DataView{Title: "probe-logs-*"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID == "" || got.Name != "probe-logs-*" {
		t.Errorf("Create() = %+v, want a generated ID", got)
	}
}

func TestDataViewsService_Create_errors(t *testing.T) {
	for _, name := range []string{"dataviews/create_dup_id.json", "dataviews/create_unknown.json"} {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/data_views/data_view", func(w http.ResponseWriter, _ *http.Request) {
				// Kibana answers 400, not 409, for an existing ID.
				writeJSON(t, w, http.StatusBadRequest, fixture(t, name))
			})
			if _, _, err := client.DataViews.Create(context.Background(), &kibana.DataView{ID: "probe-dv-2", Title: "t"}, nil); !errors.Is(err, kibana.ErrBadRequest) {
				t.Errorf("Create() error = %v, want ErrBadRequest", err)
			}
		})
	}
}

func TestDataViewsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/data_view/{viewId}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("viewId"); got != "probe-dv-2" {
			t.Errorf("viewId = %q", got)
		}
		// RefreshFields goes in the body, not the query.
		testQuery(t, r, "")
		// Only the fields Kibana accepts on update.
		testJSONBody(t, r, `{"data_view":{"title":"probe-logs-1","name":"Probe logs","timeFieldName":"@timestamp","allowNoIndex":false,
			"sourceFilters":[{"value":"msg"}],"fieldFormats":{"status":{"id":"number","params":{"pattern":"0,0"}}},
			"runtimeFieldMap":{"rt1":{"type":"keyword","script":{"source":"emit(\"x\")"}}}},"refresh_fields":true}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/get.json"))
	})
	var dv struct {
		DataView *kibana.DataView `json:"data_view"`
	}
	if err := json.Unmarshal(fixture(t, "dataviews/get.json"), &dv); err != nil {
		t.Fatal(err)
	}
	// Read, then write back unchanged.
	if _, _, err := client.DataViews.Update(context.Background(), "probe-dv-2", dv.DataView, &kibana.UpdateDataViewOptions{RefreshFields: true}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestDataViewsService_Update_clear(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/data_view/{viewId}", func(w http.ResponseWriter, r *http.Request) {
		// An empty title is left out (Kibana would save it); "" clears the
		// name and the time field; empty maps clear, nil ones are kept.
		testJSONBody(t, r, `{"data_view":{"name":"","timeFieldName":"","allowNoIndex":true,"runtimeFieldMap":{}}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/get.json"))
	})
	dv := &kibana.DataView{AllowNoIndex: true, RuntimeFieldMap: map[string]kibana.RuntimeField{}}
	if _, _, err := client.DataViews.Update(context.Background(), "probe-dv-2", dv, nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestDataViewsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/data_views/data_view/{viewId}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := client.DataViews.Delete(context.Background(), "probe-dv-2", nil); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestDataViewsService_notFound(t *testing.T) {
	mux, client := setup(t)
	missing := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "dataviews/get_missing.json"))
	}
	mux.HandleFunc("GET /api/data_views/data_view/{viewId}", missing)
	mux.HandleFunc("POST /api/data_views/data_view/{viewId}", missing)
	mux.HandleFunc("DELETE /api/data_views/data_view/{viewId}", missing)
	ctx := context.Background()
	if _, _, err := client.DataViews.Get(ctx, "probe-nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.DataViews.Update(ctx, "probe-nope", &kibana.DataView{Name: "x"}, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
	if _, err := client.DataViews.Delete(ctx, "probe-nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestDataViewsService_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	if _, _, err := client.DataViews.Create(ctx, nil, nil); err == nil {
		t.Error("Create(nil) error = nil, want error")
	}
	if _, _, err := client.DataViews.Create(ctx, &kibana.DataView{Name: "no title"}, nil); err == nil {
		t.Error("Create(no title) error = nil, want error")
	}
	if _, _, err := client.DataViews.Update(ctx, "id", nil, nil); err == nil {
		t.Error("Update(nil) error = nil, want error")
	}
	for _, id := range []string{"", ".", ".."} {
		if _, _, err := client.DataViews.Get(ctx, id, nil); err == nil {
			t.Errorf("Get(%q) error = nil, want error", id)
		}
		if _, _, err := client.DataViews.Update(ctx, id, &kibana.DataView{}, nil); err == nil {
			t.Errorf("Update(%q) error = nil, want error", id)
		}
		if _, err := client.DataViews.Delete(ctx, id, nil); err == nil {
			t.Errorf("Delete(%q) error = nil, want error", id)
		}
	}
}

func TestDataViewsService_GetDefault(t *testing.T) {
	tests := map[string]string{"dataviews/default_get.json": "probe-dv-2", "dataviews/default_get_empty.json": ""}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /s/x/api/data_views/default", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusOK, fixture(t, name))
			})
			got, _, err := client.InSpace("x").DataViews.GetDefault(context.Background(), nil)
			if err != nil || got.ID != want {
				t.Errorf("GetDefault() = %+v, %v; want %q", got, err, want)
			}
		})
	}
}

func TestDataViewsService_SetDefault(t *testing.T) {
	tests := map[string]string{
		"probe-dv-2": `{"data_view_id":"probe-dv-2","force":true}`,
		// "" removes the default; Kibana rejects "" but accepts null.
		"": `{"data_view_id":null,"force":true}`,
	}
	for id, body := range tests {
		t.Run(id, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/data_views/default", func(w http.ResponseWriter, r *http.Request) {
				testHeader(t, r, "kbn-xsrf", "true")
				testJSONBody(t, r, body)
				writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/default_set.json"))
			})
			if _, err := client.DataViews.SetDefault(context.Background(), id, nil); err != nil {
				t.Errorf("SetDefault(%q) error = %v", id, err)
			}
		})
	}
}

func TestDataViewsService_defaultErrors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/api/data_views/default", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, []byte(`{"statusCode":403,"error":"Forbidden","message":"Forbidden"}`))
	})
	ctx := context.Background()
	if _, _, err := client.DataViews.GetDefault(ctx, nil); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("GetDefault() error = %v, want ErrForbidden", err)
	}
	if _, err := client.DataViews.SetDefault(ctx, "x", nil); !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("SetDefault() error = %v, want ErrForbidden", err)
	}
	if _, _, err := client.DataViews.List(ctx, nil); err == nil {
		t.Error("List() error = nil, want an error for the unregistered path")
	}
}
