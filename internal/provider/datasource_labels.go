package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &LabelsDataSource{}
var _ datasource.DataSourceWithConfigure = &LabelsDataSource{}

type LabelsDataSource struct{ client *Client }

func NewLabelsDataSource() datasource.DataSource { return &LabelsDataSource{} }

type labelsDSModel struct {
	Items []labelItem `tfsdk:"items"`
}

type labelItem struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	Raw            types.String `tfsdk:"raw"`
}

func (d *LabelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_labels"
}

func (d *LabelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "List labels for the organization (org-scoped).",
		Attributes: map[string]dschema.Attribute{
			"items": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Labels returned by the API.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":              dschema.StringAttribute{Computed: true, Description: "Taint ID (UUID)."},
						"organization_id": dschema.StringAttribute{Computed: true},
						"key":             dschema.StringAttribute{Computed: true},
						"value":           dschema.StringAttribute{Computed: true},
						"created_at":      dschema.StringAttribute{Computed: true, Description: "RFC3339, UTC."},
						"updated_at":      dschema.StringAttribute{Computed: true, Description: "RFC3339, UTC."},
						"raw":             dschema.StringAttribute{Computed: true, Description: "Full JSON for the item."},
					},
				},
			},
		},
	}
}

func (d *LabelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*Client)
}

func (d *LabelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil || d.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var conf labelsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &conf)...)
	if resp.Diagnostics.HasError() {
		return
	}

	call := d.client.SDK.LabelsAPI.ListLabels(ctx)
	items, httpResp, err := call.Execute()
	if err != nil {
		resp.Diagnostics.AddError("List labels failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	out := labelsDSModel{
		Items: make([]labelItem, 0, len(items)),
	}

	for _, item := range items {
		raw, _ := json.Marshal(item)
		out.Items = append(out.Items, labelItem{
			ID:             types.StringPointerValue(item.Id),
			OrganizationID: types.StringPointerValue(item.OrganizationId),
			Key:            types.StringPointerValue(item.Key),
			Value:          types.StringPointerValue(item.Value),
			CreatedAt:      types.StringPointerValue(item.CreatedAt),
			UpdatedAt:      types.StringPointerValue(item.UpdatedAt),
			Raw:            types.StringValue(string(raw)),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
