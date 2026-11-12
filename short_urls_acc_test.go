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
		t.Errorf("Resolve() = %+v, want %s as it was before the access", resolved, created.ID)
	}
	// Resolve counts as an access, recorded in the background; Get does not.
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, _, err = client.ShortURLs.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.AccessCount == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if got.AccessCount != 1 || !got.AccessDate.After(created.AccessDate) {
		t.Errorf("after Resolve: AccessCount = %d, AccessDate = %v; want 1 and later than %v", got.AccessCount, got.AccessDate, created.AccessDate)
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
