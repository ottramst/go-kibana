package kibana

import (
	"context"
	"errors"
	"net/http"
)

// UserSessionsService handles communication with the user session methods
// of the Kibana API.
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/group/endpoint-user-session
type UserSessionsService service

// SessionMatch selects which sessions [UserSessionsService.Invalidate]
// invalidates.
type SessionMatch string

// Session matches.
const (
	// SessionMatchAll invalidates all sessions of all users.
	SessionMatchAll SessionMatch = "all"

	// SessionMatchQuery invalidates the sessions that match
	// InvalidateSessionsRequest.Query.
	SessionMatchQuery SessionMatch = "query"
)

// InvalidateSessionsRequest is the request of [UserSessionsService.Invalidate].
type InvalidateSessionsRequest struct {
	_ [0]func()

	// Match selects the sessions to invalidate. Required.
	Match SessionMatch `json:"match"`

	// Query selects the sessions to invalidate with SessionMatchQuery. It
	// must be nil with SessionMatchAll.
	Query *SessionQuery `json:"query,omitempty"`
}

// SessionQuery selects sessions by authentication provider and user.
type SessionQuery struct {
	_ [0]func()

	// Provider is the authentication provider of the sessions. Required.
	Provider SessionProvider `json:"provider"`

	// Username limits the query to the sessions of a user.
	Username string `json:"username,omitempty"`
}

// SessionProvider is a Kibana authentication provider.
type SessionProvider struct {
	_ [0]func()

	// Type is the provider type, such as "basic", "token", "saml", "oidc",
	// "kerberos" or "pki". Required.
	Type string `json:"type"`

	// Name is the provider name in the Kibana configuration, to select one
	// of several providers of the type.
	Name string `json:"name,omitempty"`
}

// InvalidateSessionsResult is the result of [UserSessionsService.Invalidate].
type InvalidateSessionsResult struct {
	_ [0]func()

	// Total is the number of sessions that were invalidated.
	Total int `json:"total"`
}

// InvalidateSessionsOptions are the options of [UserSessionsService.Invalidate].
type InvalidateSessionsOptions struct {
	_ [0]func()
}

// Invalidate invalidates user sessions, logging the users out. It requires
// the superuser role and fails with [ErrForbidden] otherwise.
//
// Kibana API: POST /api/security/session/_invalidate
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-security-session-invalidate
func (s *UserSessionsService) Invalidate(ctx context.Context, req *InvalidateSessionsRequest, opt *InvalidateSessionsOptions, opts ...RequestOption) (*InvalidateSessionsResult, *Response, error) {
	if req == nil {
		return nil, nil, errors.New("kibana: request must not be nil")
	}
	switch req.Match {
	case SessionMatchAll:
		// Kibana rejects a query with "all"; an explicit error avoids
		// mistaking a query for a filter of "all".
		if req.Query != nil {
			return nil, nil, errors.New("kibana: Query must be nil with SessionMatchAll")
		}
	case SessionMatchQuery:
		if req.Query == nil || req.Query.Provider.Type == "" {
			return nil, nil, errors.New("kibana: SessionMatchQuery requires Query.Provider.Type")
		}
	default:
		return nil, nil, errors.New("kibana: Match must be SessionMatchAll or SessionMatchQuery")
	}
	result := new(InvalidateSessionsResult)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/security/session/_invalidate", req, result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}
