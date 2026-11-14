package kibana_test

import (
	"context"
	"encoding/json"
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
	}, nil)
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
	space, _, err := client.Spaces.Get(ctx, "marketing", nil)
	if err != nil {
		log.Fatal(err)
	}
	space.Description = "Campaign dashboards"
	if _, _, err := client.Spaces.Update(ctx, space.ID, space, nil); err != nil {
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
		Elasticsearch: kibana.ElasticsearchPrivileges{
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

	_, _, err = client.Spaces.Get(context.Background(), "does-not-exist", nil)
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
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range results["sales"].Errors {
		if e.Detail != nil {
			fmt.Printf("%s %s: %s\n", e.Type, e.ID, e.Detail.Type)
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
	for role, err := range client.Roles.QueryAll(context.Background(), &kibana.QueryRolesRequest{Query: "analyst", Size: 50}, nil) {
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
	}, nil)
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

	status, _, err := client.Status.Get(context.Background(), nil)
	if err != nil {
		// Kibana answers 503 when a core service is unavailable, and sends
		// the status with it, to see which services are affected.
		if status, ok := kibana.StatusFromError(err); ok {
			log.Fatalf("Kibana is %s: %s", status.Health.Overall.Level, status.Health.Overall.Summary)
		}
		log.Fatal(err)
	}
	fmt.Println("Kibana", status.Version.Number, status.Health.Overall.Level)
	// A degraded Kibana answers 200, so check the levels.
	for name, s := range status.Health.Core {
		if s.Level != kibana.StatusLevelAvailable {
			fmt.Println(name, s.Level, s.Summary)
		}
	}
}

func ExampleShortURLsService_Create() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	u, _, err := client.ShortURLs.Create(context.Background(), &kibana.CreateShortURLRequest{
		LocatorID: kibana.LegacyShortURLLocatorID,
		Params:    map[string]any{"url": "/app/dashboards#/view/my-dashboard"},
		Slug:      "my-dashboard",
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("http://localhost:5601/goto/" + u.Slug)
}

func ExampleTagsService_ListAll() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Tags need Kibana 9.5 or later.
	for tag, err := range client.InSpace("marketing").Tags.ListAll(context.Background(), &kibana.ListTagsOptions{Query: "campaign"}) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(tag.ID, tag.Name, tag.Color)
	}
}

func ExampleSavedObjectsService_Export() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("dashboards.ndjson")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Export all dashboards and the objects they use.
	details, _, err := client.SavedObjects.Export(context.Background(), f, &kibana.ExportSavedObjectsRequest{
		Types:                 []string{"dashboard"},
		IncludeReferencesDeep: true,
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("exported", details.ExportedCount, "objects; missing references:", details.MissingRefCount)
}

func ExampleSavedObjectsService_Import() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Open("dashboards.ndjson")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	result, _, err := client.InSpace("marketing").SavedObjects.Import(context.Background(), f, &kibana.ImportSavedObjectsOptions{Overwrite: true})
	if err != nil {
		log.Fatal(err)
	}
	// Objects can fail although the import succeeded.
	for _, e := range result.Errors {
		fmt.Println("not imported:", e.Error())
	}
	fmt.Println("imported", result.SuccessCount, "objects")
}

func ExampleSavedObjectsService_Get() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	obj, _, err := client.SavedObjects.Get(ctx, "dashboard", "my-dashboard", nil)
	if err != nil {
		log.Fatal(err)
	}
	// Decode the attributes into a struct of your own.
	var attrs struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(obj.Attributes, &attrs); err != nil {
		log.Fatal(err)
	}

	// Update the title. The version makes the update fail with ErrConflict
	// if the dashboard changed since it was read.
	attrs.Title += " (archived)"
	if obj.Attributes, err = json.Marshal(attrs); err != nil {
		log.Fatal(err)
	}
	if _, _, err := client.SavedObjects.Update(ctx, "dashboard", obj.ID, obj, nil); err != nil {
		log.Fatal(err)
	}
}

func ExampleSavedObjectsService_FindAll() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// Find the visualizations that use a data view.
	opt := &kibana.FindSavedObjectsOptions{
		HasReference: []kibana.SavedObjectRef{{Type: "index-pattern", ID: "logs-data-view"}},
		Fields:       []string{"title"},
	}
	for obj, err := range client.SavedObjects.FindAll(context.Background(), []string{"visualization", "lens"}, opt) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(obj.Type, obj.ID)
	}
}

func ExampleDataViewsService_Create() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	dv, _, err := client.InSpace("marketing").DataViews.Create(context.Background(), &kibana.DataView{
		Title:         "logs-*",
		Name:          "Logs",
		TimeFieldName: "@timestamp",
		RuntimeFieldMap: map[string]kibana.RuntimeField{
			"status_class": {Type: "keyword", Script: &kibana.RuntimeFieldScript{Source: `emit(doc["status"].value >= 500 ? "error" : "ok")`}},
		},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("created data view", dv.ID)
}

func ExampleDataViewsService_CreateRuntimeField() {
	client, err := kibana.NewClient("http://localhost:5601", kibana.WithAPIKey(os.Getenv("KIBANA_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	_, _, err = client.DataViews.CreateRuntimeField(context.Background(), "logs-data-view", "status_class", &kibana.RuntimeField{
		Type:        "keyword",
		Script:      &kibana.RuntimeFieldScript{Source: `emit(doc["status"].value >= 500 ? "error" : "ok")`},
		CustomLabel: "Status class",
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
}
