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
	_ resource.Resource                = &annotationResource{}
	_ resource.ResourceWithConfigure   = &annotationResource{}
	_ resource.ResourceWithImportState = &annotationResource{}
)

type annotationResource struct {
	client *autoglueClient
}

type annotationResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewAnnotationResource() resource.Resource {
	return &annotationResource{}
}

func (r *annotationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation"
}

func (r *annotationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue annotation.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique annotation ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": resourceschema.StringAttribute{
				Required:    true,
				Description: "Annotation key.",
			},
			"value": resourceschema.StringAttribute{
				Required:    true,
				Description: "Annotation value.",
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

func (r *annotationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *annotationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan annotationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createAnnotationPayload{
		Key:   plan.Key.ValueString(),
		Value: plan.Value.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue annotation", map[string]any{
		"key":   payload.Key,
		"value": payload.Value,
	})

	var apiResp annotation
	if err := r.client.doJSON(ctx, http.MethodPost, "/annotations", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating annotation", err.Error())
		return
	}

	mapAnnotationToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *annotationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state annotationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Annotation ID is required in state to read annotation.")
		return
	}

	path := fmt.Sprintf("/annotations/%s", id)
	tflog.Info(ctx, "Reading Autoglue annotation", map[string]any{"id": id})

	var apiResp annotation
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapAnnotationToModel(&state, &apiResp)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *annotationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan annotationResourceModel
	var state annotationResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Annotation ID is required in state to update annotation.")
		return
	}

	payload := createAnnotationPayload{
		Key:   plan.Key.ValueString(),
		Value: plan.Value.ValueString(),
	}

	path := fmt.Sprintf("/annotations/%s", id)
	tflog.Info(ctx, "Updating Autoglue annotation", map[string]any{
		"id":    id,
		"key":   payload.Key,
		"value": payload.Value,
	})

	var apiResp annotation
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating annotation", err.Error())
		return
	}

	mapAnnotationToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *annotationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state annotationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		return
	}

	path := fmt.Sprintf("/annotations/%s", id)
	tflog.Info(ctx, "Deleting Autoglue annotation", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting annotation", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *annotationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_annotation.example <annotation_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapAnnotationToModel(m *annotationResourceModel, a *annotation) {
	m.ID = types.StringValue(a.ID)
	m.Key = types.StringValue(a.Key)
	m.Value = types.StringValue(a.Value)
	m.OrganizationID = types.StringValue(a.OrganizationID)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
}
