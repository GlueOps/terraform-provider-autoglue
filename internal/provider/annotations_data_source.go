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
	_ datasource.DataSource              = &annotationsDataSource{}
	_ datasource.DataSourceWithConfigure = &annotationsDataSource{}
)

type annotationsDataSource struct {
	client *autoglueClient
}

type annotationsDataSourceModel struct {
	Annotations []annotationDataModel `tfsdk:"annotations"`
}

type annotationDataModel struct {
	ID             types.String `tfsdk:"id"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func NewAnnotationsDataSource() datasource.DataSource {
	return &annotationsDataSource{}
}

func (d *annotationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotations"
}

func (d *annotationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Lists all annotations in the organization.",
		Attributes: map[string]dsschema.Attribute{
			"annotations": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "All annotations visible to the organization.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.StringAttribute{
							Computed:    true,
							Description: "Annotation ID.",
						},
						"key": dsschema.StringAttribute{
							Computed:    true,
							Description: "Annotation key.",
						},
						"value": dsschema.StringAttribute{
							Computed:    true,
							Description: "Annotation value.",
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

func (d *annotationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *annotationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state annotationsDataSourceModel

	tflog.Info(ctx, "Listing Autoglue annotations")

	var apiResp []annotation
	if err := d.client.doJSON(ctx, http.MethodGet, "/annotations", "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error listing annotations", err.Error())
		return
	}

	state.Annotations = make([]annotationDataModel, 0, len(apiResp))
	for _, a := range apiResp {
		state.Annotations = append(state.Annotations, annotationDataModel{
			ID:             types.StringValue(a.ID),
			Key:            types.StringValue(a.Key),
			Value:          types.StringValue(a.Value),
			OrganizationID: types.StringValue(a.OrganizationID),
			CreatedAt:      types.StringValue(a.CreatedAt),
			UpdatedAt:      types.StringValue(a.UpdatedAt),
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}
