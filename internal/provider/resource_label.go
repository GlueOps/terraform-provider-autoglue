package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/glueops/autoglue-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &LabelResource{}
var _ resource.ResourceWithConfigure = &LabelResource{}
var _ resource.ResourceWithImportState = &LabelResource{}

type LabelResource struct{ client *Client }

func NewLabelResource() resource.Resource { return &LabelResource{} }

type labelResModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	Raw            types.String `tfsdk:"raw"`
}

func (r *LabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *LabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Create and manage a label (org-scoped).",
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
			"raw": rschema.StringAttribute{
				Computed:    true,
				Description: "Full server JSON from API.",
			},
		},
	}
}

func (r *LabelResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*Client)
}

func (r *LabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan labelResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := autoglue.NewDtoCreateLabelRequest()
	if !plan.Key.IsNull() {
		k := plan.Key.ValueString()
		payload.SetKey(k)
	}
	if !plan.Value.IsNull() {
		v := plan.Value.ValueString()
		payload.SetValue(v)
	}

	call := r.client.SDK.LabelsAPI.CreateLabel(ctx).Body(*payload)

	out, httpResp, err := call.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Create label failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	raw, _ := json.Marshal(out)

	var state labelResModel
	r.mapRespToState(out, &state)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state labelResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.ID.IsNull() || state.ID.ValueString() == "" {
		// Nothing to read; treat as gone
		resp.State.RemoveResource(ctx)
		return
	}

	call := r.client.SDK.LabelsAPI.GetLabel(ctx, state.ID.ValueString())

	out, httpResp, err := call.Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read label failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	raw, _ := json.Marshal(out)
	r.mapRespToState(out, &state)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan labelResModel
	var prior labelResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := autoglue.NewDtoUpdateLabelRequest()
	if !plan.Key.IsNull() {
		k := plan.Key.ValueString()
		body.SetKey(k)
	}
	if !plan.Value.IsNull() {
		v := plan.Value.ValueString()
		body.SetValue(v)
	}

	call := r.client.SDK.LabelsAPI.UpdateLabel(ctx, prior.ID.ValueString()).Body(*body)

	out, httpResp, err := call.Execute()
	if err != nil {
		// If 404 on update, treat as gone
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Update label failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	raw, _ := json.Marshal(out)

	var newState labelResModel
	r.mapRespToState(out, &newState)
	newState.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *LabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state labelResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	call := r.client.SDK.LabelsAPI.DeleteLabel(ctx, state.ID.ValueString())

	_, httpResp, err := call.Execute()
	if err != nil {
		// If already gone, that's fine
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Delete label failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}
}

func (r *LabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *LabelResource) mapRespToState(s *autoglue.DtoLabelResponse, out *labelResModel) {
	out.ID = types.StringPointerValue(s.Id)
	out.OrganizationID = types.StringPointerValue(s.OrganizationId)
	out.Key = types.StringPointerValue(s.Key)
	out.Value = types.StringPointerValue(s.Value)
	out.CreatedAt = types.StringPointerValue(s.CreatedAt)
	out.UpdatedAt = types.StringPointerValue(s.UpdatedAt)
}
