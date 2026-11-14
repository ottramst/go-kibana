package kibana_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ottramst/go-kibana"
)

// TestAccRetry runs requests with bodies and errors through the retrying
// transport.
func TestAccRetry(t *testing.T) {
	client := accClient(t, kibana.WithRetry(kibana.RetryPolicy{MinBackoff: 100 * time.Millisecond, MaxBackoff: time.Second}))
	ctx := t.Context()

	id := createSpace(t, client)
	space, _, err := client.Spaces.Get(ctx, id, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	space.Description = "retried"
	if space, _, err = client.Spaces.Update(ctx, id, space, nil); err != nil || space.Description != "retried" {
		t.Errorf("Update() = %+v, %v; want the description", space, err)
	}
	// 404 is not retried: a retry would wait at least 10 seconds.
	slow := accClient(t, kibana.WithRetry(kibana.RetryPolicy{MinBackoff: 10 * time.Second, MaxBackoff: 10 * time.Second}))
	start := time.Now()
	if _, _, err := slow.Spaces.Get(ctx, randName(t, "acc"), nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Get(missing) took %v, want no retries", d)
	}
}
