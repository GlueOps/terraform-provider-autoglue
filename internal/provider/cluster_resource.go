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
	_ resource.Resource                = &clusterResource{}
	_ resource.ResourceWithConfigure   = &clusterResource{}
	_ resource.ResourceWithImportState = &clusterResource{}
)

type clusterResource struct {
	client *autoglueClient
}

type clusterResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ClusterProvider types.String `tfsdk:"cluster_provider"`
	Region          types.String `tfsdk:"region"`
	Status          types.String `tfsdk:"status"`
}

func NewClusterResource() resource.Resource {
	return &clusterResource{}
}

func (r *clusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *clusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue Kubernetes cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster name.",
			},
			"cluster_provider": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cloud provider identifier (for example, aws, gcp, azure).",
			},
			"region": resourceschema.StringAttribute{
				Required:    true,
				Description: "Region to deploy the cluster into.",
			},
			"status": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Current cluster status as reported by Autoglue.",
			},
		},
	}
}

func (r *clusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createClusterPayload{
		Name:            plan.Name.ValueString(),
		ClusterProvider: plan.ClusterProvider.ValueString(),
		Region:          plan.Region.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue cluster", map[string]any{
		"name":             payload.Name,
		"cluster_provider": payload.ClusterProvider,
		"region":           payload.Region,
	})

	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodPost, "/clusters", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.Status = types.StringValue(apiResp.Status)
	plan.Name = types.StringValue(apiResp.Name)
	plan.ClusterProvider = types.StringValue(apiResp.ClusterProvider)
	plan.Region = types.StringValue(apiResp.Region)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Cluster ID is required in state to read cluster.")
		return
	}

	var apiResp cluster
	path := fmt.Sprintf("/clusters/%s", id)

	tflog.Info(ctx, "Reading Autoglue cluster", map[string]any{"id": id})

	err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp)
	if err != nil {
		// If the cluster no longer exists, tell Terraform it's gone
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(apiResp.Name)
	state.ClusterProvider = types.StringValue(apiResp.ClusterProvider)
	state.Region = types.StringValue(apiResp.Region)
	state.Status = types.StringValue(apiResp.Status)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterResourceModel
	var state clusterResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Cluster ID is required in state to update cluster.")
		return
	}

	payload := createClusterPayload{
		Name:            plan.Name.ValueString(),
		ClusterProvider: plan.ClusterProvider.ValueString(),
		Region:          plan.Region.ValueString(),
	}

	path := fmt.Sprintf("/clusters/%s", id)

	tflog.Info(ctx, "Updating Autoglue cluster", map[string]any{
		"id":               id,
		"name":             payload.Name,
		"cluster_provider": payload.ClusterProvider,
		"region":           payload.Region,
	})

	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.Status = types.StringValue(apiResp.Status)
	plan.Name = types.StringValue(apiResp.Name)
	plan.ClusterProvider = types.StringValue(apiResp.ClusterProvider)
	plan.Region = types.StringValue(apiResp.Region)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		// Nothing to do
		return
	}

	path := fmt.Sprintf("/clusters/%s", id)

	tflog.Info(ctx, "Deleting Autoglue cluster", map[string]any{"id": id})

	// Best-effort delete: if API says 404, treat as already gone
	err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting cluster", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Support: terraform import autoglue_cluster.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
