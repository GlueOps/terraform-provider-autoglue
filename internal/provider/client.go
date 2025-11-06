package provider

import (
	"context"
	"net/http"

	"github.com/glueops/autoglue-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type Client struct {
	SDK *autoglue.APIClient
}

func NewClient(_ context.Context, cfg providerModel) (*Client, diag.Diagnostics) {
	var diags diag.Diagnostics

	conf := autoglue.NewConfiguration()
	conf.Servers = autoglue.ServerConfigurations{{URL: cfg.Addr.ValueString()}}

	// Attach auth headers for *every* request
	rt := http.DefaultTransport
	conf.HTTPClient = &http.Client{
		Transport: headerRoundTripper{
			under:     rt,
			bearer:    strOrEmpty(cfg.Bearer),
			apiKey:    strOrEmpty(cfg.APIKey),
			orgKey:    strOrEmpty(cfg.OrgKey),
			orgSecret: strOrEmpty(cfg.OrgSecret),
			orgID:     strOrEmpty(cfg.OrgID),
		},
	}

	return &Client{SDK: autoglue.NewAPIClient(conf)}, diags
}

type headerRoundTripper struct {
	under     http.RoundTripper
	bearer    string
	apiKey    string
	orgKey    string
	orgSecret string
	orgID     string
}

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Bearer -> Authorization
	if h.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+h.bearer)
	}
	// User API Key
	if h.apiKey != "" {
		req.Header.Set("X-API-KEY", h.apiKey)
	}
	// Org key/secret
	if h.orgKey != "" {
		req.Header.Set("X-ORG-KEY", h.orgKey)
	}
	if h.orgSecret != "" {
		req.Header.Set("X-ORG-SECRET", h.orgSecret)
	}
	// Org selection header (user or key where needed)
	if h.orgID != "" {
		req.Header.Set("X-Org-ID", h.orgID)
	}
	return h.under.RoundTrip(req)
}

func strOrEmpty(v interface {
	IsNull() bool
	IsUnknown() bool
	ValueString() string
}) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}
