// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceconnector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-sdk/resource-manager/servicelinker/2022-05-01/links"
	"github.com/hashicorp/go-azure-sdk/resource-manager/servicelinker/2024-04-01/servicelinker"
	sdkclient "github.com/hashicorp/go-azure-sdk/sdk/client"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/features"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	serviceconnectorclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/serviceconnector/client"
)

func TestKubernetesClusterConnectorImportScope(t *testing.T) {
	const resourceGroup = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test"
	for _, testCase := range []struct {
		name  string
		scope string
		valid bool
	}{
		{name: "kubernetes_cluster", scope: resourceGroup + "/providers/Microsoft.ContainerService/managedClusters/test", valid: true},
		{name: "app_service", scope: resourceGroup + "/providers/Microsoft.Web/sites/test"},
		{name: "app_service_slot", scope: resourceGroup + "/providers/Microsoft.Web/sites/test/slots/test"},
		{name: "spring_deployment", scope: resourceGroup + "/providers/Microsoft.AppPlatform/Spring/test/apps/test/deployments/test"},
		{name: "resource_group", scope: resourceGroup},
		{name: "subscription", scope: "/subscriptions/00000000-0000-0000-0000-000000000000"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			wrapped := sdk.WrappedResource(KubernetesClusterConnectorResource{})
			data := schema.TestResourceDataRaw(t, wrapped.Schema, map[string]interface{}{})
			id := servicelinker.NewScopedLinkerID(testCase.scope, "test").ID()
			data.SetId(id)
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			imported, err := wrapped.Importer.StateContext(ctx, data, nil)
			if !testCase.valid {
				if err == nil || !strings.Contains(err.Error(), "Kubernetes Cluster ID") {
					t.Fatalf("expected a Kubernetes Cluster scope error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("importing AKS linker: %v", err)
			}
			if len(imported) != 1 || imported[0].Id() != id {
				t.Fatalf("unexpected import result: %#v", imported)
			}
		})
	}

	for _, input := range []interface{}{123, "not-an-id"} {
		if _, errors := (KubernetesClusterConnectorResource{}).IDValidationFunc()(input, "id"); len(errors) == 0 {
			t.Errorf("expected invalid ID %v to be rejected", input)
		}
	}
}

func TestKubernetesClusterConnectorDeferredConfiguration(t *testing.T) {
	wrapped := sdk.WrappedResource(KubernetesClusterConnectorResource{})
	config := terraform.NewResourceConfigShimmed(cty.ObjectVal(map[string]cty.Value{
		"name":                  cty.StringVal("test"),
		"kubernetes_cluster_id": cty.UnknownVal(cty.String),
		"target_resource_id":    cty.UnknownVal(cty.String),
		"authentication": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"type":   cty.StringVal("secret"),
			"name":   cty.StringVal("test"),
			"secret": cty.UnknownVal(cty.String),
		})}),
		"secret_store": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"key_vault_id": cty.UnknownVal(cty.String),
		})}),
	}), schema.InternalMap(wrapped.Schema).CoreConfigSchema())
	if diagnostics := wrapped.Validate(config); diagnostics.HasError() {
		t.Fatalf("valid deferred configuration rejected: %v", diagnostics)
	}
	diff, err := wrapped.Diff(t.Context(), nil, config, nil)
	if err != nil {
		t.Fatalf("planning deferred configuration: %v", err)
	}
	if diff == nil {
		t.Fatal("expected a creation diff")
	}
	for _, name := range []string{"kubernetes_cluster_id", "target_resource_id", "authentication.0.secret", "secret_store.0.key_vault_id"} {
		attribute := diff.Attributes[name]
		if attribute == nil || !attribute.NewComputed {
			t.Errorf("expected %s to remain unknown in the plan, got %#v", name, attribute)
		}
	}
}

func TestKubernetesClusterConnectorCollectionAndDefaultDiff(t *testing.T) {
	const clusterID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test"
	const targetID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.Storage/storageAccounts/test"
	for _, testCase := range []struct {
		name         string
		oldClient    string
		oldVnet      string
		newSecret    string
		newAuthType  string
		nullOptions  bool
		replacement  bool
		expectedDiff map[string]string
	}{
		{name: "unchanged_omitted_options", oldClient: "none", newSecret: "bar"},
		{name: "unchanged_null_options", oldClient: "none", newSecret: "bar", nullOptions: true},
		{name: "client_default_restored", oldClient: "java", newSecret: "bar", expectedDiff: map[string]string{"client_type": "none"}},
		{name: "vnet_omitted", oldClient: "none", oldVnet: "privateLink", newSecret: "bar", expectedDiff: map[string]string{"vnet_solution": ""}},
		{name: "populated_options_set_to_null", oldClient: "java", oldVnet: "privateLink", newSecret: "bar", nullOptions: true, expectedDiff: map[string]string{"client_type": "none", "vnet_solution": ""}},
		{name: "same_authentication_type_new_secret", oldClient: "none", newSecret: "new-secret", expectedDiff: map[string]string{"authentication.0.secret": "new-secret"}},
		{name: "authentication_type_replacement", oldClient: "none", newAuthType: "systemAssignedIdentity", replacement: true, expectedDiff: map[string]string{"authentication.0.type": "systemAssignedIdentity"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			wrapped := sdk.WrappedResource(KubernetesClusterConnectorResource{})
			state := &terraform.InstanceState{
				ID: servicelinker.NewScopedLinkerID(clusterID, "test").ID(),
				Attributes: map[string]string{
					"name": "test", "kubernetes_cluster_id": clusterID, "target_resource_id": targetID,
					"client_type": testCase.oldClient, "vnet_solution": testCase.oldVnet,
					"authentication.#": "1", "authentication.0.type": "secret",
					"authentication.0.name": "foo", "authentication.0.secret": "bar", "secret_store.#": "0",
				},
			}
			authentication := map[string]cty.Value{
				"type": cty.StringVal("secret"), "name": cty.StringVal("foo"), "secret": cty.StringVal(testCase.newSecret),
			}
			if testCase.newAuthType != "" {
				authentication = map[string]cty.Value{"type": cty.StringVal(testCase.newAuthType)}
			}
			values := map[string]cty.Value{
				"name": cty.StringVal("test"), "kubernetes_cluster_id": cty.StringVal(clusterID), "target_resource_id": cty.StringVal(targetID),
				"authentication": cty.ListVal([]cty.Value{cty.ObjectVal(authentication)}),
			}
			coreSchema := schema.InternalMap(wrapped.Schema).CoreConfigSchema()
			if testCase.nullOptions {
				for _, name := range []string{"client_type", "vnet_solution", "secret_store"} {
					values[name] = cty.NullVal(coreSchema.ImpliedType().AttributeType(name))
				}
			}
			config := terraform.NewResourceConfigShimmed(cty.ObjectVal(values), coreSchema)
			diff, err := wrapped.Diff(t.Context(), state, config, nil)
			if err != nil {
				t.Fatalf("planning configuration: %v", err)
			}
			if len(testCase.expectedDiff) == 0 {
				if diff != nil && !diff.Empty() {
					t.Fatalf("unchanged omitted options planned a change: %#v", diff.Attributes)
				}
				return
			}
			if diff == nil || diff.RequiresNew() != testCase.replacement {
				t.Fatalf("expected replacement=%t, got %#v", testCase.replacement, diff)
			}
			for name, expected := range testCase.expectedDiff {
				attribute := diff.Attributes[name]
				if attribute == nil || attribute.New != expected {
					t.Errorf("expected %s to become %q, got %#v", name, expected, attribute)
				}
			}
		})
	}
}

func TestKubernetesClusterConnectorSingletonValidation(t *testing.T) {
	wrapped := sdk.WrappedResource(KubernetesClusterConnectorResource{})
	for _, attribute := range []string{"authentication", "secret_store"} {
		t.Run(attribute, func(t *testing.T) {
			config := map[string]interface{}{
				"name":                  "test",
				"kubernetes_cluster_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test",
				"target_resource_id":    "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.Storage/storageAccounts/test",
				"authentication":        []interface{}{map[string]interface{}{"type": "systemAssignedIdentity"}},
			}
			block := map[string]interface{}{"type": "systemAssignedIdentity"}
			if attribute == "secret_store" {
				block = map[string]interface{}{"key_vault_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.KeyVault/vaults/test"}
			}
			config[attribute] = []interface{}{block, block}
			if diagnostics := wrapped.Validate(terraform.NewResourceConfigRaw(config)); !diagnostics.HasError() {
				t.Fatal("expected duplicate singleton blocks to be rejected")
			}
		})
	}
}

func TestKubernetesClusterConnectorImportCheck(t *testing.T) {
	for _, allowOverwrite := range []bool{false, true} {
		name := "requires_import"
		if allowOverwrite {
			name = "allow_overwrite"
		}
		t.Run(name, func(t *testing.T) {
			requests := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests <- req.Method
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set(sdkclient.SkipPollingDelayHeader, "true")
				if req.Method != http.MethodGet && req.Method != http.MethodPut {
					t.Errorf("unexpected request method: %s", req.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if _, err := w.Write([]byte(`{"properties":{"provisioningState":"Succeeded"}}`)); err != nil {
					t.Errorf("writing response: %v", err)
				}
			}))
			defer server.Close()

			client, err := servicelinker.NewServicelinkerClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			if err != nil {
				t.Fatalf("building client: %v", err)
			}
			client.Client.AuthorizeRequest = nil
			client.Client.DisableRetries = true

			resource := KubernetesClusterConnectorResource{}
			metadata := sdk.NewResourceMetaData(&clients.Client{
				Features: features.UserFeatures{
					SkipImportCheckOnCreateAndAllowOverwritingExistingResources: allowOverwrite,
				},
				ServiceConnector: &serviceconnectorclient.Client{ServiceLinkerClient: client},
			}, resource)
			if err := metadata.Encode(&KubernetesClusterConnectorResourceModel{
				Name:                "test",
				KubernetesClusterId: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test",
				TargetResourceId:    "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.Storage/storageAccounts/test",
				ClientType:          "none",
				AuthInfo: []AuthInfoModel{{
					Type: "secret", Name: "foo", Secret: "bar",
				}},
			}); err != nil {
				t.Fatalf("encoding resource data: %v", err)
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			err = resource.Create().Func(ctx, metadata)
			if allowOverwrite {
				if err != nil {
					t.Fatalf("creating with the overwrite feature enabled: %v", err)
				}
				if method := <-requests; method != http.MethodPut {
					t.Errorf("expected PUT without an import precheck, got %s", method)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "already exists") {
					t.Fatalf("expected an import-required error, got %v", err)
				}
				if method := <-requests; method != http.MethodGet {
					t.Errorf("expected an existence check, got %s", method)
				}
				if len(requests) != 0 {
					t.Error("unexpected requests after the import-required check")
				}
			}
		})
	}
}

func TestKubernetesClusterConnectorSecretStoreUpdate(t *testing.T) {
	const clusterID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test"
	const targetID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.Storage/storageAccounts/test"
	const vaultID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.KeyVault/vaults/test"

	for _, testCase := range []struct {
		name       string
		oldVaultID string
		newVaultID string
	}{
		{name: "add", newVaultID: vaultID},
		{name: "change", oldVaultID: vaultID, newVaultID: vaultID + "2"},
		{name: "remove", oldVaultID: vaultID},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			id := links.NewScopedLinkerID(clusterID, "test")
			patches := make(chan links.LinkerPatch, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != id.ID() || req.URL.Query().Get("api-version") != "2022-05-01" {
					t.Errorf("unexpected request URL: %s", req.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set(sdkclient.SkipPollingDelayHeader, "true")
				switch req.Method {
				case http.MethodPatch:
					var patch links.LinkerPatch
					if err := json.NewDecoder(req.Body).Decode(&patch); err != nil {
						t.Errorf("decoding update payload: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					patches <- patch
				case http.MethodGet:
				default:
					t.Errorf("unexpected request method: %s", req.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if _, err := w.Write([]byte(`{"properties":{"provisioningState":"Succeeded"}}`)); err != nil {
					t.Errorf("writing response: %v", err)
				}
			}))
			defer server.Close()

			client, err := links.NewLinksClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			if err != nil {
				t.Fatalf("building client: %v", err)
			}
			client.Client.AuthorizeRequest = nil
			client.Client.DisableRetries = true

			resource := KubernetesClusterConnectorResource{}
			wrapped := sdk.WrappedResource(resource)
			state := &terraform.InstanceState{
				ID: id.ID(),
				Attributes: map[string]string{
					"name":                    "test",
					"kubernetes_cluster_id":   clusterID,
					"target_resource_id":      targetID,
					"client_type":             "none",
					"authentication.#":        "1",
					"authentication.0.type":   "secret",
					"authentication.0.name":   "foo",
					"authentication.0.secret": "bar",
					"secret_store.#":          "0",
				},
			}
			if testCase.oldVaultID != "" {
				state.Attributes["secret_store.#"] = "1"
				state.Attributes["secret_store.0.key_vault_id"] = testCase.oldVaultID
			}
			config := map[string]interface{}{
				"name":                  "test",
				"kubernetes_cluster_id": clusterID,
				"target_resource_id":    targetID,
				"client_type":           "none",
				"authentication": []interface{}{map[string]interface{}{
					"type": "secret", "name": "foo", "secret": "bar",
				}},
			}
			if testCase.newVaultID != "" {
				config["secret_store"] = []interface{}{map[string]interface{}{
					"key_vault_id": testCase.newVaultID,
				}}
			}

			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			diff, err := wrapped.Diff(ctx, state, terraform.NewResourceConfigRaw(config), nil)
			if err != nil {
				t.Fatalf("building resource diff: %v", err)
			}
			if diff == nil || diff.RequiresNew() {
				t.Fatal("expected an in-place update")
			}
			data, err := schema.InternalMap(wrapped.Schema).Data(state, diff)
			if err != nil {
				t.Fatalf("building resource data: %v", err)
			}
			if !data.HasChange("secret_store") {
				t.Fatal("expected a secret_store change")
			}
			metadata := sdk.NewResourceMetaData(&clients.Client{
				ServiceConnector: &serviceconnectorclient.Client{LinksClient: client},
			}, resource)
			metadata.ResourceData = data

			if err := resource.Update().Func(ctx, metadata); err != nil {
				t.Fatalf("updating secret_store: %v", err)
			}
			select {
			case patch := <-patches:
				if patch.Properties == nil || patch.Properties.SecretStore == nil {
					t.Fatal("expected an explicit secretStore update")
				}
				actual := patch.Properties.SecretStore.KeyVaultId
				if testCase.newVaultID == "" {
					if actual != nil {
						t.Errorf("expected an empty secretStore object on removal, got %q", *actual)
					}
				} else if actual == nil || *actual != testCase.newVaultID {
					t.Errorf("unexpected keyVaultId: %v", actual)
				}
			default:
				t.Fatal("no update request received")
			}
		})
	}
}
