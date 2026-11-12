package kibana_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestShortURLsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/short_url", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testJSONBody(t, r, `{"locatorId":"LEGACY_SHORT_URL_LOCATOR","params":{"url":"/app/discover#/"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "shorturls/create.json"))
	})

	got, _, err := client.ShortURLs.Create(context.Background(), &kibana.CreateShortURLRequest{
		LocatorID: kibana.LegacyShortURLLocator,
		Params:    map[string]any{"url": "/app/discover#/"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	created := time.Date(2026, 11, 12, 16, 43, 27, 335e6, time.UTC)
	want := &kibana.ShortURL{
		ID:   "7cfb4636-4483-46d2-9ed7-72baed3d53e4",
		Slug: "ipYsn",
		Locator: kibana.ShortURLLocator{
			ID:      kibana.LegacyShortURLLocator,
			Version: "9.5.6",
			State:   map[string]any{"url": "/app/discover#/"},
		},
		AccessDate: created,
		CreateDate: created,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Create() = %+v, want %+v", got, want)
	}
}

func TestShortURLsService_Create_8_19(t *testing.T) {
	// Kibana 8.19 sends the dates as epoch milliseconds too, although its
	// API specification says they are strings.
	mux, client := setup(t)
	mux.HandleFunc("POST /api/short_url", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "shorturls/create_8.19.json"))
	})
	got, _, err := client.ShortURLs.Create(context.Background(), &kibana.CreateShortURLRequest{LocatorID: kibana.LegacyShortURLLocator})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.CreateDate.IsZero() || got.CreateDate.Year() != 2026 || got.Locator.Version != "8.19.25" {
		t.Errorf("Create() = %+v, want a 2026 creation date from 8.19.25", got)
	}
}

func TestShortURLsService_Create_options(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/short_url", func(w http.ResponseWriter, r *http.Request) {
		// Params are required, so nil is sent as {}.
		testJSONBody(t, r, `{"locatorId":"L","params":{},"slug":"probe-slug-1"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "shorturls/create_slug.json"))
	})
	req := &kibana.CreateShortURLRequest{LocatorID: "L", Slug: "probe-slug-1"}
	got, _, err := client.InSpace("x").ShortURLs.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.Slug != "probe-slug-1" {
		t.Errorf("Slug = %q, want probe-slug-1", got.Slug)
	}
	if req.Params != nil {
		t.Error("Create() modified the request")
	}
}

func TestShortURLsService_Create_conflict(t *testing.T) {
	for _, name := range []string{"shorturls/create_dupslug.json", "shorturls/create_badloc.json"} {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/short_url", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusConflict, fixture(t, name))
			})
			_, _, err := client.ShortURLs.Create(context.Background(), &kibana.CreateShortURLRequest{LocatorID: "L", Slug: "probe-slug-1"})
			if !errors.Is(err, kibana.ErrConflict) {
				t.Errorf("Create() error = %v, want ErrConflict", err)
			}
		})
	}
}

func TestShortURLsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/short_url/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/short_url/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "shorturls/create.json"))
	})
	got, _, err := client.ShortURLs.Get(context.Background(), "a/b?")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != "7cfb4636-4483-46d2-9ed7-72baed3d53e4" {
		t.Errorf("ID = %q", got.ID)
	}
}

func TestShortURLsService_Resolve(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/short_url/_slug/{slug}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/s/x/api/short_url/_slug/a%2Fb%3F"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "shorturls/create.json"))
	})
	got, _, err := client.InSpace("x").ShortURLs.Resolve(context.Background(), "a/b?")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Slug != "ipYsn" {
		t.Errorf("Slug = %q, want ipYsn", got.Slug)
	}
}

func TestShortURLsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/short_url/{id}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		if got := r.PathValue("id"); got != "u1" {
			t.Errorf("id = %q, want u1", got)
		}
		// Kibana answers 200 with a JSON null.
		writeJSON(t, w, http.StatusOK, []byte("null"))
	})
	if _, err := client.ShortURLs.Delete(context.Background(), "u1"); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestShortURLsService_notFound(t *testing.T) {
	mux, client := setup(t)
	missing := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "shorturls/get_missing.json"))
	}
	mux.HandleFunc("GET /api/short_url/{id}", missing)
	mux.HandleFunc("DELETE /api/short_url/{id}", missing)
	mux.HandleFunc("GET /api/short_url/_slug/{slug}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "shorturls/resolve_missing.json"))
	})
	ctx := context.Background()
	if _, _, err := client.ShortURLs.Get(ctx, "nope"); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.ShortURLs.Resolve(ctx, "nope"); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Resolve() error = %v, want ErrNotFound", err)
	}
	if _, err := client.ShortURLs.Delete(ctx, "nope"); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestShortURLsService_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	if _, _, err := client.ShortURLs.Create(ctx, nil); err == nil {
		t.Error("Create(nil) error = nil, want error")
	}
	if _, _, err := client.ShortURLs.Create(ctx, &kibana.CreateShortURLRequest{}); err == nil {
		t.Error("Create(no locator) error = nil, want error")
	}
	for _, id := range []string{"", ".", ".."} {
		if _, _, err := client.ShortURLs.Get(ctx, id); err == nil {
			t.Errorf("Get(%q) error = nil, want error", id)
		}
		if _, _, err := client.ShortURLs.Resolve(ctx, id); err == nil {
			t.Errorf("Resolve(%q) error = nil, want error", id)
		}
		if _, err := client.ShortURLs.Delete(ctx, id); err == nil {
			t.Errorf("Delete(%q) error = nil, want error", id)
		}
	}
}

func TestShortURL_UnmarshalJSON(t *testing.T) {
	var got kibana.ShortURL
	if err := got.UnmarshalJSON([]byte(`{"id":"u","slug":"s","accessCount":3,"accessDate":0,"createDate":1000}`)); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}
	if got.AccessCount != 3 || !got.AccessDate.IsZero() || !got.CreateDate.Equal(time.Unix(1, 0)) {
		t.Errorf("ShortURL = %+v, want 3 accesses, no access date, created at 1s", got)
	}
	if err := got.UnmarshalJSON([]byte(`{"createDate":"2026-01-01"}`)); err == nil {
		t.Error("UnmarshalJSON(string date) error = nil, want error")
	}
}

func TestShortURL_roundTrip(t *testing.T) {
	var u kibana.ShortURL
	if err := json.Unmarshal(fixture(t, "shorturls/create.json"), &u); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got kibana.ShortURL
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(Marshal()) error = %v", err)
	}
	if !reflect.DeepEqual(got, u) {
		t.Errorf("round trip = %+v, want %+v", got, u)
	}
	if !bytes.Contains(data, []byte(`"createDate":1794501807335`)) {
		t.Errorf("Marshal() = %s, want epoch milliseconds", data)
	}
	data, _ = json.Marshal(kibana.ShortURL{})
	if !bytes.Contains(data, []byte(`"accessDate":0`)) {
		t.Errorf("Marshal(zero) = %s, want a zero access date", data)
	}
}
