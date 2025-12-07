package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &domainResource{}
	_ resource.ResourceWithConfigure   = &domainResource{}
	_ resource.ResourceWithImportState = &domainResource{}
)

type domainResource struct {
	client *autoglueClient
}

type domainResourceModel struct {
	ID             types.String `tfsdk:"id"`
	DomainName     types.String `tfsdk:"domain_name"`
	CredentialID   types.String `tfsdk:"credential_id"`
	ZoneID         types.String `tfsdk:"zone_id"`
	Status         types.String `tfsdk:"status"`
	LastError      types.String `tfsdk:"last_error"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewDomainResource() resource.Resource {
	return &domainResource{}
}

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue DNS domain (backed by AWS Route 53 credentials).",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique domain ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"domain_name": resourceschema.StringAttribute{
				Required: true,
				Description: "DNS domain name (FQDN, lowercase, without trailing dot). " +
					"Changing this requires creating a new domain.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"credential_id": resourceschema.StringAttribute{
				Required: true,
				Description: "Credential ID (UUID) bound to this domain. " +
					"Must be an AWS Route 53 service-scoped credential.",
			},

			"zone_id": resourceschema.StringAttribute{
				Optional: true,
				Description: "Optional zone ID for the backing Route 53 hosted zone. " +
					"If omitted, the control plane may backfill this automatically.",
			},

			"status": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Provisioning status (pending, provisioning, ready, failed).",
			},

			"last_error": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last provisioning error message, if any.",
			},

			"organization_id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Owning organization UUID.",
			},

			"created_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp (RFC3339).",
			},

			"updated_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp (RFC3339).",
			},
		},
	}
}

func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*autoglueClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *autoglueClient, got %T", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan domainResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createDomainPayload{
		DomainName:   plan.DomainName.ValueString(),
		CredentialID: plan.CredentialID.ValueString(),
		ZoneID:       plan.ZoneID.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue domain", map[string]any{
		"domain_name":   payload.DomainName,
		"credential_id": payload.CredentialID,
		"zone_id":       payload.ZoneID,
	})

	var apiResp domain
	if err := r.client.doJSON(ctx, http.MethodPost, "/dns/domains", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating domain", err.Error())
		return
	}

	syncDomainFromAPI(&plan, &apiResp)
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state domainResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/dns/domains/%s", id)
	tflog.Info(ctx, "Reading Autoglue domain", map[string]any{"id": id})

	var apiResp domain
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		// If not found, drop from state
		resp.State.RemoveResource(ctx)
		return
	}

	syncDomainFromAPI(&state, &apiResp)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan domainResourceModel
	var state domainResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Domain ID is required in state to update domain.")
		return
	}

	var payload updateDomainPayload

	// We only support updating credential_id and zone_id via Terraform.
	if !plan.CredentialID.Equal(state.CredentialID) {
		v := plan.CredentialID.ValueString()
		payload.CredentialID = &v
	}
	if !plan.ZoneID.Equal(state.ZoneID) {
		v := plan.ZoneID.ValueString()
		payload.ZoneID = &v
	}

	if payload.CredentialID == nil && payload.ZoneID == nil {
		// Nothing to update
		return
	}

	path := fmt.Sprintf("/dns/domains/%s", id)

	tflog.Info(ctx, "Updating Autoglue domain", map[string]any{
		"id":            id,
		"credential_id": plan.CredentialID.ValueString(),
		"zone_id":       plan.ZoneID.ValueString(),
	})

	var apiResp domain
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating domain", err.Error())
		return
	}

	syncDomainFromAPI(&plan, &apiResp)
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state domainResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		return
	}

	path := fmt.Sprintf("/dns/domains/%s", id)
	tflog.Info(ctx, "Deleting Autoglue domain", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting domain", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_domain.example <domain_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func syncDomainFromAPI(state *domainResourceModel, api *domain) {
	state.ID = types.StringValue(api.ID)
	state.DomainName = types.StringValue(api.DomainName)
	state.CredentialID = types.StringValue(api.CredentialID)
	state.ZoneID = types.StringValue(api.ZoneID)
	state.Status = types.StringValue(api.Status)
	state.LastError = types.StringValue(api.LastError)
	state.OrganizationID = types.StringValue(api.OrganizationID)
	state.CreatedAt = types.StringValue(api.CreatedAt)
	state.UpdatedAt = types.StringValue(api.UpdatedAt)
}
