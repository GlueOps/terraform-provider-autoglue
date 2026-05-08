package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &loadBalancersDataSource{}
	_ datasource.DataSourceWithConfigure = &loadBalancersDataSource{}
)

type loadBalancersDataSource struct {
	client *autoglueClient
}

type loadBalancersDataSourceModel struct {
	Kind   types.String `tfsdk:"kind"`
	Name   types.String `tfsdk:"name"`
	Search types.String `tfsdk:"search"`

	LoadBalancers []loadBalancerListItemModel `tfsdk:"load_balancers"`
}

type loadBalancerListItemModel struct {
	ID               types.String `tfsdk:"id"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	Name             types.String `tfsdk:"name"`
	Kind             types.String `tfsdk:"kind"`
	PublicIPAddress  types.String `tfsdk:"public_ip_address"`
	PrivateIPAddress types.String `tfsdk:"private_ip_address"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func NewLoadBalancersDataSource() datasource.DataSource {
	return &loadBalancersDataSource{}
}

func (d *loadBalancersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_load_balancers"
}

func (d *loadBalancersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists Autoglue load balancers for the current organization.",
		Attributes: map[string]dsschema.Attribute{
			"kind": dsschema.StringAttribute{
				Optional:    true,
				Description: "Optional kind filter (`glueops` or `public`).",
			},
			"name": dsschema.StringAttribute{
				Optional:    true,
				Description: "Optional exact name filter.",
			},
			"search": dsschema.StringAttribute{
				Optional: true,
				Description: "Optional substring match filter applied to load balancer name " +
					"(maps to the `q` query parameter).",
			},

			"load_balancers": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching load balancers.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Load balancer ID.",
						},
						"organization_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Owning organization UUID.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Load balancer name.",
						},
						"kind": dsschema.StringAttribute{
							Computed:    true,
							Description: "Load balancer kind.",
						},
						"public_ip_address": dsschema.StringAttribute{
							Computed:    true,
							Description: "Public IPv4 address.",
						},
						"private_ip_address": dsschema.StringAttribute{
							Computed:    true,
							Description: "Private IPv4 address.",
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

func (d *loadBalancersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *loadBalancersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config loadBalancersDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	q := url.Values{}
	if !config.Kind.IsNull() && !config.Kind.IsUnknown() {
		q.Set("kind", strings.TrimSpace(strings.ToLower(config.Kind.ValueString())))
	}
	if !config.Name.IsNull() && !config.Name.IsUnknown() {
		q.Set("name", strings.TrimSpace(config.Name.ValueString()))
	}
	if !config.Search.IsNull() && !config.Search.IsUnknown() {
		q.Set("q", strings.TrimSpace(config.Search.ValueString()))
	}

	query := q.Encode()
	apiPath := "/load-balancers"

	tflog.Info(ctx, "Listing Autoglue load balancers", map[string]any{"query": query})

	var apiResp []loadBalancer
	if err := d.client.doJSON(ctx, http.MethodGet, apiPath, query, nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing load balancers", err.Error())
		return
	}

	config.LoadBalancers = make([]loadBalancerListItemModel, 0, len(apiResp))
	for _, lb := range apiResp {
		config.LoadBalancers = append(config.LoadBalancers, loadBalancerListItemModel{
			ID:               types.StringValue(lb.ID),
			OrganizationID:   types.StringValue(lb.OrganizationID),
			Name:             types.StringValue(lb.Name),
			Kind:             types.StringValue(lb.Kind),
			PublicIPAddress:  types.StringValue(lb.PublicIPAddress),
			PrivateIPAddress: types.StringValue(lb.PrivateIPAddress),
			CreatedAt:        types.StringValue(lb.CreatedAt),
			UpdatedAt:        types.StringValue(lb.UpdatedAt),
		})
	}

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
