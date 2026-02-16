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
	_ resource.Resource                = &labelResource{}
	_ resource.ResourceWithConfigure   = &labelResource{}
	_ resource.ResourceWithImportState = &labelResource{}
)

type labelResource struct {
	client *autoglueClient
}

type labelResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewLabelResource() resource.Resource {
	return &labelResource{}
}

func (r *labelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *labelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue label.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique label ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": resourceschema.StringAttribute{
				Required:    true,
				Description: "Label key.",
			},
			"value": resourceschema.StringAttribute{
				Required:    true,
				Description: "Label value.",
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

func (r *labelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *labelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan labelResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createLabelPayload{
		Key:   plan.Key.ValueString(),
		Value: plan.Value.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue label", map[string]any{
		"key":   payload.Key,
		"value": payload.Value,
	})

	var apiResp label
	if err := r.client.doJSON(ctx, http.MethodPost, "/labels", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating label", err.Error())
		return
	}

	mapLabelToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *labelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state labelResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Label ID is required in state to read label.")
		return
	}

	path := fmt.Sprintf("/labels/%s", id)
	tflog.Info(ctx, "Reading Autoglue label", map[string]any{"id": id})

	var apiResp label
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading label", fmt.Sprintf("Error reading label: %s", err.Error()))
		return
	}

	mapLabelToModel(&state, &apiResp)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *labelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan labelResourceModel
	var state labelResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Label ID is required in state to update label.")
		return
	}

	payload := createLabelPayload{
		Key:   plan.Key.ValueString(),
		Value: plan.Value.ValueString(),
	}

	path := fmt.Sprintf("/labels/%s", id)
	tflog.Info(ctx, "Updating Autoglue label", map[string]any{
		"id":    id,
		"key":   payload.Key,
		"value": payload.Value,
	})

	var apiResp label
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating label", err.Error())
		return
	}

	mapLabelToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *labelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state labelResourceModel
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

	resourcePath := fmt.Sprintf("/labels/%s", id)
	tflog.Info(ctx, "Deleting Autoglue label", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, resourcePath, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting label", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *labelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_label.example <label_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapLabelToModel(m *labelResourceModel, l *label) {
	m.ID = types.StringValue(l.ID)
	m.Key = types.StringValue(l.Key)
	m.Value = types.StringValue(l.Value)
	m.OrganizationID = types.StringValue(l.OrganizationID)
	m.CreatedAt = types.StringValue(l.CreatedAt)
	m.UpdatedAt = types.StringValue(l.UpdatedAt)
}
