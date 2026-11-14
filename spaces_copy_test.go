package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestSpacesService_CopySavedObjects(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/src/api/spaces/_copy_saved_objects", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testJSONBody(t, r, `{"spaces":["probe-dst"],"objects":[{"type":"index-pattern","id":"probe-ip1"}],"includeReferences":true,"createNewCopies":false}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/copy.json"))
	})

	got, _, err := client.InSpace("src").Spaces.CopySavedObjects(context.Background(), &kibana.CopySavedObjectsRequest{
		Spaces:            []string{"probe-dst"},
		Objects:           []kibana.SavedObjectRef{{Type: "index-pattern", ID: "probe-ip1"}},
		IncludeReferences: true,
		CreateNewCopies:   new(false),
	}, nil)
	if err != nil {
		t.Fatalf("CopySavedObjects() error = %v", err)
	}
	want := map[string]*kibana.ImportSavedObjectsResult{"probe-dst": {
		Success:      true,
		SuccessCount: 1,
		SuccessResults: []kibana.ImportSuccess{{
			SavedObjectRef: kibana.SavedObjectRef{Type: "index-pattern", ID: "probe-ip1"},
			Meta:           kibana.SavedObjectMeta{Title: "logs-*", Icon: "indexPatternApp"},
			DestinationID:  "9b2ff723-07fa-4e02-a0e6-c1174fb61a76",
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CopySavedObjects() = %+v, want %+v", got["probe-dst"], want["probe-dst"])
	}
}

func TestSpacesService_CopySavedObjects_defaultCreateNewCopies(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/spaces/_copy_saved_objects", func(w http.ResponseWriter, r *http.Request) {
		// Left out, so that Kibana applies its default (true).
		testJSONBody(t, r, `{"spaces":["b"],"objects":[{"type":"dashboard","id":"d"}]}`)
		writeJSON(t, w, http.StatusOK, []byte(`{"b":{"success":true,"successCount":1}}`))
	})
	_, _, err := client.Spaces.CopySavedObjects(context.Background(), &kibana.CopySavedObjectsRequest{
		Spaces:  []string{"b"},
		Objects: []kibana.SavedObjectRef{{Type: "dashboard", ID: "d"}},
	}, nil)
	if err != nil {
		t.Fatalf("CopySavedObjects() error = %v", err)
	}
}

func TestSpacesService_CopySavedObjects_errors(t *testing.T) {
	tests := []struct {
		fixture string
		want    kibana.ImportError
	}{
		{
			fixture: "spaces/copy_conflict.json",
			want: kibana.ImportError{
				SavedObjectRef: kibana.SavedObjectRef{Type: "index-pattern", ID: "probe-ip1"},
				Meta:           kibana.SavedObjectMeta{Title: "logs-*", Icon: "indexPatternApp"},
				Detail:         &kibana.ImportErrorDetail{Type: kibana.ImportErrorTypeConflict, DestinationID: "9b2ff723-07fa-4e02-a0e6-c1174fb61a76"},
			},
		},
		{
			fixture: "spaces/copy_missing_references.json",
			want: kibana.ImportError{
				SavedObjectRef: kibana.SavedObjectRef{Type: "visualization", ID: "probe-vis1"},
				Meta:           kibana.SavedObjectMeta{Title: "probe vis", Icon: "visualizeApp"},
				Detail: &kibana.ImportErrorDetail{
					Type:       kibana.ImportErrorTypeMissingReferences,
					References: []kibana.SavedObjectRef{{Type: "index-pattern", ID: "probe-missing-ip"}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/spaces/_copy_saved_objects", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusOK, fixture(t, tt.fixture))
			})
			got, _, err := client.Spaces.CopySavedObjects(context.Background(), &kibana.CopySavedObjectsRequest{
				Spaces:  []string{"probe-dst"},
				Objects: []kibana.SavedObjectRef{{Type: tt.want.Type, ID: tt.want.ID}},
			}, nil)
			if err != nil {
				t.Fatalf("CopySavedObjects() error = %v", err)
			}
			res := got["probe-dst"]
			if res == nil || res.Success || len(res.Errors) != 1 {
				t.Fatalf("result = %+v, want one error", res)
			}
			if !reflect.DeepEqual(res.Errors[0], tt.want) {
				t.Errorf("error = %+v (detail %+v), want %+v (detail %+v)", res.Errors[0], res.Errors[0].Detail, tt.want, tt.want.Detail)
			}
		})
	}
}

func TestImportError_spaceFailure(t *testing.T) {
	// Kibana reports a failure of the whole copy into a space as a Boom
	// error payload, whose "error" field is the HTTP status text.
	var got kibana.ImportError
	if err := json.Unmarshal([]byte(`{"statusCode":403,"error":"Forbidden","message":"Unable to bulk_create index-pattern"}`), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := kibana.ImportError{StatusCode: 403, StatusText: "Forbidden", Message: "Unable to bulk_create index-pattern"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImportError = %+v, want %+v", got, want)
	}
	if err := json.Unmarshal([]byte(`{"error":`), &got); err == nil {
		t.Error("Unmarshal(invalid) error = nil, want error")
	}
}

func TestImportError_unknownError(t *testing.T) {
	// An object error of type "unknown", in the shape of Kibana's
	// SavedObjectsImportUnknownError (not reproducible on demand).
	var got kibana.ImportError
	data := `{"type":"dashboard","id":"d","meta":{"title":"D"},"overwrite":true,"error":{"type":"unknown","statusCode":500,"message":"boom"}}`
	if err := json.Unmarshal([]byte(data), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := kibana.ImportError{
		SavedObjectRef: kibana.SavedObjectRef{Type: "dashboard", ID: "d"}, Meta: kibana.SavedObjectMeta{Title: "D"}, Overwrite: true,
		Detail: &kibana.ImportErrorDetail{Type: kibana.ImportErrorTypeUnknown, StatusCode: 500, Message: "boom"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ImportError = %+v (detail %+v), want %+v (detail %+v)", got, got.Detail, want, want.Detail)
	}
}

func TestSpacesService_CopySavedObjects_missingObject(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/spaces/_copy_saved_objects", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "spaces/copy_missing_object.json"))
	})
	_, _, err := client.Spaces.CopySavedObjects(context.Background(), &kibana.CopySavedObjectsRequest{
		Spaces:          []string{"probe-dst"},
		Objects:         []kibana.SavedObjectRef{{Type: "index-pattern", ID: "probe-nope"}},
		CreateNewCopies: new(false),
	}, nil)
	if !errors.Is(err, kibana.ErrNotFound) {
		t.Fatalf("CopySavedObjects() error = %v, want ErrNotFound", err)
	}
	var errResp *kibana.ErrorResponse
	if errors.As(err, &errResp) && len(errResp.Attributes) == 0 {
		t.Error("ErrorResponse.Attributes is empty, want the missing objects")
	}
}

func TestSpacesService_ResolveCopySavedObjectsErrors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/src/api/spaces/_resolve_copy_saved_objects_errors", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{
			"objects":[{"type":"index-pattern","id":"probe-ip1"}],
			"retries":{"probe-dst":[{"type":"index-pattern","id":"probe-ip1","overwrite":true,"destinationId":"9b2ff723-07fa-4e02-a0e6-c1174fb61a76"}]},
			"createNewCopies":false}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/resolve.json"))
	})

	got, _, err := client.InSpace("src").Spaces.ResolveCopySavedObjectsErrors(context.Background(), &kibana.ResolveCopySavedObjectsErrorsRequest{
		Objects: []kibana.SavedObjectRef{{Type: "index-pattern", ID: "probe-ip1"}},
		Retries: map[string][]kibana.CopySavedObjectsRetry{"probe-dst": {{
			Type: "index-pattern", ID: "probe-ip1", Overwrite: true, DestinationID: "9b2ff723-07fa-4e02-a0e6-c1174fb61a76",
		}}},
		CreateNewCopies: new(false),
	}, nil)
	if err != nil {
		t.Fatalf("ResolveCopySavedObjectsErrors() error = %v", err)
	}
	res := got["probe-dst"]
	if res == nil || !res.Success || len(res.SuccessResults) != 1 || !res.SuccessResults[0].Overwrite {
		t.Errorf("result = %+v, want one overwritten object", res)
	}
}

func TestSpacesService_copyInvalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	obj := []kibana.SavedObjectRef{{Type: "dashboard", ID: "d"}}
	copyCases := map[string]*kibana.CopySavedObjectsRequest{
		"nil request":                           nil,
		"no spaces":                             {Objects: obj},
		"empty space ID":                        {Spaces: []string{""}, Objects: obj},
		"no objects":                            {Spaces: []string{"b"}},
		"object without ID":                     {Spaces: []string{"b"}, Objects: []kibana.SavedObjectRef{{Type: "dashboard"}}},
		"overwrite with default new copies":     {Spaces: []string{"b"}, Objects: obj, Overwrite: true},
		"overwrite with new copies":             {Spaces: []string{"b"}, Objects: obj, Overwrite: true, CreateNewCopies: new(true)},
		"compatibility with default new copies": {Spaces: []string{"b"}, Objects: obj, CompatibilityMode: true},
	}
	for name, req := range copyCases {
		if _, _, err := client.Spaces.CopySavedObjects(ctx, req, nil); err == nil {
			t.Errorf("CopySavedObjects(%s): error = nil, want error", name)
		}
	}
	retry := map[string][]kibana.CopySavedObjectsRetry{"b": {{Type: "dashboard", ID: "d"}}}
	resolveCases := map[string]*kibana.ResolveCopySavedObjectsErrorsRequest{
		"nil request":                       nil,
		"no objects":                        {Retries: retry},
		"no retries":                        {Objects: obj},
		"empty space ID":                    {Objects: obj, Retries: map[string][]kibana.CopySavedObjectsRetry{"": {{Type: "dashboard", ID: "d"}}}},
		"retry without type":                {Objects: obj, Retries: map[string][]kibana.CopySavedObjectsRetry{"b": {{ID: "d"}}}},
		"overwrite with default new copies": {Objects: obj, Retries: map[string][]kibana.CopySavedObjectsRetry{"b": {{Type: "dashboard", ID: "d", Overwrite: true}}}},
	}
	for name, req := range resolveCases {
		if _, _, err := client.Spaces.ResolveCopySavedObjectsErrors(ctx, req, nil); err == nil {
			t.Errorf("ResolveCopySavedObjectsErrors(%s): error = nil, want error", name)
		}
	}
}

func TestImportError_Error(t *testing.T) {
	tests := []struct {
		err  *kibana.ImportError
		want string
	}{
		{&kibana.ImportError{
			SavedObjectRef: kibana.SavedObjectRef{Type: "dashboard", ID: "d"},
			Detail:         &kibana.ImportErrorDetail{Type: kibana.ImportErrorTypeUnknown, Message: "boom"},
		}, "kibana: dashboard d: unknown: boom"},
		{&kibana.ImportError{
			SavedObjectRef: kibana.SavedObjectRef{Type: "index-pattern", ID: "i"},
			Detail:         &kibana.ImportErrorDetail{Type: kibana.ImportErrorTypeConflict},
		}, "kibana: index-pattern i: conflict"},
		{&kibana.ImportError{StatusCode: 403, StatusText: "Forbidden", Message: "Unable to bulk_create"}, "kibana: 403 Unable to bulk_create"},
		{&kibana.ImportError{StatusCode: 403, StatusText: "Forbidden"}, "kibana: 403 Forbidden"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
	var target *kibana.ImportError
	if !errors.As(fmt.Errorf("copy: %w", &kibana.ImportError{StatusCode: 500}), &target) {
		t.Error("errors.As(*ImportError) = false, want true")
	}
}

func TestImportResult_marshal(t *testing.T) {
	// The embedded SavedObjectRef is flattened, and left out of a failure
	// of the whole space.
	data, err := json.Marshal(kibana.ImportSavedObjectsResult{
		SuccessResults: []kibana.ImportSuccess{{SavedObjectRef: kibana.SavedObjectRef{Type: "dashboard", ID: "d"}}},
		Errors:         []kibana.ImportError{{StatusCode: 403, StatusText: "Forbidden", Message: "m"}},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got struct {
		SuccessResults []map[string]any `json:"successResults"`
		Errors         []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if s := got.SuccessResults[0]; s["type"] != "dashboard" || s["id"] != "d" {
		t.Errorf("successResults[0] = %v, want type and id", s)
	}
	if e := got.Errors[0]; e["type"] != nil || e["id"] != nil {
		t.Errorf("errors[0] = %v, want no type or id", e)
	}
}
