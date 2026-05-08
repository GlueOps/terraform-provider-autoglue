package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &loadBalancerResource{}
	_ resource.ResourceWithConfigure   = &loadBalancerResource{}
	_ resource.ResourceWithImportState = &loadBalancerResource{}
)

type loadBalancerResource struct {
	client *autoglueClient
}

type loadBalancerResourceModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	Name             types.String `tfsdk:"name"`
	Kind             types.String `tfsdk:"kind"`
	PublicIPAddress  types.String `tfsdk:"public_ip_address"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func NewLoadBalancerResource() resource.Resource {
	return &loadBalancerResource{}
}

func (r *loadBalancerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_load_balancer"
}

func (r *loadBalancerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue load balancer (logical endpoint, identified by kind and IPs).",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "LoadBalancer ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"organization_id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Owning organization UUID.",
			},

			"name": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Load balancer name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"kind": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Load balancer kind. One of `glueops` or `public`.",
				Validators: []validator.String{
					stringvalidator.OneOf("glueops", "public"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"public_ip_address": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Public IPv4 address advertised by this load balancer.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"private_ip_address": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Private IPv4 address advertised by this load balancer.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

func (r *loadBalancerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *loadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan loadBalancerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createLoadBalancerPayload{
		Name:             plan.Name.ValueString(),
		Kind:             plan.Kind.ValueString(),
		PublicIPAddress:  plan.PublicIPAddress.ValueString(),
		PrivateIPAddress: plan.PrivateIPAddress.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue load balancer", map[string]any{
		"name":               payload.Name,
		"kind":               payload.Kind,
		"public_ip_address":  payload.PublicIPAddress,
		"private_ip_address": payload.PrivateIPAddress,
	})

	var apiResp loadBalancer
	if err := r.client.doJSON(ctx, http.MethodPost, "/load-balancers", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating load balancer", err.Error())
		return
	}

	syncLoadBalancerFromAPI(&plan, &apiResp)
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *loadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state loadBalancerResourceModel
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

	apiPath := fmt.Sprintf("/load-balancers/%s", id)
	tflog.Info(ctx, "Reading Autoglue load balancer", map[string]any{"id": id})

	var apiResp loadBalancer
	if err := r.client.doJSON(ctx, http.MethodGet, apiPath, "", nil, &apiResp); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading load balancer", err.Error())
		return
	}

	syncLoadBalancerFromAPI(&state, &apiResp)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *loadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan loadBalancerResourceModel
	var state loadBalancerResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Load balancer ID is required in state to update.")
		return
	}

	var payload updateLoadBalancerPayload
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		payload.Name = &v
	}
	if !plan.Kind.Equal(state.Kind) {
		v := plan.Kind.ValueString()
		payload.Kind = &v
	}
	if !plan.PublicIPAddress.Equal(state.PublicIPAddress) {
		v := plan.PublicIPAddress.ValueString()
		payload.PublicIPAddress = &v
	}
	if !plan.PrivateIPAddress.Equal(state.PrivateIPAddress) {
		v := plan.PrivateIPAddress.ValueString()
		payload.PrivateIPAddress = &v
	}

	if payload.Name == nil && payload.Kind == nil && payload.PublicIPAddress == nil && payload.PrivateIPAddress == nil {
		// Nothing to update.
		return
	}

	apiPath := fmt.Sprintf("/load-balancers/%s", id)
	tflog.Info(ctx, "Updating Autoglue load balancer", map[string]any{"id": id})

	var apiResp loadBalancer
	if err := r.client.doJSON(ctx, http.MethodPatch, apiPath, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating load balancer", err.Error())
		return
	}

	syncLoadBalancerFromAPI(&plan, &apiResp)
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *loadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state loadBalancerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		return
	}

	apiPath := fmt.Sprintf("/load-balancers/%s", id)
	tflog.Info(ctx, "Deleting Autoglue load balancer", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, apiPath, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting load balancer", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *loadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_load_balancer.example <load_balancer_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func syncLoadBalancerFromAPI(state *loadBalancerResourceModel, api *loadBalancer) {
	state.ID = types.StringValue(api.ID)
	state.OrganizationID = types.StringValue(api.OrganizationID)
	state.Name = types.StringValue(api.Name)
	state.Kind = types.StringValue(api.Kind)
	state.PublicIPAddress = types.StringValue(api.PublicIPAddress)
	state.PrivateIPAddress = types.StringValue(api.PrivateIPAddress)
	state.CreatedAt = types.StringValue(api.CreatedAt)
	state.UpdatedAt = types.StringValue(api.UpdatedAt)
}
