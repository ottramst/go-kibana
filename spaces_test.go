package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestSpacesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/spaces/space", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "include_authorized_purposes=true")
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/list.json"))
	})

	spaces, _, err := client.Spaces.List(context.Background(), &kibana.ListSpacesOptions{IncludeAuthorizedPurposes: true})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(spaces) != 2 {
		t.Fatalf("len(spaces) = %d, want 2", len(spaces))
	}
	def := spaces[0]
	if def.ID != "default" || !def.Reserved || def.Name != "Default" {
		t.Errorf("spaces[0] = %+v, want the reserved default space", def)
	}
	if !def.AuthorizedPurposes["any"] {
		t.Errorf("spaces[0].AuthorizedPurposes = %v, want any=true", def.AuthorizedPurposes)
	}
	want := &kibana.Space{
		ID:                 "marketing",
		Name:               "Marketing",
		Description:        "Marketing team",
		Color:              "#aabbcc",
		Initials:           "MK",
		DisabledFeatures:   []string{"dev_tools"},
		Solution:           kibana.SpaceSolutionClassic,
		AuthorizedPurposes: spaces[1].AuthorizedPurposes,
	}
	if !reflect.DeepEqual(spaces[1], want) {
		t.Errorf("spaces[1] = %+v, want %+v", spaces[1], want)
	}
}

func TestSpacesService_List_purpose(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/spaces/space", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "purpose=copySavedObjectsIntoSpace")
		writeJSON(t, w, http.StatusOK, []byte(`[]`))
	})
	opt := &kibana.ListSpacesOptions{Purpose: kibana.SpacePurposeCopySavedObjectsIntoSpace}
	if _, _, err := client.Spaces.List(context.Background(), opt); err != nil {
		t.Fatalf("List() error = %v", err)
	}
}

func TestSpacesService_List_noOptions(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/spaces/space", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		writeJSON(t, w, http.StatusOK, []byte(`[]`))
	})
	spaces, _, err := client.Spaces.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(spaces) != 0 {
		t.Errorf("len(spaces) = %d, want 0", len(spaces))
	}
}

func TestSpacesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/spaces/space/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "marketing" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/get.json"))
	})

	space, _, err := client.Spaces.Get(context.Background(), "marketing")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if space.ID != "marketing" || space.Name != "Marketing" || space.Solution != kibana.SpaceSolutionClassic {
		t.Errorf("Get() = %+v", space)
	}

	_, resp, err := client.Spaces.Get(context.Background(), "missing")
	if !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("Get(missing) response = %v, want 404", resp)
	}
}

func TestSpacesService_Get_escapesID(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/spaces/space/{id}", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/spaces/space/a%2Fb%3Fc"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got := r.PathValue("id"); got != "a/b?c" {
			t.Errorf("id = %q, want %q", got, "a/b?c")
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"id":"a/b?c","name":"x"}`))
	})
	if _, _, err := client.Spaces.Get(context.Background(), "a/b?c"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestSpacesService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/spaces/space", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, `{"id":"marketing","name":"Marketing","description":"Marketing team","color":"#aabbcc","initials":"MK","disabledFeatures":["dev_tools"],"solution":"classic"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/get.json"))
	})

	in := &kibana.Space{
		ID:                 "marketing",
		Name:               "Marketing",
		Description:        "Marketing team",
		Color:              "#aabbcc",
		Initials:           "MK",
		DisabledFeatures:   []string{"dev_tools"},
		Solution:           kibana.SpaceSolutionClassic,
		AuthorizedPurposes: map[string]bool{"any": true}, // read-only, must not be sent
	}
	space, _, err := client.Spaces.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if space.ID != "marketing" {
		t.Errorf("Create() ID = %q, want %q", space.ID, "marketing")
	}
	if in.AuthorizedPurposes == nil {
		t.Error("Create() modified the caller's space")
	}
}

func TestSpacesService_Create_conflict(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/spaces/space", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusConflict, []byte(`{"statusCode":409,"error":"Conflict","message":"A space with the identifier marketing already exists."}`))
	})
	_, _, err := client.Spaces.Create(context.Background(), &kibana.Space{ID: "marketing", Name: "Marketing"})
	if !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Create() error = %v, want ErrConflict", err)
	}
}

func TestSpacesService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/spaces/space/marketing", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"id":"marketing","name":"Marketing 2","_reserved":true}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/get.json"))
	})

	in := &kibana.Space{ID: "ignored", Name: "Marketing 2", Reserved: true, AuthorizedPurposes: map[string]bool{"any": true}}
	if _, _, err := client.Spaces.Update(context.Background(), "marketing", in); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if in.ID != "ignored" || in.AuthorizedPurposes == nil {
		t.Errorf("Update() modified the caller's space: %+v", in)
	}
}

func TestSpacesService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/spaces/space/marketing", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Spaces.Delete(context.Background(), "marketing")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete() status = %d, want 204", resp.StatusCode)
	}
}

func TestSpacesService_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	checks := map[string]error{}
	_, _, checks["Get empty ID"] = client.Spaces.Get(ctx, "")
	_, _, checks["Create nil"] = client.Spaces.Create(ctx, nil)
	_, _, checks["Update empty ID"] = client.Spaces.Update(ctx, "", &kibana.Space{})
	_, _, checks["Update nil"] = client.Spaces.Update(ctx, "x", nil)
	_, checks["Delete empty ID"] = client.Spaces.Delete(ctx, "")
	for name, err := range checks {
		if err == nil {
			t.Errorf("%s: error = nil, want error", name)
		}
	}
}
