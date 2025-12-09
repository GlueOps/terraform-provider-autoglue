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
	_ datasource.DataSource              = &clustersDataSource{}
	_ datasource.DataSourceWithConfigure = &clustersDataSource{}
)

type clustersDataSource struct {
	client *autoglueClient
}

type clustersDataSourceModel struct {
	Search   types.String            `tfsdk:"search"`
	Clusters []clusterDataSourceItem `tfsdk:"clusters"`
}

type clusterDataSourceItem struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`

	ClusterProvider types.String `tfsdk:"cluster_provider"`
	Region          types.String `tfsdk:"region"`

	Status    types.String `tfsdk:"status"`
	LastError types.String `tfsdk:"last_error"`

	RandomToken    types.String `tfsdk:"random_token"`
	CertificateKey types.String `tfsdk:"certificate_key"`

	CaptainDomainID         types.String `tfsdk:"captain_domain_id"`
	ControlPlaneRecordSetID types.String `tfsdk:"control_plane_record_set_id"`
	ControlPlaneFQDN        types.String `tfsdk:"control_plane_fqdn"`
	AppsLoadBalancerID      types.String `tfsdk:"apps_load_balancer_id"`
	GlueOpsLoadBalancerID   types.String `tfsdk:"glueops_load_balancer_id"`
	BastionServerID         types.String `tfsdk:"bastion_server_id"`
	DockerImage             types.String `tfsdk:"docker_image"`
	DockerTag               types.String `tfsdk:"docker_tag"`

	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewClustersDataSource() datasource.DataSource {
	return &clustersDataSource{}
}

func (d *clustersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clusters"
}

func (d *clustersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists clusters for the current organization.",
		Attributes: map[string]dsschema.Attribute{
			"search": dsschema.StringAttribute{
				Optional:    true,
				Description: "Optional substring filter over cluster name (maps to `q`).",
			},

			"clusters": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching clusters.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Cluster ID.",
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
						"last_error": dsschema.StringAttribute{
							Computed:    true,
							Description: "Last error message.",
						},
						"random_token": dsschema.StringAttribute{
							Computed:    true,
							Sensitive:   true,
							Description: "Random token for the cluster.",
						},
						"certificate_key": dsschema.StringAttribute{
							Computed:    true,
							Sensitive:   true,
							Description: "Cluster certificate key.",
						},
						"captain_domain_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Attached captain domain ID, if any.",
						},
						"control_plane_record_set_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Attached control plane record set ID, if any.",
						},
						"control_plane_fqdn": dsschema.StringAttribute{
							Computed:    true,
							Description: "Control plane FQDN, if present.",
						},
						"apps_load_balancer_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Attached apps load balancer ID, if any.",
						},
						"glueops_load_balancer_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Attached GlueOps load balancer ID, if any.",
						},
						"bastion_server_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Attached bastion server ID, if any.",
						},
						"docker_image": dsschema.StringAttribute{
							Computed:    true,
							Description: "Docker image.",
						},
						"docker_tag": dsschema.StringAttribute{
							Computed:    true,
							Description: "Docker tag.",
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

func (d *clustersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *clustersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config clustersDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	values := url.Values{}
	if !config.Search.IsNull() && !config.Search.IsUnknown() {
		values.Set("q", strings.TrimSpace(config.Search.ValueString()))
	}
	query := values.Encode()

	tflog.Info(ctx, "Listing Autoglue clusters", map[string]any{
		"query": query,
	})

	var apiResp []cluster
	if err := d.client.doJSON(ctx, http.MethodGet, "/clusters", query, nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing clusters", err.Error())
		return
	}

	config.Clusters = make([]clusterDataSourceItem, 0, len(apiResp))
	for _, c := range apiResp {
		item := clusterDataSourceItem{
			ID:              types.StringValue(c.ID),
			Name:            types.StringValue(c.Name),
			ClusterProvider: types.StringValue(c.ClusterProvider),
			Region:          types.StringValue(c.Region),
			Status:          types.StringValue(c.Status),
			LastError:       types.StringValue(c.LastError),
			RandomToken:     types.StringValue(c.RandomToken),
			CertificateKey:  types.StringValue(c.CertificateKey),
			DockerImage:     types.StringValue(c.DockerImage),
			DockerTag:       types.StringValue(c.DockerTag),
			CreatedAt:       types.StringValue(c.CreatedAt),
			UpdatedAt:       types.StringValue(c.UpdatedAt),
		}

		if c.CaptainDomain != nil {
			item.CaptainDomainID = types.StringValue(c.CaptainDomain.ID)
		} else {
			item.CaptainDomainID = types.StringNull()
		}
		if c.ControlPlaneRecordSet != nil {
			item.ControlPlaneRecordSetID = types.StringValue(c.ControlPlaneRecordSet.ID)
		} else {
			item.ControlPlaneRecordSetID = types.StringNull()
		}
		if c.ControlPlaneFQDN != nil {
			item.ControlPlaneFQDN = types.StringValue(*c.ControlPlaneFQDN)
		} else {
			item.ControlPlaneFQDN = types.StringNull()
		}
		if c.AppsLoadBalancer != nil {
			item.AppsLoadBalancerID = types.StringValue(c.AppsLoadBalancer.ID)
		} else {
			item.AppsLoadBalancerID = types.StringNull()
		}
		if c.GlueOpsLoadBalancer != nil {
			item.GlueOpsLoadBalancerID = types.StringValue(c.GlueOpsLoadBalancer.ID)
		} else {
			item.GlueOpsLoadBalancerID = types.StringNull()
		}
		if c.BastionServer != nil {
			item.BastionServerID = types.StringValue(c.BastionServer.ID)
		} else {
			item.BastionServerID = types.StringNull()
		}

		config.Clusters = append(config.Clusters, item)
	}

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
