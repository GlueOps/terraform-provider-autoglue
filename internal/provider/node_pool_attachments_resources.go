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

//
// ---- Common helpers ----
//

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

// sliceDifference returns:
//   - onlyA: values in a but not in b
//   - onlyB: values in b but not in a
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

//
// ---- Servers attachment (existing) ----
//

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

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching servers to node pool", err.Error())
		return
	}

	plan.ID = types.StringValue(nodePoolID)

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
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading servers for node pool", fmt.Sprintf("Error reading servers for node pool: %s", err))
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

	var apiResp []server // assumes you already have `type server struct{ ID string ... }`
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		return err
	}

	ids := make([]string, 0, len(apiResp))
	for _, s := range apiResp {
		ids = append(ids, s.ID)
	}

	setVal, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	if setVal.IsNull() || setVal.IsUnknown() {
		setVal = model.ServerIDs
	}

	model.ServerIDs = setVal
	model.NodePoolID = types.StringValue(nodePoolID)
	model.ID = types.StringValue(nodePoolID)

	return nil
}

//
// ---- Taints attachment ----
//

type attachTaintsPayload struct {
	TaintIDs []string `json:"taint_ids"`
}

var (
	_ resource.Resource                = &nodePoolTaintsResource{}
	_ resource.ResourceWithConfigure   = &nodePoolTaintsResource{}
	_ resource.ResourceWithImportState = &nodePoolTaintsResource{}
)

type nodePoolTaintsResource struct {
	client *autoglueClient
}

type nodePoolTaintsModel struct {
	ID         types.String `tfsdk:"id"`
	NodePoolID types.String `tfsdk:"node_pool_id"`
	TaintIDs   types.Set    `tfsdk:"taint_ids"`
}

func NewNodePoolTaintsResource() resource.Resource {
	return &nodePoolTaintsResource{}
}

func (r *nodePoolTaintsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_pool_taints"
}

func (r *nodePoolTaintsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages the set of taints attached to a node pool.",
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
			"taint_ids": resourceschema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Set of taint IDs to attach to the node pool.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *nodePoolTaintsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nodePoolTaintsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolTaintsModel
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

	taintIDs := stringSetToSlice(ctx, plan.TaintIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := attachTaintsPayload{TaintIDs: taintIDs}
	path := fmt.Sprintf("/node-pools/%s/taints", nodePoolID)

	tflog.Info(ctx, "Attaching taints to node pool", map[string]any{
		"node_pool_id": nodePoolID,
		"taint_ids":    taintIDs,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching taints to node pool", err.Error())
		return
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readTaintsIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading taints after attach", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolTaintsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolTaintsModel
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

	if err := r.readTaintsIntoModel(ctx, nodePoolID, &state, &resp.Diagnostics); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading taints for node pool", fmt.Sprintf("Error reading taints for node pool: %s", err))
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolTaintsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolTaintsModel
	var state nodePoolTaintsModel

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

	oldIDs := stringSetToSlice(ctx, state.TaintIDs, &resp.Diagnostics)
	newIDs := stringSetToSlice(ctx, plan.TaintIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	toDetach, toAttach := sliceDifference(oldIDs, newIDs)

	if len(toAttach) > 0 {
		path := fmt.Sprintf("/node-pools/%s/taints", nodePoolID)
		payload := attachTaintsPayload{TaintIDs: toAttach}
		tflog.Info(ctx, "Attaching taints to node pool (update)", map[string]any{
			"node_pool_id": nodePoolID,
			"taint_ids":    toAttach,
		})
		if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
			resp.Diagnostics.AddError("Error attaching taints to node pool", err.Error())
			return
		}
	}

	for _, tid := range toDetach {
		path := fmt.Sprintf("/node-pools/%s/taints/%s", nodePoolID, tid)
		tflog.Info(ctx, "Detaching taint from node pool", map[string]any{
			"node_pool_id": nodePoolID,
			"taint_id":     tid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching taint from node pool", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readTaintsIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading taints after update", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolTaintsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolTaintsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodePoolID := state.NodePoolID.ValueString()
	if nodePoolID == "" {
		return
	}

	taintIDs := stringSetToSlice(ctx, state.TaintIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, tid := range taintIDs {
		path := fmt.Sprintf("/node-pools/%s/taints/%s", nodePoolID, tid)
		tflog.Info(ctx, "Detaching taint from node pool (delete)", map[string]any{
			"node_pool_id": nodePoolID,
			"taint_id":     tid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching taint from node pool", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *nodePoolTaintsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_node_pool_taints.example <node_pool_id>
	resource.ImportStatePassthroughID(ctx, path.Root("node_pool_id"), req, resp)
}

func (r *nodePoolTaintsResource) readTaintsIntoModel(
	ctx context.Context,
	nodePoolID string,
	model *nodePoolTaintsModel,
	diags *diag.Diagnostics,
) error {
	path := fmt.Sprintf("/node-pools/%s/taints", nodePoolID)

	var apiResp []taint // assumes you have `type taint struct{ ID string ... }`
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		return err
	}

	ids := make([]string, 0, len(apiResp))
	for _, t := range apiResp {
		ids = append(ids, t.ID)
	}

	setVal, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	if setVal.IsNull() || setVal.IsUnknown() {
		setVal = model.TaintIDs
	}

	model.TaintIDs = setVal
	model.NodePoolID = types.StringValue(nodePoolID)
	model.ID = types.StringValue(nodePoolID)

	return nil
}

//
// ---- Labels attachment ----
//

type attachLabelsPayload struct {
	LabelIDs []string `json:"label_ids"`
}

var (
	_ resource.Resource                = &nodePoolLabelsResource{}
	_ resource.ResourceWithConfigure   = &nodePoolLabelsResource{}
	_ resource.ResourceWithImportState = &nodePoolLabelsResource{}
)

type nodePoolLabelsResource struct {
	client *autoglueClient
}

type nodePoolLabelsModel struct {
	ID         types.String `tfsdk:"id"`
	NodePoolID types.String `tfsdk:"node_pool_id"`
	LabelIDs   types.Set    `tfsdk:"label_ids"`
}

func NewNodePoolLabelsResource() resource.Resource {
	return &nodePoolLabelsResource{}
}

func (r *nodePoolLabelsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_pool_labels"
}

func (r *nodePoolLabelsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages the set of labels attached to a node pool.",
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
			"label_ids": resourceschema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Set of label IDs to attach to the node pool.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *nodePoolLabelsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nodePoolLabelsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolLabelsModel
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

	labelIDs := stringSetToSlice(ctx, plan.LabelIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := attachLabelsPayload{LabelIDs: labelIDs}
	path := fmt.Sprintf("/node-pools/%s/labels", nodePoolID)

	tflog.Info(ctx, "Attaching labels to node pool", map[string]any{
		"node_pool_id": nodePoolID,
		"label_ids":    labelIDs,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching labels to node pool", err.Error())
		return
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readLabelsIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading labels after attach", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolLabelsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolLabelsModel
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

	if err := r.readLabelsIntoModel(ctx, nodePoolID, &state, &resp.Diagnostics); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading labels for node pool", fmt.Sprintf("Error reading labels for node pool: %s", err))
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolLabelsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolLabelsModel
	var state nodePoolLabelsModel

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

	oldIDs := stringSetToSlice(ctx, state.LabelIDs, &resp.Diagnostics)
	newIDs := stringSetToSlice(ctx, plan.LabelIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	toDetach, toAttach := sliceDifference(oldIDs, newIDs)

	if len(toAttach) > 0 {
		path := fmt.Sprintf("/node-pools/%s/labels", nodePoolID)
		payload := attachLabelsPayload{LabelIDs: toAttach}
		tflog.Info(ctx, "Attaching labels to node pool (update)", map[string]any{
			"node_pool_id": nodePoolID,
			"label_ids":    toAttach,
		})
		if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
			resp.Diagnostics.AddError("Error attaching labels to node pool", err.Error())
			return
		}
	}

	for _, lid := range toDetach {
		path := fmt.Sprintf("/node-pools/%s/labels/%s", nodePoolID, lid)
		tflog.Info(ctx, "Detaching label from node pool", map[string]any{
			"node_pool_id": nodePoolID,
			"label_id":     lid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching label from node pool", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readLabelsIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading labels after update", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolLabelsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolLabelsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodePoolID := state.NodePoolID.ValueString()
	if nodePoolID == "" {
		return
	}

	labelIDs := stringSetToSlice(ctx, state.LabelIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, lid := range labelIDs {
		path := fmt.Sprintf("/node-pools/%s/labels/%s", nodePoolID, lid)
		tflog.Info(ctx, "Detaching label from node pool (delete)", map[string]any{
			"node_pool_id": nodePoolID,
			"label_id":     lid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching label from node pool", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *nodePoolLabelsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_node_pool_labels.example <node_pool_id>
	resource.ImportStatePassthroughID(ctx, path.Root("node_pool_id"), req, resp)
}

func (r *nodePoolLabelsResource) readLabelsIntoModel(
	ctx context.Context,
	nodePoolID string,
	model *nodePoolLabelsModel,
	diags *diag.Diagnostics,
) error {
	path := fmt.Sprintf("/node-pools/%s/labels", nodePoolID)

	var apiResp []label // assumes you have `type label struct{ ID string ... }`
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		return err
	}

	ids := make([]string, 0, len(apiResp))
	for _, l := range apiResp {
		ids = append(ids, l.ID)
	}

	setVal, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	if setVal.IsNull() || setVal.IsUnknown() {
		setVal = model.LabelIDs
	}

	model.LabelIDs = setVal
	model.NodePoolID = types.StringValue(nodePoolID)
	model.ID = types.StringValue(nodePoolID)

	return nil
}

//
// ---- Annotations attachment ----
//

type attachAnnotationsPayload struct {
	AnnotationIDs []string `json:"annotation_ids"`
}

var (
	_ resource.Resource                = &nodePoolAnnotationsResource{}
	_ resource.ResourceWithConfigure   = &nodePoolAnnotationsResource{}
	_ resource.ResourceWithImportState = &nodePoolAnnotationsResource{}
)

type nodePoolAnnotationsResource struct {
	client *autoglueClient
}

type nodePoolAnnotationsModel struct {
	ID            types.String `tfsdk:"id"`
	NodePoolID    types.String `tfsdk:"node_pool_id"`
	AnnotationIDs types.Set    `tfsdk:"annotation_ids"`
}

func NewNodePoolAnnotationsResource() resource.Resource {
	return &nodePoolAnnotationsResource{}
}

func (r *nodePoolAnnotationsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_pool_annotations"
}

func (r *nodePoolAnnotationsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages the set of annotations attached to a node pool.",
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
			"annotation_ids": resourceschema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Set of annotation IDs to attach to the node pool.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *nodePoolAnnotationsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nodePoolAnnotationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolAnnotationsModel
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

	annotationIDs := stringSetToSlice(ctx, plan.AnnotationIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := attachAnnotationsPayload{AnnotationIDs: annotationIDs}
	path := fmt.Sprintf("/node-pools/%s/annotations", nodePoolID)

	tflog.Info(ctx, "Attaching annotations to node pool", map[string]any{
		"node_pool_id":   nodePoolID,
		"annotation_ids": annotationIDs,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching annotations to node pool", err.Error())
		return
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readAnnotationsIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading annotations after attach", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolAnnotationsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolAnnotationsModel
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

	if err := r.readAnnotationsIntoModel(ctx, nodePoolID, &state, &resp.Diagnostics); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading annotations for node pool", fmt.Sprintf("Error reading annotations for node pool: %s", err))
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolAnnotationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan nodePoolAnnotationsModel
	var state nodePoolAnnotationsModel

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

	oldIDs := stringSetToSlice(ctx, state.AnnotationIDs, &resp.Diagnostics)
	newIDs := stringSetToSlice(ctx, plan.AnnotationIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	toDetach, toAttach := sliceDifference(oldIDs, newIDs)

	if len(toAttach) > 0 {
		path := fmt.Sprintf("/node-pools/%s/annotations", nodePoolID)
		payload := attachAnnotationsPayload{AnnotationIDs: toAttach}
		tflog.Info(ctx, "Attaching annotations to node pool (update)", map[string]any{
			"node_pool_id":   nodePoolID,
			"annotation_ids": toAttach,
		})
		if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
			resp.Diagnostics.AddError("Error attaching annotations to node pool", err.Error())
			return
		}
	}

	for _, aid := range toDetach {
		path := fmt.Sprintf("/node-pools/%s/annotations/%s", nodePoolID, aid)
		tflog.Info(ctx, "Detaching annotation from node pool", map[string]any{
			"node_pool_id":  nodePoolID,
			"annotation_id": aid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching annotation from node pool", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(nodePoolID)

	if err := r.readAnnotationsIntoModel(ctx, nodePoolID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading annotations after update", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *nodePoolAnnotationsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state nodePoolAnnotationsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodePoolID := state.NodePoolID.ValueString()
	if nodePoolID == "" {
		return
	}

	annotationIDs := stringSetToSlice(ctx, state.AnnotationIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, aid := range annotationIDs {
		path := fmt.Sprintf("/node-pools/%s/annotations/%s", nodePoolID, aid)
		tflog.Info(ctx, "Detaching annotation from node pool (delete)", map[string]any{
			"node_pool_id":  nodePoolID,
			"annotation_id": aid,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching annotation from node pool", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *nodePoolAnnotationsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_node_pool_annotations.example <node_pool_id>
	resource.ImportStatePassthroughID(ctx, path.Root("node_pool_id"), req, resp)
}

func (r *nodePoolAnnotationsResource) readAnnotationsIntoModel(
	ctx context.Context,
	nodePoolID string,
	model *nodePoolAnnotationsModel,
	diags *diag.Diagnostics,
) error {
	path := fmt.Sprintf("/node-pools/%s/annotations", nodePoolID)

	var apiResp []annotation // assumes you have `type annotation struct{ ID string ... }`
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		return err
	}

	ids := make([]string, 0, len(apiResp))
	for _, a := range apiResp {
		ids = append(ids, a.ID)
	}

	setVal, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	if setVal.IsNull() || setVal.IsUnknown() {
		setVal = model.AnnotationIDs
	}

	model.AnnotationIDs = setVal
	model.NodePoolID = types.StringValue(nodePoolID)
	model.ID = types.StringValue(nodePoolID)

	return nil
}
