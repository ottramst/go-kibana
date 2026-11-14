package kibana_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccRolesQueryAndBulk(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	prefix := randName(t, "acc")
	names := []string{prefix + "-1", prefix + "-2", prefix + "-3"}
	cleanup(t, func(ctx context.Context) error {
		var errs []error
		for _, n := range append(names, prefix+"-bad") {
			if _, err := client.Roles.Delete(ctx, n, nil); err != nil && !errors.Is(err, kibana.ErrNotFound) {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	})

	role := func(cluster string) *kibana.Role {
		return &kibana.Role{
			Elasticsearch: kibana.ElasticsearchPrivileges{Cluster: []string{cluster}},
			Kibana:        []kibana.KibanaPrivilege{{Base: []string{"read"}, Spaces: []string{"default"}}},
		}
	}

	// Create in bulk.
	result, _, err := client.Roles.BulkCreateOrUpdate(ctx, map[string]*kibana.Role{
		names[0]: role("monitor"), names[1]: role("monitor"), names[2]: role("monitor"),
	}, nil)
	if err != nil {
		t.Fatalf("BulkCreateOrUpdate(create) error = %v", err)
	}
	created := slices.Sorted(slices.Values(result.Created))
	if !reflect.DeepEqual(created, names) || len(result.Errors) != 0 {
		t.Fatalf("BulkCreateOrUpdate(create) = %+v, want %v created", result, names)
	}

	// Unchanged, changed and invalid roles.
	result, _, err = client.Roles.BulkCreateOrUpdate(ctx, map[string]*kibana.Role{
		names[0]: role("monitor"), names[1]: role("manage"), prefix + "-bad": role("not_a_privilege"),
	}, nil)
	if err != nil {
		t.Fatalf("BulkCreateOrUpdate(update) error = %v", err)
	}
	if !reflect.DeepEqual(result.Noop, names[:1]) || !reflect.DeepEqual(result.Updated, names[1:2]) {
		t.Errorf("BulkCreateOrUpdate(update) = %+v, want %s unchanged and %s updated", result, names[0], names[1])
	}
	if e, ok := result.Errors[prefix+"-bad"]; !ok || e.Reason == "" {
		t.Errorf("Errors = %+v, want an error for %s-bad", result.Errors, prefix)
	}

	// One page.
	page, _, err := client.Roles.Query(ctx, &kibana.QueryRolesRequest{
		Query: prefix, Size: 2, Sort: &kibana.RoleSort{Field: "name", Direction: kibana.SortDirectionDesc},
	}, nil)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if page.Total != 3 || page.Count != 2 || len(page.Roles) != 2 || page.Roles[0].Name != names[2] {
		t.Errorf("Query() = total %d, count %d, first %q; want 3, 2, %q", page.Total, page.Count, firstName(page.Roles), names[2])
	}

	// All pages.
	var all []string
	for r, err := range client.Roles.QueryAll(ctx, &kibana.QueryRolesRequest{Query: prefix, Size: 2}, nil) {
		if err != nil {
			t.Fatalf("QueryAll() error = %v", err)
		}
		all = append(all, r.Name)
	}
	if !reflect.DeepEqual(all, names) {
		t.Errorf("QueryAll() = %v, want %v", all, names)
	}

	// Built-in roles only with ShowReservedRoles.
	reserved, _, err := client.Roles.Query(ctx, &kibana.QueryRolesRequest{Query: "superuser", Filters: &kibana.QueryRolesFilters{ShowReservedRoles: true}}, nil)
	if err != nil {
		t.Fatalf("Query(reserved) error = %v", err)
	}
	if !slices.ContainsFunc(reserved.Roles, func(r *kibana.Role) bool { return r.Name == "superuser" }) {
		t.Errorf("Query(ShowReservedRoles) does not return superuser")
	}
}

func firstName(roles []*kibana.Role) string {
	if len(roles) == 0 {
		return ""
	}
	return roles[0].Name
}
