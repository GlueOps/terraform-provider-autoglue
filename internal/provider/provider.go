package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ provider.Provider              = &autoglueProvider{}
	_ provider.ProviderWithFunctions = &autoglueProvider{}
)

type autoglueProvider struct {
	version string
}

type autoglueProviderModel struct {
	BaseURL     types.String `tfsdk:"base_url"`
	OrgID       types.String `tfsdk:"org_id"`
	APIKey      types.String `tfsdk:"api_key"`
	OrgKey      types.String `tfsdk:"org_key"`
	OrgSecret   types.String `tfsdk:"org_secret"`
	BearerToken types.String `tfsdk:"bearer_token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &autoglueProvider{version: version}
	}
}

func (p *autoglueProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "autoglue"
	resp.Version = p.version
}

func (p *autoglueProvider) Schema(
	_ context.Context,
	_ provider.SchemaRequest,
	resp *provider.SchemaResponse,
) {
	resp.Schema = providerschema.Schema{
		Attributes: map[string]providerschema.Attribute{
			"base_url": providerschema.StringAttribute{
				Optional:    true,
				Description: "Base URL for the Autoglue API (default: https://autoglue.glueopshosted.com/api/v1).",
			},
			"org_id": providerschema.StringAttribute{
				Required:    true,
				Description: "Organization UUID used for X-Org-ID header.",
			},
			"api_key": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "User API key (X-API-KEY).",
			},
			"org_key": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Org key (X-ORG-KEY). Use together with org_secret for org-scoped auth.",
			},
			"org_secret": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Org secret (X-ORG-SECRET). Use together with org_key for org-scoped auth.",
			},
			"bearer_token": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer token for Authorization header.",
			},
		},
	}
}

func (p *autoglueProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Autoglue provider")

	var config autoglueProviderModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := stringOrEnv(config.BaseURL, "AUTOGLUE_BASE_URL")
	orgID := stringOrEnv(config.OrgID, "AUTOGLUE_ORG_ID")
	apiKey := stringOrEnv(config.APIKey, "AUTOGLUE_API_KEY")
	orgKey := stringOrEnv(config.OrgKey, "AUTOGLUE_ORG_KEY")
	orgSecret := stringOrEnv(config.OrgSecret, "AUTOGLUE_ORG_SECRET")
	bearerToken := stringOrEnv(config.BearerToken, "AUTOGLUE_BEARER_TOKEN")

	client, err := newAutoglueClient(clientConfig{
		BaseURL:     baseURL,
		OrgID:       orgID,
		APIKey:      apiKey,
		OrgKey:      orgKey,
		OrgSecret:   orgSecret,
		BearerToken: bearerToken,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Autoglue client", err.Error())
		return
	}

	tflog.Info(ctx, "Autoglue client configured", map[string]any{
		"base_url": client.baseURL,
	})

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *autoglueProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSSHKeyResource,
		NewCredentialResource,
		NewServerResource,
		NewTaintResource,
		NewLabelResource,
		NewAnnotationResource,
		NewNodePoolResource,
		NewNodePoolServersResource,
		NewNodePoolTaintsResource,
		NewNodePoolLabelsResource,
		NewNodePoolAnnotationsResource,
		NewDomainResource,
		NewRecordSetResource,
		NewClusterResource,
		NewClusterCaptainDomainResource,
		NewClusterControlPlaneRecordSetResource,
		NewClusterAppsLoadBalancerResource,
		NewClusterGlueOpsLoadBalancerResource,
		NewClusterBastionResource,
		NewClusterNodePoolsResource,
		NewClusterKubeconfigResource,
	}
}

func (p *autoglueProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSSHKeysDataSource,
		NewSSHKeyDownloadDataSource,
		NewCredentialDataSource,
		NewServersDataSource,
		NewTaintsDataSource,
		NewLabelsDataSource,
		NewAnnotationsDataSource,
		NewNodePoolDataSource,
		NewDomainsDataSource,
		NewRecordSetsDataSource,
		NewClustersDataSource,
	}

}

func (p *autoglueProvider) Functions(_ context.Context) []func() function.Function {
	// No custom functions (yet).
	return nil
}

func stringOrEnv(v types.String, envName string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	if val := os.Getenv(envName); val != "" {
		return val
	}
	return ""
}
