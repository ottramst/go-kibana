package kibana_test

import (
	"errors"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccUserSessions(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	// No user has this name, so no session is invalidated.
	got, _, err := client.UserSessions.Invalidate(ctx, &kibana.InvalidateSessionsRequest{
		Match: kibana.SessionMatchQuery,
		Query: &kibana.SessionQuery{Provider: kibana.SessionProvider{Type: "basic"}, Username: randName(t, "nobody")},
	})
	if errors.Is(err, kibana.ErrForbidden) {
		t.Skip("invalidating sessions needs the superuser role")
	}
	if err != nil {
		t.Fatalf("Invalidate(query) error = %v", err)
	}
	if got.Total != 0 {
		t.Errorf("Invalidate(query).Total = %d, want 0", got.Total)
	}

	// Invalidating all sessions logs everyone out, so only on the local
	// compose stack.
	if localStack() {
		if _, _, err := client.UserSessions.Invalidate(ctx, &kibana.InvalidateSessionsRequest{Match: kibana.SessionMatchAll}); err != nil {
			t.Errorf("Invalidate(all) error = %v", err)
		}
	}
}
