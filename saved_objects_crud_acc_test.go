package kibana_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccSavedObjectsCRUD(t *testing.T) {
	client := accClient(t)
	// Objects are deleted with the space.
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	ipID := randName(t, "acc")
	createIndexPattern(t, client, ipID, ipID+"-*")

	refs := []kibana.SavedObjectReference{{Name: "kibanaSavedObjectMeta.searchSourceJSON.index", Type: "index-pattern", ID: ipID}}
	vis := &kibana.SavedObject{
		Type:       "visualization",
		ID:         randName(t, "acc"),
		Attributes: json.RawMessage(`{"title":"acc vis","description":"d","visState":"{}","uiStateJSON":"{}","kibanaSavedObjectMeta":{"searchSourceJSON":"{\"indexRefName\":\"kibanaSavedObjectMeta.searchSourceJSON.index\"}"}}`),
		References: refs,
	}
	created, _, err := client.SavedObjects.Create(ctx, vis, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != vis.ID || created.Version == "" || created.CreatedAt.IsZero() || !reflect.DeepEqual(created.References, refs) {
		t.Errorf("Create() = %+v, want the visualization", created)
	}
	if _, _, err := client.SavedObjects.Create(ctx, vis, nil); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Create(again) error = %v, want ErrConflict", err)
	}
	if _, _, err := client.SavedObjects.Create(ctx, vis, &kibana.CreateSavedObjectOptions{Overwrite: true}); err != nil {
		t.Errorf("Create(overwrite) error = %v", err)
	}

	// A copy of an object that was read, with a generated ID.
	got, _, err := client.SavedObjects.Get(ctx, "visualization", vis.ID, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	cp := *got
	cp.ID = ""
	copied, _, err := client.SavedObjects.Create(ctx, &cp, nil)
	if err != nil {
		t.Fatalf("Create(copy) error = %v", err)
	}
	if copied.ID == "" || copied.ID == vis.ID {
		t.Errorf("Create(copy).ID = %q, want a generated ID", copied.ID)
	}

	// Attributes are merged; nil references are kept.
	got.Attributes = json.RawMessage(`{"title":"acc vis 2"}`)
	references := got.References
	got.References = nil
	if _, _, err := client.SavedObjects.Update(ctx, got.Type, got.ID, got, nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	after, _, err := client.SavedObjects.Get(ctx, "visualization", vis.ID, nil)
	if err != nil {
		t.Fatalf("Get(updated) error = %v", err)
	}
	var attrs struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(after.Attributes, &attrs); err != nil || attrs.Title != "acc vis 2" || attrs.Description != "d" {
		t.Errorf("attributes after Update() = %s, want the new title and the old description", after.Attributes)
	}
	if !reflect.DeepEqual(after.References, references) {
		t.Errorf("references after Update(nil references) = %+v, want %+v", after.References, references)
	}

	// The version of the first read is stale now.
	if _, _, err := client.SavedObjects.Update(ctx, got.Type, got.ID, got, nil); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Update(stale version) error = %v, want ErrConflict", err)
	}

	// Empty references remove them.
	after.References = []kibana.SavedObjectReference{}
	if _, _, err := client.SavedObjects.Update(ctx, after.Type, after.ID, after, nil); err != nil {
		t.Fatalf("Update(no references) error = %v", err)
	}
	if after, _, err = client.SavedObjects.Get(ctx, "visualization", vis.ID, nil); err != nil {
		t.Fatalf("Get(updated) error = %v", err)
	}
	if len(after.References) != 0 {
		t.Errorf("references after Update(empty references) = %+v, want none", after.References)
	}

	// Upsert creates a missing object.
	missing := &kibana.SavedObject{Type: "index-pattern", ID: randName(t, "acc"), Attributes: json.RawMessage(`{"title":"u-*"}`)}
	if _, _, err := client.SavedObjects.Update(ctx, missing.Type, missing.ID, missing, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Update(missing) error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.SavedObjects.Update(ctx, missing.Type, missing.ID, missing, &kibana.UpdateSavedObjectOptions{Upsert: missing.Attributes}); err != nil {
		t.Errorf("Update(upsert) error = %v", err)
	}
	if _, _, err := client.SavedObjects.Get(ctx, "index-pattern", missing.ID, nil); err != nil {
		t.Errorf("Get(upserted) error = %v", err)
	}

	if _, err := client.SavedObjects.Delete(ctx, "visualization", vis.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.SavedObjects.Get(ctx, "visualization", vis.ID, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	if _, err := client.SavedObjects.Delete(ctx, "visualization", vis.ID, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Delete(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestAccSavedObjectsBulk(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	id1, id2, missing := randName(t, "acc"), randName(t, "acc"), randName(t, "missing")
	created, _, err := client.SavedObjects.BulkCreate(ctx, []*kibana.SavedObject{
		{Type: "index-pattern", ID: id1, Attributes: json.RawMessage(`{"title":"b1-*"}`)},
		{Type: "index-pattern", ID: id2, Attributes: json.RawMessage(`{"title":"b2-*"}`)},
	}, nil)
	if err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}
	if len(created) != 2 || created[0].Error != nil || created[1].Error != nil {
		t.Fatalf("BulkCreate() = %+v, want 2 objects", created)
	}
	again, _, err := client.SavedObjects.BulkCreate(ctx, []*kibana.SavedObject{{Type: "index-pattern", ID: id1, Attributes: json.RawMessage(`{"title":"x"}`)}}, nil)
	if err != nil || len(again) != 1 || !errors.Is(again[0].Error, kibana.ErrConflict) {
		t.Errorf("BulkCreate(again) = %+v, %v; want a conflict", again, err)
	}

	got, _, err := client.SavedObjects.BulkGet(ctx, []kibana.BulkGetSavedObjectsItem{
		{Type: "index-pattern", ID: id1}, {Type: "index-pattern", ID: missing},
	}, nil)
	if err != nil {
		t.Fatalf("BulkGet() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != id1 || got[0].Error != nil || !errors.Is(got[1].Error, kibana.ErrNotFound) {
		t.Errorf("BulkGet() = %+v, want %s and a missing object", got, id1)
	}

	got[0].Attributes = json.RawMessage(`{"title":"b1u-*"}`)
	updated, _, err := client.SavedObjects.BulkUpdate(ctx, []*kibana.SavedObject{
		got[0],
		{Type: "index-pattern", ID: missing, Attributes: json.RawMessage(`{"title":"x"}`)},
	}, nil)
	if err != nil {
		t.Fatalf("BulkUpdate() error = %v", err)
	}
	if len(updated) != 2 || updated[0].Error != nil || updated[0].Version == got[0].Version || !errors.Is(updated[1].Error, kibana.ErrNotFound) {
		t.Errorf("BulkUpdate() = %+v, want the update and a missing object", updated)
	}

	resolved, _, err := client.SavedObjects.BulkResolve(ctx, []kibana.SavedObjectRef{{Type: "index-pattern", ID: id2}}, nil)
	if err != nil || len(resolved) != 1 || resolved[0].Outcome != kibana.ResolveOutcomeExactMatch || resolved[0].SavedObject.ID != id2 {
		t.Errorf("BulkResolve() = %+v, %v; want an exact match", resolved, err)
	}

	statuses, _, err := client.SavedObjects.BulkDelete(ctx, []kibana.SavedObjectRef{
		{Type: "index-pattern", ID: id1}, {Type: "index-pattern", ID: id2}, {Type: "index-pattern", ID: missing},
	}, nil)
	if err != nil {
		t.Fatalf("BulkDelete() error = %v", err)
	}
	if len(statuses) != 3 || !statuses[0].Success || !statuses[1].Success || statuses[2].Success || !errors.Is(statuses[2].Error, kibana.ErrNotFound) {
		t.Errorf("BulkDelete() = %+v, want 2 deleted and a missing object", statuses)
	}
}
