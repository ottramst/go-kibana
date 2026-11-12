package kibana_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

func TestAccShortURLs(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	slug := randName(t, "acc")
	params := map[string]any{"url": "/app/dashboards#/list"}

	created, _, err := client.ShortURLs.Create(ctx, &kibana.CreateShortURLRequest{
		LocatorID: kibana.LegacyShortURLLocator,
		Params:    params,
		Slug:      slug,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.ShortURLs.Delete(ctx, created.ID)
		if errors.Is(err, kibana.ErrNotFound) {
			return nil
		}
		return err
	})
	if created.Slug != slug || created.Locator.ID != kibana.LegacyShortURLLocator || created.Locator.State["url"] != params["url"] {
		t.Errorf("Create() = %+v, want slug %s and the locator", created, slug)
	}
	if time.Since(created.CreateDate) > time.Hour || created.Locator.Version == "" {
		t.Errorf("Create() = %+v, want a recent creation date and the Kibana version", created)
	}

	_, _, err = client.ShortURLs.Create(ctx, &kibana.CreateShortURLRequest{LocatorID: kibana.LegacyShortURLLocator, Params: params, Slug: slug})
	if !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Create(same slug) error = %v, want ErrConflict", err)
	}

	got, _, err := client.ShortURLs.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != created.ID || got.Slug != slug {
		t.Errorf("Get() = %+v, want %+v", got, created)
	}

	resolved, _, err := client.ShortURLs.Resolve(ctx, slug)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.ID != created.ID || resolved.AccessCount != 0 {
		t.Errorf("Resolve() = %+v, want %s with no accesses", resolved, created.ID)
	}

	// Short URLs belong to a space.
	space := createSpace(t, client)
	if _, _, err := client.InSpace(space).ShortURLs.Get(ctx, created.ID); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(other space) error = %v, want ErrNotFound", err)
	}

	if _, err := client.ShortURLs.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.ShortURLs.Get(ctx, created.ID); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.ShortURLs.Resolve(ctx, slug); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Resolve(deleted) error = %v, want ErrNotFound", err)
	}
}
