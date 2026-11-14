package kibana_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccSpacesSavedObjects(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	src, dst := createSpace(t, client), createSpace(t, client)
	srcClient := client.InSpace(src)
	ip := kibana.SavedObjectRef{Type: "index-pattern", ID: randName(t, "acc-ip")}
	createIndexPattern(t, srcClient, ip.ID, "acc-logs-*")

	// Copy, then copy again to get a conflict, and resolve it by overwriting.
	copyReq := &kibana.CopySavedObjectsRequest{Spaces: []string{dst}, Objects: []kibana.SavedObjectRef{ip}, CreateNewCopies: new(false)}
	results, _, err := srcClient.Spaces.CopySavedObjects(ctx, copyReq, nil)
	if err != nil {
		t.Fatalf("CopySavedObjects() error = %v", err)
	}
	if r := results[dst]; r == nil || !r.Success || len(r.SuccessResults) != 1 || r.SuccessResults[0].DestinationID == "" {
		t.Fatalf("CopySavedObjects() = %+v, want one copied object with a destination ID", r)
	}
	results, _, err = srcClient.Spaces.CopySavedObjects(ctx, copyReq, nil)
	if err != nil {
		t.Fatalf("CopySavedObjects(again) error = %v", err)
	}
	r := results[dst]
	if r == nil || r.Success || len(r.Errors) != 1 || r.Errors[0].Detail == nil || r.Errors[0].Detail.Type != kibana.ImportErrorTypeConflict {
		t.Fatalf("CopySavedObjects(again) = %+v, want a conflict", r)
	}
	destID := r.Errors[0].Detail.DestinationID
	results, _, err = srcClient.Spaces.ResolveCopySavedObjectsErrors(ctx, &kibana.ResolveCopySavedObjectsErrorsRequest{
		Objects:         []kibana.SavedObjectRef{ip},
		Retries:         map[string][]kibana.CopySavedObjectsRetry{dst: {{Type: ip.Type, ID: ip.ID, Overwrite: true, DestinationID: destID}}},
		CreateNewCopies: new(false),
	}, nil)
	if err != nil {
		t.Fatalf("ResolveCopySavedObjectsErrors() error = %v", err)
	}
	if r := results[dst]; r == nil || !r.Success || len(r.SuccessResults) != 1 || !r.SuccessResults[0].Overwrite {
		t.Fatalf("ResolveCopySavedObjectsErrors() = %+v, want one overwritten object", r)
	}

	// A missing object fails the whole copy.
	_, _, err = srcClient.Spaces.CopySavedObjects(ctx, &kibana.CopySavedObjectsRequest{
		Spaces: []string{dst}, Objects: []kibana.SavedObjectRef{{Type: "index-pattern", ID: randName(t, "missing")}},
	}, nil)
	if !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("CopySavedObjects(missing object) error = %v, want ErrNotFound", err)
	}

	// Share the object into a third space and back. (Sharing into dst would
	// conflict with the copy there.)
	shared := createSpace(t, client)
	missing := kibana.SavedObjectRef{Type: "index-pattern", ID: randName(t, "missing")}
	updated, _, err := srcClient.Spaces.UpdateObjectsSpaces(ctx, &kibana.UpdateObjectsSpacesRequest{
		Objects: []kibana.SavedObjectRef{ip, missing}, SpacesToAdd: []string{shared},
	}, nil)
	if err != nil {
		t.Fatalf("UpdateObjectsSpaces(add) error = %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("UpdateObjectsSpaces(add) returned %d objects, want 2", len(updated))
	}
	if !slices.Contains(updated[0].Spaces, shared) || updated[0].Error != nil {
		t.Errorf("UpdateObjectsSpaces(add)[0] = %+v, want it in %s", updated[0], shared)
	}
	if updated[1].Error == nil || !errors.Is(updated[1].Error, kibana.ErrNotFound) {
		t.Errorf("UpdateObjectsSpaces(add)[1].Error = %v, want ErrNotFound", updated[1].Error)
	}
	refs, _, err := srcClient.Spaces.GetShareableReferences(ctx, &kibana.GetShareableReferencesRequest{Objects: []kibana.SavedObjectRef{ip, missing}}, nil)
	if err != nil {
		t.Fatalf("GetShareableReferences() error = %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("GetShareableReferences() returned %d objects, want 2", len(refs))
	}
	if !slices.Contains(refs[0].Spaces, src) || !slices.Contains(refs[0].Spaces, shared) || !refs[1].IsMissing {
		t.Errorf("GetShareableReferences() = %+v, %+v; want the object in both spaces and the missing one marked", refs[0], refs[1])
	}
	if _, _, err := srcClient.Spaces.UpdateObjectsSpaces(ctx, &kibana.UpdateObjectsSpacesRequest{
		Objects: []kibana.SavedObjectRef{ip}, SpacesToRemove: []string{shared},
	}, nil); err != nil {
		t.Fatalf("UpdateObjectsSpaces(remove) error = %v", err)
	}

	// Kibana accepts disabling aliases that don't exist.
	if _, err := client.Spaces.DisableLegacyURLAliases(ctx, &kibana.DisableLegacyURLAliasesRequest{Aliases: []kibana.LegacyURLAlias{
		{TargetSpace: src, TargetType: "index-pattern", SourceID: randName(t, "alias")},
	}}, nil); err != nil {
		t.Errorf("DisableLegacyURLAliases() error = %v", err)
	}
}
