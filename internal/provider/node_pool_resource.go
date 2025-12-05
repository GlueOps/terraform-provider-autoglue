package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &nodePoolResource{}
	_ resource.ResourceWithConfigure   = &nodePoolResource{}
	_ resource.ResourceWithImportState = &nodePoolResource{}
)

type nodePoolResource struct {
	client *autoglueClient
}

type nodePoolResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ApiserverURL   types.String `tfsdk:"apiserver_url"`
	KubeletVersion types.String `tfsdk:"kubelet_version"`
	KubeletOptions types.Map    `tfsdk:"kubelet_options"`
	Role           types.String `tfsdk:"role"`

	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func NewNodePoolResource() resource.Resource {
	return &nodePoolResource{}
}

func (r *nodePoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_pool"
}

func (r *nodePoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue node pool.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique node pool ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": resourceschema.StringAttribute{
				Required:    true,
				Description: "Node pool name.",
			},

			"apiserver_url": resourceschema.StringAttribute{
				Required:    true,
				Description: "Kubernetes API server URL for this node pool.",
			},

			"kubelet_version": resourceschema.StringAttribute{
				Required:    true,
				Description: "Kubelet version for nodes in this pool.",
			},

			"kubelet_options": resourceschema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Additional kubelet options as key/value pairs.",
			},

			"role": resourceschema.StringAttribute{
				Required: true,
				Description: "Node pool role (for example: \"control-plane\" or \"worker\"). " +
					"Exact values must match the Autoglue API's accepted roles.",
			},

			"created_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp.",
			},
			"organization_id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Owning organization UUID.",
			},
		},
	}
}

func (r *nodePoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nodePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var kubeletOpts map[string]string
	if !plan.KubeletOptions.IsNull() && !plan.KubeletOptions.IsUnknown() {
		diags = plan.KubeletOptions.ElementsAs(ctx, &kubeletOpts, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	payload := createNodePoolPayload{
		Name:           plan.Name.ValueString(),
		ApiserverURL:   plan.ApiserverURL.ValueString(),
		KubeletVersion: plan.KubeletVersion.ValueString(),
		KubeletOptions: kubeletOpts,
		Role:           plan.Role.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue node pool", map[string]any{
		"name":            payload.Name,
		"apiserver_url":   payload.ApiserverURL,
		"kubelet_version": payload.KubeletVersion,
		"role":            payload.Role,
	})

	var apiResp nodePool
	if err := r.client.doJSON(ctx, http.MethodPost, "/node-pools", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating node pool", err.Error())
		return
	}

	syncNodePoolToState(ctx, &plan, &apiResp, &resp.Diagnostics)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Node pool ID is required in state to read node pool.")
		return
	}

	path := fmt.Sprintf("/node-pools/%s", id)

	tflog.Info(ctx, "Reading Autoglue node pool", map[string]any{"id": id})

	var apiResp nodePool
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		// If gone, drop from state
		resp.State.RemoveResource(ctx)
		return
	}

	syncNodePoolToState(ctx, &state, &apiResp, &resp.Diagnostics)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolResourceModel
	var state nodePoolResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Node pool ID is required in state to update node pool.")
		return
	}

	var kubeletOpts map[string]string
	if !plan.KubeletOptions.IsNull() && !plan.KubeletOptions.IsUnknown() {
		diags = plan.KubeletOptions.ElementsAs(ctx, &kubeletOpts, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	name := plan.Name.ValueString()
	apiserverURL := plan.ApiserverURL.ValueString()
	kubeletVersion := plan.KubeletVersion.ValueString()
	role := plan.Role.ValueString()

	payload := updateNodePoolPayload{
		Name:           &name,
		ApiserverURL:   &apiserverURL,
		KubeletVersion: &kubeletVersion,
		KubeletOptions: kubeletOpts,
		Role:           &role,
	}

	path := fmt.Sprintf("/node-pools/%s", id)

	tflog.Info(ctx, "Updating Autoglue node pool", map[string]any{
		"id":              id,
		"name":            name,
		"apiserver_url":   apiserverURL,
		"kubelet_version": kubeletVersion,
		"role":            role,
	})

	var apiResp nodePool
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating node pool", err.Error())
		return
	}

	syncNodePoolToState(ctx, &plan, &apiResp, &resp.Diagnostics)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		return
	}

	path := fmt.Sprintf("/node-pools/%s", id)

	tflog.Info(ctx, "Deleting Autoglue node pool", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting node pool", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *nodePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_node_pool.example <node_pool_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func syncNodePoolToState(
	ctx context.Context,
	state *nodePoolResourceModel,
	apiResp *nodePool,
	diags *diag.Diagnostics,
) {
	if apiResp == nil {
		diags.AddError(
			"Nil node pool response",
			"Internal provider error: apiResp was nil in syncNodePoolToState.",
		)
		return
	}

	state.ID = types.StringValue(apiResp.ID)
	state.Name = types.StringValue(apiResp.Name)
	state.ApiserverURL = types.StringValue(apiResp.ApiserverURL)
	state.KubeletVersion = types.StringValue(apiResp.KubeletVersion)
	state.Role = types.StringValue(apiResp.Role)
	state.CreatedAt = types.StringValue(apiResp.CreatedAt)
	state.UpdatedAt = types.StringValue(apiResp.UpdatedAt)
	state.OrganizationID = types.StringValue(apiResp.OrganizationID)

	if apiResp.KubeletOptions == nil {
		state.KubeletOptions = types.MapNull(types.StringType)
	} else {
		m, d := types.MapValueFrom(ctx, types.StringType, apiResp.KubeletOptions)
		diags.Append(d...)
		state.KubeletOptions = m
	}
}
