package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &serversDataSource{}
	_ datasource.DataSourceWithConfigure = &serversDataSource{}
)

type serversDataSource struct {
	client *autoglueClient
}

type serversDataSourceModel struct {
	Servers []serversDataSourceServerModel `tfsdk:"servers"`
}

type serversDataSourceServerModel struct {
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

func NewServersDataSource() datasource.DataSource {
	return &serversDataSource{}
}

func (d *serversDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	// Data source type name: autoglue_servers
	resp.TypeName = req.ProviderTypeName + "_servers"
}

func (d *serversDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists all servers visible to the organization.",
		Attributes: map[string]dsschema.Attribute{
			"servers": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "All servers visible to the organization.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Server ID.",
						},
						"hostname": dsschema.StringAttribute{
							Computed:    true,
							Description: "Server hostname.",
						},
						"role": dsschema.StringAttribute{
							Computed:    true,
							Description: "Server role (for example `master`, `worker`, `bastion`).",
						},
						"private_ip_address": dsschema.StringAttribute{
							Computed:    true,
							Description: "Private IP address.",
						},
						"public_ip_address": dsschema.StringAttribute{
							Computed:    true,
							Description: "Public IP address.",
						},
						"ssh_key_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "SSH key ID associated with this server.",
						},
						"ssh_user": dsschema.StringAttribute{
							Computed:    true,
							Description: "SSH username.",
						},
						"organization_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Owning organization UUID.",
						},
						"status": dsschema.StringAttribute{
							Computed:    true,
							Description: "Server status as reported by Autoglue.",
						},
						"created_at": dsschema.StringAttribute{
							Computed:    true,
							Description: "Creation timestamp.",
						},
						"updated_at": dsschema.StringAttribute{
							Computed:    true,
							Description: "Last update timestamp.",
						},
					},
				},
			},
		},
	}
}

func (d *serversDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = client
}

func (d *serversDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	tflog.Info(ctx, "Listing Autoglue servers")

	var apiResp []server
	if err := d.client.doJSON(ctx, http.MethodGet, "/servers", "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing servers", err.Error())
		return
	}

	state := serversDataSourceModel{
		Servers: make([]serversDataSourceServerModel, 0, len(apiResp)),
	}

	for _, s := range apiResp {
		state.Servers = append(state.Servers, serversDataSourceServerModel{
			ID:               types.StringValue(s.ID),
			Hostname:         types.StringValue(s.Hostname),
			Role:             types.StringValue(s.Role),
			PrivateIPAddress: types.StringValue(s.PrivateIPAddress),
			PublicIPAddress:  types.StringValue(s.PublicIPAddress),
			SSHKeyID:         types.StringValue(s.SSHKeyID),
			SSHUser:          types.StringValue(s.SSHUser),
			OrganizationID:   types.StringValue(s.OrganizationID),
			Status:           types.StringValue(s.Status),
			CreatedAt:        types.StringValue(s.CreatedAt),
			UpdatedAt:        types.StringValue(s.UpdatedAt),
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
