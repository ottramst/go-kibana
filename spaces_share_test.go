package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestSpacesService_GetShareableReferences(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/probe-src/api/spaces/_get_shareable_references", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"objects":[{"type":"index-pattern","id":"probe-ip1"},{"type":"index-pattern","id":"probe-nope"}]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/shareable_references.json"))
	})

	got, _, err := client.InSpace("probe-src").Spaces.GetShareableReferences(context.Background(), []kibana.SavedObjectRef{
		{Type: "index-pattern", ID: "probe-ip1"},
		{Type: "index-pattern", ID: "probe-nope"},
	}, nil)
	if err != nil {
		t.Fatalf("GetShareableReferences() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := &kibana.ShareableReference{
		Type: "index-pattern", ID: "probe-ip1",
		Spaces:                    []string{"probe-src"},
		InboundReferences:         []kibana.SavedObjectReference{},
		SpacesWithMatchingOrigins: []string{"probe-dst", "probe-nope"},
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("objects[0] = %+v, want %+v", got[0], want)
	}
	if !got[1].IsMissing {
		t.Errorf("objects[1].IsMissing = false, want true")
	}
}

func TestSpacesService_UpdateObjectsSpaces(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/probe-src/api/spaces/_update_objects_spaces", func(w http.ResponseWriter, r *http.Request) {
		// Kibana rejects null lists, so the nil SpacesToRemove is sent as [].
		testJSONBody(t, r, `{"objects":[{"type":"index-pattern","id":"probe-ip2"},{"type":"index-pattern","id":"probe-nope"}],"spacesToAdd":["probe-dst"],"spacesToRemove":[]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "spaces/update_objects_spaces.json"))
	})

	req := &kibana.UpdateObjectsSpacesRequest{
		Objects:     []kibana.SavedObjectRef{{Type: "index-pattern", ID: "probe-ip2"}, {Type: "index-pattern", ID: "probe-nope"}},
		SpacesToAdd: []string{"probe-dst"},
	}
	got, _, err := client.InSpace("probe-src").Spaces.UpdateObjectsSpaces(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("UpdateObjectsSpaces() error = %v", err)
	}
	if req.SpacesToRemove != nil {
		t.Error("UpdateObjectsSpaces() modified the caller's request")
	}
	if len(got) != 2 {
		t.Fatalf("UpdateObjectsSpaces() returned %d objects, want 2", len(got))
	}
	want := []*kibana.ObjectSpaces{
		{Type: "index-pattern", ID: "probe-ip2", Spaces: []string{"probe-dst", "probe-src"}},
		{Type: "index-pattern", ID: "probe-nope", Spaces: []string{}, Error: &kibana.ObjectError{
			StatusCode: 404, StatusText: "Not Found", Message: "Saved object [index-pattern/probe-nope] not found",
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateObjectsSpaces() = %+v, %+v; want %+v, %+v", got[0], got[1], want[0], want[1])
	}
	if msg := got[1].Error.Error(); msg != "kibana: 404 Saved object [index-pattern/probe-nope] not found" {
		t.Errorf("Error() = %q", msg)
	}
	if !errors.Is(got[1].Error, kibana.ErrNotFound) || errors.Is(got[1].Error, kibana.ErrConflict) {
		t.Error("errors.Is(ObjectError) does not match the status code")
	}
}

func TestSpacesService_shareInvalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	obj := []kibana.SavedObjectRef{{Type: "dashboard", ID: "d"}}
	if _, _, err := client.Spaces.GetShareableReferences(ctx, nil, nil); err == nil {
		t.Error("GetShareableReferences(nil): error = nil, want error")
	}
	if _, _, err := client.Spaces.GetShareableReferences(ctx, []kibana.SavedObjectRef{{ID: "d"}}, nil); err == nil {
		t.Error("GetShareableReferences(no type): error = nil, want error")
	}
	cases := map[string]*kibana.UpdateObjectsSpacesRequest{
		"nil request":     nil,
		"no objects":      {SpacesToAdd: []string{"b"}},
		"no space change": {Objects: obj},
		"empty lists":     {Objects: obj, SpacesToAdd: []string{}, SpacesToRemove: []string{}},
	}
	for name, req := range cases {
		if _, _, err := client.Spaces.UpdateObjectsSpaces(ctx, req, nil); err == nil {
			t.Errorf("UpdateObjectsSpaces(%s): error = nil, want error", name)
		}
	}
}

func TestSpacesService_DisableLegacyURLAliases(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/spaces/_disable_legacy_url_aliases", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"aliases":[{"targetSpace":"marketing","targetType":"dashboard","sourceId":"123"}]}`)
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Spaces.DisableLegacyURLAliases(context.Background(), []kibana.LegacyURLAlias{
		{TargetSpace: "marketing", TargetType: "dashboard", SourceID: "123"},
	}, nil)
	if err != nil {
		t.Fatalf("DisableLegacyURLAliases() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}

	for name, aliases := range map[string][]kibana.LegacyURLAlias{
		"nil":            nil,
		"missing source": {{TargetSpace: "marketing", TargetType: "dashboard"}},
	} {
		if _, err := client.Spaces.DisableLegacyURLAliases(context.Background(), aliases, nil); err == nil {
			t.Errorf("DisableLegacyURLAliases(%s): error = nil, want error", name)
		}
	}
}
