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
	_ datasource.DataSource              = &sshKeyDownloadDataSource{}
	_ datasource.DataSourceWithConfigure = &sshKeyDownloadDataSource{}
)

type sshKeyDownloadDataSource struct {
	client *autoglueClient
}

type sshKeyDownloadModel struct {
	ID      types.String `tfsdk:"id"`
	Part    types.String `tfsdk:"part"`
	Content types.String `tfsdk:"content"`
}

func NewSSHKeyDownloadDataSource() datasource.DataSource {
	return &sshKeyDownloadDataSource{}
}

func (d *sshKeyDownloadDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key_download"
}

func (d *sshKeyDownloadDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Downloads SSH key files by ID. WARNING: this can place private key material into Terraform state.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "SSH key ID to download.",
			},
			"part": dsschema.StringAttribute{
				Optional: true,
				Description: "Optional part selector if supported by the API (for example: " +
					"`all`, `public`, or `private`). If omitted, the server default is used.",
			},
			"content": dsschema.StringAttribute{
				Computed:    true,
				Description: "File content returned by the server (format depends on API; often a ZIP or PEM).",
			},
		},
	}
}

func (d *sshKeyDownloadDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sshKeyDownloadDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config sshKeyDownloadModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "id must be set to download an SSH key.")
		return
	}

	var query string
	if !config.Part.IsNull() && !config.Part.IsUnknown() && config.Part.ValueString() != "" {
		q := url.Values{}
		q.Set("part", config.Part.ValueString())
		query = q.Encode()
	}

	path := fmt.Sprintf("/ssh/%s/download", id)

	tflog.Info(ctx, "Downloading Autoglue SSH key", map[string]any{
		"id":   id,
		"part": config.Part.ValueString(),
	})

	var content string
	if err := d.client.doJSON(ctx, http.MethodGet, path, query, nil, &content); err != nil {
		resp.Diagnostics.AddError("Error downloading SSH key", err.Error())
		return
	}

	config.Content = types.StringValue(content)

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
