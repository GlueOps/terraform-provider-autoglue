package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &recordSetResource{}
	_ resource.ResourceWithConfigure   = &recordSetResource{}
	_ resource.ResourceWithImportState = &recordSetResource{}
)

type recordSetResource struct {
	client *autoglueClient
}

type recordSetResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DomainID    types.String `tfsdk:"domain_id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	TTL         types.Int64  `tfsdk:"ttl"`
	Values      types.List   `tfsdk:"values"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	Status      types.String `tfsdk:"status"`
	LastError   types.String `tfsdk:"last_error"`
	Owner       types.String `tfsdk:"owner"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func NewRecordSetResource() resource.Resource {
	return &recordSetResource{}
}

func (r *recordSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record_set"
}

func (r *recordSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Manages a DNS record set for an Autoglue domain.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Unique record set ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"domain_id": resourceschema.StringAttribute{
				Required: true,
				Description: "Domain ID this record set belongs to. " +
					"Changing this requires creating a new record set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"name": resourceschema.StringAttribute{
				Required: true,
				Description: "Record name. May be relative to the domain (e.g. `api`) " +
					"or FQDN (e.g. `api.example.com`).",
			},

			"type": resourceschema.StringAttribute{
				Required: true,
				Description: "DNS record type (A, AAAA, CNAME, TXT, MX, NS, SRV, CAA). " +
					"Validation is enforced by the API.",
			},

			"ttl": resourceschema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "TTL in seconds (1–86400). Optional; if omitted, the " +
					"upstream default may apply.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},

			"values": resourceschema.ListAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Record values (e.g. IP addresses, hostnames, TXT values).",
			},

			"fingerprint": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Deterministic fingerprint for the desired record content.",
			},

			"status": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Provisioning status (pending, provisioning, ready, failed).",
			},

			"last_error": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last provisioning error, if any.",
			},

			"owner": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Owner marker for the record (e.g. `autoglue`).",
			},

			"created_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp (RFC3339).",
			},

			"updated_at": resourceschema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp (RFC3339).",
			},
		},
	}
}

func (r *recordSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *recordSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan recordSetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := plan.DomainID.ValueString()
	if domainID == "" {
		resp.Diagnostics.AddError("Missing domain_id", "domain_id must be set.")
		return
	}

	var values []string
	diags = plan.Values.ElementsAs(ctx, &values, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ttlPtr *int
	if !plan.TTL.IsNull() && !plan.TTL.IsUnknown() {
		v := int(plan.TTL.ValueInt64())
		ttlPtr = &v
	}

	payload := createRecordSetPayload{
		Name:   plan.Name.ValueString(),
		Type:   plan.Type.ValueString(),
		TTL:    ttlPtr,
		Values: values,
	}

	path := fmt.Sprintf("/dns/domains/%s/records", domainID)
	tflog.Info(ctx, "Creating Autoglue record set", map[string]any{
		"domain_id": domainID,
		"name":      payload.Name,
		"type":      payload.Type,
	})

	var apiResp recordSet
	if err := r.client.doJSON(ctx, http.MethodPost, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error creating record set", err.Error())
		return
	}

	if err := syncRecordSetFromAPI(ctx, &plan, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error mapping record set response", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *recordSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state recordSetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	path := fmt.Sprintf("/dns/records/%s", id)
	tflog.Info(ctx, "Reading Autoglue record set", map[string]any{"id": id})

	var apiResp recordSet
	if err := r.client.doJSON(ctx, http.MethodGet, path, "", nil, &apiResp); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading record set", fmt.Sprintf("Error reading record set: %s", err.Error()))
		return
	}

	if err := syncRecordSetFromAPI(ctx, &state, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error mapping record set response", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *recordSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var plan recordSetResourceModel
	var state recordSetResourceModel

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
		resp.Diagnostics.AddError("Missing ID", "Record set ID is required in state to update.")
		return
	}

	var payload updateRecordSetPayload

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		payload.Name = &v
	}
	if !plan.Type.Equal(state.Type) {
		v := plan.Type.ValueString()
		payload.Type = &v
	}
	if !plan.TTL.Equal(state.TTL) {
		if !plan.TTL.IsNull() && !plan.TTL.IsUnknown() {
			v := int(plan.TTL.ValueInt64())
			payload.TTL = &v
		} else {
			// TTL cleared, send explicit nil to let server handle
			var nilTTL *int
			payload.TTL = nilTTL
		}
	}
	if !plan.Values.Equal(state.Values) {
		var vals []string
		diags = plan.Values.ElementsAs(ctx, &vals, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		payload.Values = &vals
	}

	if payload.Name == nil && payload.Type == nil && payload.TTL == nil && payload.Values == nil {
		// Nothing to update
		return
	}

	path := fmt.Sprintf("/dns/records/%s", id)
	tflog.Info(ctx, "Updating Autoglue record set", map[string]any{
		"id":   id,
		"name": plan.Name.ValueString(),
		"type": plan.Type.ValueString(),
	})

	var apiResp recordSet
	if err := r.client.doJSON(ctx, http.MethodPatch, path, "", payload, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error updating record set", err.Error())
		return
	}

	if err := syncRecordSetFromAPI(ctx, &plan, &apiResp); err != nil {
		resp.Diagnostics.AddError("Error mapping record set response", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r *recordSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client not configured", "The provider client was not configured.")
		return
	}

	var state recordSetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if id == "" {
		return
	}

	path := fmt.Sprintf("/dns/records/%s", id)
	tflog.Info(ctx, "Deleting Autoglue record set", map[string]any{"id": id})

	if err := r.client.doJSON(ctx, http.MethodDelete, path, "", nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting record set", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *recordSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// terraform import autoglue_record_set.example <record_set_id>
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func syncRecordSetFromAPI(
	ctx context.Context,
	state *recordSetResourceModel,
	api *recordSet,
) error {
	state.ID = types.StringValue(api.ID)
	state.DomainID = types.StringValue(api.DomainID)
	state.Name = types.StringValue(api.Name)
	state.Type = types.StringValue(api.Type)

	if api.TTL != nil {
		state.TTL = types.Int64Value(int64(*api.TTL))
	} else {
		state.TTL = types.Int64Null()
	}

	// Values comes back as JSON; we expect []string
	var vals []string
	if len(api.Values) > 0 && string(api.Values) != "null" {
		if err := json.Unmarshal(api.Values, &vals); err != nil {
			return err
		}
	}
	listVal, diags := types.ListValueFrom(ctx, types.StringType, vals)
	if diags.HasError() {
		// propagate one of the diagnostic messages as error
		return fmt.Errorf("failed to convert values to list")
	}
	state.Values = listVal

	state.Fingerprint = types.StringValue(api.Fingerprint)
	state.Status = types.StringValue(api.Status)
	state.LastError = types.StringValue(api.LastError)
	state.Owner = types.StringValue(api.Owner)
	state.CreatedAt = types.StringValue(api.CreatedAt)
	state.UpdatedAt = types.StringValue(api.UpdatedAt)

	return nil
}
