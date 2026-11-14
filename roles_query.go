package kibana

import (
	"context"
	"iter"
	"net/http"
)

// SortDirection is the direction of a sort.
type SortDirection string

// Sort directions.
const (
	SortDirectionAsc  SortDirection = "asc"
	SortDirectionDesc SortDirection = "desc"
)

// QueryRolesRequest is the request of [RolesService.Query] and
// [RolesService.QueryAll].
type QueryRolesRequest struct {
	_ [0]func()

	// Query is a free-text query on the role name and description.
	Query string `json:"query,omitempty"`

	// From is the offset of the first role to return.
	From int `json:"from,omitempty"`

	// Size is the number of roles to return. Kibana defaults to 10. From
	// plus Size must not exceed 10,000.
	Size int `json:"size,omitempty"`

	// Sort sorts the roles.
	Sort *RoleSort `json:"sort,omitempty"`

	// Filters filters the roles.
	Filters *QueryRolesFilters `json:"filters,omitempty"`
}

// RoleSort sorts roles by a field, such as "name".
type RoleSort struct {
	Field     string        `json:"field"`
	Direction SortDirection `json:"direction"`
}

// QueryRolesFilters filters the roles of a query.
type QueryRolesFilters struct {
	// ShowReservedRoles includes the built-in roles, such as "superuser" or
	// "viewer", which a query leaves out by default.
	ShowReservedRoles bool `json:"showReservedRoles,omitempty"`
}

// QueryRolesResult is a page of roles.
type QueryRolesResult struct {
	Roles []Role `json:"roles"`

	// Count is the number of roles in this page.
	Count int `json:"count"`

	// Total is the number of roles that match the query.
	Total int `json:"total"`
}

// QueryRolesOptions are the options of [RolesService.Query] and [RolesService.QueryAll].
type QueryRolesOptions struct {
	_ [0]func()
}

// Query returns a page of the roles that match the request. A nil request
// returns the first page of all roles except the built-in ones. Use
// [RolesService.QueryAll] to iterate over all pages.
//
// Kibana API: POST /api/security/role/_query
//
// Kibana API docs: https://www.elastic.co/docs/api/doc/kibana/operation/operation-post-security-role-query
func (s *RolesService) Query(ctx context.Context, req *QueryRolesRequest, opt *QueryRolesOptions, opts ...RequestOption) (*QueryRolesResult, *Response, error) {
	if req == nil {
		req = &QueryRolesRequest{}
	}
	result := new(QueryRolesResult)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/security/role/_query", req, result, opt, opts)
	if err != nil {
		return nil, resp, err
	}
	return result, resp, nil
}

// QueryAll returns an iterator over all roles that match the request,
// starting at req.From and fetching req.Size roles per page (100 if zero).
// Roles are sorted by name unless req.Sort is set, so that pages don't
// overlap. If a page fails, the iterator yields the error and stops.
// Elasticsearch rejects pages beyond the first 10,000 roles: once the offset
// plus the page size exceeds 10,000, the iterator yields an [ErrBadRequest]
// error.
//
//	for role, err := range client.Roles.QueryAll(ctx, nil) {
//		if err != nil {
//			return err
//		}
//		fmt.Println(role.Name)
//	}
func (s *RolesService) QueryAll(ctx context.Context, req *QueryRolesRequest, opt *QueryRolesOptions, opts ...RequestOption) iter.Seq2[*Role, error] {
	// Copy the request, including what it points to, so that changes by
	// the caller don't affect later pages.
	base := QueryRolesRequest{}
	if req != nil {
		base = *req
	}
	if base.Sort == nil {
		base.Sort = &RoleSort{Field: "name", Direction: SortDirectionAsc}
	} else {
		sort := *base.Sort
		base.Sort = &sort
	}
	if base.Filters != nil {
		filters := *base.Filters
		base.Filters = &filters
	}
	return paginate(ctx, base.From, base.Size, func(ctx context.Context, from, size int) ([]*Role, int, error) {
		page := base
		page.From, page.Size = from, size
		result, _, err := s.Query(ctx, &page, opt, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Roles), result.Total, nil
	})
}
