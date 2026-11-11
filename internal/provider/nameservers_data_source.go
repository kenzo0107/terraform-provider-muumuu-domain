package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

var (
	_ datasource.DataSource              = &nameserversDataSource{}
	_ datasource.DataSourceWithConfigure = &nameserversDataSource{}
)

func newNameserversDataSource() datasource.DataSource {
	return &nameserversDataSource{}
}

type nameserversDataSource struct {
	client *muumuu.Client
}

type nameserversDataSourceModel struct {
	DomainID    types.String `tfsdk:"domain_id"`
	SetupType   types.String `tfsdk:"setup_type"`
	Nameservers types.List   `tfsdk:"nameservers"`
}

func (d *nameserversDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nameservers"
}

func (d *nameserversDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the nameserver settings of a domain.",
		Attributes: map[string]schema.Attribute{
			"domain_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the domain (e.g. `MU00000001`).",
				Required:            true,
			},
			"setup_type": schema.StringAttribute{
				MarkdownDescription: "How the nameservers are set up.",
				Computed:            true,
			},
			"nameservers": schema.ListAttribute{
				MarkdownDescription: "The nameservers without a trailing dot.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *nameserversDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, err := clientFromProviderData(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", err.Error())
		return
	}
	d.client = client
}

func (d *nameserversDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config nameserversDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := config.DomainID.ValueString()
	o, err := d.client.GetNameservers(ctx, domainID)
	if err != nil {
		resp.Diagnostics.AddError("Reading nameservers", "Unable to read nameservers (domain_id: "+domainID+"), got error: "+err.Error())
		return
	}

	nameservers := make([]string, len(o.Nameservers))
	for i, ns := range o.Nameservers {
		nameservers[i] = normalizeFQDN(ns)
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, nameservers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.SetupType = types.StringValue(o.SetupType)
	config.Nameservers = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
