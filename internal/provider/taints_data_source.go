package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ datasource.DataSource              = &taintsDataSource{}
	_ datasource.DataSourceWithConfigure = &taintsDataSource{}
)

type taintsDataSource struct {
	client *autoglueClient
}

type taintsDataSourceModel struct {
	Key    types.String            `tfsdk:"key"`
	Value  types.String            `tfsdk:"value"`
	Q      types.String            `tfsdk:"q"`
	Taints []taintsDataSourceTaint `tfsdk:"taints"`
}

type taintsDataSourceTaint struct {
	ID        types.String `tfsdk:"id"`
	Key       types.String `tfsdk:"key"`
	Value     types.String `tfsdk:"value"`
	Effect    types.String `tfsdk:"effect"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewTaintsDataSource() datasource.DataSource {
	return &taintsDataSource{}
}

func (d *taintsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_taints"
}

func (d *taintsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists taints, optionally filtered by key, value, or query string.",
		Attributes: map[string]dsschema.Attribute{
			"key": dsschema.StringAttribute{
				Optional:    true,
				Description: "Exact taint key to filter by.",
			},
			"value": dsschema.StringAttribute{
				Optional:    true,
				Description: "Exact taint value to filter by.",
			},
			"q": dsschema.StringAttribute{
				Optional:    true,
				Description: "Case-insensitive substring match on key.",
			},
			"taints": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "All taints returned by the API.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Taint ID.",
						},
						"key": dsschema.StringAttribute{
							Computed:    true,
							Description: "Taint key.",
						},
						"value": dsschema.StringAttribute{
							Computed:    true,
							Description: "Taint value (may be empty).",
						},
						"effect": dsschema.StringAttribute{
							Computed:    true,
							Description: "Taint effect.",
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

func (d *taintsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *taintsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config taintsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query params
	u := "/taints"
	q := url.Values{}

	if !config.Key.IsNull() && !config.Key.IsUnknown() && config.Key.ValueString() != "" {
		q.Set("key", config.Key.ValueString())
	}

	if !config.Value.IsNull() && !config.Value.IsUnknown() && config.Value.ValueString() != "" {
		q.Set("value", config.Value.ValueString())
	}

	if !config.Q.IsNull() && !config.Q.IsUnknown() && config.Q.ValueString() != "" {
		q.Set("q", config.Q.ValueString())
	}

	if encoded := q.Encode(); encoded != "" {
		u = u + "?" + encoded
	}

	tflog.Info(ctx, "Listing Autoglue taints", map[string]any{
		"path":  u,
		"key":   config.Key.ValueString(),
		"value": config.Value.ValueString(),
		"q":     config.Q.ValueString(),
	})

	var apiResp []taint
	if err := d.client.doJSON(ctx, http.MethodGet, u, "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing taints", err.Error())
		return
	}

	result := taintsDataSourceModel{
		Key:   config.Key,
		Value: config.Value,
		Q:     config.Q,
	}

	for _, t := range apiResp {
		item := taintsDataSourceTaint{
			ID:        types.StringValue(t.ID),
			Key:       types.StringValue(t.Key),
			Effect:    types.StringValue(t.Effect),
			CreatedAt: types.StringValue(t.CreatedAt),
			UpdatedAt: types.StringValue(t.UpdatedAt),
		}
		if t.Value != nil {
			item.Value = types.StringValue(*t.Value)
		} else {
			item.Value = types.StringNull()
		}

		result.Taints = append(result.Taints, item)
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}
