package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ServersDataSource{}
var _ datasource.DataSourceWithConfigure = &ServersDataSource{}

type ServersDataSource struct{ client *Client }

func NewServersDataSource() datasource.DataSource { return &ServersDataSource{} }

type serversDSModel struct {
	Status types.String `tfsdk:"status"` // pending|provisioning|ready|failed
	Role   types.String `tfsdk:"role"`
	Items  []serverItem `tfsdk:"items"`
}

type serverItem struct {
	// IDs & timestamps
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`

	// Desired/actual fields (DTO)
	Hostname         types.String `tfsdk:"hostname"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	PublicIPAddress  types.String `tfsdk:"public_ip_address"`
	Role             types.String `tfsdk:"role"`
	SSHKeyID         types.String `tfsdk:"ssh_key_id"`
	SSHUser          types.String `tfsdk:"ssh_user"`
	Status           types.String `tfsdk:"status"`

	// Raw JSON payload from API for debugging
	Raw types.String `tfsdk:"raw"`
}

func (d *ServersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_servers"
}

func (d *ServersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "List servers for the organization (org-scoped).",
		Attributes: map[string]dschema.Attribute{
			"status": dschema.StringAttribute{
				Optional:    true,
				Description: "Filter by status (pending|provisioning|ready|failed).",
				Validators: []validator.String{
					stringvalidator.OneOf("pending", "provisioning", "ready", "failed"),
				},
			},
			"role": dschema.StringAttribute{
				Optional:    true,
				Description: "Filter by role.",
			},
			"items": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Servers returned by the API.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":                 dschema.StringAttribute{Computed: true, Description: "Server ID (UUID)."},
						"organization_id":    dschema.StringAttribute{Computed: true},
						"hostname":           dschema.StringAttribute{Computed: true},
						"private_ip_address": dschema.StringAttribute{Computed: true},
						"public_ip_address":  dschema.StringAttribute{Computed: true},
						"role":               dschema.StringAttribute{Computed: true},
						"ssh_key_id":         dschema.StringAttribute{Computed: true},
						"ssh_user":           dschema.StringAttribute{Computed: true},
						"status":             dschema.StringAttribute{Computed: true},
						"created_at":         dschema.StringAttribute{Computed: true, Description: "RFC3339, UTC."},
						"updated_at":         dschema.StringAttribute{Computed: true, Description: "RFC3339, UTC."},
						"raw":                dschema.StringAttribute{Computed: true, Description: "Full JSON for the item."},
					},
				},
			},
		},
	}
}

func (d *ServersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*Client)
}

func (d *ServersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil || d.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var conf serversDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &conf)...)
	if resp.Diagnostics.HasError() {
		return
	}

	call := d.client.SDK.ServersAPI.ListServers(ctx)
	if !conf.Status.IsNull() && !conf.Status.IsUnknown() {
		call = call.Status(conf.Status.ValueString())
	}
	if !conf.Role.IsNull() && !conf.Role.IsUnknown() {
		call = call.Role(conf.Role.ValueString())
	}

	items, httpResp, err := call.Execute()
	if err != nil {
		resp.Diagnostics.AddError("List servers failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	out := serversDSModel{
		Status: conf.Status,
		Role:   conf.Role,
		Items:  make([]serverItem, 0, len(items)),
	}

	for _, s := range items {
		raw, _ := json.Marshal(s)
		out.Items = append(out.Items, serverItem{
			ID:               types.StringPointerValue(s.Id),
			OrganizationID:   types.StringPointerValue(s.OrganizationId),
			Hostname:         types.StringPointerValue(s.Hostname),
			PrivateIPAddress: types.StringPointerValue(s.PrivateIpAddress),
			PublicIPAddress:  types.StringPointerValue(s.PublicIpAddress),
			Role:             types.StringPointerValue(s.Role),
			SSHKeyID:         types.StringPointerValue(s.SshKeyId),
			SSHUser:          types.StringPointerValue(s.SshUser),
			Status:           types.StringPointerValue(s.Status),
			CreatedAt:        types.StringPointerValue(s.CreatedAt),
			UpdatedAt:        types.StringPointerValue(s.UpdatedAt),
			Raw:              types.StringValue(string(raw)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
