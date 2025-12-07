package provider

import (
	"context"
	"encoding/json"
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
	_ datasource.DataSource              = &recordSetsDataSource{}
	_ datasource.DataSourceWithConfigure = &recordSetsDataSource{}
)

type recordSetsDataSource struct {
	client *autoglueClient
}

type recordSetsDataSourceModel struct {
	DomainID types.String `tfsdk:"domain_id"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Status   types.String `tfsdk:"status"`

	Records []recordSetListItemModel `tfsdk:"records"`
}

type recordSetListItemModel struct {
	ID          types.String `tfsdk:"id"`
	DomainID    types.String `tfsdk:"domain_id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	TTL         types.Int64  `tfsdk:"ttl"`
	Values      types.List   `tfsdk:"values"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	Status      types.String `tfsdk:"status"`
	LastError   types.String `tfsdk:"last_error"`
	Owner       types.String `tfsdk:"owner"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func NewRecordSetsDataSource() datasource.DataSource {
	return &recordSetsDataSource{}
}

func (d *recordSetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record_sets"
}

func (d *recordSetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists DNS record sets for an Autoglue domain.",
		Attributes: map[string]dsschema.Attribute{
			"domain_id": dsschema.StringAttribute{
				Required:    true,
				Description: "Domain ID (UUID) whose records to list.",
			},
			"name": dsschema.StringAttribute{
				Optional: true,
				Description: "Optional name filter. May be relative or FQDN; " +
					"the server normalizes to a relative name.",
			},
			"type": dsschema.StringAttribute{
				Optional: true,
				Description: "Optional RR type filter (A, AAAA, CNAME, TXT, MX, NS, SRV, CAA). " +
					"Validation is handled by the API.",
			},
			"status": dsschema.StringAttribute{
				Optional:    true,
				Description: "Optional status filter (pending, provisioning, ready, failed).",
			},

			"records": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching record sets.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Record set ID.",
						},
						"domain_id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Owning domain ID.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Record name (relative to domain).",
						},
						"type": dsschema.StringAttribute{
							Computed:    true,
							Description: "Record type.",
						},
						"ttl": dsschema.Int64Attribute{
							Computed:    true,
							Description: "TTL in seconds.",
						},
						"values": dsschema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Record values.",
						},
						"fingerprint": dsschema.StringAttribute{
							Computed:    true,
							Description: "Fingerprint of desired state.",
						},
						"status": dsschema.StringAttribute{
							Computed:    true,
							Description: "Provisioning status.",
						},
						"last_error": dsschema.StringAttribute{
							Computed:    true,
							Description: "Last provisioning error, if any.",
						},
						"owner": dsschema.StringAttribute{
							Computed:    true,
							Description: "Owner marker.",
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

func (d *recordSetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *recordSetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config recordSetsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := config.DomainID.ValueString()
	if domainID == "" {
		resp.Diagnostics.AddError("Missing domain_id", "domain_id must be set.")
		return
	}

	q := url.Values{}
	if !config.Name.IsNull() && !config.Name.IsUnknown() {
		q.Set("name", strings.TrimSpace(config.Name.ValueString()))
	}
	if !config.Type.IsNull() && !config.Type.IsUnknown() {
		q.Set("type", strings.TrimSpace(strings.ToUpper(config.Type.ValueString())))
	}
	if !config.Status.IsNull() && !config.Status.IsUnknown() {
		q.Set("status", strings.TrimSpace(strings.ToLower(config.Status.ValueString())))
	}

	query := q.Encode()
	path := fmt.Sprintf("/dns/domains/%s/records", domainID)

	tflog.Info(ctx, "Listing Autoglue record sets", map[string]any{
		"domain_id": domainID,
		"query":     query,
	})

	var apiResp []recordSet
	if err := d.client.doJSON(ctx, http.MethodGet, path, query, nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing record sets", err.Error())
		return
	}

	config.Records = make([]recordSetListItemModel, 0, len(apiResp))
	for _, rs := range apiResp {
		item := recordSetListItemModel{
			ID:          types.StringValue(rs.ID),
			DomainID:    types.StringValue(rs.DomainID),
			Name:        types.StringValue(rs.Name),
			Type:        types.StringValue(rs.Type),
			Fingerprint: types.StringValue(rs.Fingerprint),
			Status:      types.StringValue(rs.Status),
			LastError:   types.StringValue(rs.LastError),
			Owner:       types.StringValue(rs.Owner),
			CreatedAt:   types.StringValue(rs.CreatedAt),
			UpdatedAt:   types.StringValue(rs.UpdatedAt),
		}

		if rs.TTL != nil {
			item.TTL = types.Int64Value(int64(*rs.TTL))
		} else {
			item.TTL = types.Int64Null()
		}

		var vals []string
		if len(rs.Values) > 0 && string(rs.Values) != "null" {
			if err := json.Unmarshal(rs.Values, &vals); err != nil {
				resp.Diagnostics.AddError("Error decoding record set values", err.Error())
				return
			}
		}
		listVal, d2 := types.ListValueFrom(ctx, types.StringType, vals)
		resp.Diagnostics.Append(d2...)
		if resp.Diagnostics.HasError() {
			return
		}
		item.Values = listVal

		config.Records = append(config.Records, item)
	}

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
