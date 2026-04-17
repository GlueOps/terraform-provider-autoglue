package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &clusterMetadataResource{}
	_ resource.ResourceWithConfigure   = &clusterMetadataResource{}
	_ resource.ResourceWithImportState = &clusterMetadataResource{}
)

type clusterMetadataResource struct {
	client *autoglueClient
}

type clusterMetadataResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ClusterID      types.String `tfsdk:"cluster_id"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewClusterMetadataResource() resource.Resource {
	return &clusterMetadataResource{}
}

func (r *clusterMetadataResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_metadata"
}

func (r *clusterMetadataResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages a cluster metadata key-value pair. Keys are automatically lowercased; values preserve case sensitivity.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique metadata entry ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID this metadata is attached to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": resourceschema.StringAttribute{
				Required:    true,
				Description: "Metadata key (automatically lowercased).",
			},
			"value": resourceschema.StringAttribute{
				Required:    true,
				Description: "Metadata value (case preserved).",
			},
			"organization_id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Owning organization UUID.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp.",
			},
		},
	}
}

func (r *clusterMetadataResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterMetadataResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterMetadataResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	payload := createClusterMetadataPayload{
		Key:   strings.ToLower(strings.TrimSpace(plan.Key.ValueString())),
		Value: plan.Value.ValueString(),
	}

	tflog.Info(ctx, "Creating cluster metadata", map[string]any{
		"cluster_id": clusterID,
		"key":        payload.Key,
	})

	apiPath := fmt.Sprintf("/clusters/%s/metadata", clusterID)
	var apiResp clusterMetadata
	if err := r.client.doJSON(ctx, http.MethodPost, apiPath, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating cluster metadata", err.Error())
		return
	}

	mapClusterMetadataToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterMetadataResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterMetadataResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Metadata ID is required in state.")
		return
	}

	apiPath := fmt.Sprintf("/clusters/%s/metadata/%s", clusterID, id)
	tflog.Info(ctx, "Reading cluster metadata", map[string]any{"cluster_id": clusterID, "id": id})

	var apiResp clusterMetadata
	if err := r.client.doJSON(ctx, http.MethodGet, apiPath, "", nil, &apiResp); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading cluster metadata", err.Error())
		return
	}

	mapClusterMetadataToModel(&state, &apiResp)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterMetadataResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterMetadataResourceModel
	var state clusterMetadataResourceModel

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

	clusterID := state.ClusterID.ValueString()
	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Metadata ID is required in state.")
		return
	}

	key := strings.ToLower(strings.TrimSpace(plan.Key.ValueString()))
	value := plan.Value.ValueString()
	payload := updateClusterMetadataPayload{
		Key:   &key,
		Value: &value,
	}

	apiPath := fmt.Sprintf("/clusters/%s/metadata/%s", clusterID, id)
	tflog.Info(ctx, "Updating cluster metadata", map[string]any{
		"cluster_id": clusterID,
		"id":         id,
		"key":        key,
	})

	var apiResp clusterMetadata
	if err := r.client.doJSON(ctx, http.MethodPatch, apiPath, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating cluster metadata", err.Error())
		return
	}

	mapClusterMetadataToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterMetadataResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterMetadataResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	id := state.ID.ValueString()
	if id == "" {
		return
	}

	apiPath := fmt.Sprintf("/clusters/%s/metadata/%s", clusterID, id)
	tflog.Info(ctx, "Deleting cluster metadata", map[string]any{"cluster_id": clusterID, "id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, apiPath, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting cluster metadata", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterMetadataResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_metadata.example <cluster_id>/<metadata_id>
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected format: <cluster_id>/<metadata_id>",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func mapClusterMetadataToModel(m *clusterMetadataResourceModel, a *clusterMetadata) {
	m.ID = types.StringValue(a.ID)
	m.ClusterID = types.StringValue(a.ClusterID)
	m.Key = types.StringValue(a.Key)
	m.Value = types.StringValue(a.Value)
	m.OrganizationID = types.StringValue(a.OrganizationID)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
}
