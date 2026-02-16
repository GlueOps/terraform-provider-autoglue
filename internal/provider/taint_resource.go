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
	_ resource.Resource                = &taintResource{}
	_ resource.ResourceWithConfigure   = &taintResource{}
	_ resource.ResourceWithImportState = &taintResource{}
)

type taintResource struct {
	client *autoglueClient
}

type taintResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Key       types.String `tfsdk:"key"`
	Value     types.String `tfsdk:"value"`
	Effect    types.String `tfsdk:"effect"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewTaintResource() resource.Resource {
	return &taintResource{}
}

func (r *taintResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_taint"
}

func (r *taintResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue taint.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique taint ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"key": resourceschema.StringAttribute{
				Required:    true,
				Description: "Taint key. Changing this forces a new taint to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"value": resourceschema.StringAttribute{
				Optional:    true,
				Description: "Taint value (optional).",
			},

			"effect": resourceschema.StringAttribute{
				Required: true,
				Description: "Taint effect, for example `NoSchedule`, `PreferNoSchedule`, or `NoExecute`." +
					" See Autoglue / Kubernetes taint documentation for valid options.",
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

func (r *taintResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *taintResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan taintResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createTaintPayload{
		Key:    plan.Key.ValueString(),
		Value:  stringPointerFromAttr(plan.Value),
		Effect: plan.Effect.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue taint", map[string]any{
		"key":    payload.Key,
		"value":  payload.Value,
		"effect": payload.Effect,
	})

	var apiResp taint
	if err := r.client.doJSON(ctx, http.MethodPost, "/taints", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating taint", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.Key = types.StringValue(apiResp.Key)
	if apiResp.Value != nil {
		plan.Value = types.StringValue(*apiResp.Value)
	} else {
		plan.Value = types.StringNull()
	}
	plan.Effect = types.StringValue(apiResp.Effect)
	plan.CreatedAt = types.StringValue(apiResp.CreatedAt)
	plan.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *taintResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state taintResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Taint ID is required in state to read taint.")
		return
	}

	path := fmt.Sprintf("/taints/%s", id)

	tflog.Info(ctx, "Reading Autoglue taint", map[string]any{"id": id})

	var apiResp taint
	err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading taint", fmt.Sprintf("Error reading taint: %s", err))
		return
	}

	state.ID = types.StringValue(apiResp.ID)
	state.Key = types.StringValue(apiResp.Key)
	if apiResp.Value != nil {
		state.Value = types.StringValue(*apiResp.Value)
	} else {
		state.Value = types.StringNull()
	}
	state.Effect = types.StringValue(apiResp.Effect)
	state.CreatedAt = types.StringValue(apiResp.CreatedAt)
	state.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *taintResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan taintResourceModel
	var state taintResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Taint ID is required in state to update taint.")
		return
	}

	payload := updateTaintPayload{
		Key:    stringPointerFromAttr(plan.Key),
		Value:  stringPointerFromAttr(plan.Value),
		Effect: stringPointerFromAttr(plan.Effect),
	}

	path := fmt.Sprintf("/taints/%s", id)

	tflog.Info(ctx, "Updating Autoglue taint", map[string]any{
		"id":     id,
		"key":    payload.Key,
		"value":  payload.Value,
		"effect": payload.Effect,
	})

	var apiResp taint
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating taint", err.Error())
		return
	}

	plan.ID = types.StringValue(apiResp.ID)
	plan.Key = types.StringValue(apiResp.Key)
	if apiResp.Value != nil {
		plan.Value = types.StringValue(*apiResp.Value)
	} else {
		plan.Value = types.StringNull()
	}
	plan.Effect = types.StringValue(apiResp.Effect)
	plan.CreatedAt = types.StringValue(apiResp.CreatedAt)
	plan.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *taintResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state taintResourceModel
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

	path := fmt.Sprintf("/taints/%s", id)

	tflog.Info(ctx, "Deleting Autoglue taint", map[string]any{"id": id})

	err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil)
	if err != nil {
		// Treat 404 as already gone; doJSON likely wraps HTTP error,
		// so if you distinguish 404 elsewhere you can mirror that here.
		resp.Diagnostics.AddError("Error deleting taint", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *taintResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Support: terraform import autoglue_taint.example <taint_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
