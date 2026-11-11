package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

var (
	_ datasource.DataSource                     = &domainDataSource{}
	_ datasource.DataSourceWithConfigure        = &domainDataSource{}
	_ datasource.DataSourceWithConfigValidators = &domainDataSource{}
)

func newDomainDataSource() datasource.DataSource {
	return &domainDataSource{}
}

type domainDataSource struct {
	client *muumuu.Client
}

type domainDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	FQDN              types.String `tfsdk:"fqdn"`
	SLD               types.String `tfsdk:"sld"`
	TLD               types.String `tfsdk:"tld"`
	State             types.String `tfsdk:"state"`
	SetupState        types.String `tfsdk:"setup_state"`
	StatusGroup       types.String `tfsdk:"status_group"`
	Registrar         types.String `tfsdk:"registrar"`
	WhoisProxyEnabled types.Bool   `tfsdk:"whois_proxy_enabled"`
	AutoRenewEnabled  types.Bool   `tfsdk:"auto_renew_enabled"`
	ContractEndDate   types.String `tfsdk:"contract_end_date"`
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a domain of the account by ID or FQDN.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the domain (e.g. `MU00000001`). Exactly one of `id` and `fqdn` must be set.",
				Optional:            true,
				Computed:            true,
			},
			"fqdn": schema.StringAttribute{
				MarkdownDescription: "The domain name (e.g. `example.com`). Exactly one of `id` and `fqdn` must be set.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{fqdnValidator()},
			},
			"sld":                 schema.StringAttribute{MarkdownDescription: "The second-level domain.", Computed: true},
			"tld":                 schema.StringAttribute{MarkdownDescription: "The top-level domain.", Computed: true},
			"state":               schema.StringAttribute{MarkdownDescription: "The state of the domain.", Computed: true},
			"setup_state":         schema.StringAttribute{MarkdownDescription: "The setup state of the domain.", Computed: true},
			"status_group":        schema.StringAttribute{MarkdownDescription: "The status group of the domain.", Computed: true},
			"registrar":           schema.StringAttribute{MarkdownDescription: "The registrar of the domain.", Computed: true},
			"whois_proxy_enabled": schema.BoolAttribute{MarkdownDescription: "Whether the WHOIS proxy is enabled.", Computed: true},
			"auto_renew_enabled":  schema.BoolAttribute{MarkdownDescription: "Whether the domain is renewed automatically.", Computed: true},
			"contract_end_date":   schema.StringAttribute{MarkdownDescription: "The end date of the contract.", Computed: true},
		},
	}
}

func (d *domainDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("fqdn")),
	}
}

func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config domainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		o   *muumuu.Domain
		err error
	)
	if !config.ID.IsNull() {
		o, err = d.client.GetDomain(ctx, config.ID.ValueString())
	} else {
		o, err = d.findDomainByFQDN(ctx, config.FQDN.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading domain", err.Error())
		return
	}

	autoRenew, ok := autoRenewEnabled(o)
	state := domainDataSourceModel{
		ID:                types.StringValue(o.ID),
		FQDN:              types.StringValue(normalizeFQDN(o.FQDN)),
		SLD:               types.StringValue(o.SLD),
		TLD:               types.StringValue(o.TLD),
		State:             types.StringValue(o.State),
		SetupState:        types.StringValue(o.SetupState),
		StatusGroup:       types.StringValue(o.StatusGroup),
		Registrar:         types.StringValue(o.Registrar),
		WhoisProxyEnabled: types.BoolPointerValue(o.WhoisProxyEnabled),
		AutoRenewEnabled:  types.BoolNull(),
		ContractEndDate:   types.StringPointerValue(o.ContractEndDate),
	}
	if ok {
		state.AutoRenewEnabled = types.BoolValue(autoRenew)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *domainDataSource) findDomainByFQDN(ctx context.Context, fqdn string) (*muumuu.Domain, error) {
	input := &muumuu.InputGetDomains{ListOptions: muumuu.ListOptions{Page: 1, PageSize: 100}, FQDN: fqdn}
	for {
		out, err := d.client.GetDomains(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("unable to list domains (fqdn: %s), got error: %w", fqdn, err)
		}
		for i := range out.Domains {
			if normalizeFQDN(out.Domains[i].FQDN) == fqdn {
				return &out.Domains[i], nil
			}
		}
		if !out.Meta.HasNext() || len(out.Domains) == 0 {
			return nil, fmt.Errorf("domain %s was not found in the account", fqdn)
		}
		input.Page++
	}
}
