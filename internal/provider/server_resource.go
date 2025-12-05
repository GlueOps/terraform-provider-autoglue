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
	_ resource.Resource                = &serverResource{}
	_ resource.ResourceWithConfigure   = &serverResource{}
	_ resource.ResourceWithImportState = &serverResource{}
)

type serverResource struct {
	client *autoglueClient
}

type serverResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Hostname         types.String `tfsdk:"hostname"`
	Role             types.String `tfsdk:"role"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	PublicIPAddress  types.String `tfsdk:"public_ip_address"`
	SSHKeyID         types.String `tfsdk:"ssh_key_id"`
	SSHUser          types.String `tfsdk:"ssh_user"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	Status           types.String `tfsdk:"status"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func NewServerResource() resource.Resource {
	return &serverResource{}
}

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Resource type name: autoglue_server
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue server.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique server ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"hostname": resourceschema.StringAttribute{
				Required:    true,
				Description: "Hostname of the server.",
			},

			"role": resourceschema.StringAttribute{
				Optional: true,
				Description: "Logical role for the server. " +
					"Typically one of `master`, `worker`, or `bastion`.",
			},

			"private_ip_address": resourceschema.StringAttribute{
				Optional:    true,
				Description: "Private IP address of the server.",
			},

			"public_ip_address": resourceschema.StringAttribute{
				Optional:    true,
				Description: "Public IP address of the server.",
			},

			"ssh_key_id": resourceschema.StringAttribute{
				Required:    true,
				Description: "SSH key ID associated with this server.",
			},

			"ssh_user": resourceschema.StringAttribute{
				Required:    true,
				Description: "SSH username used to access this server.",
			},

			"organization_id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Owning organization UUID.",
			},

			"status": resourceschema.StringAttribute{
				Computed: true,
				Description: "Server status as reported by Autoglue. " +
					"One of `pending`, `provisioning`, `ready`, or `failed`.",
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

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan serverResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createServerPayload{
		Hostname:         plan.Hostname.ValueString(),
		Role:             plan.Role.ValueString(),
		PrivateIPAddress: plan.PrivateIPAddress.ValueString(),
		PublicIPAddress:  plan.PublicIPAddress.ValueString(),
		SSHKeyID:         plan.SSHKeyID.ValueString(),
		SSHUser:          plan.SSHUser.ValueString(),
	}

	tflog.Info(ctx, "Creating Autoglue server", map[string]any{
		"hostname":   payload.Hostname,
		"role":       payload.Role,
		"private_ip": payload.PrivateIPAddress,
		"public_ip":  payload.PublicIPAddress,
		"ssh_key_id": payload.SSHKeyID,
		"ssh_user":   payload.SSHUser,
	})

	var apiResp server
	if err := r.client.doJSON(ctx, http.MethodPost, "/servers", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating server", err.Error())
		return
	}

	mapServerAPIToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state serverResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Server ID is required in state to read server.")
		return
	}

	path := fmt.Sprintf("/servers/%s", id)

	tflog.Info(ctx, "Reading Autoglue server", map[string]any{"id": id})

	var apiResp server
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		// If the server no longer exists, tell Terraform it's gone.
		// You can refine this if your client differentiates 404s.
		resp.State.RemoveResource(ctx)
		return
	}

	mapServerAPIToModel(&state, &apiResp)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan serverResourceModel
	var state serverResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Server ID is required in state to update server.")
		return
	}

	payload := updateServerPayload{
		Hostname:         plan.Hostname.ValueString(),
		Role:             plan.Role.ValueString(),
		PrivateIPAddress: plan.PrivateIPAddress.ValueString(),
		PublicIPAddress:  plan.PublicIPAddress.ValueString(),
		SSHKeyID:         plan.SSHKeyID.ValueString(),
		SSHUser:          plan.SSHUser.ValueString(),
	}

	path := fmt.Sprintf("/servers/%s", id)

	tflog.Info(ctx, "Updating Autoglue server", map[string]any{
		"id":         id,
		"hostname":   payload.Hostname,
		"role":       payload.Role,
		"private_ip": payload.PrivateIPAddress,
		"public_ip":  payload.PublicIPAddress,
		"ssh_key_id": payload.SSHKeyID,
		"ssh_user":   payload.SSHUser,
	})

	var apiResp server
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating server", err.Error())
		return
	}

	mapServerAPIToModel(&plan, &apiResp)

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state serverResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		// Nothing to do.
		return
	}

	path := fmt.Sprintf("/servers/%s", id)

	tflog.Info(ctx, "Deleting Autoglue server", map[string]any{"id": id})

	// Best-effort delete: if API says 404, treat as already gone.
	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting server", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Support: terraform import autoglue_server.example <server_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper: copy API struct -> Terraform model
func mapServerAPIToModel(m *serverResourceModel, s *server) {
	m.ID = types.StringValue(s.ID)
	m.Hostname = types.StringValue(s.Hostname)
	m.Role = types.StringValue(s.Role)
	m.PrivateIPAddress = types.StringValue(s.PrivateIPAddress)
	m.PublicIPAddress = types.StringValue(s.PublicIPAddress)
	m.SSHKeyID = types.StringValue(s.SSHKeyID)
	m.SSHUser = types.StringValue(s.SSHUser)
	m.OrganizationID = types.StringValue(s.OrganizationID)
	m.Status = types.StringValue(s.Status)
	m.CreatedAt = types.StringValue(s.CreatedAt)
	m.UpdatedAt = types.StringValue(s.UpdatedAt)
}
