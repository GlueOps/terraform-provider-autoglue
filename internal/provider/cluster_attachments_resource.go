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

type attachCaptainDomainPayload struct {
	DomainID string `json:"domain_id"`
}

var (
	_ resource.Resource                = &clusterCaptainDomainResource{}
	_ resource.ResourceWithConfigure   = &clusterCaptainDomainResource{}
	_ resource.ResourceWithImportState = &clusterCaptainDomainResource{}
)

type clusterCaptainDomainResource struct {
	client *autoglueClient
}

type clusterCaptainDomainModel struct {
	ID        types.String `tfsdk:"id"`
	ClusterID types.String `tfsdk:"cluster_id"`
	DomainID  types.String `tfsdk:"domain_id"`
}

func NewClusterCaptainDomainResource() resource.Resource {
	return &clusterCaptainDomainResource{}
}

func (r *clusterCaptainDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_captain_domain"
}

func (r *clusterCaptainDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Attaches a captain domain to a cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Domain ID to attach as captain domain.",
			},
		},
	}
}

func (r *clusterCaptainDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterCaptainDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterCaptainDomainModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	domainID := plan.DomainID.ValueString()
	if clusterID == "" || domainID == "" {
		resp.Diagnostics.AddError("Missing fields", "cluster_id and domain_id must be set.")
		return
	}

	path := fmt.Sprintf("/clusters/%s/captain-domain", clusterID)
	payload := attachCaptainDomainPayload{DomainID: domainID}

	tflog.Info(ctx, "Attaching captain domain to cluster", map[string]any{
		"cluster_id": clusterID,
		"domain_id":  domainID,
	})

	// API returns ClusterResponse; we don't need it, ignore body.
	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching captain domain", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterCaptainDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterCaptainDomainModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/clusters/%s", clusterID)

	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		// cluster gone
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.CaptainDomain == nil {
		// attachment gone
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(clusterID)
	state.DomainID = types.StringValue(apiResp.CaptainDomain.ID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterCaptainDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterCaptainDomainModel
	var state clusterCaptainDomainModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	if plan.DomainID.Equal(state.DomainID) {
		return
	}

	path := fmt.Sprintf("/clusters/%s/captain-domain", clusterID)
	payload := attachCaptainDomainPayload{DomainID: plan.DomainID.ValueString()}

	tflog.Info(ctx, "Updating captain domain attachment", map[string]any{
		"cluster_id": clusterID,
		"domain_id":  payload.DomainID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error updating captain domain attachment", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterCaptainDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterCaptainDomainModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	path := fmt.Sprintf("/clusters/%s/captain-domain", clusterID)

	tflog.Info(ctx, "Detaching captain domain from cluster", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error detaching captain domain", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterCaptainDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_captain_domain.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

type attachRecordSetPayload struct {
	RecordSetID string `json:"record_set_id"`
}

var (
	_ resource.Resource                = &clusterControlPlaneRecordSetResource{}
	_ resource.ResourceWithConfigure   = &clusterControlPlaneRecordSetResource{}
	_ resource.ResourceWithImportState = &clusterControlPlaneRecordSetResource{}
)

type clusterControlPlaneRecordSetResource struct {
	client *autoglueClient
}

type clusterControlPlaneRecordSetModel struct {
	ID          types.String `tfsdk:"id"`
	ClusterID   types.String `tfsdk:"cluster_id"`
	RecordSetID types.String `tfsdk:"record_set_id"`
}

func NewClusterControlPlaneRecordSetResource() resource.Resource {
	return &clusterControlPlaneRecordSetResource{}
}

func (r *clusterControlPlaneRecordSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_control_plane_record_set"
}

func (r *clusterControlPlaneRecordSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Attaches a control-plane record set to a cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"record_set_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Record set ID to attach as control-plane record set.",
			},
		},
	}
}

func (r *clusterControlPlaneRecordSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterControlPlaneRecordSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterControlPlaneRecordSetModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	rsID := plan.RecordSetID.ValueString()
	if clusterID == "" || rsID == "" {
		resp.Diagnostics.AddError("Missing fields", "cluster_id and record_set_id must be set.")
		return
	}

	path := fmt.Sprintf("/clusters/%s/control-plane-record-set", clusterID)
	payload := attachRecordSetPayload{RecordSetID: rsID}

	tflog.Info(ctx, "Attaching control-plane record set to cluster", map[string]any{
		"cluster_id":    clusterID,
		"record_set_id": rsID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching control-plane record set", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterControlPlaneRecordSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterControlPlaneRecordSetModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/clusters/%s", clusterID)
	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.ControlPlaneRecordSet == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(clusterID)
	state.RecordSetID = types.StringValue(apiResp.ControlPlaneRecordSet.ID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterControlPlaneRecordSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterControlPlaneRecordSetModel
	var state clusterControlPlaneRecordSetModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	if plan.RecordSetID.Equal(state.RecordSetID) {
		return
	}

	path := fmt.Sprintf("/clusters/%s/control-plane-record-set", clusterID)
	payload := attachRecordSetPayload{RecordSetID: plan.RecordSetID.ValueString()}

	tflog.Info(ctx, "Updating control-plane record set attachment", map[string]any{
		"cluster_id":    clusterID,
		"record_set_id": payload.RecordSetID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error updating control-plane record set attachment", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterControlPlaneRecordSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterControlPlaneRecordSetModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	path := fmt.Sprintf("/clusters/%s/control-plane-record-set", clusterID)

	tflog.Info(ctx, "Detaching control-plane record set from cluster", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error detaching control-plane record set", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterControlPlaneRecordSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_control_plane_record_set.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

type attachLoadBalancerPayload struct {
	LoadBalancerID string `json:"load_balancer_id"`
}

var (
	_ resource.Resource                = &clusterAppsLoadBalancerResource{}
	_ resource.ResourceWithConfigure   = &clusterAppsLoadBalancerResource{}
	_ resource.ResourceWithImportState = &clusterAppsLoadBalancerResource{}
)

type clusterAppsLoadBalancerResource struct {
	client *autoglueClient
}

type clusterAppsLoadBalancerModel struct {
	ID             types.String `tfsdk:"id"`
	ClusterID      types.String `tfsdk:"cluster_id"`
	LoadBalancerID types.String `tfsdk:"load_balancer_id"`
}

func NewClusterAppsLoadBalancerResource() resource.Resource {
	return &clusterAppsLoadBalancerResource{}
}

func (r *clusterAppsLoadBalancerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_apps_load_balancer"
}

func (r *clusterAppsLoadBalancerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Attaches an apps load balancer to a cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"load_balancer_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Load balancer ID to attach as apps LB.",
			},
		},
	}
}

func (r *clusterAppsLoadBalancerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterAppsLoadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterAppsLoadBalancerModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	lbID := plan.LoadBalancerID.ValueString()
	if clusterID == "" || lbID == "" {
		resp.Diagnostics.AddError("Missing fields", "cluster_id and load_balancer_id must be set.")
		return
	}

	path := fmt.Sprintf("/clusters/%s/apps-load-balancer", clusterID)
	payload := attachLoadBalancerPayload{LoadBalancerID: lbID}

	tflog.Info(ctx, "Attaching apps load balancer to cluster", map[string]any{
		"cluster_id":       clusterID,
		"load_balancer_id": lbID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching apps load balancer", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterAppsLoadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterAppsLoadBalancerModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/clusters/%s", clusterID)
	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.AppsLoadBalancer == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(clusterID)
	state.LoadBalancerID = types.StringValue(apiResp.AppsLoadBalancer.ID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterAppsLoadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterAppsLoadBalancerModel
	var state clusterAppsLoadBalancerModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	if plan.LoadBalancerID.Equal(state.LoadBalancerID) {
		return
	}

	path := fmt.Sprintf("/clusters/%s/apps-load-balancer", clusterID)
	payload := attachLoadBalancerPayload{LoadBalancerID: plan.LoadBalancerID.ValueString()}

	tflog.Info(ctx, "Updating apps load balancer attachment", map[string]any{
		"cluster_id":       clusterID,
		"load_balancer_id": payload.LoadBalancerID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error updating apps load balancer attachment", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterAppsLoadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterAppsLoadBalancerModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	path := fmt.Sprintf("/clusters/%s/apps-load-balancer", clusterID)

	tflog.Info(ctx, "Detaching apps load balancer from cluster", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error detaching apps load balancer", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterAppsLoadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_apps_load_balancer.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

type attachNodePoolPayload struct {
	NodePoolID string `json:"node_pool_id"`
}

var (
	_ resource.Resource                = &clusterNodePoolsResource{}
	_ resource.ResourceWithConfigure   = &clusterNodePoolsResource{}
	_ resource.ResourceWithImportState = &clusterNodePoolsResource{}
)

type clusterNodePoolsResource struct {
	client *autoglueClient
}

type clusterNodePoolsModel struct {
	ID          types.String `tfsdk:"id"`
	ClusterID   types.String `tfsdk:"cluster_id"`
	NodePoolIDs types.Set    `tfsdk:"node_pool_ids"`
}

func NewClusterNodePoolsResource() resource.Resource {
	return &clusterNodePoolsResource{}
}

func (r *clusterNodePoolsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_node_pools"
}

func (r *clusterNodePoolsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages the set of node pools attached to a cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_pool_ids": resourceschema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Set of node pool IDs to attach to the cluster.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *clusterNodePoolsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterNodePoolsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterNodePoolsModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set.")
		return
	}

	nodePoolIDs := stringSetToSlice(ctx, plan.NodePoolIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, npID := range nodePoolIDs {
		path := fmt.Sprintf("/clusters/%s/node-pools", clusterID)
		payload := attachNodePoolPayload{NodePoolID: npID}

		tflog.Info(ctx, "Attaching node pool to cluster", map[string]any{
			"cluster_id":   clusterID,
			"node_pool_id": npID,
		})

		if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
			resp.Diagnostics.AddError("Error attaching node pool to cluster", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(clusterID)

	if err := r.readNodePoolsIntoModel(ctx, clusterID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading node pools after attach", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterNodePoolsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterNodePoolsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.readNodePoolsIntoModel(ctx, clusterID, &state, &resp.Diagnostics); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterNodePoolsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterNodePoolsModel
	var state clusterNodePoolsModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	oldIDs := stringSetToSlice(ctx, state.NodePoolIDs, &resp.Diagnostics)
	newIDs := stringSetToSlice(ctx, plan.NodePoolIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	toDetach, toAttach := sliceDifference(oldIDs, newIDs)

	// Attach new
	for _, npID := range toAttach {
		path := fmt.Sprintf("/clusters/%s/node-pools", clusterID)
		payload := attachNodePoolPayload{NodePoolID: npID}
		tflog.Info(ctx, "Attaching node pool to cluster (update)", map[string]any{
			"cluster_id":   clusterID,
			"node_pool_id": npID,
		})
		if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
			resp.Diagnostics.AddError("Error attaching node pool to cluster", err.Error())
			return
		}
	}

	// Detach removed
	for _, npID := range toDetach {
		path := fmt.Sprintf("/clusters/%s/node-pools/%s", clusterID, npID)
		tflog.Info(ctx, "Detaching node pool from cluster", map[string]any{
			"cluster_id":   clusterID,
			"node_pool_id": npID,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching node pool from cluster", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(clusterID)

	if err := r.readNodePoolsIntoModel(ctx, clusterID, &plan, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Error reading node pools after update", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterNodePoolsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterNodePoolsModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	nodePoolIDs := stringSetToSlice(ctx, state.NodePoolIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, npID := range nodePoolIDs {
		path := fmt.Sprintf("/clusters/%s/node-pools/%s", clusterID, npID)
		tflog.Info(ctx, "Detaching node pool from cluster (delete)", map[string]any{
			"cluster_id":   clusterID,
			"node_pool_id": npID,
		})
		if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
			resp.Diagnostics.AddError("Error detaching node pool from cluster", err.Error())
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterNodePoolsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_node_pools.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

func (r *clusterNodePoolsResource) readNodePoolsIntoModel(
	ctx context.Context,
	clusterID string,
	model *clusterNodePoolsModel,
	diags *diag.Diagnostics,
) error {
	path := fmt.Sprintf("/clusters/%s", clusterID)

	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		return err
	}

	var ids []string
	for _, np := range apiResp.NodePools {
		ids = append(ids, np.ID)
	}

	setVal, d := types.SetValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)

	model.NodePoolIDs = setVal
	model.ClusterID = types.StringValue(clusterID)
	model.ID = types.StringValue(clusterID)

	return nil
}

type setKubeconfigPayload struct {
	Kubeconfig string `json:"kubeconfig"`
}

var (
	_ resource.Resource                = &clusterKubeconfigResource{}
	_ resource.ResourceWithConfigure   = &clusterKubeconfigResource{}
	_ resource.ResourceWithImportState = &clusterKubeconfigResource{}
)

type clusterKubeconfigResource struct {
	client *autoglueClient
}

type clusterKubeconfigModel struct {
	ID         types.String `tfsdk:"id"`
	ClusterID  types.String `tfsdk:"cluster_id"`
	Kubeconfig types.String `tfsdk:"kubeconfig"`
}

func NewClusterKubeconfigResource() resource.Resource {
	return &clusterKubeconfigResource{}
}

func (r *clusterKubeconfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_kubeconfig"
}

func (r *clusterKubeconfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages a cluster's kubeconfig (write-only). " +
			"The kubeconfig is encrypted server-side and never returned by the API.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kubeconfig": resourceschema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Kubeconfig YAML for the cluster. " +
					"This value is never read back from the API; it only drives writes.",
			},
		},
	}
}

func (r *clusterKubeconfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterKubeconfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterKubeconfigModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set.")
		return
	}

	payload := setKubeconfigPayload{
		Kubeconfig: plan.Kubeconfig.ValueString(),
	}

	path := fmt.Sprintf("/clusters/%s/kubeconfig", clusterID)

	tflog.Info(ctx, "Setting cluster kubeconfig", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error setting cluster kubeconfig", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterKubeconfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterKubeconfigModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/clusters/%s", clusterID)
	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// We don't touch state.Kubeconfig; it's write-only and persisted from state.
	state.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterKubeconfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterKubeconfigModel
	var state clusterKubeconfigModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	if plan.Kubeconfig.Equal(state.Kubeconfig) {
		return
	}

	payload := setKubeconfigPayload{
		Kubeconfig: plan.Kubeconfig.ValueString(),
	}
	path := fmt.Sprintf("/clusters/%s/kubeconfig", clusterID)

	tflog.Info(ctx, "Updating cluster kubeconfig", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error updating cluster kubeconfig", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterKubeconfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterKubeconfigModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	path := fmt.Sprintf("/clusters/%s/kubeconfig", clusterID)

	tflog.Info(ctx, "Clearing cluster kubeconfig", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error clearing cluster kubeconfig", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterKubeconfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_kubeconfig.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

var (
	_ resource.Resource                = &clusterGlueOpsLoadBalancerResource{}
	_ resource.ResourceWithConfigure   = &clusterGlueOpsLoadBalancerResource{}
	_ resource.ResourceWithImportState = &clusterGlueOpsLoadBalancerResource{}
)

type clusterGlueOpsLoadBalancerResource struct {
	client *autoglueClient
}

type clusterGlueOpsLoadBalancerModel struct {
	ID             types.String `tfsdk:"id"`
	ClusterID      types.String `tfsdk:"cluster_id"`
	LoadBalancerID types.String `tfsdk:"load_balancer_id"`
}

func NewClusterGlueOpsLoadBalancerResource() resource.Resource {
	return &clusterGlueOpsLoadBalancerResource{}
}

func (r *clusterGlueOpsLoadBalancerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_glueops_load_balancer"
}

func (r *clusterGlueOpsLoadBalancerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Attaches a GlueOps/control-plane load balancer to a cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"load_balancer_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Load balancer ID to attach as GlueOps/control-plane LB.",
			},
		},
	}
}

func (r *clusterGlueOpsLoadBalancerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterGlueOpsLoadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterGlueOpsLoadBalancerModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	lbID := plan.LoadBalancerID.ValueString()
	if clusterID == "" || lbID == "" {
		resp.Diagnostics.AddError("Missing fields", "cluster_id and load_balancer_id must be set.")
		return
	}

	path := fmt.Sprintf("/clusters/%s/glueops-load-balancer", clusterID)
	payload := attachLoadBalancerPayload{LoadBalancerID: lbID}

	tflog.Info(ctx, "Attaching GlueOps load balancer to cluster", map[string]any{
		"cluster_id":       clusterID,
		"load_balancer_id": lbID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching GlueOps load balancer", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterGlueOpsLoadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterGlueOpsLoadBalancerModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/clusters/%s", clusterID)
	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.GlueOpsLoadBalancer == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(clusterID)
	state.LoadBalancerID = types.StringValue(apiResp.GlueOpsLoadBalancer.ID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterGlueOpsLoadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterGlueOpsLoadBalancerModel
	var state clusterGlueOpsLoadBalancerModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	if plan.LoadBalancerID.Equal(state.LoadBalancerID) {
		return
	}

	path := fmt.Sprintf("/clusters/%s/glueops-load-balancer", clusterID)
	payload := attachLoadBalancerPayload{LoadBalancerID: plan.LoadBalancerID.ValueString()}

	tflog.Info(ctx, "Updating GlueOps load balancer attachment", map[string]any{
		"cluster_id":       clusterID,
		"load_balancer_id": payload.LoadBalancerID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error updating GlueOps load balancer attachment", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterGlueOpsLoadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterGlueOpsLoadBalancerModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	path := fmt.Sprintf("/clusters/%s/glueops-load-balancer", clusterID)

	tflog.Info(ctx, "Detaching GlueOps load balancer from cluster", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error detaching GlueOps load balancer", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterGlueOpsLoadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_glueops_load_balancer.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}

type attachBastionPayload struct {
	ServerID string `json:"server_id"`
}

var (
	_ resource.Resource                = &clusterBastionResource{}
	_ resource.ResourceWithConfigure   = &clusterBastionResource{}
	_ resource.ResourceWithImportState = &clusterBastionResource{}
)

type clusterBastionResource struct {
	client *autoglueClient
}

type clusterBastionModel struct {
	ID        types.String `tfsdk:"id"`
	ClusterID types.String `tfsdk:"cluster_id"`
	ServerID  types.String `tfsdk:"server_id"`
}

func NewClusterBastionResource() resource.Resource {
	return &clusterBastionResource{}
}

func (r *clusterBastionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_bastion"
}

func (r *clusterBastionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Attaches a bastion server to a cluster.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Synthetic ID, equal to cluster_id.",
			},
			"cluster_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"server_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "Server ID to attach as bastion.",
			},
		},
	}
}

func (r *clusterBastionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterBastionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterBastionModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := plan.ClusterID.ValueString()
	serverID := plan.ServerID.ValueString()
	if clusterID == "" || serverID == "" {
		resp.Diagnostics.AddError("Missing fields", "cluster_id and server_id must be set.")
		return
	}

	path := fmt.Sprintf("/clusters/%s/bastion", clusterID)
	payload := attachBastionPayload{ServerID: serverID}

	tflog.Info(ctx, "Attaching bastion server to cluster", map[string]any{
		"cluster_id": clusterID,
		"server_id":  serverID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error attaching bastion server", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterBastionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterBastionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/clusters/%s", clusterID)
	var apiResp cluster
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if apiResp.BastionServer == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(clusterID)
	state.ServerID = types.StringValue(apiResp.BastionServer.ID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterBastionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan clusterBastionModel
	var state clusterBastionModel

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
	if clusterID == "" {
		resp.Diagnostics.AddError("Missing cluster_id", "cluster_id must be set in state.")
		return
	}

	if plan.ServerID.Equal(state.ServerID) {
		return
	}

	path := fmt.Sprintf("/clusters/%s/bastion", clusterID)
	payload := attachBastionPayload{ServerID: plan.ServerID.ValueString()}

	tflog.Info(ctx, "Updating bastion server attachment", map[string]any{
		"cluster_id": clusterID,
		"server_id":  payload.ServerID,
	})

	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, nil); err != nil {
		resp.Diagnostics.AddError("Error updating bastion server attachment", err.Error())
		return
	}

	plan.ID = types.StringValue(clusterID)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *clusterBastionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state clusterBastionModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := state.ClusterID.ValueString()
	if clusterID == "" {
		return
	}

	path := fmt.Sprintf("/clusters/%s/bastion", clusterID)

	tflog.Info(ctx, "Detaching bastion server from cluster", map[string]any{
		"cluster_id": clusterID,
	})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error detaching bastion server", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *clusterBastionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_cluster_bastion.example <cluster_id>
	resource.ImportStatePassthroughID(ctx, path.Root("cluster_id"), req, resp)
}
