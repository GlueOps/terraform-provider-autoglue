package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TaintsDataSource{}
var _ datasource.DataSourceWithConfigure = &TaintsDataSource{}

type TaintsDataSource struct{ client *Client }

func NewTaintsDataSource() datasource.DataSource { return &TaintsDataSource{} }

type taintsDSModel struct {
	Items []taintItem `tfsdk:"items"`
}

type taintItem struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	Key            types.String `tfsdk:"key"`
	Value          types.String `tfsdk:"value"`
	Effect         types.String `tfsdk:"effect"`
	Raw            types.String `tfsdk:"raw"`
}

func (d *TaintsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_taints"
}

func (d *TaintsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "List taints for the organization (org-scoped).",
		Attributes: map[string]dschema.Attribute{
			"items": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Taints returned by the API.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":              dschema.StringAttribute{Computed: true, Description: "Taint ID (UUID)."},
						"organization_id": dschema.StringAttribute{Computed: true},
						"key":             dschema.StringAttribute{Computed: true},
						"value":           dschema.StringAttribute{Computed: true},
						"effect":          dschema.StringAttribute{Computed: true},
						"created_at":      dschema.StringAttribute{Computed: true, Description: "RFC3339, UTC."},
						"updated_at":      dschema.StringAttribute{Computed: true, Description: "RFC3339, UTC."},
						"raw":             dschema.StringAttribute{Computed: true, Description: "Full JSON for the item."},
					},
				},
			},
		},
	}
}

func (d *TaintsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*Client)
}

func (d *TaintsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil || d.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var conf taintsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &conf)...)
	if resp.Diagnostics.HasError() {
		return
	}

	call := d.client.SDK.TaintsAPI.ListTaints(ctx)
	items, httpResp, err := call.Execute()
	if err != nil {
		resp.Diagnostics.AddError("List taints failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	out := taintsDSModel{
		Items: make([]taintItem, 0, len(items)),
	}

	for _, item := range items {
		raw, _ := json.Marshal(item)
		out.Items = append(out.Items, taintItem{
			ID:             types.StringPointerValue(item.Id),
			OrganizationID: types.StringPointerValue(item.OrganizationId),
			Key:            types.StringPointerValue(item.Key),
			Value:          types.StringPointerValue(item.Value),
			Effect:         types.StringPointerValue(item.Effect),
			CreatedAt:      types.StringPointerValue(item.CreatedAt),
			UpdatedAt:      types.StringPointerValue(item.UpdatedAt),
			Raw:            types.StringValue(string(raw)),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
