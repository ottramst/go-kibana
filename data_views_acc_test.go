package kibana_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccDataViews(t *testing.T) {
	client := accClient(t)
	// Data views and the default data view belong to the space, which is
	// deleted with them.
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	// A new space has no default data view.
	if id, _, err := client.DataViews.GetDefault(ctx); err != nil || id != "" {
		t.Fatalf("GetDefault() = %q, %v; want none", id, err)
	}

	// No index matches the title, so the view needs AllowNoIndex to be
	// usable; Kibana creates it either way.
	dv := &kibana.DataView{
		ID:            randName(t, "acc"),
		Title:         randName(t, "acc") + "-*",
		Name:          "acc view",
		TimeFieldName: "@timestamp",
		AllowNoIndex:  true,
		SourceFilters: []kibana.DataViewSourceFilter{{Value: "secret*"}},
		RuntimeFieldMap: map[string]kibana.RuntimeField{
			"rt": {Type: "keyword", Script: &kibana.RuntimeFieldScript{Source: `emit("x")`}},
		},
		FieldAttrs: map[string]kibana.DataViewFieldAttrs{"rt": {CustomLabel: "Runtime"}},
	}
	created, _, err := client.DataViews.Create(ctx, dv, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != dv.ID || created.Title != dv.Title || created.Version == "" || created.RuntimeFieldMap["rt"].Type != "keyword" || created.FieldAttrs["rt"].CustomLabel != "Runtime" {
		t.Errorf("Create() = %+v, want the data view", created)
	}

	// The first data view of a space becomes its default.
	if id, _, err := client.DataViews.GetDefault(ctx); err != nil || id != dv.ID {
		t.Errorf("GetDefault() = %q, %v; want %s", id, err, dv.ID)
	}

	// An existing ID is a 400, unless Override is set.
	if _, _, err := client.DataViews.Create(ctx, dv, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("Create(same ID) error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.DataViews.Create(ctx, dv, &kibana.CreateDataViewOptions{Override: true}); err != nil {
		t.Errorf("Create(override) error = %v", err)
	}

	got, _, err := client.DataViews.Get(ctx, dv.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "acc view" || got.TimeFieldName != "@timestamp" || !got.AllowNoIndex || len(got.SourceFilters) != 1 {
		t.Errorf("Get() = %+v, want the data view", got)
	}

	// Writing back an unchanged data view works, although Kibana returns a
	// typeMeta of null, which it rejects in requests.
	if _, _, err := client.DataViews.Update(ctx, dv.ID, got, nil); err != nil {
		t.Fatalf("Update(unchanged) error = %v", err)
	}

	// Read, modify, write: clear the time field and the runtime fields.
	got.Name = "acc view 2"
	got.TimeFieldName = ""
	got.RuntimeFieldMap = map[string]kibana.RuntimeField{}
	updated, _, err := client.DataViews.Update(ctx, dv.ID, got, &kibana.UpdateDataViewOptions{RefreshFields: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "acc view 2" || updated.TimeFieldName != "" || len(updated.RuntimeFieldMap) != 0 || updated.Title != dv.Title {
		t.Errorf("Update() = %+v, want the new name, no time field and no runtime fields", updated)
	}

	// An empty Title keeps the stored title.
	if updated, _, err = client.DataViews.Update(ctx, dv.ID, &kibana.DataView{Name: "acc view 3"}, nil); err != nil || updated.Title != dv.Title {
		t.Errorf("Update(no title) = %+v, %v; want the title kept", updated, err)
	}

	other, _, err := client.DataViews.Create(ctx, &kibana.DataView{Title: randName(t, "acc") + "-*", AllowNoIndex: true}, nil)
	if err != nil {
		t.Fatalf("Create(other) error = %v", err)
	}
	list, _, err := client.DataViews.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var ids []string
	for _, v := range list {
		ids = append(ids, v.ID)
	}
	if !slices.Contains(ids, dv.ID) || !slices.Contains(ids, other.ID) {
		t.Errorf("List() = %v, want %s and %s", ids, dv.ID, other.ID)
	}

	// SetDefault replaces an existing default, and "" removes it.
	if _, err := client.DataViews.SetDefault(ctx, other.ID); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}
	if id, _, err := client.DataViews.GetDefault(ctx); err != nil || id != other.ID {
		t.Errorf("GetDefault() = %q, %v; want %s", id, err, other.ID)
	}
	if _, err := client.DataViews.SetDefault(ctx, ""); err != nil {
		t.Fatalf("SetDefault(\"\") error = %v", err)
	}
	if id, _, err := client.DataViews.GetDefault(ctx); err != nil || id != "" {
		t.Errorf("GetDefault() = %q, %v; want none", id, err)
	}

	if _, err := client.DataViews.Delete(ctx, dv.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.DataViews.Get(ctx, dv.ID); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	if _, err := client.DataViews.Delete(ctx, dv.ID); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Delete(deleted) error = %v, want ErrNotFound", err)
	}
}
