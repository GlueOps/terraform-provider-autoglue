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
	_ datasource.DataSource              = &labelsDataSource{}
	_ datasource.DataSourceWithConfigure = &labelsDataSource{}
)

type labelsDataSource struct {
	client *autoglueClient
}

type labelsDataSourceModel struct {
	Labels []labelDataModel `tfsdk:"labels"`
}

type labelDataModel struct {
	ID             types.String `tfsdk:"id"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewLabelsDataSource() datasource.DataSource {
	return &labelsDataSource{}
}

func (d *labelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_labels"
}

func (d *labelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists all labels in the organization.",
		Attributes: map[string]dsschema.Attribute{
			"labels": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "All labels visible to the organization.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Label ID.",
						},
						"key": dsschema.StringAttribute{
							Computed:    true,
							Description: "Label key.",
						},
						"value": dsschema.StringAttribute{
							Computed:    true,
							Description: "Label value.",
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

func (d *labelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *labelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state labelsDataSourceModel

	tflog.Info(ctx, "Listing Autoglue labels")

	var apiResp []label
	if err := d.client.doJSON(ctx, http.MethodGet, "/labels", "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing labels", err.Error())
		return
	}

	state.Labels = make([]labelDataModel, 0, len(apiResp))
	for _, l := range apiResp {
		state.Labels = append(state.Labels, labelDataModel{
			ID:             types.StringValue(l.ID),
			Key:            types.StringValue(l.Key),
			Value:          types.StringValue(l.Value),
			OrganizationID: types.StringValue(l.OrganizationID),
			CreatedAt:      types.StringValue(l.CreatedAt),
			UpdatedAt:      types.StringValue(l.UpdatedAt),
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
