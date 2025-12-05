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
	_ datasource.DataSource              = &clusterDataSource{}
	_ datasource.DataSourceWithConfigure = &clusterDataSource{}
)

type clusterDataSource struct {
	client *autoglueClient
}

type clusterDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ClusterProvider types.String `tfsdk:"cluster_provider"`
	Region          types.String `tfsdk:"region"`
	Status          types.String `tfsdk:"status"`
}

func NewClusterDataSource() datasource.DataSource {
	return &clusterDataSource{}
}

func (d *clusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (d *clusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Reads an Autoglue cluster by ID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "Cluster ID to look up.",
			},
			"name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Cluster name.",
			},
			"cluster_provider": dsschema.StringAttribute{
				Computed:    true,
				Description: "Cluster provider.",
			},
			"region": dsschema.StringAttribute{
				Computed:    true,
				Description: "Cluster region.",
			},
			"status": dsschema.StringAttribute{
				Computed:    true,
				Description: "Cluster status.",
			},
		},
	}
}

func (d *clusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *clusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config clusterDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "id must be set to read a cluster.")
		return
	}

	path := fmt.Sprintf("/clusters/%s", id)

	tflog.Info(ctx, "Reading Autoglue cluster data source", map[string]any{"id": id})

	var apiResp cluster
	if err := d.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error reading cluster", err.Error())
		return
	}

	config.Name = types.StringValue(apiResp.Name)
	config.ClusterProvider = types.StringValue(apiResp.ClusterProvider)
	config.Region = types.StringValue(apiResp.Region)
	config.Status = types.StringValue(apiResp.Status)

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
