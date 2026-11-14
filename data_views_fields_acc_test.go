package kibana_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccDataViewsFields(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	dv, _, err := client.DataViews.Create(ctx, &kibana.DataView{Title: randName(t, "acc") + "-*", AllowNoIndex: true}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Field metadata: set, then remove. The field does not need to exist.
	got, _, err := client.DataViews.UpdateFieldsMetadata(ctx, dv.ID, &kibana.UpdateFieldsMetadataRequest{Fields: map[string]kibana.DataViewFieldMetadata{
		"host": {CustomLabel: new("Host"), CustomDescription: new("The host"), Count: new(3), Format: &kibana.DataViewFieldFormat{ID: "string"}},
	}}, nil)
	if err != nil {
		t.Fatalf("UpdateFieldsMetadata() error = %v", err)
	}
	if a := got.FieldAttrs["host"]; a.CustomLabel != "Host" || a.CustomDescription != "The host" || a.Count != 3 || got.FieldFormats["host"].ID != "string" {
		t.Errorf("UpdateFieldsMetadata() = %+v, %+v; want the metadata", got.FieldAttrs, got.FieldFormats)
	}
	got, _, err = client.DataViews.UpdateFieldsMetadata(ctx, dv.ID, &kibana.UpdateFieldsMetadataRequest{Fields: map[string]kibana.DataViewFieldMetadata{
		"host": {CustomLabel: new(""), Count: new(0), Format: &kibana.DataViewFieldFormat{}},
	}}, nil)
	if err != nil {
		t.Fatalf("UpdateFieldsMetadata(remove) error = %v", err)
	}
	if a := got.FieldAttrs["host"]; a.CustomLabel != "" || a.Count != 0 || a.CustomDescription != "The host" {
		t.Errorf("FieldAttrs[host] = %+v, want only the description", a)
	}
	if _, ok := got.FieldFormats["host"]; ok {
		t.Errorf("FieldFormats = %+v, want no host format", got.FieldFormats)
	}

	// Runtime fields.
	rf := &kibana.RuntimeField{Type: "long", Script: &kibana.RuntimeFieldScript{Source: "emit(1)"}, CustomLabel: "One"}
	res, _, err := client.DataViews.CreateRuntimeField(ctx, dv.ID, "one", rf, nil)
	if err != nil {
		t.Fatalf("CreateRuntimeField() error = %v", err)
	}
	if len(res.Fields) != 1 || res.Fields[0].Name != "one" || res.Fields[0].CustomLabel != "One" || res.DataView.ID != dv.ID {
		t.Errorf("CreateRuntimeField() = %+v, want the field", res)
	}
	if _, _, err := client.DataViews.CreateRuntimeField(ctx, dv.ID, "one", rf, nil); !errors.Is(err, kibana.ErrBadRequest) {
		t.Errorf("CreateRuntimeField(again) error = %v, want ErrBadRequest", err)
	}
	if res, _, err = client.DataViews.UpdateRuntimeField(ctx, dv.ID, "one", &kibana.RuntimeField{Type: "double"}, nil); err != nil {
		t.Fatalf("UpdateRuntimeField() error = %v", err)
	}
	if r := res.Fields[0].RuntimeField; r == nil || r.Type != "double" || r.Script == nil || r.Script.Source != "emit(1)" {
		t.Errorf("UpdateRuntimeField() = %+v, want the new type and the old script", r)
	}
	if res, _, err = client.DataViews.CreateOrUpdateRuntimeField(ctx, dv.ID, "one", &kibana.RuntimeField{Type: "keyword"}, nil); err != nil {
		t.Fatalf("CreateOrUpdateRuntimeField(replace) error = %v", err)
	}
	if r := res.Fields[0].RuntimeField; r.Type != "keyword" || r.Script != nil {
		t.Errorf("CreateOrUpdateRuntimeField(replace) = %+v, want keyword without a script", r)
	}
	if res, _, err = client.DataViews.GetRuntimeField(ctx, dv.ID, "one", nil); err != nil || res.Fields[0].Name != "one" {
		t.Errorf("GetRuntimeField() = %+v, %v; want the field", res, err)
	}
	if _, err := client.DataViews.DeleteRuntimeField(ctx, dv.ID, "one", nil); err != nil {
		t.Fatalf("DeleteRuntimeField() error = %v", err)
	}
	if _, _, err := client.DataViews.GetRuntimeField(ctx, dv.ID, "one", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("GetRuntimeField(deleted) error = %v, want ErrNotFound", err)
	}
	if _, err := client.DataViews.DeleteRuntimeField(ctx, dv.ID, "one", nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("DeleteRuntimeField(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestAccDataViewsSwapReferences(t *testing.T) {
	client := accClient(t)
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()

	from, _, err := client.DataViews.Create(ctx, &kibana.DataView{Title: randName(t, "acc") + "-*", AllowNoIndex: true}, nil)
	if err != nil {
		t.Fatalf("Create(from) error = %v", err)
	}
	to, _, err := client.DataViews.Create(ctx, &kibana.DataView{Title: randName(t, "acc") + "-*", AllowNoIndex: true}, nil)
	if err != nil {
		t.Fatalf("Create(to) error = %v", err)
	}
	var visIDs []string
	for range 2 {
		vis := &kibana.SavedObject{
			Type:       "visualization",
			ID:         randName(t, "acc"),
			Attributes: json.RawMessage(`{"title":"acc vis","visState":"{}","uiStateJSON":"{}","description":"","kibanaSavedObjectMeta":{"searchSourceJSON":"{}"}}`),
			References: []kibana.SavedObjectReference{{Name: "ref", Type: "index-pattern", ID: from.ID}},
		}
		if _, _, err := client.SavedObjects.Create(ctx, vis, nil); err != nil {
			t.Fatalf("SavedObjects.Create() error = %v", err)
		}
		visIDs = append(visIDs, vis.ID)
	}

	preview, _, err := client.DataViews.PreviewSwapReferences(ctx, &kibana.SwapReferencesRequest{FromID: from.ID, ToID: to.ID}, nil)
	if err != nil || len(preview.Result) != 2 || preview.DeleteStatus != nil {
		t.Fatalf("PreviewSwapReferences() = %+v, %v; want the 2 visualizations", preview, err)
	}

	// Swapping one keeps the other reference, so the data view stays.
	res, _, err := client.DataViews.SwapReferences(ctx, &kibana.SwapReferencesRequest{FromID: from.ID, ToID: to.ID, ForID: visIDs[:1], Delete: true}, nil)
	if err != nil {
		t.Fatalf("SwapReferences(one) error = %v", err)
	}
	if len(res.Result) != 1 || res.Result[0].ID != visIDs[0] || res.DeleteStatus == nil || res.DeleteStatus.DeletePerformed || res.DeleteStatus.RemainingRefs != 1 {
		t.Errorf("SwapReferences(one) = %+v (%+v), want 1 swapped and 1 remaining", res, res.DeleteStatus)
	}
	vis, _, err := client.SavedObjects.Get(ctx, "visualization", visIDs[0], nil)
	if err != nil || len(vis.References) != 1 || vis.References[0].ID != to.ID {
		t.Errorf("references of %s = %+v, %v; want %s", visIDs[0], vis, err, to.ID)
	}

	// Swapping the rest deletes it.
	res, _, err = client.DataViews.SwapReferences(ctx, &kibana.SwapReferencesRequest{FromID: from.ID, ToID: to.ID, Delete: true}, nil)
	if err != nil {
		t.Fatalf("SwapReferences(rest) error = %v", err)
	}
	if len(res.Result) != 1 || res.DeleteStatus == nil || !res.DeleteStatus.DeletePerformed {
		t.Errorf("SwapReferences(rest) = %+v (%+v), want the data view deleted", res, res.DeleteStatus)
	}
	if _, _, err := client.DataViews.Get(ctx, from.ID, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(swapped) error = %v, want ErrNotFound", err)
	}
}
