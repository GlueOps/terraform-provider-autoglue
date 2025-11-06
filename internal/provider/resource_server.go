package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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

var _ resource.Resource = &ServerResource{}
var _ resource.ResourceWithConfigure = &ServerResource{}
var _ resource.ResourceWithImportState = &ServerResource{}

type ServerResource struct{ client *Client }

func NewServerResource() resource.Resource { return &ServerResource{} }

type serverResModel struct {
	// Identity
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`

	// DTO fields
	Hostname         types.String `tfsdk:"hostname"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	PublicIPAddress  types.String `tfsdk:"public_ip_address"`
	Role             types.String `tfsdk:"role"`
	SSHKeyID         types.String `tfsdk:"ssh_key_id"`
	SSHUser          types.String `tfsdk:"ssh_user"`
	Status           types.String `tfsdk:"status"`

	// Raw JSON for debugging
	Raw types.String `tfsdk:"raw"`
}

func (r *ServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

var uuidRx = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func (r *ServerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Create and manage a server (org-scoped). Mirrors API validation for role/status/ssh_key_id.",
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

			"hostname": rschema.StringAttribute{
				Required:    true,
				Description: "Hostname.",
			},
			"private_ip_address": rschema.StringAttribute{
				Required:    true, // API requires on create
				Description: "Private IP address (required).",
			},
			"public_ip_address": rschema.StringAttribute{
				Optional:    true, // required only if role=bastion
				Description: "Public IP address (required when role = bastion).",
			},
			"role": rschema.StringAttribute{
				Required:    true, // API requires on create
				Description: "Server role (e.g., agent/manager/bastion). Lowercased by the provider.",
			},
			"ssh_key_id": rschema.StringAttribute{
				Required:    true, // API requires on create
				Description: "SSH key ID (UUID) that belongs to the org.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(uuidRx, "must be a valid UUID"),
				},
			},
			"ssh_user": rschema.StringAttribute{
				Required:    true, // API requires on create
				Description: "SSH username (required).",
			},
			"status": rschema.StringAttribute{
				Optional:    true, // patchable; if omitted, server sets/returns it
				Computed:    true,
				Description: "Status (pending|provisioning|ready|failed). Lowercased by the provider.",
				Validators: []validator.String{
					stringvalidator.OneOf("", "pending", "provisioning", "ready", "failed"),
				},
			},
			"raw": rschema.StringAttribute{
				Computed:    true,
				Description: "Full server JSON from API.",
			},
		},
	}
}

func (r *ServerResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*Client)
}

func (r *ServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan serverResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normalize + validate against backend rules
	role := strings.ToLower(strings.TrimSpace(plan.Role.ValueString()))
	status := strings.ToLower(strings.TrimSpace(plan.Status.ValueString()))
	pub := strings.TrimSpace(plan.PublicIPAddress.ValueString())

	if role == "bastion" && pub == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("public_ip_address"),
			"Public IP required for bastion",
			"public_ip_address must be set when role is 'bastion'.",
		)
		return
	}

	body := autoglue.DtoCreateServerRequest{
		Hostname:         stringPtrFromAttr(plan.Hostname),
		PrivateIpAddress: stringPtrFromAttr(plan.PrivateIPAddress),
		PublicIpAddress:  nil,
		Role:             &role,
		SshKeyId:         stringPtrFromAttr(plan.SSHKeyID),
		SshUser:          stringPtrFromAttr(plan.SSHUser),
	}
	if pub != "" {
		body.PublicIpAddress = &pub
	}
	if status != "" {
		body.Status = &status // validator already checked allowed values
	}

	created, httpResp, err := r.client.SDK.
		ServersAPI.
		CreateServer(ctx).
		Body(body).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Create server failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	var state serverResModel
	r.mapRespToState(created, &state)
	raw, _ := json.Marshal(created)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state serverResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, httpResp, err := r.client.SDK.
		ServersAPI.
		GetServer(ctx, state.ID.ValueString()).
		Execute()
	if err != nil {
		if isNotFound(httpResp) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read server failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	r.mapRespToState(got, &state)
	raw, _ := json.Marshal(got)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan, state serverResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := autoglue.DtoUpdateServerRequest{}

	// helpers to set changed fields
	setIfChanged := func(p types.String, s types.String, setter func(string)) {
		if p.IsUnknown() || p.IsNull() {
			return
		}
		if s.IsNull() || s.IsUnknown() || p.ValueString() != s.ValueString() {
			setter(p.ValueString())
		}
	}

	setIfChanged(plan.Hostname, state.Hostname, func(v string) { patch.Hostname = strPtr(v) })
	setIfChanged(plan.PrivateIPAddress, state.PrivateIPAddress, func(v string) { patch.PrivateIpAddress = strPtr(v) })
	setIfChanged(plan.PublicIPAddress, state.PublicIPAddress, func(v string) { patch.PublicIpAddress = strPtr(strings.TrimSpace(v)) })
	setIfChanged(plan.SSHUser, state.SSHUser, func(v string) { patch.SshUser = strPtr(v) })

	// Normalize role/status and enforce rules
	if !plan.Role.IsNull() && !plan.Role.IsUnknown() {
		role := strings.ToLower(strings.TrimSpace(plan.Role.ValueString()))
		if state.Role.IsNull() || state.Role.IsUnknown() || role != strings.ToLower(state.Role.ValueString()) {
			patch.Role = &role
		}
	}
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		status := strings.ToLower(strings.TrimSpace(plan.Status.ValueString()))
		patch.Status = &status
	}

	// ssh_key_id: validate UUID via regex at runtime too (gives precise attribute error)
	if !plan.SSHKeyID.IsNull() && !plan.SSHKeyID.IsUnknown() {
		if !uuidRx.MatchString(plan.SSHKeyID.ValueString()) {
			resp.Diagnostics.AddAttributeError(
				path.Root("ssh_key_id"),
				"Invalid ssh_key_id",
				"ssh_key_id must be a valid UUID.",
			)
			return
		}
		if state.SSHKeyID.IsNull() || state.SSHKeyID.IsUnknown() || plan.SSHKeyID.ValueString() != state.SSHKeyID.ValueString() {
			patch.SshKeyId = strPtr(plan.SSHKeyID.ValueString())
		}
	}

	// Bastion rule: if resulting role == "bastion" ensure resulting public IP is non-empty
	resultRole := firstNonEmptyLower(plan.Role, state.Role)
	resultPub := firstNonEmptyTrim(plan.PublicIPAddress, state.PublicIPAddress)
	if resultRole == "bastion" && resultPub == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("public_ip_address"),
			"Public IP required for bastion",
			"public_ip_address must be set when role is 'bastion'.",
		)
		return
	}

	if isEmptyUpdateServerRequest(patch) {
		// Nothing to do; persist state unchanged.
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	updated, httpResp, err := r.client.SDK.
		ServersAPI.
		UpdateServer(ctx, state.ID.ValueString()).
		Body(patch).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Update server failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	r.mapRespToState(updated, &state)
	raw, _ := json.Marshal(updated)
	state.Raw = types.StringValue(string(raw))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state serverResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResp, err := r.client.SDK.
		ServersAPI.
		DeleteServer(ctx, state.ID.ValueString()).
		Execute()
	if err != nil && !isNotFound(httpResp) {
		resp.Diagnostics.AddError("Delete server failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
	}
}

func (r *ServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// --- helpers ---

func (r *ServerResource) mapRespToState(s *autoglue.DtoServerResponse, out *serverResModel) {
	out.ID = types.StringPointerValue(s.Id)
	out.OrganizationID = types.StringPointerValue(s.OrganizationId)
	out.Hostname = types.StringPointerValue(s.Hostname)
	out.PrivateIPAddress = types.StringPointerValue(s.PrivateIpAddress)
	out.PublicIPAddress = types.StringPointerValue(s.PublicIpAddress)
	out.Role = types.StringPointerValue(s.Role)
	out.SSHKeyID = types.StringPointerValue(s.SshKeyId)
	out.SSHUser = types.StringPointerValue(s.SshUser)
	out.Status = types.StringPointerValue(s.Status)
	out.CreatedAt = types.StringPointerValue(s.CreatedAt)
	out.UpdatedAt = types.StringPointerValue(s.UpdatedAt)
}

func stringPtrFromAttr(a types.String) *string {
	if a.IsNull() || a.IsUnknown() {
		return nil
	}
	v := a.ValueString()
	return &v
}

func strPtr(v string) *string { return &v }

func isEmptyUpdateServerRequest(u autoglue.DtoUpdateServerRequest) bool {
	return u.Hostname == nil &&
		u.PrivateIpAddress == nil &&
		u.PublicIpAddress == nil &&
		u.Role == nil &&
		u.SshKeyId == nil &&
		u.SshUser == nil &&
		u.Status == nil
}

func firstNonEmptyLower(a, b types.String) string {
	if !a.IsNull() && !a.IsUnknown() && strings.TrimSpace(a.ValueString()) != "" {
		return strings.ToLower(strings.TrimSpace(a.ValueString()))
	}
	if !b.IsNull() && !b.IsUnknown() {
		return strings.ToLower(strings.TrimSpace(b.ValueString()))
	}
	return ""
}

func firstNonEmptyTrim(a, b types.String) string {
	if !a.IsNull() && !a.IsUnknown() && strings.TrimSpace(a.ValueString()) != "" {
		return strings.TrimSpace(a.ValueString())
	}
	if !b.IsNull() && !b.IsUnknown() {
		return strings.TrimSpace(b.ValueString())
	}
	return ""
}
