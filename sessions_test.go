package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestUserSessionsService_Invalidate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/security/session/_invalidate", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "kbn-xsrf", "true")
		testJSONBody(t, r, `{"match":"query","query":{"provider":{"type":"basic"},"username":"probe-nobody"}}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "sessions/invalidate.json"))
	})

	got, _, err := client.UserSessions.Invalidate(context.Background(), &kibana.InvalidateSessionsRequest{
		Match: kibana.SessionMatchQuery,
		Query: &kibana.SessionQuery{Provider: kibana.SessionProvider{Type: "basic"}, Username: "probe-nobody"},
	})
	if err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	if got.Total != 0 {
		t.Errorf("Total = %d, want 0", got.Total)
	}
}

func TestUserSessionsService_Invalidate_all(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /s/x/api/security/session/_invalidate", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"match":"all"}`)
		writeJSON(t, w, http.StatusOK, []byte(`{"total":3}`))
	})
	// The service is available on scoped clients too.
	got, _, err := client.InSpace("x").UserSessions.Invalidate(context.Background(), &kibana.InvalidateSessionsRequest{Match: kibana.SessionMatchAll})
	if err != nil || got.Total != 3 {
		t.Errorf("Invalidate(all) = %+v, %v; want total 3", got, err)
	}
}

func TestUserSessionsService_Invalidate_forbidden(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/security/session/_invalidate", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, []byte(`{"statusCode":403,"error":"Forbidden","message":"Forbidden"}`))
	})
	_, _, err := client.UserSessions.Invalidate(context.Background(), &kibana.InvalidateSessionsRequest{Match: kibana.SessionMatchAll})
	if !errors.Is(err, kibana.ErrForbidden) {
		t.Errorf("Invalidate() error = %v, want ErrForbidden", err)
	}
}

func TestUserSessionsService_Invalidate_invalidArguments(t *testing.T) {
	_, client := setup(t)
	query := &kibana.SessionQuery{Provider: kibana.SessionProvider{Type: "basic"}}
	cases := map[string]*kibana.InvalidateSessionsRequest{
		"nil request":         nil,
		"no match":            {Query: query},
		"unknown match":       {Match: "some"},
		"all with query":      {Match: kibana.SessionMatchAll, Query: query},
		"query without query": {Match: kibana.SessionMatchQuery},
		"query without type":  {Match: kibana.SessionMatchQuery, Query: &kibana.SessionQuery{Username: "u"}},
	}
	for name, req := range cases {
		if _, _, err := client.UserSessions.Invalidate(context.Background(), req); err == nil {
			t.Errorf("Invalidate(%s): error = nil, want error", name)
		}
	}
}
