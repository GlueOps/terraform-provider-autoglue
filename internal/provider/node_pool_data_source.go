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
	_ datasource.DataSource              = &nodePoolDataSource{}
	_ datasource.DataSourceWithConfigure = &nodePoolDataSource{}
)

type nodePoolDataSource struct {
	client *autoglueClient
}

type nodePoolDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Role           types.String `tfsdk:"role"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

func NewNodePoolDataSource() datasource.DataSource {
	return &nodePoolDataSource{}
}

func (d *nodePoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node_pool"
}

func (d *nodePoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Reads an Autoglue node pool by ID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "Node pool ID to look up.",
			},
			"name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Node pool name.",
			},
			"role": dsschema.StringAttribute{
				Computed:    true,
				Description: "Node pool role (\"master\" or \"worker\").",
			},
			"created_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
			},
			"updated_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp.",
			},
			"organization_id": dsschema.StringAttribute{
				Computed:    true,
				Description: "Owning organization UUID.",
			},
		},
	}
}

func (d *nodePoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *nodePoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config nodePoolDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "id must be set to read a node pool.")
		return
	}

	path := fmt.Sprintf("/node-pools/%s", id)

	tflog.Info(ctx, "Reading Autoglue node pool data source", map[string]any{"id": id})

	var apiResp nodePool
	if err := d.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error reading node pool", err.Error())
		return
	}

	config.Name = types.StringValue(apiResp.Name)
	config.Role = types.StringValue(apiResp.Role)
	config.CreatedAt = types.StringValue(apiResp.CreatedAt)
	config.UpdatedAt = types.StringValue(apiResp.UpdatedAt)
	config.OrganizationID = types.StringValue(apiResp.OrganizationID)

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
