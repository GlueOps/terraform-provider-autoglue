package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	pschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type providerModel struct {
	Addr      types.String `tfsdk:"addr"`
	Bearer    types.String `tfsdk:"bearer"`
	APIKey    types.String `tfsdk:"api_key"`
	OrgKey    types.String `tfsdk:"org_key"`
	OrgSecret types.String `tfsdk:"org_secret"`
	OrgID     types.String `tfsdk:"org_id"`
}

func providerConfigSchema() map[string]pschema.Attribute {
	return map[string]pschema.Attribute{
		"addr": pschema.StringAttribute{
			Optional:    true,
			Description: "Base URL to the autoglue API (e.g. https://autoglue.example.com/api/v1). Defaults to http://localhost:8080/api/v1.",
		},
		"bearer": pschema.StringAttribute{
			Optional:    true,
			Sensitive:   true,
			Description: "Bearer token (user access token).",
		},
		"api_key": pschema.StringAttribute{
			Optional:    true,
			Sensitive:   true,
			Description: "User API key for key-only auth.",
		},
		"org_key": pschema.StringAttribute{
			Optional:    true,
			Sensitive:   true,
			Description: "Org-scoped key for machine auth.",
		},
		"org_secret": pschema.StringAttribute{
			Optional:    true,
			Sensitive:   true,
			Description: "Org-scoped secret for machine auth.",
		},
		"org_id": pschema.StringAttribute{
			Optional:    true,
			Description: "Organization ID (UUID). Required for user/bearer and user API key auth unless single-org membership. Omitted for org key/secret (derived server-side).",
			Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
		},
	}
}

func readConfig(ctx context.Context, req provider.ConfigureRequest) (providerModel, diag.Diagnostics) {
	var cfg providerModel
	var diags diag.Diagnostics

	req.Config.Get(ctx, &cfg)

	if cfg.Addr.IsNull() || cfg.Addr.IsUnknown() {
		if v := os.Getenv("AUTOGLUE_ADDR"); v != "" {
			cfg.Addr = types.StringValue(v)
		} else {
			cfg.Addr = types.StringValue("http://localhost:8080/api/v1")
		}
	}
	if cfg.Bearer.IsNull() || cfg.Bearer.IsUnknown() {
		if v := os.Getenv("AUTOGLUE_TOKEN"); v != "" {
			cfg.Bearer = types.StringValue(v)
		}
	}
	if cfg.APIKey.IsNull() || cfg.APIKey.IsUnknown() {
		if v := os.Getenv("AUTOGLUE_API_KEY"); v != "" {
			cfg.APIKey = types.StringValue(v)
		}
	}
	if cfg.OrgKey.IsNull() || cfg.OrgKey.IsUnknown() {
		if v := os.Getenv("AUTOGLUE_ORG_KEY"); v != "" {
			cfg.OrgKey = types.StringValue(v)
		}
	}
	if cfg.OrgSecret.IsNull() || cfg.OrgSecret.IsUnknown() {
		if v := os.Getenv("AUTOGLUE_ORG_SECRET"); v != "" {
			cfg.OrgSecret = types.StringValue(v)
		}
	}
	if cfg.OrgID.IsNull() || cfg.OrgID.IsUnknown() {
		if v := os.Getenv("AUTOGLUE_ORG_ID"); v != "" {
			cfg.OrgID = types.StringValue(v)
		} else {
			cfg.OrgID = types.StringNull()
		}
	}
	return cfg, diags
}
