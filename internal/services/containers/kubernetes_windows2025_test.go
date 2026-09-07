// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package containers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/containers"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

func TestKubernetesNodePoolWindows2025Schema(t *testing.T) {
	schema := containers.Registration{}.SupportedResources()["azurerm_kubernetes_cluster_node_pool"].Schema["os_sku"]
	if !schema.Optional || !schema.Computed || schema.ForceNew || schema.Default != nil {
		t.Fatal("os_sku must retain its Optional+Computed schema and conditional replacement behavior")
	}
	for _, sku := range []string{"AzureLinux", "AzureLinux3", "Ubuntu", "Ubuntu2204", "Ubuntu2404", "Windows2019", "Windows2022", "Windows2025"} {
		if _, errors := schema.ValidateFunc(sku, "os_sku"); len(errors) != 0 {
			t.Fatalf("expected %s to be accepted: %v", sku, errors)
		}
	}
	for _, sku := range []string{"windows2025", "AzureContainerLinux", "invalid"} {
		if _, errors := schema.ValidateFunc(sku, "os_sku"); len(errors) == 0 {
			t.Fatalf("expected %s to be rejected", sku)
		}
	}
	defaultSchema := containers.SchemaDefaultNodePool().Elem.(*pluginsdk.Resource).Schema["os_sku"]
	if _, errors := defaultSchema.ValidateFunc("Windows2025", "default_node_pool.0.os_sku"); len(errors) == 0 {
		t.Fatal("Windows2025 must not be accepted for the Linux system default node pool")
	}
}

func TestKubernetesNodePoolWindows2025FIPS(t *testing.T) {
	const unknownValue = "74D93920-ED26-11E3-AC10-0800200C9A66" // Legacy SDK unknown-value sentinel.
	resource := containers.Registration{}.SupportedResources()["azurerm_kubernetes_cluster_node_pool"]
	for _, test := range []struct {
		name    string
		sku     string
		fips    interface{}
		wantErr bool
	}{
		{name: "omitted", sku: "Windows2025", wantErr: true},
		{name: "disabled", sku: "Windows2025", fips: false, wantErr: true},
		{name: "enabled", sku: "Windows2025", fips: true},
		{name: "unknown", sku: "Windows2025", fips: unknownValue},
		{name: "unknown SKU", sku: unknownValue},
		{name: "computed SKU"},
		{name: "Windows2019 unchanged", sku: "Windows2019"},
		{name: "Windows2022 unchanged", sku: "Windows2022"},
		{name: "Ubuntu unchanged", sku: "Ubuntu"},
		{name: "AzureLinux unchanged", sku: "AzureLinux"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := map[string]interface{}{
				"name":                  "wnp25",
				"kubernetes_cluster_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test",
				"vm_size":               "Standard_D4s_v3",
				"os_type":               "Windows",
				"os_sku":                test.sku,
			}
			if test.fips != nil {
				config["fips_enabled"] = test.fips
			}
			_, err := resource.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(config), nil)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "`fips_enabled` must be `true`") {
					t.Fatalf("expected Windows2025 FIPS validation error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestKubernetesNodePoolWindows2025Replacement(t *testing.T) {
	resource := containers.Registration{}.SupportedResources()["azurerm_kubernetes_cluster_node_pool"]
	for _, test := range []struct {
		name    string
		oldSKU  string
		newSKU  string
		osType  string
		replace bool
	}{
		{name: "Windows2022 to Windows2025", oldSKU: "Windows2022", newSKU: "Windows2025", osType: "Windows", replace: true},
		{name: "Windows2025 to Windows2022", oldSKU: "Windows2025", newSKU: "Windows2022", osType: "Windows", replace: true},
		{name: "Windows2025 unchanged", oldSKU: "Windows2025", newSKU: "Windows2025", osType: "Windows"},
		{name: "Linux migration unchanged", oldSKU: "Ubuntu", newSKU: "AzureLinux", osType: "Linux"},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := map[string]interface{}{
				"name":                  "wnp25",
				"kubernetes_cluster_id": "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test/providers/Microsoft.ContainerService/managedClusters/test",
				"vm_size":               "Standard_D4s_v3",
				"os_type":               test.osType,
				"os_sku":                test.oldSKU,
				"fips_enabled":          true,
			}
			data := schema.TestResourceDataRaw(t, resource.Schema, config)
			data.SetId(config["kubernetes_cluster_id"].(string) + "/agentPools/wnp25")
			state := data.State()
			config["os_sku"] = test.newSKU
			diff, err := resource.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
			if err != nil {
				t.Fatal(err)
			}
			replace := diff != nil && diff.RequiresNew()
			if replace != test.replace {
				t.Fatalf("expected replacement %t, got %t", test.replace, replace)
			}
		})
	}
}
