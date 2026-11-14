package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestDataViewsService_UpdateFieldsMetadata(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/data_views/data_view/{viewId}/fields", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		// Unset fields are left out; zero values remove the value (null).
		testJSONBody(t, r, `{"fields":{
			"host":{"customLabel":"H","customDescription":"desc","count":5},
			"status":{"format":{"id":"bytes"}},
			"msg":{"customLabel":null,"customDescription":null,"count":null,"format":null}}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/fields_metadata.json"))
	})
	got, _, err := client.InSpace("x").DataViews.UpdateFieldsMetadata(context.Background(), "probe-dv-2", &kibana.UpdateFieldsMetadataRequest{Fields: map[string]kibana.DataViewFieldMetadata{
		"host":   {CustomLabel: new("H"), CustomDescription: new("desc"), Count: new(5)},
		"status": {Format: &kibana.DataViewFieldFormat{ID: "bytes"}},
		"msg":    {CustomLabel: new(""), CustomDescription: new(""), Count: new(0), Format: &kibana.DataViewFieldFormat{}},
	}}, nil)
	if err != nil {
		t.Fatalf("UpdateFieldsMetadata() error = %v", err)
	}
	if a := got.FieldAttrs["host"]; a.CustomLabel != "H" || a.CustomDescription != "desc" || a.Count != 5 || got.FieldFormats["status"].ID != "bytes" {
		t.Errorf("UpdateFieldsMetadata() = %+v, want the new metadata", got)
	}
}

func TestDataViewsService_UpdateFieldsMetadata_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/data_view/{viewId}/fields", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "dataviews/fm_long.json"))
	})
	ctx := context.Background()
	if _, _, err := client.DataViews.UpdateFieldsMetadata(ctx, "v", &kibana.UpdateFieldsMetadataRequest{Fields: map[string]kibana.DataViewFieldMetadata{"host": {CustomLabel: new("x")}}}, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("UpdateFieldsMetadata() error = %v, want ErrBadRequest", err)
	}
	for name, fields := range map[string]map[string]kibana.DataViewFieldMetadata{
		"no fields":        nil,
		"empty name":       {"": {CustomLabel: new("x")}},
		"long description": {"host": {CustomDescription: new(strings.Repeat("é", 301))}},
	} {
		if _, _, err := client.DataViews.UpdateFieldsMetadata(ctx, "v", &kibana.UpdateFieldsMetadataRequest{Fields: fields}, nil); err == nil {
			t.Errorf("UpdateFieldsMetadata(%s) error = nil, want error", name)
		}
	}
	if _, _, err := client.DataViews.UpdateFieldsMetadata(ctx, "v", nil, nil); err == nil {
		t.Error("UpdateFieldsMetadata(nil request) error = nil, want error")
	}
	if _, _, err := client.DataViews.UpdateFieldsMetadata(ctx, "", &kibana.UpdateFieldsMetadataRequest{Fields: map[string]kibana.DataViewFieldMetadata{"host": {}}}, nil); err == nil {
		t.Error("UpdateFieldsMetadata(no view) error = nil, want error")
	}
	// 300 characters, not bytes, are allowed.
	mux2, client2 := setup(t)
	mux2.HandleFunc("POST /api/data_views/data_view/{viewId}/fields", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/fields_metadata.json"))
	})
	if _, _, err := client2.DataViews.UpdateFieldsMetadata(ctx, "v", &kibana.UpdateFieldsMetadataRequest{Fields: map[string]kibana.DataViewFieldMetadata{"host": {CustomDescription: new(strings.Repeat("é", 300))}}}, nil); err != nil {
		t.Errorf("UpdateFieldsMetadata(300 characters) error = %v", err)
	}
}

func TestDataViewsService_CreateRuntimeField(t *testing.T) {
	tests := []struct {
		method string
		call   func(*kibana.DataViewsService, *kibana.RuntimeField) (*kibana.RuntimeFieldResult, *kibana.Response, error)
	}{
		{"POST", func(s *kibana.DataViewsService, rf *kibana.RuntimeField) (*kibana.RuntimeFieldResult, *kibana.Response, error) {
			return s.CreateRuntimeField(context.Background(), "probe-dv-2", "rt2", rf, nil)
		}},
		{"PUT", func(s *kibana.DataViewsService, rf *kibana.RuntimeField) (*kibana.RuntimeFieldResult, *kibana.Response, error) {
			return s.CreateOrUpdateRuntimeField(context.Background(), "probe-dv-2", "rt2", rf, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc(tt.method+" /api/data_views/data_view/{viewId}/runtime_field", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, `{"name":"rt2","runtimeField":{"type":"long","script":{"source":"emit(1)"},"customLabel":"RT","popularity":2}}`)
				writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/rf_create.json"))
			})
			got, _, err := tt.call(client.DataViews, &kibana.RuntimeField{
				Type: "long", Script: &kibana.RuntimeFieldScript{Source: "emit(1)"}, CustomLabel: "RT", Popularity: 2,
			})
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if len(got.Fields) != 1 || got.Fields[0].Name != "rt2" || got.Fields[0].RuntimeField == nil || got.DataView == nil || got.DataView.ID != "probe-dv-2" {
				t.Errorf("result = %+v, want rt2 and the data view", got)
			}
		})
	}
}

func TestDataViewsService_CreateRuntimeField_composite(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/data_view/{viewId}/runtime_field", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"comp","runtimeField":{"type":"composite","script":{"source":"emit(\"a\", \"x\"); emit(\"b\", 1)"},"fields":{"a":{"type":"keyword","customLabel":"A"},"b":{"type":"long"}}}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/rf_create_composite.json"))
	})
	got, _, err := client.DataViews.CreateRuntimeField(context.Background(), "probe-dv-2", "comp", &kibana.RuntimeField{
		Type:   "composite",
		Script: &kibana.RuntimeFieldScript{Source: `emit("a", "x"); emit("b", 1)`},
		Fields: map[string]kibana.RuntimeSubField{"a": {Type: "keyword", CustomLabel: "A"}, "b": {Type: "long"}},
	}, nil)
	if err != nil {
		t.Fatalf("CreateRuntimeField() error = %v", err)
	}
	// A field for each subfield.
	var names []string
	for _, f := range got.Fields {
		names = append(names, f.Name)
	}
	if !reflect.DeepEqual(names, []string{"comp.a", "comp.b"}) {
		t.Errorf("fields = %v, want comp.a and comp.b", names)
	}
}

func TestDataViewsService_GetRuntimeField(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/data_views/data_view/{viewId}/runtime_field/{fieldName}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/data_views/data_view/v%2F1/runtime_field/f%3F1"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/rf_get_extras.json"))
	})
	got, _, err := client.DataViews.GetRuntimeField(context.Background(), "v/1", "f?1", nil)
	if err != nil {
		t.Fatalf("GetRuntimeField() error = %v", err)
	}
	if f := got.Fields[0]; f.Name != "rt4" || f.CustomLabel != "RT4" || f.CustomDescription != "d4" || f.Count != 2 {
		t.Errorf("GetRuntimeField() = %+v, want rt4 with its metadata", f)
	}
}

func TestDataViewsService_UpdateRuntimeField(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/data_view/{viewId}/runtime_field/{fieldName}", func(w http.ResponseWriter, r *http.Request) {
		// A partial update: the script is kept.
		testJSONBody(t, r, `{"runtimeField":{"type":"long"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "dataviews/rf_update_type.json"))
	})
	got, _, err := client.DataViews.UpdateRuntimeField(context.Background(), "probe-dv-2", "rt2", &kibana.RuntimeField{Type: "long"}, nil)
	if err != nil {
		t.Fatalf("UpdateRuntimeField() error = %v", err)
	}
	if got.Fields[0].RuntimeField.Type != "long" {
		t.Errorf("UpdateRuntimeField() = %+v, want type long", got.Fields[0].RuntimeField)
	}
}

func TestDataViewsService_DeleteRuntimeField(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/data_views/data_view/{viewId}/runtime_field/{fieldName}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		// Kibana answers 200 without a body.
		w.WriteHeader(http.StatusOK)
	})
	if _, err := client.DataViews.DeleteRuntimeField(context.Background(), "probe-dv-2", "rt3", nil); err != nil {
		t.Errorf("DeleteRuntimeField() error = %v", err)
	}
}

func TestDataViewsService_runtimeFieldErrors(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		want    error
	}{
		{"dataviews/rf_create_dup.json", http.StatusBadRequest, kibana.ErrBadRequest},
		{"dataviews/rf_get_missing.json", http.StatusNotFound, kibana.ErrNotFound},
		// Kibana fails to return composite runtime fields.
		{"dataviews/rf_get_composite.json", http.StatusBadRequest, kibana.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("/api/data_views/data_view/v/runtime_field/", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			mux.HandleFunc("/api/data_views/data_view/v/runtime_field", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			ctx := context.Background()
			rf := &kibana.RuntimeField{Type: "keyword"}
			if _, _, err := client.DataViews.CreateRuntimeField(ctx, "v", "f", rf, nil); !errors.Is(err, tt.want) {
				t.Errorf("CreateRuntimeField() error = %v, want %v", err, tt.want)
			}
			if _, _, err := client.DataViews.GetRuntimeField(ctx, "v", "f", nil); !errors.Is(err, tt.want) {
				t.Errorf("GetRuntimeField() error = %v, want %v", err, tt.want)
			}
			if _, _, err := client.DataViews.UpdateRuntimeField(ctx, "v", "f", rf, nil); !errors.Is(err, tt.want) {
				t.Errorf("UpdateRuntimeField() error = %v, want %v", err, tt.want)
			}
			if _, err := client.DataViews.DeleteRuntimeField(ctx, "v", "f", nil); !errors.Is(err, tt.want) {
				t.Errorf("DeleteRuntimeField() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDataViewsService_runtimeFieldInvalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	rf := &kibana.RuntimeField{Type: "keyword"}
	for _, args := range [][2]string{{"", "f"}, {"v", ""}, {"..", "f"}, {"v", "."}} {
		if _, _, err := client.DataViews.CreateRuntimeField(ctx, args[0], args[1], rf, nil); err == nil {
			t.Errorf("CreateRuntimeField(%q, %q) error = nil, want error", args[0], args[1])
		}
		if _, _, err := client.DataViews.GetRuntimeField(ctx, args[0], args[1], nil); err == nil {
			t.Errorf("GetRuntimeField(%q, %q) error = nil, want error", args[0], args[1])
		}
		if _, _, err := client.DataViews.UpdateRuntimeField(ctx, args[0], args[1], rf, nil); err == nil {
			t.Errorf("UpdateRuntimeField(%q, %q) error = nil, want error", args[0], args[1])
		}
		if _, err := client.DataViews.DeleteRuntimeField(ctx, args[0], args[1], nil); err == nil {
			t.Errorf("DeleteRuntimeField(%q, %q) error = nil, want error", args[0], args[1])
		}
	}
	for name, rf := range map[string]*kibana.RuntimeField{"nil": nil, "no type": {Script: &kibana.RuntimeFieldScript{Source: "emit(1)"}}} {
		if _, _, err := client.DataViews.CreateRuntimeField(ctx, "v", "f", rf, nil); err == nil {
			t.Errorf("CreateRuntimeField(%s) error = nil, want error", name)
		}
		if _, _, err := client.DataViews.CreateOrUpdateRuntimeField(ctx, "v", "f", rf, nil); err == nil {
			t.Errorf("CreateOrUpdateRuntimeField(%s) error = nil, want error", name)
		}
	}
	if _, _, err := client.DataViews.UpdateRuntimeField(ctx, "v", "f", nil, nil); err == nil {
		t.Error("UpdateRuntimeField(nil) error = nil, want error")
	}
}

func TestDataViewsService_SwapReferences(t *testing.T) {
	tests := []struct {
		path, fixture, body string
		req                 *kibana.SwapReferencesRequest
		preview             bool
		want                *kibana.SwapReferencesResult
	}{
		{
			path: "/api/data_views/swap_references/_preview", fixture: "dataviews/swap_preview.json", preview: true,
			body: `{"fromId":"a","toId":"b"}`,
			req:  &kibana.SwapReferencesRequest{FromID: "a", ToID: "b"},
			want: &kibana.SwapReferencesResult{Result: []kibana.SavedObjectRef{{Type: "visualization", ID: "probe-vis1"}, {Type: "visualization", ID: "probe-vis2"}}},
		},
		{
			path: "/api/data_views/swap_references", fixture: "dataviews/swap_forid_string.json",
			body: `{"fromId":"a","toId":"b","forId":["probe-vis1"]}`,
			req:  &kibana.SwapReferencesRequest{FromID: "a", ToID: "b", ForID: []string{"probe-vis1"}},
			want: &kibana.SwapReferencesResult{Result: []kibana.SavedObjectRef{{Type: "visualization", ID: "probe-vis1"}}},
		},
		{
			path: "/api/data_views/swap_references", fixture: "dataviews/swap_delete_remaining_refs.json",
			body: `{"fromId":"a","fromType":"index-pattern","toId":"b","forId":["probe-vis3"],"forType":"visualization","delete":true}`,
			req:  &kibana.SwapReferencesRequest{FromID: "a", FromType: "index-pattern", ToID: "b", ForID: []string{"probe-vis3"}, ForType: "visualization", Delete: true},
			want: &kibana.SwapReferencesResult{
				Result:       []kibana.SavedObjectRef{{Type: "visualization", ID: "probe-vis3"}},
				DeleteStatus: &kibana.SwapReferencesDeleteStatus{RemainingRefs: 1},
			},
		},
		{
			path: "/api/data_views/swap_references", fixture: "dataviews/swap_delete_remaining.json",
			body: `{"fromId":"a","toId":"b","delete":true}`,
			req:  &kibana.SwapReferencesRequest{FromID: "a", ToID: "b", Delete: true},
			want: &kibana.SwapReferencesResult{
				Result:       []kibana.SavedObjectRef{{Type: "visualization", ID: "probe-vis1"}},
				DeleteStatus: &kibana.SwapReferencesDeleteStatus{DeletePerformed: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /s/x"+tt.path, func(w http.ResponseWriter, r *http.Request) {
				testHeader(t, r, "kbn-xsrf", "true")
				testJSONBody(t, r, tt.body)
				writeJSON(t, w, http.StatusOK, fixture(t, tt.fixture))
			})
			svc := client.InSpace("x").DataViews
			var (
				got *kibana.SwapReferencesResult
				err error
			)
			if tt.preview {
				got, _, err = svc.PreviewSwapReferences(context.Background(), tt.req, nil)
			} else {
				got, _, err = svc.SwapReferences(context.Background(), tt.req, nil)
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("result = %+v (%+v), want %+v (%+v)", got, got.DeleteStatus, tt.want, tt.want.DeleteStatus)
			}
		})
	}
}

func TestDataViewsService_SwapReferences_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/data_views/swap_references", func(w http.ResponseWriter, _ *http.Request) {
		// The target must exist.
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "dataviews/swap_missing_target.json"))
	})
	ctx := context.Background()
	if _, _, err := client.DataViews.SwapReferences(ctx, &kibana.SwapReferencesRequest{FromID: "a", ToID: "gone"}, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("SwapReferences() error = %v, want ErrBadRequest", err)
	}
	for name, req := range map[string]*kibana.SwapReferencesRequest{
		"nil":          nil,
		"no from":      {ToID: "b"},
		"no to":        {FromID: "a"},
		"empty for ID": {FromID: "a", ToID: "b", ForID: []string{""}},
		// Left out, it would mean all saved objects.
		"empty, non-nil for IDs": {FromID: "a", ToID: "b", ForID: []string{}, Delete: true},
	} {
		if _, _, err := client.DataViews.SwapReferences(ctx, req, nil); err == nil {
			t.Errorf("SwapReferences(%s) error = nil, want error", name)
		}
		if _, _, err := client.DataViews.PreviewSwapReferences(ctx, req, nil); err == nil {
			t.Errorf("PreviewSwapReferences(%s) error = nil, want error", name)
		}
	}
}
