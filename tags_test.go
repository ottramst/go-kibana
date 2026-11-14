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
	"time"

	"github.com/ottramst/go-kibana"
)

func TestTagsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got := r.PathValue("id"); got != "fab28ebf-63c5-4d90-94c4-4ac294dae221" {
			t.Errorf("id = %q", got)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "tags/create.json"))
	})

	got, _, err := client.Tags.Get(context.Background(), "fab28ebf-63c5-4d90-94c4-4ac294dae221", nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	at := time.Date(2026, 11, 12, 16, 43, 36, 53e6, time.UTC)
	want := &kibana.Tag{
		ID:          "fab28ebf-63c5-4d90-94c4-4ac294dae221",
		Name:        "probe-tag-1",
		Description: "d1",
		Color:       "#FF0000",
		Meta:        kibana.TagMeta{CreatedAt: at, UpdatedAt: at, Version: "WzE0LDFd"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

func TestTagsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/tags", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		// The read-only ID and meta are not sent.
		testJSONBody(t, r, `{"name":"probe-tag-1","description":"d1","color":"#FF0000"}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "tags/create.json"))
	})
	tag := &kibana.Tag{ID: "ignored", Name: "probe-tag-1", Description: "d1", Color: "#FF0000", Meta: kibana.TagMeta{Version: "v"}}
	before := *tag
	got, resp, err := client.InSpace("x").Tags.Create(context.Background(), tag, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || got.ID != "fab28ebf-63c5-4d90-94c4-4ac294dae221" {
		t.Errorf("Create() = %+v (%d), want the created tag", got, resp.StatusCode)
	}
	if !reflect.DeepEqual(*tag, before) {
		t.Error("Create() modified the tag")
	}
}

func TestTagsService_Create_emptyFields(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/tags", func(w http.ResponseWriter, r *http.Request) {
		// Kibana rejects an empty color and picks one when it is left out.
		testJSONBody(t, r, `{"name":"n","description":""}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "tags/create.json"))
	})
	if _, _, err := client.Tags.Create(context.Background(), &kibana.Tag{Name: "n"}, nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestTagsService_Create_errors(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		want    error
	}{
		{"tags/create_dup.json", http.StatusConflict, kibana.ErrConflict},
		{"tags/create_emptycolor.json", http.StatusBadRequest, kibana.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/tags", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			if _, _, err := client.Tags.Create(context.Background(), &kibana.Tag{Name: "n"}, nil); !errors.Is(err, tt.want) {
				t.Errorf("Create() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestTagsService_CreateOrUpdate(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
	}{
		{"tags/put_nodesc.json", http.StatusOK},
		// Update creates missing tags.
		{"tags/put_new.json", http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PUT /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.EscapedPath(), "/api/tags/a%2Fb%3F"; got != want {
					t.Errorf("path = %q, want %q", got, want)
				}
				testJSONBody(t, r, `{"name":"probe-tag-1b","description":"","color":"#01d6fe"}`)
				writeJSON(t, w, tt.status, fixture(t, tt.fixture))
			})
			got, resp, err := client.Tags.CreateOrUpdate(context.Background(), "a/b?", &kibana.Tag{Name: "probe-tag-1b", Color: "#01d6fe"}, nil)
			if err != nil {
				t.Fatalf("CreateOrUpdate() error = %v", err)
			}
			if resp.StatusCode != tt.status || got.Name == "" || got.Meta.UpdatedAt.IsZero() {
				t.Errorf("CreateOrUpdate() = %+v (%d), want the tag with status %d", got, resp.StatusCode, tt.status)
			}
		})
	}
}

func TestTagsService_CreateOrUpdate_readModifyWrite(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/tags/{id}", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "tags/create.json"))
	})
	mux.HandleFunc("PUT /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		// The color and description of the tag that was read are kept.
		testJSONBody(t, r, `{"name":"renamed","description":"d1","color":"#FF0000"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "tags/put_nodesc.json"))
	})
	ctx := context.Background()
	tag, _, err := client.Tags.Get(ctx, "fab28ebf-63c5-4d90-94c4-4ac294dae221", nil)
	if err != nil {
		t.Fatal(err)
	}
	tag.Name = "renamed"
	got, _, err := client.Tags.CreateOrUpdate(ctx, tag.ID, tag, nil)
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	// Kibana leaves created_at out of the update response.
	if !got.Meta.CreatedAt.IsZero() {
		t.Errorf("Meta.CreatedAt = %v, want zero", got.Meta.CreatedAt)
	}
}

func TestTagsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := client.Tags.Delete(context.Background(), "t1", nil); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestTagsService_notFound(t *testing.T) {
	mux, client := setup(t)
	missing := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "tags/get_missing.json"))
	}
	mux.HandleFunc("GET /api/tags/{id}", missing)
	mux.HandleFunc("DELETE /api/tags/{id}", missing)
	// Kibana 8.19 has no tags API.
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "tags/list_8.19.json"))
	})
	ctx := context.Background()
	if _, _, err := client.Tags.Get(ctx, "nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if _, err := client.Tags.Delete(ctx, "nope", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.Tags.List(ctx, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("List() error = %v, want ErrNotFound", err)
	}
}

func TestTagsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "page=2&per_page=1&query=probe")
		writeJSON(t, w, http.StatusOK, fixture(t, "tags/list_p.json"))
	})
	got, _, err := client.Tags.List(context.Background(), &kibana.ListTagsOptions{Query: "probe", Page: 2, PerPage: 1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got.Page != 2 || got.PerPage != 1 || got.Total != 2 || len(got.Tags) != 1 || got.Tags[0].Name != "probe-tag-2" || got.Tags[0].Color != "#ff09e2" {
		t.Errorf("List() = %+v (%+v), want page 2 of 2 with probe-tag-2", got, got.Tags)
	}
}

// tagPages serves total tags named tag-0 to tag-<total-1>, paginated.
func tagPages(t *testing.T, mux *http.ServeMux, total int, requests *[]string) {
	t.Helper()
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.RawQuery)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		var data []json.RawMessage
		for i := (page - 1) * perPage; i < min(page*perPage, total); i++ {
			data = append(data, json.RawMessage(fmt.Sprintf(`{"id":"t%d","data":{"name":"tag-%d","color":"#000000"},"meta":{"managed":false}}`, i, i)))
		}
		body, _ := json.Marshal(map[string]any{"data": data, "meta": map[string]int{"page": page, "per_page": perPage, "total": total}})
		writeJSON(t, w, http.StatusOK, body)
	})
}

func TestTagsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	var requests []string
	tagPages(t, mux, 5, &requests)

	var names []string
	for tag, err := range client.Tags.ListAll(context.Background(), &kibana.ListTagsOptions{Query: "q", PerPage: 2}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, tag.Name)
	}
	if want := []string{"tag-0", "tag-1", "tag-2", "tag-3", "tag-4"}; !reflect.DeepEqual(names, want) {
		t.Errorf("ListAll() = %v, want %v", names, want)
	}
	if want := []string{"page=1&per_page=2&query=q", "page=2&per_page=2&query=q", "page=3&per_page=2&query=q"}; !reflect.DeepEqual(requests, want) {
		t.Errorf("requests = %v, want %v", requests, want)
	}
}

func TestTagsService_ListAll_startPageAndBreak(t *testing.T) {
	mux, client := setup(t)
	var requests []string
	tagPages(t, mux, 250, &requests)

	var names []string
	for tag, err := range client.Tags.ListAll(context.Background(), &kibana.ListTagsOptions{Page: 2}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, tag.Name)
		if len(names) == 101 {
			break
		}
	}
	// The default page size is 100, and the iteration starts at page 2.
	if names[0] != "tag-100" || names[100] != "tag-200" {
		t.Errorf("ListAll() = %s ... %s, want tag-100 ... tag-200", names[0], names[100])
	}
	if want := []string{"page=2&per_page=100", "page=3&per_page=100"}; !reflect.DeepEqual(requests, want) {
		t.Errorf("requests = %v, want %v", requests, want)
	}
}

func TestTagsService_ListAll_shortPage(t *testing.T) {
	mux, client := setup(t)
	calls := 0
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, _ *http.Request) {
		calls++
		// The total says 3 tags, but the page has 1 of 2: a tag was deleted.
		writeJSON(t, w, http.StatusOK, []byte(`{"data":[{"id":"a","data":{"name":"a"},"meta":{}}],"meta":{"page":1,"per_page":2,"total":3}}`))
	})
	n := 0
	for _, err := range client.Tags.ListAll(context.Background(), &kibana.ListTagsOptions{PerPage: 2}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		n++
	}
	if n != 1 || calls != 1 {
		t.Errorf("ListAll() yielded %d tags in %d requests, want 1 in 1", n, calls)
	}
}

func TestTagsService_ListAll_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, fixture(t, "tags/list_8.19.json"))
	})
	errs := 0
	for tag, err := range client.Tags.ListAll(context.Background(), nil) {
		if tag != nil || !errors.Is(err, kibana.ErrNotFound) {
			t.Errorf("ListAll() = %v, %v; want nil, ErrNotFound", tag, err)
		}
		errs++
	}
	if errs != 1 {
		t.Errorf("ListAll() yielded %d errors, want 1", errs)
	}
}

func TestTagsService_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	for name, tag := range map[string]*kibana.Tag{"nil tag": nil, "no name": {Color: "#000000"}} {
		if _, _, err := client.Tags.Create(ctx, tag, nil); err == nil {
			t.Errorf("Create(%s) error = nil, want error", name)
		}
		if _, _, err := client.Tags.CreateOrUpdate(ctx, "t", tag, nil); err == nil {
			t.Errorf("CreateOrUpdate(%s) error = nil, want error", name)
		}
	}
	for _, id := range []string{"", ".", ".."} {
		if _, _, err := client.Tags.Get(ctx, id, nil); err == nil {
			t.Errorf("Get(%q) error = nil, want error", id)
		}
		if _, _, err := client.Tags.CreateOrUpdate(ctx, id, &kibana.Tag{Name: "n"}, nil); err == nil {
			t.Errorf("CreateOrUpdate(%q) error = nil, want error", id)
		}
		if _, err := client.Tags.Delete(ctx, id, nil); err == nil {
			t.Errorf("Delete(%q) error = nil, want error", id)
		}
	}
}

func TestTag_roundTrip(t *testing.T) {
	var tag kibana.Tag
	if err := json.Unmarshal(fixture(t, "tags/create.json"), &tag); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	data, err := json.Marshal(tag)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got kibana.Tag
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal(Marshal()) error = %v", err)
	}
	if !reflect.DeepEqual(got, tag) {
		t.Errorf("round trip = %+v, want %+v", got, tag)
	}
}

func TestTagMeta_users(t *testing.T) {
	// The user fields are only set for users with a profile.
	var tag kibana.Tag
	data := `{"id":"t","data":{"name":"n","color":"#000000"},"meta":{"created_by":"u_1","updated_by":"u_2","owner":"u_1","managed":true}}`
	if err := json.Unmarshal([]byte(data), &tag); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := kibana.TagMeta{CreatedBy: "u_1", UpdatedBy: "u_2", Owner: "u_1", Managed: true}
	if !reflect.DeepEqual(tag.Meta, want) {
		t.Errorf("Meta = %+v, want %+v", tag.Meta, want)
	}
}
