package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/glueops/autoglue-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TaintResource{}
var _ resource.ResourceWithConfigure = &TaintResource{}
var _ resource.ResourceWithImportState = &TaintResource{}

type TaintResource struct{ client *Client }

func NewTaintResource() resource.Resource { return &TaintResource{} }

type taintResModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	Effect         types.String `tfsdk:"effect"`
	Raw            types.String `tfsdk:"raw"`
}

func (r *TaintResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_taint"
}

func (r *TaintResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Create and manage a taint (org-scoped).",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "Server ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": rschema.StringAttribute{Computed: true},
			"created_at":      rschema.StringAttribute{Computed: true},
			"updated_at":      rschema.StringAttribute{Computed: true},
			"key": rschema.StringAttribute{
				Required:    true,
				Description: "Key.",
			},
			"value": rschema.StringAttribute{
				Required:    true,
				Description: "Value.",
			},
			"effect": rschema.StringAttribute{
				Required:    true,
				Description: "Effect.",
				Validators: []validator.String{
					stringvalidator.OneOf("NoSchedule", "NoExecute", "PreferNoSchedule"),
				},
			},
			"raw": rschema.StringAttribute{
				Computed:    true,
				Description: "Full server JSON from API.",
			},
		},
	}
}

func (r *TaintResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*Client)
}

func (r *TaintResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan taintResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := autoglue.NewDtoCreateTaintRequest()
	if !plan.Key.IsNull() {
		k := plan.Key.ValueString()
		payload.SetKey(k)
	}
	if !plan.Value.IsNull() {
		v := plan.Value.ValueString()
		payload.SetValue(v)
	}
	if !plan.Effect.IsNull() {
		e := plan.Effect.ValueString()
		payload.SetEffect(e)
	}

	call := r.client.SDK.TaintsAPI.CreateTaint(ctx).Body(*payload)

	out, httpResp, err := call.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Create taint failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	raw, _ := json.Marshal(out)

	var state taintResModel
	r.mapRespToState(out, &state)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TaintResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state taintResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.ID.IsNull() || state.ID.ValueString() == "" {
		// Nothing to read; treat as gone
		resp.State.RemoveResource(ctx)
		return
	}

	call := r.client.SDK.TaintsAPI.GetTaint(ctx, state.ID.ValueString())

	out, httpResp, err := call.Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read taint failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	raw, _ := json.Marshal(out)
	r.mapRespToState(out, &state)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TaintResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan taintResModel
	var prior taintResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := autoglue.NewDtoUpdateTaintRequest()
	if !plan.Key.IsNull() {
		k := plan.Key.ValueString()
		body.SetKey(k)
	}
	if !plan.Value.IsNull() {
		v := plan.Value.ValueString()
		body.SetValue(v)
	}
	if !plan.Effect.IsNull() {
		e := plan.Effect.ValueString()
		body.SetEffect(e)
	}

	call := r.client.SDK.TaintsAPI.UpdateTaint(ctx, prior.ID.ValueString()).Body(*body)

	out, httpResp, err := call.Execute()
	if err != nil {
		// If 404 on update, treat as gone
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Update taint failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	raw, _ := json.Marshal(out)

	var newState taintResModel
	r.mapRespToState(out, &newState)
	newState.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *TaintResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state taintResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	call := r.client.SDK.TaintsAPI.DeleteTaint(ctx, state.ID.ValueString())

	_, httpResp, err := call.Execute()
	if err != nil {
		// If already gone, that's fine
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Delete taint failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}
}

func (r *TaintResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// --- helpers ---

func (r *TaintResource) mapRespToState(s *autoglue.DtoTaintResponse, out *taintResModel) {
	out.ID = types.StringPointerValue(s.Id)
	out.OrganizationID = types.StringPointerValue(s.OrganizationId)
	out.Key = types.StringPointerValue(s.Key)
	out.Value = types.StringPointerValue(s.Value)
	out.Effect = types.StringPointerValue(s.Effect)
	out.CreatedAt = types.StringPointerValue(s.CreatedAt)
	out.UpdatedAt = types.StringPointerValue(s.UpdatedAt)
}
