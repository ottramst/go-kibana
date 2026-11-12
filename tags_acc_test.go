package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccTags_unsupported(t *testing.T) {
	client := accClient(t)
	status, _, err := client.Status.Get(t.Context())
	if err != nil {
		t.Fatalf("Status.Get() error = %v", err)
	}
	if compareVersions(status.Version.Number, "9.5.0") >= 0 {
		t.Skip("Kibana 9.5+ has the tags API")
	}
	if _, _, err := client.Tags.List(t.Context(), nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("List() error = %v, want ErrNotFound before Kibana 9.5", err)
	}
}

func TestAccTags(t *testing.T) {
	client := accClient(t)
	skipBelow(t, client, "9.5.0")
	// Tags belong to a space, so a new space keeps the test independent.
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	name := randName(t, "acc")

	created, resp, err := client.Tags.Create(ctx, &kibana.Tag{Name: name, Description: "acceptance test", Color: "#FF0000"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Tags.Delete(ctx, created.ID)
		if errors.Is(err, kibana.ErrNotFound) {
			return nil
		}
		return err
	})
	if resp.StatusCode != http.StatusCreated || created.ID == "" || created.Name != name || created.Color != "#FF0000" || created.Meta.CreatedAt.IsZero() {
		t.Errorf("Create() = %+v (%d), want the new tag", created, resp.StatusCode)
	}

	if _, _, err := client.Tags.Create(ctx, &kibana.Tag{Name: name}); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Create(same name) error = %v, want ErrConflict", err)
	}

	got, _, err := client.Tags.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != name || got.Description != "acceptance test" {
		t.Errorf("Get() = %+v, want %+v", got, created)
	}

	// Read, modify, write: the color and description are kept.
	got.Name = name + "-renamed"
	updated, resp, err := client.Tags.Update(ctx, got.ID, got)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK || updated.Name != got.Name || updated.Color != "#FF0000" || updated.Description != "acceptance test" {
		t.Errorf("Update() = %+v (%d), want the renamed tag with its color and description", updated, resp.StatusCode)
	}

	// Update creates a missing tag with the given ID.
	newID := randName(t, "acc")
	upserted, resp, err := client.Tags.Update(ctx, newID, &kibana.Tag{Name: newID})
	if err != nil {
		t.Fatalf("Update(new) error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Tags.Delete(ctx, newID)
		return err
	})
	if resp.StatusCode != http.StatusCreated || upserted.ID != newID || upserted.Color == "" {
		t.Errorf("Update(new) = %+v (%d), want a created tag with a random color", upserted, resp.StatusCode)
	}

	var ids []string
	for tag, err := range client.Tags.ListAll(ctx, &kibana.ListTagsOptions{PerPage: 1}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		ids = append(ids, tag.ID)
	}
	if len(ids) != 2 {
		t.Errorf("ListAll() = %v, want the 2 tags of the space", ids)
	}

	if _, err := client.Tags.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Tags.Get(ctx, created.ID); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}
