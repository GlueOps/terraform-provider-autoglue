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
	_ datasource.DataSource              = &credentialDataSource{}
	_ datasource.DataSourceWithConfigure = &credentialDataSource{}
)

type credentialDataSource struct {
	client *autoglueClient
}

type credentialDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	CredentialProvider types.String `tfsdk:"credential_provider"`
	Kind               types.String `tfsdk:"kind"`
	SchemaVersion      types.Int64  `tfsdk:"schema_version"`

	Scope        types.Map    `tfsdk:"scope"`
	ScopeKind    types.String `tfsdk:"scope_kind"`
	ScopeVersion types.Int64  `tfsdk:"scope_version"`
	AccountID    types.String `tfsdk:"account_id"`
	Region       types.String `tfsdk:"region"`

	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewCredentialDataSource() datasource.DataSource {
	return &credentialDataSource{}
}

func (d *credentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (d *credentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Reads an Autoglue credential by ID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "Credential ID to look up.",
			},
			"name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Credential name.",
			},
			"credential_provider": dsschema.StringAttribute{
				Computed:    true,
				Description: "Credential provider.",
			},
			"kind": dsschema.StringAttribute{
				Computed:    true,
				Description: "Credential kind.",
			},
			"schema_version": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Schema version.",
			},
			"scope": dsschema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Scope metadata associated with the credential.",
			},
			"scope_kind": dsschema.StringAttribute{
				Computed:    true,
				Description: "Logical scope kind.",
			},
			"scope_version": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Scope version.",
			},
			"account_id": dsschema.StringAttribute{
				Computed:    true,
				Description: "Cloud account ID.",
			},
			"region": dsschema.StringAttribute{
				Computed:    true,
				Description: "Cloud region.",
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
	}
}

func (d *credentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *credentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var config credentialDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "id must be set to read a credential.")
		return
	}

	path := fmt.Sprintf("/credentials/%s", id)

	tflog.Info(ctx, "Reading Autoglue credential data source", map[string]any{"id": id})

	var apiResp credential
	if err := d.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error reading credential", err.Error())
		return
	}

	config.Name = types.StringValue(apiResp.Name)
	config.CredentialProvider = types.StringValue(apiResp.CredentialProvider)
	config.Kind = types.StringValue(apiResp.Kind)
	config.SchemaVersion = types.Int64Value(int64(apiResp.SchemaVersion))
	config.AccountID = types.StringValue(apiResp.AccountID)
	config.Region = types.StringValue(apiResp.Region)
	config.CreatedAt = types.StringValue(apiResp.CreatedAt)
	config.UpdatedAt = types.StringValue(apiResp.UpdatedAt)

	scopeVal, d2 := types.MapValueFrom(ctx, types.StringType, apiResp.Scope)
	resp.Diagnostics.Append(d2...)
	config.Scope = scopeVal
	config.ScopeKind = types.StringValue(apiResp.ScopeKind)
	config.ScopeVersion = types.Int64Value(int64(apiResp.ScopeVersion))

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
}
