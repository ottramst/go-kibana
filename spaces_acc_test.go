package kibana_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccSpaces(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	id := randName(t, "acc")

	created, _, err := client.Spaces.Create(ctx, &kibana.Space{
		ID:               id,
		Name:             "Acceptance " + id,
		Description:      "created by go-kibana acceptance tests",
		Color:            "#aabbcc",
		Initials:         "AT",
		DisabledFeatures: []string{"dev_tools"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		if _, err := client.Spaces.Delete(ctx, id); err != nil && !errors.Is(err, kibana.ErrNotFound) {
			return err
		}
		return nil
	})
	if created.ID != id || created.Initials != "AT" {
		t.Errorf("Create() = %+v", created)
	}

	if _, _, err := client.Spaces.Create(ctx, &kibana.Space{ID: id, Name: "duplicate"}); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Create(duplicate) error = %v, want ErrConflict", err)
	}

	space, _, err := client.Spaces.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !slices.Equal(space.DisabledFeatures, []string{"dev_tools"}) {
		t.Errorf("DisabledFeatures = %v, want [dev_tools]", space.DisabledFeatures)
	}

	// Read, modify, write back: the other settings must be kept.
	space.Description = "updated"
	updated, _, err := client.Spaces.Update(ctx, id, space)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Description != "updated" || updated.Color != "#aabbcc" {
		t.Errorf("Update() = %+v, want new description and unchanged color", updated)
	}

	spaces, _, err := client.Spaces.List(ctx, &kibana.ListSpacesOptions{IncludeAuthorizedPurposes: true})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	i := slices.IndexFunc(spaces, func(s *kibana.Space) bool { return s.ID == id })
	if i < 0 {
		t.Fatalf("List() does not include %s", id)
	}
	if !spaces[i].AuthorizedPurposes["any"] {
		t.Errorf("AuthorizedPurposes = %v, want any=true", spaces[i].AuthorizedPurposes)
	}

	// The default space, read with a client scoped to the new space, can be
	// written back unchanged although it is reserved.
	def, _, err := client.InSpace(id).Spaces.Get(ctx, kibana.DefaultSpace)
	if err != nil {
		t.Fatalf("InSpace().Get(default) error = %v", err)
	}
	if !def.Reserved {
		t.Errorf("default space Reserved = false, want true")
	}
	if _, _, err := client.Spaces.Update(ctx, kibana.DefaultSpace, def); err != nil {
		t.Errorf("Update(default) error = %v", err)
	}

	if _, err := client.Spaces.Delete(ctx, id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Spaces.Get(ctx, id); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}
