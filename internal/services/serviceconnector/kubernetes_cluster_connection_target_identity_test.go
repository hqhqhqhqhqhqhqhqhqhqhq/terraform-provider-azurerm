// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceconnector

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-sdk/resource-manager/servicelinker/2024-04-01/servicelinker"
	"github.com/hashicorp/go-azure-sdk/sdk/environments"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	serviceconnectorclient "github.com/hashicorp/terraform-provider-azurerm/internal/services/serviceconnector/client"
)

func TestKubernetesClusterConnectorReadTargetIdentity(t *testing.T) {
	const clusterID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test"
	const accountID = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.Storage/storageAccounts/test"
	id := servicelinker.NewScopedLinkerID(clusterID, "test")

	for _, testCase := range []struct {
		name        string
		configured  string
		returned    string
		previous    string
		expected    string
		imported    bool
		refresh     bool
		replacement bool
	}{
		{name: "account_id", configured: accountID, returned: accountID + "/blobServices/default", expected: accountID},
		{name: "explicit_blob_service_id", configured: accountID + "/blobServices/default", returned: accountID + "/blobServices/default", expected: accountID},
		{name: "explicit_queue_service_id", configured: accountID + "/queueServices/default", returned: accountID + "/queueServices/default", expected: accountID + "/queueServices/default"},
		{name: "refresh_preserves_blob_service_id", configured: accountID + "/blobServices/default", returned: accountID + "/blobServices/default", expected: accountID + "/blobServices/default", refresh: true},
		{name: "previously_flattened_blob_service_id", configured: accountID + "/blobServices/default", previous: accountID, returned: accountID + "/blobServices/default", expected: accountID},
		{name: "import_account_id", configured: accountID, returned: accountID + "/blobServices/default", expected: accountID, imported: true},
		{name: "import_blob_service_id", configured: accountID + "/blobServices/default", returned: accountID + "/blobServices/default", expected: accountID, imported: true},
		{name: "different_account_drift", configured: accountID + "/blobServices/default", returned: accountID + "other/blobServices/default", expected: accountID + "other", replacement: true},
		{name: "different_service_drift", configured: accountID + "/blobServices/default", returned: accountID + "/queueServices/default", expected: accountID + "/queueServices/default", replacement: true},
		{name: "different_service_configuration", configured: accountID + "/queueServices/default", previous: accountID + "/blobServices/default", returned: accountID + "/blobServices/default", expected: accountID, replacement: true},
		{name: "non_default_blob_drift", configured: accountID + "/blobServices/default", returned: accountID + "/blobServices/other", expected: accountID + "/blobServices/other", replacement: true},
		{name: "non_default_blob_not_collapsed", configured: accountID + "/blobServices/other", returned: accountID + "/blobServices/other", expected: accountID + "/blobServices/other"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			requests := make(chan struct{}, 1)
			response := servicelinker.LinkerResource{
				Id: pointer.To(id.ID()), Name: pointer.To("test"),
				Properties: servicelinker.LinkerProperties{
					AuthInfo:      servicelinker.SystemAssignedIdentityAuthInfo{},
					TargetService: servicelinker.AzureResource{Id: pointer.To(testCase.returned)},
					ClientType:    pointer.To(servicelinker.ClientTypeNone),
				},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != id.ID() || req.URL.Query().Get("api-version") != "2024-04-01" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				select {
				case requests <- struct{}{}:
				default:
					t.Error("unexpected additional GET request")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("encoding response: %v", err)
				}
			}))
			defer server.Close()
			client, err := servicelinker.NewServicelinkerClientWithBaseURI(environments.ResourceManagerAPI(server.URL))
			if err != nil {
				t.Fatalf("building client: %v", err)
			}
			client.Client.AuthorizeRequest = nil
			client.Client.DisableRetries = true
			metadata := &clients.Client{
				ServiceConnector: &serviceconnectorclient.Client{ServiceLinkerClient: client},
			}

			wrapped := sdk.WrappedResource(KubernetesClusterConnectorResource{})
			config := map[string]interface{}{
				"name": "test", "kubernetes_cluster_id": clusterID, "target_resource_id": testCase.configured,
				"client_type": "none",
				"authentication": []interface{}{map[string]interface{}{
					"type": "systemAssignedIdentity",
				}},
			}
			resourceConfig := terraform.NewResourceConfigRaw(config)
			if diagnostics := wrapped.Validate(resourceConfig); diagnostics.HasError() {
				t.Fatalf("target configuration rejected: %v", diagnostics)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			priorConfig := map[string]interface{}{}
			if !testCase.imported {
				maps.Copy(priorConfig, config)
				if testCase.previous != "" {
					priorConfig["target_resource_id"] = testCase.previous
				}
			}
			data := schema.TestResourceDataRaw(t, wrapped.Schema, priorConfig)
			data.SetId(id.ID())
			if testCase.imported {
				imported, err := wrapped.Importer.StateContext(ctx, data, metadata)
				if err != nil || len(imported) != 1 {
					t.Fatalf("importing connector: result=%#v, error=%v", imported, err)
				}
				data = imported[0]
			}
			var state *terraform.InstanceState
			if testCase.refresh {
				refreshed, diagnostics := wrapped.RefreshWithoutUpgrade(ctx, data.State(), metadata)
				if diagnostics.HasError() {
					t.Fatalf("refreshing target identity: %v", diagnostics)
				}
				state = refreshed
			} else {
				if diagnostics := wrapped.ReadContext(ctx, data, metadata); diagnostics.HasError() {
					t.Fatalf("reading target identity: %v", diagnostics)
				}
				state = data.State()
			}
			select {
			case <-requests:
			default:
				t.Fatal("read did not send a GET request")
			}
			if state == nil || state.ID != id.ID() {
				t.Fatalf("read changed connector identity: %#v", state)
			}
			if got := state.Attributes["target_resource_id"]; got != testCase.expected {
				t.Fatalf("expected refreshed target %q, got %q", testCase.expected, got)
			}
			diff, err := wrapped.Diff(ctx, state, resourceConfig, metadata)
			if err != nil {
				t.Fatalf("planning target after read: %v", err)
			}
			if testCase.replacement {
				if diff == nil || !diff.RequiresNew() {
					t.Fatalf("expected target replacement, got %#v", diff)
				}
				targetDiff := diff.Attributes["target_resource_id"]
				if targetDiff == nil || targetDiff.Old != testCase.expected || targetDiff.New != testCase.configured {
					t.Fatalf("expected target change %q -> %q, got %#v", testCase.expected, testCase.configured, targetDiff)
				}
				return
			}
			if diff != nil && !diff.Empty() {
				t.Fatalf("unchanged target %q, response %q, state %q: requires replacement=%t, target diff=%#v, all changes=%#v",
					testCase.configured, testCase.returned, state.Attributes["target_resource_id"],
					diff.RequiresNew(), diff.Attributes["target_resource_id"], diff.Attributes)
			}
		})
	}
}
