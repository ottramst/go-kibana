package kibana_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

// readForm parses the multipart form of an import request and returns the
// file name and content of its "file" part, and its other fields.
func readForm(t *testing.T, r *http.Request) (fileName, file string, fields map[string]string) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
	}
	fields = map[string]string{}
	mr := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return fileName, file, fields
		}
		if err != nil {
			t.Fatalf("reading form: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading part: %v", err)
		}
		if part.FormName() == "file" {
			fileName, file = part.FileName(), string(data)
		} else {
			fields[part.FormName()] = string(data)
		}
	}
}

func TestSavedObjectsService_Import(t *testing.T) {
	export := fixture(t, "savedobjects/export.ndjson")
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/saved_objects/_import", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testQuery(t, r, "")
		name, file, fields := readForm(t, r)
		if name != "import.ndjson" || file != string(export) || len(fields) != 0 {
			t.Errorf("form = %q (%d bytes), %v; want import.ndjson with the export", name, len(file), fields)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/import_ok.json"))
	})

	got, _, err := client.InSpace("x").SavedObjects.Import(context.Background(), bytes.NewReader(export), nil)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !got.Success || got.SuccessCount != 3 || len(got.SuccessResults) != 3 || got.Errors != nil {
		t.Fatalf("Import() = %+v, want 3 imported objects", got)
	}
	want := kibana.ImportSuccess{
		SavedObjectRef: kibana.SavedObjectRef{Type: "index-pattern", ID: "probe-ip"},
		Meta:           kibana.SavedObjectMeta{Title: "probe2-*", Icon: "indexPatternApp"},
		DestinationID:  "e9fa6a65-eab7-499b-b2cb-4b9fcbcb7688",
	}
	if got.SuccessResults[0] != want {
		t.Errorf("SuccessResults[0] = %+v, want %+v", got.SuccessResults[0], want)
	}
}

func TestSavedObjectsService_Import_options(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_import", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "compatibilityMode=true&overwrite=true")
		readForm(t, r)
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/import_overwrite.json"))
	})
	got, _, err := client.SavedObjects.Import(context.Background(), strings.NewReader("{}\n"), &kibana.ImportSavedObjectsOptions{Overwrite: true, CompatibilityMode: true})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !got.SuccessResults[0].Overwrite {
		t.Errorf("SuccessResults[0].Overwrite = false, want true")
	}
}

func TestSavedObjectsService_Import_conflicts(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_import", func(w http.ResponseWriter, r *http.Request) {
		readForm(t, r)
		// Kibana answers 200 when objects fail.
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/import_conflict.json"))
	})
	got, _, err := client.SavedObjects.Import(context.Background(), strings.NewReader("{}\n"), nil)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got.Success || len(got.Errors) != 3 {
		t.Fatalf("Import() = %+v, want 3 errors", got)
	}
	want := &kibana.ImportErrorDetail{Type: kibana.ImportErrorTypeConflict, DestinationID: "e9fa6a65-eab7-499b-b2cb-4b9fcbcb7688"}
	if e := got.Errors[0]; e.ID != "probe-ip" || !reflect.DeepEqual(e.Detail, want) {
		t.Errorf("Errors[0] = %+v (%+v), want a conflict with %+v", e, e.Detail, want)
	}
}

func TestSavedObjectsService_Import_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_import", func(w http.ResponseWriter, _ *http.Request) {
		// Answer before reading the form, as Kibana does for bad requests.
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "savedobjects/import_badext.json"))
	})
	// A large file, so that the form does not fit in the pipe and
	// connection buffers.
	big := strings.NewReader(strings.Repeat("{}\n", 1<<20))
	_, _, err := client.SavedObjects.Import(context.Background(), big, nil)
	if !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("Import() error = %v, want ErrBadRequest", err)
	}
}

func TestSavedObjectsService_Import_canceled(t *testing.T) {
	_, client := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Returns, without leaving the form writer blocked.
	if _, _, err := client.SavedObjects.Import(ctx, strings.NewReader(strings.Repeat("{}\n", 1<<20)), nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Import() error = %v, want context.Canceled", err)
	}
}

func TestSavedObjectsService_Import_blockedReaderCanceled(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_import", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, []byte(`{"statusCode":401,"error":"Unauthorized","message":"Unauthorized"}`))
	})
	// A reader that blocks until the test ends.
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := client.SavedObjects.Import(ctx, pr, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("Import() error = nil, want an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Import() did not return after the context was done")
	}
}

func TestExportDetails(t *testing.T) {
	lines := bytes.Split(bytes.TrimSpace(fixture(t, "savedobjects/export.ndjson")), []byte("\n"))
	var got kibana.ExportDetails
	if err := json.Unmarshal(lines[len(lines)-1], &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := kibana.ExportDetails{ExportedCount: 3, MissingReferences: []kibana.SavedObjectRef{}, ExcludedObjects: []kibana.ExportExcludedObject{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExportDetails = %+v, want %+v", got, want)
	}
}

func TestSavedObjectsService_ResolveImportErrors(t *testing.T) {
	export := fixture(t, "savedobjects/export.ndjson")
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_resolve_import_errors", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "createNewCopies=true")
		name, file, fields := readForm(t, r)
		if name != "import.ndjson" || file != string(export) {
			t.Errorf("file = %q (%d bytes), want import.ndjson with the export", name, len(file))
		}
		want := `[{"type":"dashboard","id":"d","overwrite":true,"destinationId":"d2","replaceReferences":[{"type":"index-pattern","from":"a","to":"b"}]},{"type":"index-pattern","id":"i","createNewCopy":true,"ignoreMissingReferences":true}]`
		if fields["retries"] != want {
			t.Errorf("retries = %s, want %s", fields["retries"], want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "savedobjects/rie_overwrite.json"))
	})
	got, _, err := client.SavedObjects.ResolveImportErrors(context.Background(), bytes.NewReader(export), []kibana.ImportRetry{
		{Type: "dashboard", ID: "d", Overwrite: true, DestinationID: "d2", ReplaceReferences: []kibana.ImportReferenceReplacement{{Type: "index-pattern", From: "a", To: "b"}}},
		{Type: "index-pattern", ID: "i", CreateNewCopy: true, IgnoreMissingReferences: true},
	}, &kibana.ResolveImportErrorsOptions{CreateNewCopies: true})
	if err != nil {
		t.Fatalf("ResolveImportErrors() error = %v", err)
	}
	if !got.Success || got.SuccessCount != 3 {
		t.Errorf("ResolveImportErrors() = %+v, want 3 objects", got)
	}
}

func TestSavedObjectsService_importInvalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	file := strings.NewReader("{}\n")
	if _, _, err := client.SavedObjects.Import(ctx, nil, nil); err == nil {
		t.Error("Import(nil reader) error = nil, want error")
	}
	for name, opt := range map[string]*kibana.ImportSavedObjectsOptions{
		"new copies and overwrite":     {CreateNewCopies: true, Overwrite: true},
		"new copies and compatibility": {CreateNewCopies: true, CompatibilityMode: true},
	} {
		if _, _, err := client.SavedObjects.Import(ctx, file, opt); err == nil {
			t.Errorf("Import(%s) error = nil, want error", name)
		}
	}
	retry := []kibana.ImportRetry{{Type: "dashboard", ID: "d"}}
	if _, _, err := client.SavedObjects.ResolveImportErrors(ctx, nil, retry, nil); err == nil {
		t.Error("ResolveImportErrors(nil reader) error = nil, want error")
	}
	for name, tt := range map[string]struct {
		retries []kibana.ImportRetry
		opt     *kibana.ResolveImportErrorsOptions
	}{
		"no retries":                   {nil, nil},
		"retry without":                {[]kibana.ImportRetry{{Type: "dashboard"}}, nil},
		"new copies and compatibility": {retry, &kibana.ResolveImportErrorsOptions{CreateNewCopies: true, CompatibilityMode: true}},
	} {
		if _, _, err := client.SavedObjects.ResolveImportErrors(ctx, file, tt.retries, tt.opt); err == nil {
			t.Errorf("ResolveImportErrors(%s) error = nil, want error", name)
		}
	}
}

func TestSavedObjectsService_Export(t *testing.T) {
	export := fixture(t, "savedobjects/export.ndjson")
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/saved_objects/_export", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"objects":[{"type":"dashboard","id":"probe-dash"}],"includeReferencesDeep":true}`)
		w.Header().Set("Content-Type", "application/ndjson")
		_, _ = w.Write(export)
	})

	var buf bytes.Buffer
	details, _, err := client.InSpace("x").SavedObjects.Export(context.Background(), &buf, &kibana.ExportSavedObjectsRequest{
		Objects:               []kibana.SavedObjectRef{{Type: "dashboard", ID: "probe-dash"}},
		IncludeReferencesDeep: true,
	}, nil)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if !bytes.Equal(buf.Bytes(), export) {
		t.Errorf("Export() wrote %q, want the response verbatim", buf.String())
	}
	want := &kibana.ExportDetails{ExportedCount: 3, MissingReferences: []kibana.SavedObjectRef{}, ExcludedObjects: []kibana.ExportExcludedObject{}}
	if !reflect.DeepEqual(details, want) {
		t.Errorf("Export() details = %+v, want %+v", details, want)
	}
}

func TestSavedObjectsService_Export_types(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_export", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"type":["visualization"],"search":"probe*","hasReference":[{"type":"index-pattern","id":"i"}],"excludeExportDetails":true}`)
		w.WriteHeader(http.StatusOK)
	})
	details, _, err := client.SavedObjects.Export(context.Background(), io.Discard, &kibana.ExportSavedObjectsRequest{
		Types:                []string{"visualization"},
		Search:               "probe*",
		HasReference:         []kibana.SavedObjectRef{{Type: "index-pattern", ID: "i"}},
		ExcludeExportDetails: true,
	}, nil)
	if err != nil || details != nil {
		t.Fatalf("Export() = %+v, %v; want no details and no error", details, err)
	}
}

func TestSavedObjectsService_Export_missingObject(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_export", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "savedobjects/export_missing.json"))
	})
	var buf bytes.Buffer
	_, _, err := client.SavedObjects.Export(context.Background(), &buf, &kibana.ExportSavedObjectsRequest{Objects: []kibana.SavedObjectRef{{Type: "dashboard", ID: "probe-nope"}}}, nil)
	var errResp *kibana.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, kibana.ErrBadRequest) || len(errResp.Attributes) == 0 {
		t.Errorf("Export() error = %v, want ErrBadRequest with the missing objects", err)
	}
	if buf.Len() != 0 {
		t.Errorf("Export() wrote %q on error, want nothing", buf.String())
	}
}

func TestSavedObjectsService_Export_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	obj := []kibana.SavedObjectRef{{Type: "dashboard", ID: "d"}}
	for name, req := range map[string]*kibana.ExportSavedObjectsRequest{
		"nil request":       nil,
		"nothing to export": {},
		"types and objects": {Types: []string{"dashboard"}, Objects: obj},
		"object without ID": {Objects: []kibana.SavedObjectRef{{Type: "dashboard"}}},
	} {
		if _, _, err := client.SavedObjects.Export(ctx, io.Discard, req, nil); err == nil {
			t.Errorf("Export(%s) error = nil, want error", name)
		}
	}
	if _, _, err := client.SavedObjects.Export(ctx, nil, &kibana.ExportSavedObjectsRequest{Objects: obj}, nil); err == nil {
		t.Error("Export(nil writer) error = nil, want error")
	}
}

// serveInChunks writes data in 1-byte chunks, like a slow connection.
func serveInChunks(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	w.Header().Set("Content-Type", "application/ndjson")
	f, _ := w.(http.Flusher)
	for i := range data {
		if _, err := w.Write(data[i : i+1]); err != nil {
			t.Errorf("writing response: %v", err)
			return
		}
		if f != nil && i%97 == 0 {
			f.Flush()
		}
	}
}

func TestSavedObjectsService_Export_details(t *testing.T) {
	export := fixture(t, "savedobjects/export.ndjson")
	lines := bytes.Split(bytes.TrimSpace(export), []byte("\n"))
	detailsLine := lines[len(lines)-1]
	tests := []struct {
		name    string
		body    []byte
		wantErr bool
		want    int
	}{
		{"chunked", export, false, 3},
		{"no trailing newline", bytes.TrimSuffix(export, []byte("\n")), false, 3},
		{"trailing blank lines", append(bytes.Clone(export), "\n\n"...), false, 3},
		// An export without objects only has the details line.
		{"only details", append(bytes.Clone(detailsLine), '\n'), false, 3},
		// A saved object line also decodes into ExportDetails.
		{"no details line", bytes.Join(lines[:len(lines)-1], []byte("\n")), true, 0},
		{"empty", nil, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/saved_objects/_export", func(w http.ResponseWriter, _ *http.Request) {
				serveInChunks(t, w, tt.body)
			})
			var buf bytes.Buffer
			details, _, err := client.SavedObjects.Export(context.Background(), &buf, &kibana.ExportSavedObjectsRequest{Types: []string{"dashboard"}}, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Export() = %+v, want an error", details)
				}
				return
			}
			if err != nil || details == nil || details.ExportedCount != tt.want {
				t.Errorf("Export() = %+v, %v; want %d exported", details, err, tt.want)
			}
			if !bytes.Equal(buf.Bytes(), tt.body) {
				t.Error("Export() did not write the whole response")
			}
		})
	}
}

// errWriter fails once more than n bytes have been written.
type errWriter struct{ n int }

func (w *errWriter) Write(p []byte) (int, error) {
	if len(p) > w.n {
		n := w.n
		w.n = 0
		return n, errors.New("disk full")
	}
	w.n -= len(p)
	return len(p), nil
}

func TestSavedObjectsService_Export_writeError(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/saved_objects/_export", func(w http.ResponseWriter, _ *http.Request) {
		serveInChunks(t, w, fixture(t, "savedobjects/export.ndjson"))
	})
	details, _, err := client.SavedObjects.Export(context.Background(), &errWriter{n: 100}, &kibana.ExportSavedObjectsRequest{Types: []string{"dashboard"}}, nil)
	if err == nil || details != nil {
		t.Errorf("Export() = %+v, %v; want no details and an error", details, err)
	}
}
