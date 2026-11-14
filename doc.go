// Package kibana is a client for the Kibana REST API.
//
// Create a client with [NewClient], passing the Kibana URL and an
// authentication option, then use its services to call the API:
//
//	client, err := kibana.NewClient("https://kibana.example.com:5601",
//		kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
//	if err != nil {
//		return err
//	}
//	spaces, _, err := client.Spaces.List(ctx, nil)
//
// If Kibana is served under a base path (server.basePath), include it in the
// URL. Authenticate with [WithAPIKey] (an encoded API key) or
// [WithBasicAuth].
//
// # Methods
//
// The services group the methods by API area, such as [Client.Spaces],
// [Client.Roles] or [Client.SavedObjects]. Every method takes a
// [context.Context] first and an options struct, which can be nil, before
// the variadic [RequestOption]s:
//
//	space, resp, err := client.Spaces.Get(ctx, "marketing", nil)
//
// Methods return the decoded result, if Kibana returns one, the [Response]
// (status code and headers) and an error. A method never returns a result
// together with an error.
//
// # Errors
//
// API errors are returned as [*ErrorResponse], which holds the status code,
// Kibana's message and the raw body. Match them with [errors.Is] against
// sentinel errors such as [ErrNotFound] or [ErrConflict], or use
// [errors.As] for the details:
//
//	_, _, err := client.Spaces.Get(ctx, "marketing", nil)
//	if errors.Is(err, kibana.ErrNotFound) {
//		// The space does not exist.
//	}
//
// Some operations on several objects, such as [SpacesService.CopySavedObjects],
// [SavedObjectsService.Import] and the saved objects bulk methods, succeed
// even when some objects fail: check the per-object errors in the result.
//
// # Spaces
//
// Space-aware endpoints act on the default space unless the client is
// scoped to another space with [WithSpace] or [Client.InSpace]:
//
//	marketing := client.InSpace("marketing")
//	views, _, err := marketing.DataViews.List(ctx, nil)
//
// # Iterators
//
// Paginated endpoints also have an iterator, named after the single-page
// method with an All suffix ([RolesService.QueryAll], [TagsService.ListAll],
// [SavedObjectsService.FindAll]). Iterators fetch pages lazily (100 items
// per page unless the options set a page size), yield an error once and
// stop. They don't return the [Response]. Elasticsearch rejects roles and
// saved objects beyond the first 10,000 results: iterating further yields
// an [ErrBadRequest].
//
//	for role, err := range client.Roles.QueryAll(ctx, nil, nil) {
//		if err != nil {
//			return err
//		}
//		fmt.Println(role.Name)
//	}
//
// # Endpoints without a method
//
// Call other endpoints with [Client.NewRequest] and [Client.Do], which add
// the authentication, the space prefix and the headers Kibana requires, and
// return the same errors as the methods:
//
//	req, err := client.NewRequest(ctx, http.MethodGet, "/api/alerting/rules/_find?per_page=10", nil)
//	if err != nil {
//		return err
//	}
//	var rules struct {
//		Total int `json:"total"`
//	}
//	_, err = client.Do(req, &rules)
//
// # Timeouts and retries
//
// The default HTTP client has no timeout: pass a context with a deadline,
// or set a timeout on the client of [WithHTTPClient]. The client does not
// retry unless you pass [WithRetry].
//
// # Compatibility
//
// The client is tested against the latest patch of the newest Kibana 9.x
// minor and of 8.19. Some endpoints only exist in some versions; Kibana
// returns [ErrNotFound] for the others.
//
// From v1.0.0, the module follows semantic versioning: a breaking change of
// the API needs a new major version. Two exceptions:
//   - Types and methods of endpoints that Kibana marks as technical preview
//     or experimental (tags, short URLs and features) may change in minor
//     releases if Kibana changes them.
//   - Methods of endpoints that Kibana deprecated, such as the saved objects
//     CRUD and bulk methods, keep compiling but return Kibana's error once
//     Kibana removes the endpoint.
//
// Adding fields to structs and methods to services is not a breaking change.
// Whether structs can be compared with == is not part of the API.
package kibana
