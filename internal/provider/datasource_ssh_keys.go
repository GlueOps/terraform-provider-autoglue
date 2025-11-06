package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SshDataSource{}
var _ datasource.DataSourceWithConfigure = &SshDataSource{}

type SshDataSource struct{ client *Client }

func NewSshDataSource() datasource.DataSource { return &SshDataSource{} }

type sshDSModel struct {
	NameContains types.String `tfsdk:"name_contains"`
	Fingerprint  types.String `tfsdk:"fingerprint"`
	Keys         []sshItem    `tfsdk:"keys"`
}

type sshItem struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	PublicKey   types.String `tfsdk:"public_key"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (d *SshDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_keys"
}

func (d *SshDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Attributes: map[string]dschema.Attribute{
			"name_contains": dschema.StringAttribute{
				Optional:    true,
				Description: "Filter by substring of name (client-side).",
			},
			"fingerprint": dschema.StringAttribute{
				Optional:    true,
				Description: "Filter by exact fingerprint (client-side).",
			},
			"keys": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "SSH keys",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":          dschema.StringAttribute{Computed: true},
						"name":        dschema.StringAttribute{Computed: true},
						"public_key":  dschema.StringAttribute{Computed: true},
						"fingerprint": dschema.StringAttribute{Computed: true},
						"created_at":  dschema.StringAttribute{Computed: true},
						"updated_at":  dschema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *SshDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*Client)
}

func (d *SshDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil || d.client.SDK == nil {
		resp.Diagnostics.AddError("Client not configured", "Provider configuration missing")
		return
	}

	var conf sshDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &conf)...)
	if resp.Diagnostics.HasError() {
		return
	}

	items, httpResp, err := d.client.SDK.SshAPI.ListPublicSshKeys(ctx).Execute()
	if err != nil {
		resp.Diagnostics.AddError("List ssh keys failed", fmt.Sprintf("%v", httpErr(err, httpResp)))
		return
	}

	nc := strings.ToLower(conf.NameContains.ValueString())
	fp := conf.Fingerprint.ValueString()
	out := sshDSModel{NameContains: conf.NameContains, Fingerprint: conf.Fingerprint}
	out.Keys = make([]sshItem, 0, len(items))

	for _, s := range items {
		name := ""
		if s.Name != nil {
			name = *s.Name
		}
		if nc != "" && !strings.Contains(strings.ToLower(name), nc) {
			continue
		}
		if fp != "" && (s.Fingerprint == nil || *s.Fingerprint != fp) {
			continue
		}

		out.Keys = append(out.Keys, sshItem{
			ID:          types.StringPointerValue(s.Id),
			Name:        types.StringPointerValue(s.Name),
			PublicKey:   types.StringPointerValue(s.PublicKey),
			Fingerprint: types.StringPointerValue(s.Fingerprint),
			CreatedAt:   types.StringPointerValue(s.CreatedAt),
			UpdatedAt:   types.StringPointerValue(s.UpdatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
