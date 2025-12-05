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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ---- Common helpers ----

func stringSetToSlice(ctx context.Context, s types.Set, diags *diag.Diagnostics) []string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}

	var out []string
	d := s.ElementsAs(ctx, &out, false)
	diags.Append(d...)
	if diags.HasError() {
		return nil
	}
	return out
}

func sliceDifference(a, b []string) (onlyA, onlyB []string) {
	ma := make(map[string]struct{}, len(a))
	mb := make(map[string]struct{}, len(b))
	for _, v := range a {
		ma[v] = struct{}{}
	}
	for _, v := range b {
		mb[v] = struct{}{}
	}

	for v := range ma {
		if _, ok := mb[v]; !ok {
			onlyA = append(onlyA, v)
		}
	}
	for v := range mb {
		if _, ok := ma[v]; !ok {
			onlyB = append(onlyB, v)
		}
	}
	return
}

// ---- Servers attachment ----

type attachServersPayload struct {
	ServerIDs []string `json:"server_ids"`
}

var (
	_ resource.Resource                = &nodePoolServersResource{}
	_ resource.ResourceWithConfigure   = &nodePoolServersResource{}
	_ resource.ResourceWithImportState = &nodePoolServersResource{}
)

type nodePoolServersResource struct {
	client *autoglueClient
}

type nodePoolServersModel struct {
	ID         types.String `tfsdk:"id"`
	NodePoolID types.String `tfsdk:"node_pool_id"`
	ServerIDs  types.Set    `tfsdk:"server_ids"`
}

func NewNodePoolServersResource() resource.Resource {
	return &nodePoolServersResource{}
}

func (r *nodePoolServersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_pool_servers"
}

func (r *nodePoolServersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages the set of servers attached to a node pool.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID for this attachment set (node_pool_id).",
			},
			"node_pool_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Node pool ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"server_ids": resourceschema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Set of server IDs to attach to the node pool.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *nodePoolServersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nodePoolServersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolServersModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodePoolID := plan.NodePoolID.ValueString()
	if nodePoolID == "" {
		resp.Diagnostics.AddError("Missing node_pool_id", "node_pool_id must be set.")
		return
	}

	serverIDs := stringSetToSlice(ctx, plan.ServerIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := attachServersPayload{ServerIDs: serverIDs}
	path := fmt.Sprintf("/node-pools/%s/servers", nodePoolID)

	tflog.Info(ctx, "Attaching servers to node pool", map[string]any{
		"node_pool_id": nodePoolID,
		"server_ids":   serverIDs,
	})

	// API returns dto.ServerResponse[] but we only care about IDs; ignore response body.
	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching servers to node pool", err.Error())
		return
	}

	plan.ID = types.StringValue(nodePoolID)

	// Refresh from API to sync actual attachments
	if err := r.readServersIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading servers after attach", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolServersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolServersModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodePoolID := state.NodePoolID.ValueString()
	if nodePoolID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.readServersIntoModel(ctx, nodePoolID, &state, &resp.Diagnostics); err != nil {
		// If node pool is gone or request fails, drop from state
		resp.State.RemoveResource(ctx)
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolServersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolServersModel
	var state nodePoolServersModel

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

	nodePoolID := state.NodePoolID.ValueString()
	if nodePoolID == "" {
		resp.Diagnostics.AddError("Missing node_pool_id", "node_pool_id must be set in state.")
		return
	}

	oldIDs := stringSetToSlice(ctx, state.ServerIDs, &resp.Diagnostics)
	newIDs := stringSetToSlice(ctx, plan.ServerIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	toDetach, toAttach := sliceDifference(oldIDs, newIDs)

	// Attach new
	if len(toAttach) > 0 {
		path := fmt.Sprintf("/node-pools/%s/servers", nodePoolID)
		payload := attachServersPayload{ServerIDs: toAttach}
		tflog.Info(ctx, "Attaching servers to node pool (update)", map[string]any{
			"node_pool_id": nodePoolID,
			"server_ids":   toAttach,
		})
		if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
			resp.Diagnostics.AddError("Error attaching servers to node pool", err.Error())
			return
		}
	}

	// Detach removed
	for _, sid := range toDetach {
		path := fmt.Sprintf("/node-pools/%s/servers/%s", nodePoolID, sid)
		tflog.Info(ctx, "Detaching server from node pool", map[string]any{
			"node_pool_id": nodePoolID,
			"server_id":    sid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching server from node pool", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readServersIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading servers after update", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolServersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolServersModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodePoolID := state.NodePoolID.ValueString()
	if nodePoolID == "" {
		return
	}

	serverIDs := stringSetToSlice(ctx, state.ServerIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, sid := range serverIDs {
		path := fmt.Sprintf("/node-pools/%s/servers/%s", nodePoolID, sid)
		tflog.Info(ctx, "Detaching server from node pool (delete)", map[string]any{
			"node_pool_id": nodePoolID,
			"server_id":    sid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching server from node pool", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *nodePoolServersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_node_pool_servers.example <node_pool_id>
	resource.ImportStatePassthroughID(ctx, path.Root("node_pool_id"), req, resp)
}

func (r *nodePoolServersResource) readServersIntoModel(
	ctx context.Context,
	nodePoolID string,
	model *nodePoolServersModel,
	diags *diag.Diagnostics,
) error {
	path := fmt.Sprintf("/node-pools/%s/servers", nodePoolID)

	var apiResp []server // uses your existing `server` type
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		return err
	}

	var ids []string
	for _, s := range apiResp {
		ids = append(ids, s.ID)
	}

	setVal, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	model.ServerIDs = setVal
	model.NodePoolID = types.StringValue(nodePoolID)
	model.ID = types.StringValue(nodePoolID)

	return nil
}
