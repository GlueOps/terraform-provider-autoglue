package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &credentialResource{}
	_ resource.ResourceWithConfigure   = &credentialResource{}
	_ resource.ResourceWithImportState = &credentialResource{}
)

type credentialResource struct {
	client *autoglueClient
}

type credentialResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	CredentialProvider types.String `tfsdk:"credential_provider"`
	Kind               types.String `tfsdk:"kind"`
	SchemaVersion      types.Int64  `tfsdk:"schema_version"`

	Scope        types.Map    `tfsdk:"scope"`
	ScopeKind    types.String `tfsdk:"scope_kind"`
	ScopeVersion types.Int64  `tfsdk:"scope_version"`
	Secret       types.Map    `tfsdk:"secret"`
	AccountID    types.String `tfsdk:"account_id"`
	Region       types.String `tfsdk:"region"`

	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func NewCredentialResource() resource.Resource {
	return &credentialResource{}
}

func (r *credentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (r *credentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages an Autoglue credential (for example, a cloud account credential).",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique credential ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"name": resourceschema.StringAttribute{
				Required:    true,
				Description: "Human-readable credential name.",
			},

			"credential_provider": resourceschema.StringAttribute{
				Required:    true,
				Description: "Provider for this credential (for example: aws, gcp, azure).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"kind": resourceschema.StringAttribute{
				Required:    true,
				Description: "Credential kind (for example: access-key, service-account).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"schema_version": resourceschema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Schema version for this credential's secret format.",
				Default:     int64default.StaticInt64(1),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},

			"scope": resourceschema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Arbitrary scope metadata (key/value tags) associated with this credential.",
			},

			"scope_kind": resourceschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: `Logical scope kind for this credential (for example: "cloud").`,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"scope_version": resourceschema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Version of the scope metadata schema.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},

			"secret": resourceschema.MapAttribute{
				ElementType: types.StringType,
				Required:    true,
				Sensitive:   true,
				Description: "Credential secret payload as a map of key/value pairs. WARNING: values are stored in Terraform state.",
			},

			"account_id": resourceschema.StringAttribute{
				Optional:    true,
				Description: "Optional cloud account ID associated with this credential.",
			},

			"region": resourceschema.StringAttribute{
				Optional:    true,
				Description: "Optional cloud region associated with this credential.",
			},

			"created_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
			},

			"updated_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp.",
			},
		},
	}
}

func (r *credentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

func (r *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan credentialResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	scope, secret, d := credentialMapsFromPlan(ctx, plan.Scope, plan.Secret)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createCredentialPayload{
		Name:               plan.Name.ValueString(),
		CredentialProvider: plan.CredentialProvider.ValueString(),
		Kind:               plan.Kind.ValueString(),
		Scope:              scope,
		ScopeKind:          plan.ScopeKind.ValueString(),
		ScopeVersion:       int32(plan.ScopeVersion.ValueInt64()),
		SchemaVersion:      int32(plan.SchemaVersion.ValueInt64()),
		Secret:             secret,
		AccountID:          optString(plan.AccountID),
		Region:             optString(plan.Region),
	}

	tflog.Info(ctx, "Creating Autoglue credential", map[string]any{
		"name":                payload.Name,
		"credential_provider": payload.CredentialProvider,
		"kind":                payload.Kind,
	})

	var apiResp credential
	if err := r.client.doJSON(ctx, http.MethodPost, "/credentials", "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating credential", err.Error())
		return
	}

	var state credentialResourceModel
	mapCredentialToState(ctx, &state, &apiResp, scope, secret, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state credentialResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Credential ID is required in state to read credential.")
		return
	}

	path := fmt.Sprintf("/credentials/%s", id)

	tflog.Info(ctx, "Reading Autoglue credential", map[string]any{"id": id})

	var apiResp credential
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading credential", fmt.Sprintf("Error reading credential: %s", err.Error()))
		return
	}

	// Preserve the secret from existing state – API never returns it.
	scopeMap, secretMap, d := credentialMapsFromState(ctx, &state)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	mapCredentialToState(ctx, &state, &apiResp, scopeMap, secretMap, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan credentialResourceModel
	var state credentialResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Missing ID", "Credential ID is required in state to update credential.")
		return
	}

	scope, secret, d := credentialMapsFromPlan(ctx, plan.Scope, plan.Secret)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := updateCredentialPayload{
		Name:         plan.Name.ValueString(),
		Scope:        scope,
		ScopeKind:    plan.ScopeKind.ValueString(),
		ScopeVersion: int32(plan.ScopeVersion.ValueInt64()),
		Secret:       secret,
		AccountID:    optString(plan.AccountID),
		Region:       optString(plan.Region),
	}

	path := fmt.Sprintf("/credentials/%s", id)

	tflog.Info(ctx, "Updating Autoglue credential", map[string]any{
		"id":            id,
		"name":          payload.Name,
		"account_id":    payload.AccountID,
		"region":        payload.Region,
		"scope_kind":    payload.ScopeKind,
		"scope_version": payload.ScopeVersion,
	})

	var apiResp credential
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating credential", err.Error())
		return
	}

	mapCredentialToState(ctx, &state, &apiResp, scope, secret, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state credentialResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		return
	}

	path := fmt.Sprintf("/credentials/%s", id)

	tflog.Info(ctx, "Deleting Autoglue credential", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting credential", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_credential.example <credential_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// --- helpers ---

func credentialMapsFromPlan(ctx context.Context, scope types.Map, secret types.Map) (map[string]string, map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	scopeOut := map[string]string{}
	secretOut := map[string]string{}

	if !scope.IsNull() && !scope.IsUnknown() {
		diags.Append(scope.ElementsAs(ctx, &scopeOut, false)...)
	}

	if !secret.IsNull() && !secret.IsUnknown() {
		diags.Append(secret.ElementsAs(ctx, &secretOut, false)...)
	}

	return scopeOut, secretOut, diags
}

func credentialMapsFromState(ctx context.Context, state *credentialResourceModel) (map[string]string, map[string]string, diag.Diagnostics) {
	return credentialMapsFromPlan(ctx, state.Scope, state.Secret)
}

func mapCredentialToState(
	ctx context.Context,
	state *credentialResourceModel,
	api *credential,
	scope map[string]string,
	secret map[string]string,
	diags *diag.Diagnostics,
) {
	state.ID = types.StringValue(api.ID)
	state.Name = types.StringValue(api.Name)
	state.CredentialProvider = types.StringValue(api.CredentialProvider)
	state.Kind = types.StringValue(api.Kind)
	state.SchemaVersion = types.Int64Value(int64(api.SchemaVersion))
	if api.AccountID == "" {
		state.AccountID = types.StringNull()
	} else {
		state.AccountID = types.StringValue(api.AccountID)
	}
	if api.Region == "" {
		state.Region = types.StringNull()
	} else {
		state.Region = types.StringValue(api.Region)
	}
	state.CreatedAt = types.StringValue(api.CreatedAt)
	state.UpdatedAt = types.StringValue(api.UpdatedAt)

	scopeVal, d := types.MapValueFrom(ctx, types.StringType, api.Scope)
	diags.Append(d...)
	if scopeVal.IsNull() || scopeVal.IsUnknown() {
		// fall back to plan/state scope map if API omitted
		scopeVal, d = types.MapValueFrom(ctx, types.StringType, scope)
		diags.Append(d...)
	}
	state.Scope = scopeVal

	state.ScopeKind = types.StringValue(api.ScopeKind)
	state.ScopeVersion = types.Int64Value(int64(api.ScopeVersion))

	// Secret is never returned by API; keep the value we sent / already had.
	secretVal, d := types.MapValueFrom(ctx, types.StringType, secret)
	diags.Append(d...)
	state.Secret = secretVal
}

func optString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	if s == "" {
		return nil
	}
	return &s
}
