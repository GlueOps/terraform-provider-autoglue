package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ provider.Provider = &AutoglueProvider{}

func New() provider.Provider { return &AutoglueProvider{} }

type AutoglueProvider struct {
	version string
}

func (p *AutoglueProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "autoglue"
	resp.Version = p.version
}

func (p *AutoglueProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = pschema.Schema{
		Attributes: providerConfigSchema(),
	}
}

func (p *AutoglueProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	cfg, diags := readConfig(ctx, req)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := NewClient(ctx, cfg)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *AutoglueProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSshDataSource,
		NewServersDataSource,
		NewTaintsDataSource,
		NewLabelsDataSource,
		NewAnnotationsDataSource,
	}
}

func (p *AutoglueProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSshResource,
		NewServerResource,
		NewTaintResource,
		NewLabelResource,
		NewAnnotationResource,
	}
}
