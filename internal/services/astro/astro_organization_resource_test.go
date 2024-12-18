package astro_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/astronomer/2023-08-01/organizations"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type AstroOrganizationResource struct{}

func TestAccAstroOrganization_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_astro_organization", "test")
	r := AstroOrganizationResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func TestAccAstroOrganization_requiresImport(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_astro_organization", "test")
	r := AstroOrganizationResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.RequiresImportErrorStep(r.requiresImport),
	})
}

func TestAccAstroOrganization_complete(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_astro_organization", "test")
	r := AstroOrganizationResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.complete(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func TestAccAstroOrganization_update(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_astro_organization", "test")
	r := AstroOrganizationResource{}
	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.complete(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
		{
			Config: r.update(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func (r AstroOrganizationResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := organizations.ParseOrganizationID(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Astro.OrganizationsClient.Get(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			return nil, fmt.Errorf("%s does not exist", id)
		}
		return nil, fmt.Errorf("retrieving %s: %+v", id, err)
	}
	return utils.Bool(resp.Model != nil), nil
}

func (r AstroOrganizationResource) template(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

provider "azuread" {}

resource "azurerm_resource_group" "test" {
  name     = "acctest-rg-%[1]d"
  location = "%[2]s"
}

data "azuread_domains" "test" {
	only_initial = true
}

`, data.RandomInteger, data.Locations.Primary)
}

func (r AstroOrganizationResource) basic(data acceptance.TestData) string {
	template := r.template(data)
	return fmt.Sprintf(`
%[1]s

resource "azurerm_astro_organization" "test" {
  name                = "acctest-ao-%[2]s"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  marketplace {
    offer_id     = "astro"
    plan_id      = "astro-paygo"
    plan_name    = "Monthly Pay-As-You-Go"
    publisher_id = "astronomer1591719760654"
    term_id      = "gmz7xq9ge3py"
    term_unit    = "1-Month"
  }
  partner_organization {
    organization_name = "test-organization-%[2]s"
	single_sign_on {
      aad_domains = data.azuread_domains.test.domains.*.domain_name
    }
	workspace_name    = "test-workspace-%[2]s"
  }
  user {
    email = "user@example.com"
    first_name = "John"
    last_name = "Doe"
    principal_name = "john.doe@example.com"
  }
}
`, template, data.RandomString)
}

func (r AstroOrganizationResource) requiresImport(data acceptance.TestData) string {
	config := r.basic(data)
	return fmt.Sprintf(`
%[1]s

resource "azurerm_astro_organization" "import" {
  name                = azurerm_astro_organization.test.name
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  marketplace {
    offer_id     = "astro"
    plan_id      = "astro-paygo"
    plan_name    = "Monthly Pay-As-You-Go"
    publisher_id = "astronomer1591719760654"
    term_id      = "gmz7xq9ge3py"
    term_unit    = "1-Month"
  }
  partner_organization {
    organization_name = "test-organization-%[2]s"
	single_sign_on {
      aad_domains = data.azuread_domains.test.domains.*.domain_name
    }
	workspace_name    = "test-workspace-%[2]s"
  }
  user {
    email = "user@example.com"
    first_name = "John"
    last_name = "Doe"
    principal_name = "john.doe@example.com"
  }
}
`, config, data.RandomString)
}

func (r AstroOrganizationResource) complete(data acceptance.TestData) string {
	template := r.template(data)
	return fmt.Sprintf(`
%[1]s

resource "azurerm_astro_organization" "test" {
  name                = "acctest-ao-%[2]s"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  marketplace {
    offer_id     = "astro"
    plan_id      = "astro-paygo"
    plan_name    = "Monthly Pay-As-You-Go"
    publisher_id = "astronomer1591719760654"
    term_id      = "gmz7xq9ge3py"
    term_unit    = "1-Month"
  }
  partner_organization {
    organization_name = "test-organization-%[2]s"
    single_sign_on {
      enterprise_app_id    = "00000000-0000-0000-0000-000000000000"
      aad_domains          = data.azuread_domains.test.domains.*.domain_name
    }
    workspace_name    = "test-workspace-%[2]s"
  }
  user {
    email = "user@example.com"
    first_name = "John"
    last_name = "Doe"
    phone = "+1234567890"
    principal_name = "john.doe@example.com"
  }
  tags = {
    environment = "production"
  }
}
`, template, data.RandomString)
}

func (r AstroOrganizationResource) update(data acceptance.TestData) string {
	template := r.template(data)
	return fmt.Sprintf(`
%[1]s

resource "azurerm_astro_organization" "test" {
  name                = "acctest-ao-%[2]s"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  marketplace {
    offer_id     = "astro"
    plan_id      = "astro-paygo"
    plan_name    = "Monthly Pay-As-You-Go"
    publisher_id = "astronomer1591719760654"
    term_id      = "gmz7xq9ge3py"
    term_unit    = "1-Month"
  }
  partner_organization {
    organization_name = "updated-organization-%[2]s"
    single_sign_on {
      enterprise_app_id    = "00000000-0000-0000-0000-000000000000"
      aad_domains          = data.azuread_domains.test.domains.*.domain_name
    }
    workspace_name    = "updated-workspace-%[2]s"
  }
  user {
    email = "updated_user@example.com"
    first_name = "Jane"
    last_name = "Smith"
    phone = "+0987654321"
    principal_name = "jane.smith@example.com"
  }
  tags = {
    environment = "staging"
  }
}
`, template, data.RandomString)
}
