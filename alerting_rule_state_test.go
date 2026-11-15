package kibana_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAlertingService_ruleActions(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		action string
		body   string
		call   func(c *kibana.Client, id string) (*kibana.Response, error)
	}{
		{"_enable", "", func(c *kibana.Client, id string) (*kibana.Response, error) {
			return c.Alerting.EnableRule(ctx, id, nil)
		}},
		{"_disable", "", func(c *kibana.Client, id string) (*kibana.Response, error) {
			return c.Alerting.DisableRule(ctx, id, nil)
		}},
		{"_disable", `{"untrack":true}`, func(c *kibana.Client, id string) (*kibana.Response, error) {
			return c.Alerting.DisableRule(ctx, id, &kibana.DisableRuleOptions{Untrack: true})
		}},
		{"_mute_all", "", func(c *kibana.Client, id string) (*kibana.Response, error) {
			return c.Alerting.MuteAllAlerts(ctx, id, nil)
		}},
		{"_unmute_all", "", func(c *kibana.Client, id string) (*kibana.Response, error) {
			return c.Alerting.UnmuteAllAlerts(ctx, id, nil)
		}},
		{"_update_api_key", "", func(c *kibana.Client, id string) (*kibana.Response, error) {
			return c.Alerting.UpdateRuleAPIKey(ctx, id, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.action+tt.body, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /s/x/api/alerting/rule/{id}/"+tt.action, func(w http.ResponseWriter, r *http.Request) {
				testHeader(t, r, "kbn-xsrf", "true")
				if got, want := r.URL.EscapedPath(), "/s/x/api/alerting/rule/a%2Fb%3F/"+tt.action; got != want {
					t.Errorf("path = %q, want %q", got, want)
				}
				if tt.body != "" {
					testJSONBody(t, r, tt.body)
				} else if b, _ := io.ReadAll(r.Body); len(b) != 0 {
					t.Errorf("body = %s, want none", b)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			mux.HandleFunc("POST /s/x/api/alerting/rule/nope/"+tt.action, func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusNotFound, fixture(t, "alerting/rule_get_notfound.json"))
			})
			c := client.InSpace("x")
			resp, err := tt.call(c, "a/b?")
			if err != nil || resp.StatusCode != http.StatusNoContent {
				t.Fatalf("%s = %v, %v, want 204", tt.action, resp, err)
			}
			if _, err := tt.call(c, "nope"); !errors.Is(err, kibana.ErrNotFound) {
				t.Errorf("%s(missing) error = %v, want ErrNotFound", tt.action, err)
			}
			if _, err := tt.call(c, ""); err == nil {
				t.Errorf("%s(empty ID) error = nil, want an error", tt.action)
			}
		})
	}
}
