package kibana_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestFeaturesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/features", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		writeJSON(t, w, http.StatusOK, fixture(t, "features/list.json"))
	})

	got, _, err := client.Features.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() returned %d features, want 2", len(got))
	}

	dashboard := got[0]
	if dashboard.ID != "dashboard_v2" || dashboard.Name != "Dashboard" {
		t.Errorf("features[0] = %s (%s), want dashboard_v2 (Dashboard)", dashboard.ID, dashboard.Name)
	}
	wantCategory := &kibana.FeatureCategory{ID: "kibana", Label: "Analytics", EUIIconType: "logoKibana", Order: 1000}
	if !reflect.DeepEqual(dashboard.Category, wantCategory) {
		t.Errorf("Category = %+v, want %+v", dashboard.Category, wantCategory)
	}
	all := dashboard.Privileges["all"]
	if len(all.App) == 0 || !reflect.DeepEqual(all.App, []string{"dashboards", "kibana"}) || len(all.SavedObject.All) == 0 || len(all.SavedObject.Read) == 0 {
		t.Errorf(`Privileges["all"] = %+v, want apps and saved object types`, all)
	}
	if len(dashboard.SubFeatures) == 0 || len(dashboard.SubFeatures[0].PrivilegeGroups) == 0 {
		t.Fatalf("SubFeatures = %+v, want privilege groups", dashboard.SubFeatures)
	}
	group := dashboard.SubFeatures[0].PrivilegeGroups[0]
	want := kibana.SubFeaturePrivilege{
		ID: "url_create", Name: "Create Short URLs", IncludeIn: "all",
		UI:          []string{"createShortUrl"},
		SavedObject: kibana.FeatureSavedObjectPrivileges{All: []string{"url"}, Read: []string{}},
	}
	if group.GroupType != "independent" || len(group.Privileges) != 1 || !reflect.DeepEqual(group.Privileges[0], want) {
		t.Errorf("privilege group = %+v (%+v), want %+v", group, group.Privileges[0], want)
	}

	// Features with only reserved privileges have no feature privileges.
	monitoring := got[1]
	if monitoring.ID != "monitoring" || monitoring.Privileges != nil || len(monitoring.Reserved) == 0 {
		t.Errorf("features[1] = %+v, want monitoring with only reserved privileges", monitoring)
	}
}

func TestFeaturesService_List_spaceAndErrors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /s/x/api/features", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, []byte(`{"statusCode":403,"error":"Forbidden","message":"Forbidden"}`))
	})
	got, _, err := client.InSpace("x").Features.List(context.Background(), nil)
	if !errors.Is(err, kibana.ErrForbidden) || got != nil {
		t.Errorf("List() = %v, %v; want nil, ErrForbidden", got, err)
	}
}

func TestFeaturesService_List_options(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/features", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "ignoreValidLicenses=true")
		writeJSON(t, w, http.StatusOK, []byte(`[]`))
	})
	got, _, err := client.Features.List(context.Background(), &kibana.ListFeaturesOptions{IgnoreValidLicenses: true})
	if err != nil || len(got) != 0 {
		t.Errorf("List() = %v, %v; want no features", got, err)
	}
}
