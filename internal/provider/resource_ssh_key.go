package provider

import (
	"context"
	"fmt"

	"github.com/glueops/autoglue-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SshResource{}
var _ resource.ResourceWithConfigure = &SshResource{}
var _ resource.ResourceWithImportState = &SshResource{}

type SshResource struct{ client *Client }

func NewSshResource() resource.Resource { return &SshResource{} }

type sshResModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Comment       types.String `tfsdk:"comment"`
	Type          types.String `tfsdk:"type"`
	Bits          types.Int64  `tfsdk:"bits"`
	PublicKey     types.String `tfsdk:"public_key"`
	Fingerprint   types.String `tfsdk:"fingerprint"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
	PrivateKeyPEM types.String `tfsdk:"private_key_pem"` // not populated by resource
}

func (r *SshResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (r *SshResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "SSH key ID (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				Required:    true,
				Description: "Display name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comment": rschema.StringAttribute{
				Required:    true,
				Description: "Comment appended to authorized key",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": rschema.StringAttribute{
				Optional:    true,
				Description: "Key type: rsa or ed25519 (default rsa)",
				Validators: []validator.String{
					stringvalidator.OneOf("rsa", "ed25519", ""),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bits": rschema.Int64Attribute{
				Optional:    true,
				Description: "RSA key size (2048/3072/4096). Ignored for ed25519.",
				Validators: []validator.Int64{
					int64validator.OneOf(2048, 3072, 4096),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"public_key": rschema.StringAttribute{
				Computed:    true,
				Description: "OpenSSH authorized key",
			},
			"fingerprint": rschema.StringAttribute{
				Computed:    true,
				Description: "SHA256 fingerprint",
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Creation time (RFC3339, UTC)",
			},
			"updated_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Update time (RFC3339, UTC)",
			},
			"private_key_pem": rschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Private key PEM (resource doesn’t reveal; stays empty).",
			},
		},
	}
}

func (r *SshResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*Client)
}

func (r *SshResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var plan sshResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := autoglue.DtoCreateSSHRequest{
		Name:    plan.Name.ValueStringPointer(),
		Comment: plan.Comment.ValueStringPointer(),
	}
	if t := plan.Type.ValueString(); t != "" {
		body.Type = &t
	}
	if !plan.Bits.IsNull() && !plan.Bits.IsUnknown() {
		b := int32(plan.Bits.ValueInt64())
		body.Bits = &b
	}

	created, httpResp, err := r.client.SDK.SshAPI.CreateSSHKey(ctx).Body(body).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Create ssh key failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	state := sshResModel{
		ID:          types.StringPointerValue(created.Id),
		Name:        types.StringPointerValue(created.Name),
		Comment:     plan.Comment,
		Type:        plan.Type,
		Bits:        plan.Bits,
		PublicKey:   types.StringPointerValue(created.PublicKey),
		Fingerprint: types.StringPointerValue(created.Fingerprint),
		CreatedAt:   types.StringPointerValue(created.CreatedAt),
		UpdatedAt:   types.StringPointerValue(created.UpdatedAt),
		// PrivateKeyPEM left empty (no reveal on resource)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SshResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state sshResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, httpResp, err := r.client.SDK.SshAPI.GetSSHKey(ctx, state.ID.ValueString()).Execute()
	if err != nil {
		if isNotFound(httpResp) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read ssh key failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	// Map from flat fields on DtoSshRevealResponse
	state.Name = types.StringPointerValue(got.Name)
	state.PublicKey = types.StringPointerValue(got.PublicKey)
	state.Fingerprint = types.StringPointerValue(got.Fingerprint)
	state.CreatedAt = types.StringPointerValue(got.CreatedAt)
	state.UpdatedAt = types.StringPointerValue(got.UpdatedAt)
	// We intentionally do NOT set PrivateKeyPEM here (resource doesn't reveal)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SshResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All changes are RequiresReplace; no server-side update.
	var state sshResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SshResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil || r.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var state sshResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResp, err := r.client.SDK.SshAPI.DeleteSSHKey(ctx, state.ID.ValueString()).Execute()
	if err != nil && !isNotFound(httpResp) {
		resp.Diagnostics.AddError("Delete ssh key failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
	}
}

func (r *SshResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
