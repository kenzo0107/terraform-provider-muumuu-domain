package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

var (
	_ resource.Resource                   = &nameserversResource{}
	_ resource.ResourceWithConfigure      = &nameserversResource{}
	_ resource.ResourceWithImportState    = &nameserversResource{}
	_ resource.ResourceWithValidateConfig = &nameserversResource{}
)

var nameserverSetupTypes = []string{
	muumuu.NameserverSetupTypeLolipop,
	muumuu.NameserverSetupTypeCustom,
	muumuu.NameserverSetupTypeAcquiredDomain,
	muumuu.NameserverSetupTypeMuumuuDNS,
	muumuu.NameserverSetupTypeParking,
	muumuu.NameserverSetupTypeOther,
}

func newNameserversResource() resource.Resource {
	return &nameserversResource{}
}

type nameserversResource struct {
	client *muumuu.Client
}

type nameserversResourceModel struct {
	DomainID               types.String `tfsdk:"domain_id"`
	SetupType              types.String `tfsdk:"setup_type"`
	Nameservers            types.List   `tfsdk:"nameservers"`
	ConfirmExistingRecords types.Bool   `tfsdk:"confirm_existing_records"`
}

func (r *nameserversResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nameservers"
}

func (r *nameserversResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages the nameserver settings of a domain.

The settings always exist for a domain, so destroying this resource only removes it from the Terraform state and leaves the nameservers unchanged.
`,
		Attributes: map[string]schema.Attribute{
			"domain_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the domain (e.g. `MU00000001`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"setup_type": schema.StringAttribute{
				MarkdownDescription: "How the nameservers are set up. One of `" + strings.Join(nameserverSetupTypes, "`, `") + "`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(nameserverSetupTypes...)},
			},
			"nameservers": schema.ListAttribute{
				MarkdownDescription: "The nameservers. Required when `setup_type` is `custom`; otherwise set by Muumuu Domain.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(fqdnValidator()),
				},
			},
			"confirm_existing_records": schema.BoolAttribute{
				MarkdownDescription: "Confirms that existing DNS records may be affected by the change. Only sent to the API; not read back.",
				Optional:            true,
			},
		},
	}
}

func (r *nameserversResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config nameserversResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.SetupType.IsUnknown() || config.Nameservers.IsUnknown() {
		return
	}

	isCustom := config.SetupType.ValueString() == muumuu.NameserverSetupTypeCustom
	switch {
	case isCustom && config.Nameservers.IsNull():
		resp.Diagnostics.AddAttributeError(path.Root("nameservers"), "Missing nameservers", "nameservers is required when setup_type is custom.")
	case !isCustom && !config.Nameservers.IsNull():
		resp.Diagnostics.AddAttributeError(path.Root("nameservers"), "Unexpected nameservers", "nameservers can only be set when setup_type is custom.")
	}
}

func (r *nameserversResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *nameserversResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nameserversResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nameserversResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state nameserversResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Why not always PATCH: re-applying the same setup_type only because confirm_existing_records changed may reset the settings.
	if plan.SetupType.Equal(state.SetupType) && (plan.Nameservers.IsUnknown() || plan.Nameservers.Equal(state.Nameservers)) {
		plan.Nameservers = state.Nameservers
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nameserversResource) apply(ctx context.Context, plan *nameserversResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	input := &muumuu.InputUpdateNameservers{
		SetupType: plan.SetupType.ValueString(),
	}
	if !plan.Nameservers.IsNull() && !plan.Nameservers.IsUnknown() {
		diags.Append(plan.Nameservers.ElementsAs(ctx, &input.Nameservers, false)...)
		if diags.HasError() {
			return diags
		}
	}
	if !plan.ConfirmExistingRecords.IsNull() {
		input.ConfirmExistingRecords = muumuu.Bool(plan.ConfirmExistingRecords.ValueBool())
	}

	domainID := plan.DomainID.ValueString()
	o, err := r.client.UpdateNameservers(ctx, domainID, input)
	if err != nil {
		diags.AddError("Updating nameservers", "Unable to update nameservers (domain_id: "+domainID+"), got error: "+err.Error())
		return diags
	}

	diags.Append(plan.refresh(ctx, o)...)
	return diags
}

func (r *nameserversResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nameserversResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	o, err := r.client.GetNameservers(ctx, domainID)
	if muumuu.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading nameservers", "Unable to read nameservers (domain_id: "+domainID+"), got error: "+err.Error())
		return
	}

	resp.Diagnostics.Append(state.refresh(ctx, o)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *nameserversResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Nameservers left unchanged",
		"muumuu_nameservers was removed from the Terraform state only. The nameserver settings of the domain were not changed.",
	)
}

func (r *nameserversResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain_id"), req, resp)
}

func (m *nameserversResourceModel) refresh(ctx context.Context, o *muumuu.NameserverSettings) diag.Diagnostics {
	m.SetupType = types.StringValue(o.SetupType)

	if !m.Nameservers.IsNull() && !m.Nameservers.IsUnknown() {
		var prior []string
		if diags := m.Nameservers.ElementsAs(ctx, &prior, false); diags.HasError() {
			return diags
		}
		if nameserversEqual(prior, o.Nameservers) {
			return nil
		}
	}

	nameservers := make([]string, len(o.Nameservers))
	for i, ns := range o.Nameservers {
		nameservers[i] = normalizeFQDN(ns)
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, nameservers)
	m.Nameservers = list
	return diags
}
