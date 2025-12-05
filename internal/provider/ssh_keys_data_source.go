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
	_ datasource.DataSource              = &sshKeysDataSource{}
	_ datasource.DataSourceWithConfigure = &sshKeysDataSource{}
)

type sshKeysDataSource struct {
	client *autoglueClient
}

type sshKeysDataSourceModel struct {
	Keys []sshKeyModel `tfsdk:"keys"`
}

type sshKeyModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Fingerprint    types.String `tfsdk:"fingerprint"`
	PublicKey      types.String `tfsdk:"public_key"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewSSHKeysDataSource() datasource.DataSource {
	return &sshKeysDataSource{}
}

func (d *sshKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_keys"
}

func (d *sshKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists all SSH keys for the organization.",
		Attributes: map[string]dsschema.Attribute{
			"keys": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "All SSH keys visible to the organization.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "SSH key ID.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "SSH key name.",
						},
						"fingerprint": dsschema.StringAttribute{
							Computed:    true,
							Description: "SSH key fingerprint.",
						},
						"public_key": dsschema.StringAttribute{
							Computed:    true,
							Description: "OpenSSH-formatted public key.",
						},
						"organization_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Owning organization UUID.",
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

func (d *sshKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sshKeysDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	tflog.Info(ctx, "Listing Autoglue SSH keys")

	var apiResp []sshKey
	if err := d.client.doJSON(ctx, http.MethodGet, "/ssh", "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing SSH keys", err.Error())
		return
	}

	state := sshKeysDataSourceModel{
		Keys: make([]sshKeyModel, len(apiResp)),
	}

	for i, k := range apiResp {
		state.Keys[i] = sshKeyModel{
			ID:             types.StringValue(k.ID),
			Name:           types.StringValue(k.Name),
			Fingerprint:    types.StringValue(k.Fingerprint),
			PublicKey:      types.StringValue(k.PublicKey),
			OrganizationID: types.StringValue(k.OrganizationID),
			CreatedAt:      types.StringValue(k.CreatedAt),
			UpdatedAt:      types.StringValue(k.UpdatedAt),
		}
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
