package kibana

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// LegacyURLAlias identifies a legacy URL alias: an alias from an object's ID
// before Kibana 8.0 to its current ID.
type LegacyURLAlias struct {
	// TargetSpace is the space of the object the alias points to.
	TargetSpace string `json:"targetSpace"`

	// TargetType is the type of the object the alias points to.
	TargetType string `json:"targetType"`

	// SourceID is the ID the alias is for.
	SourceID string `json:"sourceId"`
}

// DisableLegacyURLAliases disables legacy URL aliases, so that they no
// longer resolve to their target objects. Kibana succeeds for aliases that do
// not exist.
//
// Kibana API: POST /api/spaces/_disable_legacy_url_aliases
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-spaces-disable-legacy-url-aliases
func (s *SpacesService) DisableLegacyURLAliases(ctx context.Context, aliases []LegacyURLAlias, opts ...RequestOption) (*Response, error) {
	if len(aliases) == 0 {
		return nil, errors.New("kibana: aliases must not be empty")
	}
	for i, a := range aliases {
		if a.TargetSpace == "" || a.TargetType == "" || a.SourceID == "" {
			return nil, fmt.Errorf("kibana: alias %d must have a TargetSpace, TargetType and SourceID", i)
		}
	}
	body := struct {
		Aliases []LegacyURLAlias `json:"aliases"`
	}{aliases}
	// Kibana answers 204 No Content.
	return s.client.do(ctx, http.MethodPost, "/api/spaces/_disable_legacy_url_aliases", body, nil, opts)
}
