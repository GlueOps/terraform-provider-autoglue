package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &sshKeyResource{}
	_ resource.ResourceWithConfigure   = &sshKeyResource{}
	_ resource.ResourceWithImportState = &sshKeyResource{}
)

type sshKeyResource struct {
	client *autoglueClient
}

type sshKeyResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	Bits           types.Int64  `tfsdk:"bits"`
	Comment        types.String `tfsdk:"comment"`
	Fingerprint    types.String `tfsdk:"fingerprint"`
	PublicKey      types.String `tfsdk:"public_key"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewSSHKeyResource() resource.Resource {
	return &sshKeyResource{}
}

func (r *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (r *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue SSH keypair (public metadata only).",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique SSH key ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:    true,
				Description: "Human-readable SSH key name.",
			},
			"type": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: `"rsa" (default) or "ed25519".`,
				Default:     stringdefault.StaticString("rsa"),
			},
			"bits": resourceschema.Int64Attribute{
				Optional: true,
				Description: "Key size in bits (RSA only; typically 2048, 3072, or 4096). " +
					"Ignored for ED25519.",
			},
			"comment": resourceschema.StringAttribute{
				Optional:    true,
				Description: "Optional key comment (e.g. deploy@autoglue).",
			},
			"fingerprint": resourceschema.StringAttribute{
				Computed:    true,
				Description: "SSH key fingerprint.",
			},
			"public_key": resourceschema.StringAttribute{
				Computed:    true,
				Description: "OpenSSH-formatted public key.",
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

func (r *sshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan sshKeyResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var bitsPtr *int
	if !plan.Bits.IsNull() && !plan.Bits.IsUnknown() {
		b := int(plan.Bits.ValueInt64())
		bitsPtr = &b
	}

	payload := createSSHKeyPayload{
		Name:    plan.Name.ValueString(),
		Type:    plan.Type.ValueString(),
		Comment: plan.Comment.ValueString(),
		Bits:    bitsPtr,
	}

	tflog.Info(ctx, "Creating Autoglue SSH key", map[string]any{
		"name": payload.Name,
		"type": payload.Type,
		"bits": payload.Bits,
	})

	var apiResp sshKey
	if err := r.client.doJSON(ctx, http.MethodPost, "/ssh", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating SSH key", err.Error())
		return
	}

	// Fill state from API + plan
	plan.ID = types.StringValue(apiResp.ID)
	plan.Fingerprint = types.StringValue(apiResp.Fingerprint)
	plan.PublicKey = types.StringValue(apiResp.PublicKey)
	plan.OrganizationID = types.StringValue(apiResp.OrganizationID)
	plan.CreatedAt = types.StringValue(apiResp.CreatedAt)
	plan.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	if plan.Type.IsNull() || plan.Type.IsUnknown() {
		// API doesn't echo type; keep default if unset
		plan.Type = types.StringValue("rsa")
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state sshKeyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "SSH key ID is required in state to read an SSH key.")
		return
	}

	path := fmt.Sprintf("/ssh/%s", id)

	tflog.Info(ctx, "Reading Autoglue SSH key", map[string]any{"id": id})

	var apiResp sshKey
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading SSH key", fmt.Sprintf("Error reading SSH key: %s", err.Error()))
		return
	}

	// Update only fields we actually get from API; keep plan-only fields (type/bits/comment).
	state.Name = types.StringValue(apiResp.Name)
	state.Fingerprint = types.StringValue(apiResp.Fingerprint)
	state.PublicKey = types.StringValue(apiResp.PublicKey)
	state.OrganizationID = types.StringValue(apiResp.OrganizationID)
	state.CreatedAt = types.StringValue(apiResp.CreatedAt)
	state.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// There is no update endpoint for SSH keys in the API, so treat this as
	// "recreate on change" by leaving this method unimplemented. Framework will
	// force recreation when any mutable attribute changes.
	var state sshKeyResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	// Just re-read to keep in sync (or you could AddError to make changes explicit).
	r.Read(ctx, resource.ReadRequest{State: req.State}, &resource.ReadResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
	})
}

func (r *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state sshKeyResourceModel
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

	path := fmt.Sprintf("/ssh/%s", id)

	tflog.Info(ctx, "Deleting Autoglue SSH key", map[string]any{"id": id})

	err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting SSH key", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *sshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_ssh_key.my_key <ssh_key_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
