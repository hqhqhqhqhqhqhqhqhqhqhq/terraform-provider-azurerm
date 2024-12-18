package astro

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/astronomer/2023-08-01/organizations"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/astro/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

type AstroOrganizationModel struct {
	Name                string                     `tfschema:"name"`
	ResourceGroupName   string                     `tfschema:"resource_group_name"`
	Location            string                     `tfschema:"location"`
	Marketplace         []MarketplaceModel         `tfschema:"marketplace"`
	PartnerOrganization []PartnerOrganizationModel `tfschema:"partner_organization"`
	Tags                map[string]string          `tfschema:"tags"`
	User                []UserModel                `tfschema:"user"`
}

type MarketplaceModel struct {
	OfferId     string `tfschema:"offer_id"`
	PlanId      string `tfschema:"plan_id"`
	PlanName    string `tfschema:"plan_name"`
	PublisherId string `tfschema:"publisher_id"`
	TermId      string `tfschema:"term_id"`
	TermUnit    string `tfschema:"term_unit"`
}

type PartnerOrganizationModel struct {
	OrganizationId   string              `tfschema:"organization_id"`
	OrganizationName string              `tfschema:"organization_name"`
	SingleSignOn     []SingleSignOnModel `tfschema:"single_sign_on"`
	WorkspaceId      string              `tfschema:"workspace_id"`
	WorkspaceName    string              `tfschema:"workspace_name"`
}

type SingleSignOnModel struct {
	AadDomains        []string                         `tfschema:"aad_domains"`
	EnterpriseAppId   string                           `tfschema:"enterprise_app_id"`
	SingleSignOnState organizations.SingleSignOnStates `tfschema:"single_sign_on_state"`
	SingleSignOnUrl   string                           `tfschema:"single_sign_on_url"`
}

type UserModel struct {
	EmailAddress  string `tfschema:"email"`
	FirstName     string `tfschema:"first_name"`
	LastName      string `tfschema:"last_name"`
	PhoneNumber   string `tfschema:"phone"`
	PrincipalName string `tfschema:"principal_name"`
}

type AstroOrganizationResource struct{}

var _ sdk.ResourceWithUpdate = AstroOrganizationResource{}

func (r AstroOrganizationResource) ResourceType() string {
	return "azurerm_astro_organization"
}

func (r AstroOrganizationResource) ModelObject() interface{} {
	return &AstroOrganizationModel{}
}

func (r AstroOrganizationResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return organizations.ValidateOrganizationID
}

func (r AstroOrganizationResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},

		"resource_group_name": commonschema.ResourceGroupName(),

		"location": commonschema.Location(),

		"marketplace": {
			Type:     pluginsdk.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"offer_id": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ForceNew:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"plan_id": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ForceNew:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"plan_name": {
						Type:         pluginsdk.TypeString,
						Optional:     true,
						ForceNew:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"publisher_id": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ForceNew:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"term_id": {
						Type:         pluginsdk.TypeString,
						Optional:     true,
						ForceNew:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"term_unit": {
						Type:         pluginsdk.TypeString,
						Optional:     true,
						ForceNew:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
		},

		"partner_organization": {
			Type:     pluginsdk.TypeList,
			Required: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"organization_id": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},

					"organization_name": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validate.OrganizationAndWorkspaceName,
					},

					"single_sign_on": {
						Type:     pluginsdk.TypeList,
						Required: true,
						MaxItems: 1,
						Elem: &pluginsdk.Resource{
							Schema: map[string]*pluginsdk.Schema{
								"aad_domains": {
									Type:     pluginsdk.TypeList,
									Required: true,
									Elem: &pluginsdk.Schema{
										Type:         pluginsdk.TypeString,
										ValidateFunc: validation.StringIsNotEmpty,
									},
								},

								"enterprise_app_id": {
									Type:         pluginsdk.TypeString,
									Optional:     true,
									ForceNew:     true,
									ValidateFunc: validation.StringIsNotEmpty,
								},

								"single_sign_on_state": {
									Type:     pluginsdk.TypeString,
									Computed: true,
								},

								"single_sign_on_url": {
									Type:     pluginsdk.TypeString,
									Computed: true,
								},
							},
						},
					},

					"workspace_id": {
						Type:     pluginsdk.TypeString,
						Computed: true,
					},

					"workspace_name": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validate.OrganizationAndWorkspaceName,
					},
				},
			},
		},

		"user": {
			Type:     pluginsdk.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &pluginsdk.Resource{
				Schema: map[string]*pluginsdk.Schema{
					"email": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validate.UserEmailAddress,
					},

					"first_name": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"last_name": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"phone": {
						Type:         pluginsdk.TypeString,
						Optional:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},

					"principal_name": {
						Type:         pluginsdk.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
				},
			},
		},

		"tags": commonschema.Tags(),
	}
}

func (r AstroOrganizationResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (r AstroOrganizationResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			var model AstroOrganizationModel
			if err := metadata.Decode(&model); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			client := metadata.Client.Astro.OrganizationsClient
			subscriptionId := metadata.Client.Account.SubscriptionId

			id := organizations.NewOrganizationID(subscriptionId, model.ResourceGroupName, model.Name)

			existing, err := client.Get(ctx, id)
			if err != nil && !response.WasNotFound(existing.HttpResponse) {
				return fmt.Errorf("checking for existing %s: %+v", id, err)
			}

			if !response.WasNotFound(existing.HttpResponse) {
				return metadata.ResourceRequiresImport(r.ResourceType(), id)
			}

			organizationResource := &organizations.OrganizationResource{
				Location: location.Normalize(model.Location),
				Properties: &organizations.LiftrBaseDataOrganizationProperties{
					Marketplace:                   expandMarketplaceModel(model.Marketplace),
					PartnerOrganizationProperties: expandPartnerOrganizationModel(model.PartnerOrganization),
					User:                          expandUserModel(model.User),
				},
				Tags: pointer.To(model.Tags),
			}

			if err := client.CreateOrUpdateThenPoll(ctx, id, *organizationResource); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)

			return nil
		},
	}
}

func (r AstroOrganizationResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Astro.OrganizationsClient

			id, err := organizations.ParseOrganizationID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			var model AstroOrganizationModel
			if err := metadata.Decode(&model); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			resp, err := client.Get(ctx, *id)
			if err != nil {
				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			properties := resp.Model
			if resp.Model == nil {
				return fmt.Errorf("retrieving %s: `model` was nil", *id)
			}
			if resp.Model.Properties == nil {
				return fmt.Errorf("retrieving %s: `properties` was nil", *id)
			}

			if metadata.ResourceData.HasChange("marketplace") {
				properties.Properties.Marketplace = expandMarketplaceModel(model.Marketplace)
			}

			if metadata.ResourceData.HasChange("partner_organization") {
				properties.Properties.PartnerOrganizationProperties = expandPartnerOrganizationModel(model.PartnerOrganization)
			}

			if metadata.ResourceData.HasChange("user") {
				properties.Properties.User = expandUserModel(model.User)
			}

			if metadata.ResourceData.HasChange("tags") {
				properties.Tags = pointer.To(model.Tags)
			}

			if err := client.CreateOrUpdateThenPoll(ctx, *id, *properties); err != nil {
				return fmt.Errorf("updating %s: %+v", *id, err)
			}

			return nil
		},
		DiffFunc: nil,
		Timeout:  30 * time.Minute,
	}
}

func (r AstroOrganizationResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Astro.OrganizationsClient

			id, err := organizations.ParseOrganizationID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(id)
				}

				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			state := AstroOrganizationModel{
				Name:              id.OrganizationName,
				ResourceGroupName: id.ResourceGroupName,
			}

			if model := resp.Model; model != nil {
				state.Location = location.Normalize(model.Location)

				if properties := model.Properties; properties != nil {
					state.Marketplace = flattenMarketplaceModel(properties.Marketplace)

					state.PartnerOrganization = flattenPartnerOrganizationModel(properties.PartnerOrganizationProperties)

					state.User = flattenUserModel(properties.User)
				}

				state.Tags = pointer.From(model.Tags)
			}

			return metadata.Encode(&state)
		},
	}
}

func (r AstroOrganizationResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Astro.OrganizationsClient

			id, err := organizations.ParseOrganizationID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			if err := client.DeleteThenPoll(ctx, *id); err != nil {
				return fmt.Errorf("deleting %s: %+v", id, err)
			}

			return nil
		},
	}
}

func expandMarketplaceModel(inputList []MarketplaceModel) organizations.LiftrBaseMarketplaceDetails {
	if len(inputList) == 0 {
		return organizations.LiftrBaseMarketplaceDetails{}
	}

	input := pointer.To(inputList[0])

	return organizations.LiftrBaseMarketplaceDetails{
		OfferDetails: organizations.LiftrBaseOfferDetails{
			OfferId:     input.OfferId,
			PlanId:      input.PlanId,
			PlanName:    pointer.To(input.PlanName),
			PublisherId: input.PublisherId,
			TermId:      pointer.To(input.TermId),
			TermUnit:    pointer.To(input.TermUnit),
		},
	}
}

func flattenMarketplaceModel(input organizations.LiftrBaseMarketplaceDetails) []MarketplaceModel {
	var outputList []MarketplaceModel

	output := MarketplaceModel{
		OfferId:     input.OfferDetails.OfferId,
		PublisherId: input.OfferDetails.PublisherId,
		PlanId:      input.OfferDetails.PlanId,
		PlanName:    pointer.From(input.OfferDetails.PlanName),
		TermId:      pointer.From(input.OfferDetails.TermId),
		TermUnit:    pointer.From(input.OfferDetails.TermUnit),
	}

	return append(outputList, output)
}

func expandPartnerOrganizationModel(inputList []PartnerOrganizationModel) *organizations.LiftrBaseDataPartnerOrganizationProperties {
	if len(inputList) == 0 {
		return nil
	}

	input := pointer.To(inputList[0])

	output := organizations.LiftrBaseDataPartnerOrganizationProperties{
		OrganizationName:       input.OrganizationName,
		SingleSignOnProperties: expandSingleSignOnModel(input.SingleSignOn),
		WorkspaceName:          pointer.To(input.WorkspaceName),
	}

	return &output
}

func expandSingleSignOnModel(inputList []SingleSignOnModel) *organizations.LiftrBaseSingleSignOnProperties {
	if len(inputList) == 0 {
		return nil
	}

	input := pointer.To(inputList[0])

	output := organizations.LiftrBaseSingleSignOnProperties{
		AadDomains:      pointer.To(input.AadDomains),
		EnterpriseAppId: pointer.To(input.EnterpriseAppId),
	}

	return &output
}

func expandUserModel(inputList []UserModel) organizations.LiftrBaseUserDetails {
	if len(inputList) == 0 {
		return organizations.LiftrBaseUserDetails{}
	}

	input := pointer.To(inputList[0])

	output := organizations.LiftrBaseUserDetails{
		EmailAddress: input.EmailAddress,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		PhoneNumber:  pointer.To(input.PhoneNumber),
		Upn:          pointer.To(input.PrincipalName),
	}

	return output
}

func flattenPartnerOrganizationModel(input *organizations.LiftrBaseDataPartnerOrganizationProperties) []PartnerOrganizationModel {
	var outputList []PartnerOrganizationModel

	if input == nil {
		return outputList
	}

	output := PartnerOrganizationModel{
		OrganizationId:   pointer.From(input.OrganizationId),
		OrganizationName: input.OrganizationName,
		SingleSignOn:     flattenSingleSignOnModel(input.SingleSignOnProperties),
		WorkspaceId:      pointer.From(input.WorkspaceId),
		WorkspaceName:    pointer.From(input.WorkspaceName),
	}

	return append(outputList, output)
}

func flattenSingleSignOnModel(input *organizations.LiftrBaseSingleSignOnProperties) []SingleSignOnModel {
	var outputList []SingleSignOnModel

	if input == nil {
		return outputList
	}

	output := SingleSignOnModel{
		AadDomains:        pointer.From(input.AadDomains),
		EnterpriseAppId:   pointer.From(input.EnterpriseAppId),
		SingleSignOnState: pointer.From(input.SingleSignOnState),
		SingleSignOnUrl:   pointer.From(input.SingleSignOnURL),
	}

	return append(outputList, output)
}

func flattenUserModel(input organizations.LiftrBaseUserDetails) []UserModel {
	var outputList []UserModel

	output := UserModel{
		EmailAddress:  input.EmailAddress,
		FirstName:     input.FirstName,
		LastName:      input.LastName,
		PhoneNumber:   pointer.From(input.PhoneNumber),
		PrincipalName: pointer.From(input.Upn),
	}

	return append(outputList, output)
}
