// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceconnector_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	sdkcty "github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/provider"
	"github.com/zclconf/go-cty/cty"
)

func TestKubernetesClusterConnectorFixtureSchemas(t *testing.T) {
	data := acceptance.TestData{
		RandomInteger: 123456789,
		RandomString:  "abcde",
		ResourceType:  "azurerm_kubernetes_cluster_connection",
		ResourceName:  "azurerm_kubernetes_cluster_connection.test",
		Locations:     acceptance.Regions{Primary: "eastus"},
	}
	r := ServiceConnectorKubernetesClusterResource{}
	p := provider.AzureProvider()
	for _, testCase := range []struct {
		name      string
		config    string
		resources int
	}{
		{name: "cosmosdb_secret", config: r.cosmosdbWithSecretAuth(data), resources: 6},
		{name: "requires_import", config: r.requiresImport(data), resources: 7},
		{name: "cosmosdb_principal", config: r.cosmosdbWithServicePrincipalSecretAuth(data, "somesecret"), resources: 7},
		{name: "cosmosdb_principal_updated", config: r.cosmosdbWithServicePrincipalSecretAuth(data, "somesecret2"), resources: 7},
		{name: "storage_default", config: r.storageBlob(data, ""), resources: 4},
		{name: "storage_java", config: r.storageBlob(data, "java"), resources: 4},
		{name: "secret_store", config: r.secretStore(data, true), resources: 5},
		{name: "secret_store_omitted", config: r.secretStore(data, false), resources: 5},
		{name: "complete", config: r.complete(data, "privateLink"), resources: 6},
		{name: "complete_vnet_omitted", config: r.complete(data, ""), resources: 6},
		{name: "shared_app_service_secret_store", config: ServiceConnectorAppServiceResource{}.secretStore(data), resources: 8},
		{name: "shared_function_app_secret_store", config: FunctionAppConnectorResource{}.secretStore(data), resources: 6},
		{name: "shared_spring_cloud_secret_store", config: ServiceConnectorSpringCloudResource{}.secretStore(data), resources: 7},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			body, context := connectorFixtureBody(t, testCase.config)
			resources := 0
			for _, block := range body.Blocks {
				var definitions map[string]*schema.Resource
				switch block.Type {
				case "resource":
					definitions = p.ResourcesMap
					resources++
				case "data":
					definitions = p.DataSourcesMap
				default:
					continue
				}
				resource := definitions[block.Labels[0]]
				if resource == nil {
					t.Fatalf("unregistered %s %q", block.Type, block.Labels[0])
				}
				config := connectorFixtureConfig(t, block.Body, context)
				if diagnostics := resource.Validate(terraform.NewResourceConfigRaw(config)); diagnostics.HasError() {
					t.Errorf("%s %s.%s: %v", block.Type, block.Labels[0], block.Labels[1], diagnostics)
				}
			}
			if resources != testCase.resources {
				t.Fatalf("validated %d resource blocks; expected %d", resources, testCase.resources)
			}
		})
	}

	t.Run("missing_key_vault_authorization_rejected", func(t *testing.T) {
		body, context := connectorFixtureBody(t, r.secretStore(data, true))
		for _, block := range body.Blocks {
			if block.Type != "resource" || block.Labels[0] != "azurerm_key_vault" {
				continue
			}
			config := connectorFixtureConfig(t, block.Body, context)
			delete(config, "rbac_authorization_enabled")
			diagnostics := p.ResourcesMap["azurerm_key_vault"].Validate(terraform.NewResourceConfigRaw(config))
			if !diagnostics.HasError() || !strings.Contains(fmt.Sprint(diagnostics), "rbac_authorization_enabled") {
				t.Fatalf("expected missing authorization argument diagnostic, got: %v", diagnostics)
			}
			return
		}
		t.Fatal("missing Key Vault fixture")
	})

	t.Run("missing_node_provisioning_profile_rejected", func(t *testing.T) {
		body, context := connectorFixtureBody(t, r.storageBlob(data, ""))
		for _, block := range body.Blocks {
			if block.Type != "resource" || block.Labels[0] != "azurerm_kubernetes_cluster" {
				continue
			}
			config := connectorFixtureConfig(t, block.Body, context)
			delete(config, "node_provisioning_profile")
			diagnostics := p.ResourcesMap["azurerm_kubernetes_cluster"].Validate(terraform.NewResourceConfigRaw(config))
			if !diagnostics.HasError() || !strings.Contains(fmt.Sprint(diagnostics), "node_provisioning_profile") {
				t.Fatalf("expected missing node provisioning profile diagnostic, got: %v", diagnostics)
			}
			return
		}
		t.Fatal("missing Kubernetes cluster fixture")
	})
}

func connectorFixtureBody(t *testing.T, config string) (*hclsyntax.Body, *hcl.EvalContext) {
	t.Helper()
	file, diagnostics := hclsyntax.ParseConfig([]byte(config), "fixture.tf", hcl.InitialPos)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	body := file.Body.(*hclsyntax.Body)
	context := &hcl.EvalContext{Variables: map[string]cty.Value{}}
	if diagnostics := hclsyntax.VisitAll(body, func(node hclsyntax.Node) hcl.Diagnostics {
		if expression, ok := node.(hclsyntax.Expression); ok {
			for _, variable := range expression.Variables() {
				context.Variables[variable.RootName()] = cty.DynamicVal
			}
		}
		return nil
	}); diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	return body, context
}

func connectorFixtureConfig(t *testing.T, body *hclsyntax.Body, context *hcl.EvalContext) map[string]interface{} {
	t.Helper()
	config := make(map[string]interface{})
	for name, attribute := range body.Attributes {
		value, diagnostics := attribute.Expr.Value(context)
		if diagnostics.HasErrors() {
			t.Fatal(diagnostics.Error())
		}
		config[name] = connectorFixtureValue(value)
	}
	for _, block := range body.Blocks {
		if block.Type == "lifecycle" {
			continue
		}
		blocks, _ := config[block.Type].([]interface{})
		config[block.Type] = append(blocks, connectorFixtureConfig(t, block.Body, context))
	}
	return config
}

func connectorFixtureValue(value cty.Value) interface{} {
	if !value.IsKnown() {
		// Use the SDK's own unknown-value conversion rather than invent resource IDs.
		unknownSchema := schema.InternalMap{"value": {Type: schema.TypeString, Optional: true}}
		config := terraform.NewResourceConfigShimmed(
			sdkcty.ObjectVal(map[string]sdkcty.Value{"value": sdkcty.UnknownVal(sdkcty.String)}),
			unknownSchema.CoreConfigSchema(),
		)
		return config.Raw["value"]
	}
	if value.IsNull() {
		return nil
	}
	switch value.Type() {
	case cty.String:
		return value.AsString()
	case cty.Bool:
		return value.True()
	case cty.Number:
		number := value.AsBigFloat()
		if integer, accuracy := number.Int64(); accuracy == big.Exact && int64(int(integer)) == integer {
			return int(integer)
		}
		decimal, _ := number.Float64()
		return decimal
	}
	if value.Type().IsObjectType() || value.Type().IsMapType() {
		result := make(map[string]interface{})
		for iterator := value.ElementIterator(); iterator.Next(); {
			key, element := iterator.Element()
			result[key.AsString()] = connectorFixtureValue(element)
		}
		return result
	}
	result := make([]interface{}, 0)
	for iterator := value.ElementIterator(); iterator.Next(); {
		_, element := iterator.Element()
		result = append(result, connectorFixtureValue(element))
	}
	return result
}
