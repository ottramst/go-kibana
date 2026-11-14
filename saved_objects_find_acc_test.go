package kibana_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccSavedObjectsFind(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	ipID := randName(t, "acc")
	createIndexPattern(t, client, ipID, ipID+"-*")
	var visIDs []string
	for i := range 3 {
		obj := &kibana.SavedObject{
			Type:       "visualization",
			ID:         randName(t, "acc"),
			Attributes: json.RawMessage(`{"title":"acc vis","visState":"{}","uiStateJSON":"{}","description":"","kibanaSavedObjectMeta":{"searchSourceJSON":"{}"}}`),
		}
		// The first two reference the index pattern.
		if i < 2 {
			obj.References = []kibana.SavedObjectReference{{Name: "ref", Type: "index-pattern", ID: ipID}}
		}
		if _, _, err := client.SavedObjects.Create(ctx, obj, nil); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		visIDs = append(visIDs, obj.ID)
	}

	got, _, err := client.SavedObjects.Find(ctx, &kibana.FindSavedObjectsOptions{
		Types:        []string{"visualization"},
		HasReference: []kibana.SavedObjectRef{{Type: "index-pattern", ID: ipID}},
		Fields:       []string{"title"},
	})
	if err != nil {
		t.Fatalf("Find(has reference) error = %v", err)
	}
	var found []string
	for _, obj := range got.SavedObjects {
		found = append(found, obj.ID)
	}
	slices.Sort(found)
	want := slices.Sorted(slices.Values(visIDs[:2]))
	if got.Total != 2 || !slices.Equal(found, want) {
		t.Errorf("Find(has reference) = %v (total %d), want %v", found, got.Total, want)
	}

	got, _, err = client.SavedObjects.Find(ctx, &kibana.FindSavedObjectsOptions{
		Types:          []string{"visualization"},
		HasNoReference: []kibana.SavedObjectRef{{Type: "index-pattern", ID: ipID}},
	})
	if err != nil || got.Total != 1 || got.SavedObjects[0].ID != visIDs[2] {
		t.Errorf("Find(has no reference) = %+v, %v; want %s", got, err, visIDs[2])
	}

	var all []string
	for obj, err := range client.SavedObjects.FindAll(ctx, &kibana.FindSavedObjectsOptions{
		Types: []string{"visualization", "index-pattern"}, PerPage: 1, SortField: "updated_at",
	}) {
		if err != nil {
			t.Fatalf("FindAll() error = %v", err)
		}
		all = append(all, obj.ID)
	}
	if len(all) != 4 || !slices.Contains(all, ipID) {
		t.Errorf("FindAll() = %v, want the 4 objects of the space", all)
	}

	_, _, err = client.SavedObjects.Find(ctx, &kibana.FindSavedObjectsOptions{Types: []string{"visualization"}, Page: 101, PerPage: 100})
	if !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("Find(beyond 10,000) error = %v, want ErrBadRequest", err)
	}

	resolved, _, err := client.SavedObjects.Resolve(ctx, "index-pattern", ipID, nil)
	if err != nil || resolved.Outcome != kibana.ResolveOutcomeExactMatch || resolved.SavedObject.ID != ipID {
		t.Errorf("Resolve() = %+v, %v; want an exact match", resolved, err)
	}
	if _, _, err := client.SavedObjects.Resolve(ctx, "index-pattern", randName(t, "missing"), nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Resolve(missing) error = %v, want ErrNotFound", err)
	}
}
