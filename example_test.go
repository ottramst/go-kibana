package kibana_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ottramst/go-kibana"
)

func ExampleNewClient() {
	client, err := kibana.NewClient("https://kibana.example.com:5601",
		kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	spaces, _, err := client.Spaces.List(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range spaces {
		fmt.Println(s.ID, s.Name)
	}
}

func ExampleNewClient_basicAuth() {
	// Bound requests with a timeout on the HTTP client, or with a context
	// deadline per call.
	httpClient := &http.Client{Timeout: 30 * time.Second}

	client, err := kibana.NewClient("http://localhost:5601",
		kibana.WithBasicAuth("elastic", os.Getenv("KIBANA_PASSWORD")),
		kibana.WithHTTPClient(httpClient),
		kibana.WithUserAgent("my-app/1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleClient_InSpace() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Requests of the scoped client go to /s/marketing/api/...; client
	// itself still acts on the default space.
	marketing := client.InSpace("marketing")
	fmt.Println(marketing.Space())
}

func ExampleSpacesService_Create() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	space, _, err := client.Spaces.Create(context.Background(), &kibana.Space{
		ID:               "marketing",
		Name:             "Marketing",
		Description:      "Dashboards of the marketing team",
		DisabledFeatures: []string{"dev_tools"},
		Solution:         kibana.SpaceSolutionClassic,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("created space", space.ID)
}

func ExampleSpacesService_Update() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// Update replaces the whole space: read it, change it, write it back.
	space, _, err := client.Spaces.Get(ctx, "marketing")
	if err != nil {
		log.Fatal(err)
	}
	space.Description = "Campaign dashboards"
	if _, _, err := client.Spaces.Update(ctx, space.ID, space); err != nil {
		log.Fatal(err)
	}
}

func ExampleRolesService_CreateOrUpdate() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	role := &kibana.Role{
		Description: "Read logs, and use Discover and dashboards in the marketing space",
		Elasticsearch: kibana.RoleElasticsearch{
			Indices: []kibana.IndexPrivilege{{
				Names:      []string{"logs-*"},
				Privileges: []string{"read", "view_index_metadata"},
			}},
		},
		Kibana: []kibana.KibanaPrivilege{{
			Feature: map[string][]string{
				"discover":  {"read"},
				"dashboard": {"all"},
			},
			Spaces: []string{"marketing"},
		}},
	}
	_, err = client.Roles.CreateOrUpdate(context.Background(), "marketing_analyst", role, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleErrorResponse() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	_, _, err = client.Spaces.Get(context.Background(), "does-not-exist")
	switch {
	case errors.Is(err, kibana.ErrNotFound):
		fmt.Println("no such space")
	case err != nil:
		var errResp *kibana.ErrorResponse
		if errors.As(err, &errResp) {
			fmt.Println(errResp.StatusCode, errResp.Message)
		}
		log.Fatal(err)
	}
}

func ExampleClient_Do() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Call an endpoint the client has no method for yet.
	req, err := client.NewRequest(context.Background(), http.MethodGet, "/api/status", nil)
	if err != nil {
		log.Fatal(err)
	}
	var status struct {
		Version struct {
			Number string `json:"number"`
		} `json:"version"`
	}
	if _, err := client.Do(req, &status); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Kibana", status.Version.Number)
}

func ExampleSpacesService_CopySavedObjects() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Copy a dashboard and the objects it references from the marketing
	// space into the sales space. CreateNewCopies is false so that existing
	// copies are reported as conflicts instead of copied again.
	results, _, err := client.InSpace("marketing").Spaces.CopySavedObjects(context.Background(), &kibana.CopySavedObjectsRequest{
		Spaces:            []string{"sales"},
		Objects:           []kibana.SavedObjectRef{{Type: "dashboard", ID: "7adfa750-4c81-11e8-b3d7-01146121b73d"}},
		IncludeReferences: true,
		CreateNewCopies:   new(false),
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range results["sales"].Errors {
		if e.Error != nil {
			fmt.Printf("%s %s: %s\n", e.Type, e.ID, e.Error.Type)
		}
	}
}

func ExampleRolesService_QueryAll() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Iterate over all roles whose name or description matches "analyst",
	// fetching 50 roles per request.
	for role, err := range client.Roles.QueryAll(context.Background(), &kibana.QueryRolesRequest{Query: "analyst", Size: 50}) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(role.Name)
	}
}

func ExampleUserSessionsService_Invalidate() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Log a user out of all their sessions with the basic auth provider.
	result, _, err := client.UserSessions.Invalidate(context.Background(), &kibana.InvalidateSessionsRequest{
		Match: kibana.SessionMatchQuery,
		Query: &kibana.SessionQuery{Provider: kibana.SessionProvider{Type: "basic"}, Username: "jdoe"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("invalidated sessions:", result.Total)
}

func ExampleStatusService_Get() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Kibana answers 503 when a core service is unavailable. Get then
	// returns the status too, to see which services are affected.
	status, _, err := client.Status.Get(context.Background())
	if status == nil {
		log.Fatal(err)
	}
	fmt.Println("Kibana", status.Version.Number, status.Status.Overall.Level)
	// A degraded Kibana answers 200, so check the levels.
	for name, s := range status.Status.Core {
		if s.Level != kibana.StatusAvailable {
			fmt.Println(name, s.Level, s.Summary)
		}
	}
}
