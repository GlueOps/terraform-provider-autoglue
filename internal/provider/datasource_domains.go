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
	_ datasource.DataSource              = &domainsDataSource{}
	_ datasource.DataSourceWithConfigure = &domainsDataSource{}
)

type domainsDataSource struct {
	client *autoglueClient
}

type domainsDataSourceModel struct {
	DomainName types.String `tfsdk:"domain_name"`
	Status     types.String `tfsdk:"status"`
	Search     types.String `tfsdk:"search"`

	Domains []domainListItemModel `tfsdk:"domains"`
}

type domainListItemModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	DomainName     types.String `tfsdk:"domain_name"`
	ZoneID         types.String `tfsdk:"zone_id"`
	Status         types.String `tfsdk:"status"`
	LastError      types.String `tfsdk:"last_error"`
	CredentialID   types.String `tfsdk:"credential_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewDomainsDataSource() datasource.DataSource {
	return &domainsDataSource{}
}

func (d *domainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *domainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists Autoglue DNS domains for the current organization.",
		Attributes: map[string]dsschema.Attribute{
			"domain_name": dsschema.StringAttribute{
				Optional:    true,
				Description: "Optional exact domain name filter (lowercase, no trailing dot).",
			},
			"status": dsschema.StringAttribute{
				Optional:    true,
				Description: "Optional status filter (pending, provisioning, ready, failed).",
			},
			"search": dsschema.StringAttribute{
				Optional: true,
				Description: "Optional substring match filter applied to domain name " +
					"(maps to the `q` query parameter).",
			},

			"domains": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching domains.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Domain ID.",
						},
						"organization_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Owning organization UUID.",
						},
						"domain_name": dsschema.StringAttribute{
							Computed:    true,
							Description: "DNS domain name.",
						},
						"zone_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Route 53 zone ID backing this domain.",
						},
						"status": dsschema.StringAttribute{
							Computed:    true,
							Description: "Provisioning status.",
						},
						"last_error": dsschema.StringAttribute{
							Computed:    true,
							Description: "Last provisioning error, if any.",
						},
						"credential_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Credential ID bound to this domain.",
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

func (d *domainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *domainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config domainsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	q := url.Values{}
	if !config.DomainName.IsNull() && !config.DomainName.IsUnknown() {
		q.Set("domain_name", strings.TrimSpace(strings.ToLower(config.DomainName.ValueString())))
	}
	if !config.Status.IsNull() && !config.Status.IsUnknown() {
		q.Set("status", strings.TrimSpace(strings.ToLower(config.Status.ValueString())))
	}
	if !config.Search.IsNull() && !config.Search.IsUnknown() {
		q.Set("q", strings.TrimSpace(strings.ToLower(config.Search.ValueString())))
	}

	query := q.Encode()
	path := "/dns/domains"

	tflog.Info(ctx, "Listing Autoglue domains", map[string]any{
		"query": query,
	})

	var apiResp []domain
	if err := d.client.doJSON(ctx, http.MethodGet, path, query, nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing domains", err.Error())
		return
	}

	config.Domains = make([]domainListItemModel, 0, len(apiResp))
	for _, dom := range apiResp {
		item := domainListItemModel{
			ID:             types.StringValue(dom.ID),
			OrganizationID: types.StringValue(dom.OrganizationID),
			DomainName:     types.StringValue(dom.DomainName),
			ZoneID:         types.StringValue(dom.ZoneID),
			Status:         types.StringValue(dom.Status),
			LastError:      types.StringValue(dom.LastError),
			CredentialID:   types.StringValue(dom.CredentialID),
			CreatedAt:      types.StringValue(dom.CreatedAt),
			UpdatedAt:      types.StringValue(dom.UpdatedAt),
		}
		config.Domains = append(config.Domains, item)
	}

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
