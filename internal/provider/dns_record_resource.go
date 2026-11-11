package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

var (
	_ resource.Resource                = &dnsRecordResource{}
	_ resource.ResourceWithConfigure   = &dnsRecordResource{}
	_ resource.ResourceWithImportState = &dnsRecordResource{}
)

var dnsRecordTypes = []string{
	muumuu.DNSRecordTypeA,
	muumuu.DNSRecordTypeAAAA,
	muumuu.DNSRecordTypeCNAME,
	muumuu.DNSRecordTypeMX,
	muumuu.DNSRecordTypeTXT,
	muumuu.DNSRecordTypeNS,
	muumuu.DNSRecordTypeALIAS,
	muumuu.DNSRecordTypeSRV,
	muumuu.DNSRecordTypeCAA,
}

func newDNSRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

type dnsRecordResource struct {
	client *muumuu.Client
}

type dnsRecordResourceModel struct {
	ID       types.String `tfsdk:"id"`
	DomainID types.String `tfsdk:"domain_id"`
	FQDN     types.String `tfsdk:"fqdn"`
	Type     types.String `tfsdk:"type"`
	Value    types.String `tfsdk:"value"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Priority types.Int64  `tfsdk:"priority"`
}

func (r *dnsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Provides a DNS record of a domain that uses Muumuu DNS.

Only ` + "`value`" + ` and ` + "`priority`" + ` can be updated in place. Changing any other argument replaces the record.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the DNS record.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the domain (e.g. `MU00000001`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"fqdn": schema.StringAttribute{
				MarkdownDescription: "The fully qualified name of the record in lower case without a trailing dot (e.g. `www.example.com`). Use the domain name itself for the zone apex.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{fqdnValidator()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The record type. One of `" + strings.Join(dnsRecordTypes, "`, `") + "`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{stringvalidator.OneOf(dnsRecordTypes...)},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The record value in the format of the record type. `CNAME`, `MX`, `NS` and `ALIAS` values must end with a dot (e.g. `mail.example.com.`).",
				Required:            true,
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "The TTL in seconds. Muumuu DNS fixes it to `3600`.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "The priority. Used by `MX` and `SRV` records. Removing it from the configuration keeps the current value.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Validators: []validator.Int64{int64validator.AtLeast(0)},
			},
		},
	}
}

func (r *dnsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, err := clientFromProviderData(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", err.Error())
		return
	}
	r.client = client
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := &muumuu.InputCreateDNSRecord{
		FQDN:     plan.FQDN.ValueString(),
		Type:     plan.Type.ValueString(),
		Value:    plan.Value.ValueString(),
		Priority: int64PtrToIntPtr(plan.Priority),
	}

	domainID := plan.DomainID.ValueString()
	o, err := r.client.CreateDNSRecord(ctx, domainID, input)
	if err != nil {
		resp.Diagnostics.AddError(
			"Creating DNS record",
			fmt.Sprintf("Unable to create DNS record (domain_id: %s, fqdn: %s), got error: %s", domainID, input.FQDN, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newDNSRecordResourceModel(domainID, o, plan))...)
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	recordID, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Reading DNS record", fmt.Sprintf("Invalid DNS record ID %q: %s", state.ID.ValueString(), err))
		return
	}

	o, err := r.findDNSRecord(ctx, domainID, recordID, state)
	if muumuu.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Reading DNS record",
			fmt.Sprintf("Unable to read DNS record (domain_id: %s, id: %d), got error: %s", domainID, recordID, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newDNSRecordResourceModel(domainID, o, state))...)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	recordID, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Updating DNS record", fmt.Sprintf("Invalid DNS record ID %q: %s", state.ID.ValueString(), err))
		return
	}

	o, err := r.client.UpdateDNSRecord(ctx, domainID, recordID, &muumuu.InputUpdateDNSRecord{
		Value:    muumuu.String(plan.Value.ValueString()),
		Priority: int64PtrToIntPtr(plan.Priority),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Updating DNS record",
			fmt.Sprintf("Unable to update DNS record (domain_id: %s, id: %d), got error: %s", domainID, recordID, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newDNSRecordResourceModel(domainID, o, plan))...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	recordID, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Deleting DNS record", fmt.Sprintf("Invalid DNS record ID %q: %s", state.ID.ValueString(), err))
		return
	}

	err = r.client.DeleteDNSRecord(ctx, domainID, recordID)
	if err != nil && !muumuu.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Deleting DNS record",
			fmt.Sprintf("Unable to delete DNS record (domain_id: %s, id: %d), got error: %s", domainID, recordID, err),
		)
	}
}

func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domainID, recordID, ok := strings.Cut(req.ID, "/")
	if !ok || domainID == "" || recordID == "" {
		resp.Diagnostics.AddError(
			"Importing DNS record",
			fmt.Sprintf("Expected import ID in the format <domain_id>/<record_id>, got: %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_id"), domainID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), recordID)...)
}

// Why not GetDNSRecord alone: it scans the whole zone per record, which exhausts the 1,000 requests/hour limit on large zones.
func (r *dnsRecordResource) findDNSRecord(ctx context.Context, domainID string, recordID int64, state dnsRecordResourceModel) (*muumuu.DNSRecord, error) {
	if state.FQDN.IsNull() || state.Type.IsNull() {
		return r.client.GetDNSRecord(ctx, domainID, recordID)
	}

	input := &muumuu.InputGetDNSRecords{
		ListOptions: muumuu.ListOptions{Page: 1, PageSize: 100},
		FQDN:        state.FQDN.ValueString(),
		Type:        state.Type.ValueString(),
	}
	for {
		out, err := r.client.GetDNSRecords(ctx, domainID, input)
		if err != nil {
			return nil, err
		}
		for i := range out.DNSRecords {
			if out.DNSRecords[i].ID == recordID {
				return &out.DNSRecords[i], nil
			}
		}
		if !out.Meta.HasNext() || len(out.DNSRecords) == 0 {
			return nil, &muumuu.Error{StatusCode: http.StatusNotFound, Code: muumuu.ErrorCodeNotFound}
		}
		input.Page++
	}
}

func newDNSRecordResourceModel(domainID string, o *muumuu.DNSRecord, prior dnsRecordResourceModel) *dnsRecordResourceModel {
	return &dnsRecordResourceModel{
		ID:       types.StringValue(strconv.FormatInt(o.ID, 10)),
		DomainID: types.StringValue(domainID),
		FQDN:     types.StringValue(normalizeFQDN(o.FQDN)),
		Type:     types.StringValue(o.Type),
		Value:    dnsValue(prior.Value, o.Value),
		TTL:      types.Int64Value(int64(o.TTL)),
		Priority: intPtrToInt64(o.Priority),
	}
}

func int64PtrToIntPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return muumuu.Int(int(v.ValueInt64()))
}

func intPtrToInt64(v *int) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}
