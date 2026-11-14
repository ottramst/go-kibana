package kibana_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/ottramst/go-kibana"
)

func TestAccRoles(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	role := &kibana.Role{
		Description: "created by go-kibana acceptance tests",
		Metadata:    map[string]any{"owner": "go-kibana"},
		Elasticsearch: kibana.ElasticsearchPrivileges{
			Cluster: []string{"monitor"},
			Indices: []kibana.IndexPrivilege{{
				Names:         []string{"logs-*"},
				Privileges:    []string{"read"},
				FieldSecurity: map[string][]string{"grant": {"*"}, "except": {"secret"}},
			}},
		},
		Kibana: []kibana.KibanaPrivilege{
			{Feature: map[string][]string{"discover": {"read"}, "dashboard": {"all"}}, Spaces: []string{"default"}},
		},
	}
	// Kibana answers 204 No Content; this used to fail with io.EOF.
	if _, err := client.Roles.CreateOrUpdate(ctx, name, role, &kibana.CreateOrUpdateRoleOptions{CreateOnly: true}); err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		if _, err := client.Roles.Delete(ctx, name, nil); err != nil && !errors.Is(err, kibana.ErrNotFound) {
			return err
		}
		return nil
	})

	if _, err := client.Roles.CreateOrUpdate(ctx, name, role, &kibana.CreateOrUpdateRoleOptions{CreateOnly: true}); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("CreateOrUpdate(CreateOnly, existing) error = %v, want ErrConflict", err)
	}

	got, _, err := client.Roles.Get(ctx, name, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != name || got.Description != role.Description || got.Metadata["owner"] != "go-kibana" {
		t.Errorf("Get() = %+v", got)
	}
	if len(got.Kibana) != 1 || !reflect.DeepEqual(got.Kibana[0].Feature, role.Kibana[0].Feature) {
		t.Errorf("Kibana = %+v, want %+v", got.Kibana, role.Kibana)
	}

	// Read, modify, write back.
	got.Description = "updated"
	got.Kibana = append(got.Kibana, kibana.KibanaPrivilege{Base: []string{"read"}, Spaces: []string{"*"}})
	if _, err := client.Roles.CreateOrUpdate(ctx, name, got, nil); err != nil {
		t.Fatalf("CreateOrUpdate(update) error = %v", err)
	}
	got, _, err = client.Roles.Get(ctx, name, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Description != "updated" || len(got.Kibana) != 2 {
		t.Errorf("Get() after update = %+v", got)
	}

	roles, _, err := client.Roles.List(ctx, nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !slices.ContainsFunc(roles, func(r *kibana.Role) bool { return r.Name == name }) {
		t.Errorf("List() does not include %s", name)
	}

	if _, err := client.Roles.Delete(ctx, name, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Roles.Get(ctx, name, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}
