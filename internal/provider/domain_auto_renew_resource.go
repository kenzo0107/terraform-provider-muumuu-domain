package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

var (
	_ resource.Resource                = &domainAutoRenewResource{}
	_ resource.ResourceWithConfigure   = &domainAutoRenewResource{}
	_ resource.ResourceWithImportState = &domainAutoRenewResource{}
)

func newDomainAutoRenewResource() resource.Resource {
	return &domainAutoRenewResource{}
}

type domainAutoRenewResource struct {
	client *muumuu.Client
}

type domainAutoRenewResourceModel struct {
	DomainID types.String `tfsdk:"domain_id"`
	Enabled  types.Bool   `tfsdk:"enabled"`
}

func (r *domainAutoRenewResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_auto_renew"
}

func (r *domainAutoRenewResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages whether a domain is renewed automatically.

Destroying this resource only removes it from the Terraform state and leaves the setting unchanged.

The token needs permission to update domains. Sandbox personal access tokens cannot be granted it, and the API answers ` + "`403 insufficient_scope`" + `.
`,
		Attributes: map[string]schema.Attribute{
			"domain_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the domain (e.g. `MU00000001`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain is renewed automatically.",
				Required:            true,
			},
		},
	}
}

func (r *domainAutoRenewResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *domainAutoRenewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainAutoRenewResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.update(ctx, &plan, resp.Diagnostics.AddError) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainAutoRenewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainAutoRenewResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.update(ctx, &plan, resp.Diagnostics.AddError) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainAutoRenewResource) update(ctx context.Context, plan *domainAutoRenewResourceModel, addError func(string, string)) bool {
	domainID := plan.DomainID.ValueString()
	o, err := r.client.UpdateDomain(ctx, domainID, &muumuu.InputUpdateDomain{
		AutoRenewEnabled: muumuu.Bool(plan.Enabled.ValueBool()),
	})
	if err != nil {
		addError("Updating domain auto renewal", "Unable to update auto renewal (domain_id: "+domainID+"), got error: "+err.Error())
		return false
	}
	if enabled, ok := autoRenewEnabled(o); ok {
		plan.Enabled = types.BoolValue(enabled)
	}
	return true
}

func (r *domainAutoRenewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainAutoRenewResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	o, err := r.client.GetDomain(ctx, domainID)
	if muumuu.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading domain auto renewal", "Unable to read domain (domain_id: "+domainID+"), got error: "+err.Error())
		return
	}

	enabled, ok := autoRenewEnabled(o)
	if !ok {
		resp.Diagnostics.AddError("Reading domain auto renewal", "The API did not return auto-renew-enabled for domain "+domainID+".")
		return
	}
	state.Enabled = types.BoolValue(enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainAutoRenewResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Domain auto renewal left unchanged",
		"muumuu_domain_auto_renew was removed from the Terraform state only. The auto renewal setting of the domain was not changed.",
	)
}

func (r *domainAutoRenewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain_id"), req, resp)
}

func autoRenewEnabled(d *muumuu.Domain) (bool, bool) {
	if d.AutoRenewEnabled != nil {
		return *d.AutoRenewEnabled, true
	}
	if d.Contract != nil && d.Contract.AutoRenewEnabled != nil {
		return *d.Contract.AutoRenewEnabled, true
	}
	return false, false
}
