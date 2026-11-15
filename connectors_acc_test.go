package kibana_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ottramst/go-kibana"
)

// createConnector creates a connector and deletes it when the test ends.
func createConnector(t *testing.T, client *kibana.Client, c *kibana.Connector) *kibana.Connector {
	t.Helper()
	created, _, err := client.Connectors.Create(t.Context(), c, nil)
	if err != nil {
		t.Fatalf("creating connector %s: %v", c.Name, err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Connectors.Delete(ctx, created.ID, nil)
		if errors.Is(err, kibana.ErrNotFound) {
			return nil
		}
		return err
	})
	return created
}

func TestAccConnectors(t *testing.T) {
	client := accClient(t)
	// Connectors belong to a space, so a new space keeps the list
	// independent.
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	name := randName(t, "acc")

	// Predefined IDs must be UUIDs on Kibana 8.19.
	id := randUUID(t)
	created := createConnector(t, client, &kibana.Connector{ID: id, Name: name, ConnectorTypeID: kibana.ConnectorTypeIDServerLog})
	if created.ID != id || created.Name != name || created.ConnectorTypeID != kibana.ConnectorTypeIDServerLog || created.IsPreconfigured {
		t.Errorf("Create() = %+v, want the new connector", created)
	}
	if _, _, err := client.Connectors.Create(ctx, &kibana.Connector{ID: id, Name: name, ConnectorTypeID: kibana.ConnectorTypeIDServerLog}, nil); !errors.Is(err, kibana.ErrConflict) {
		t.Errorf("Create(same ID) error = %v, want ErrConflict", err)
	}
	generated := createConnector(t, client, &kibana.Connector{Name: name + "-generated", ConnectorTypeID: kibana.ConnectorTypeIDServerLog})
	if generated.ID == "" {
		t.Errorf("Create() = %+v, want a generated ID", generated)
	}

	got, _, err := client.Connectors.Get(ctx, id, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != name || compactJSON(t, got.Config) != "{}" {
		t.Errorf("Get() = %+v, want %+v", got, created)
	}

	// Read, modify, write.
	got.Name = name + "-renamed"
	updated, _, err := client.Connectors.Update(ctx, id, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != got.Name {
		t.Errorf("Update() = %+v, want the renamed connector", updated)
	}

	list, _, err := client.Connectors.List(ctx, nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	ids := make([]string, 0, len(list))
	for _, c := range list {
		if !c.IsPreconfigured {
			ids = append(ids, c.ID)
		}
	}
	slices.Sort(ids)
	if want := []string{generated.ID, id}; !slices.Equal(ids, slices.Sorted(slices.Values(want))) {
		t.Errorf("List() IDs = %v, want %v", ids, want)
	}

	result, _, err := client.Connectors.Execute(ctx, id, &kibana.ExecuteConnectorRequest{Params: json.RawMessage(`{"message":"go-kibana acceptance test"}`)}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != kibana.ExecuteConnectorStatusOK || result.ConnectorID != id {
		t.Errorf("Execute() = %+v, want ok", result)
	}
	// Invalid params fail the execution, not the request.
	result, _, err = client.Connectors.Execute(ctx, id, nil, nil)
	if err != nil {
		t.Fatalf("Execute(no params) error = %v", err)
	}
	if result.Status != kibana.ExecuteConnectorStatusError || result.ErrorSource != kibana.ExecuteConnectorErrorSourceUser || result.Message == "" {
		t.Errorf("Execute(no params) = %+v, want a user error", result)
	}

	if _, err := client.Connectors.Delete(ctx, id, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Connectors.Get(ctx, id, nil); !errors.Is(err, kibana.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

// TestAccConnectors_secrets checks that an update without secrets removes
// them: a webhook to Elasticsearch fails with 401 afterwards.
func TestAccConnectors_secrets(t *testing.T) {
	client := accClient(t)
	if !localStack() {
		t.Skip("needs the compose stack, where Kibana reaches Elasticsearch at http://elasticsearch:9200")
	}
	client = client.InSpace(createSpace(t, client))
	ctx := t.Context()
	config := json.RawMessage(`{"url":"http://elasticsearch:9200/_count","method":"post","hasAuth":true,"authType":"webhook-authentication-basic","headers":{"Content-Type":"application/json"}}`)
	secrets, err := json.Marshal(map[string]string{"user": envOr("KIBANA_USERNAME", "elastic"), "password": envOr("KIBANA_PASSWORD", "changeme")})
	if err != nil {
		t.Fatal(err)
	}
	c := createConnector(t, client, &kibana.Connector{Name: randName(t, "acc"), ConnectorTypeID: kibana.ConnectorTypeIDWebhook, Config: config, Secrets: secrets})
	params := &kibana.ExecuteConnectorRequest{Params: json.RawMessage(`{"body":"{}"}`)}

	execute := func() *kibana.ExecuteConnectorResult {
		t.Helper()
		result, _, err := client.Connectors.Execute(ctx, c.ID, params, nil)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		return result
	}
	if result := execute(); result.Status != kibana.ExecuteConnectorStatusOK {
		t.Fatalf("Execute() = %+v, want ok", result)
	}

	got, _, err := client.Connectors.Get(ctx, c.ID, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Secrets != nil {
		t.Errorf("Get().Secrets = %s, want nil", got.Secrets)
	}
	if _, _, err := client.Connectors.Update(ctx, c.ID, got, nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if result := execute(); result.Status != kibana.ExecuteConnectorStatusError || !strings.Contains(result.ServiceMessage, "401") {
		t.Errorf("Execute(after update without secrets) = %+v, want a 401 error", result)
	}
}

func TestAccConnectors_ListTypes(t *testing.T) {
	client := accClient(t)
	types, _, err := client.Connectors.ListTypes(t.Context(), &kibana.ListConnectorTypesOptions{FeatureID: "alerting"})
	if err != nil {
		t.Fatalf("ListTypes() error = %v", err)
	}
	i := slices.IndexFunc(types, func(ct *kibana.ConnectorType) bool { return ct.ID == kibana.ConnectorTypeIDServerLog })
	if i < 0 {
		t.Fatalf("ListTypes() has no %s type", kibana.ConnectorTypeIDServerLog)
	}
	if ct := types[i]; !ct.Enabled || ct.MinimumLicenseRequired != kibana.LicenseLevelBasic || !slices.Contains(ct.SupportedFeatureIDs, "alerting") {
		t.Errorf("ListTypes() %s = %+v", ct.ID, ct)
	}
}
