// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceconnector_test

import (
	"slices"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/services/serviceconnector"
)

func TestServiceConnectorRegistration(t *testing.T) {
	resources := serviceconnector.Registration{}.Resources()
	names := make([]string, 0, len(resources))
	for _, resource := range resources {
		names = append(names, resource.ResourceType())
	}

	if !slices.IsSorted(names) {
		t.Errorf("resources are not alphabetized: %v", names)
	}

	for _, name := range []string{
		"azurerm_app_service_connection",
		"azurerm_function_app_connection",
		"azurerm_kubernetes_cluster_connection",
		"azurerm_spring_cloud_connection",
	} {
		t.Run(name, func(t *testing.T) {
			count := 0
			for _, registered := range names {
				if registered == name {
					count++
				}
			}
			if count != 1 {
				t.Errorf("expected one registration for %s, got %d", name, count)
			}
		})
	}
}
