package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

var (
	_ datasource.DataSource              = &dnsRecordsDataSource{}
	_ datasource.DataSourceWithConfigure = &dnsRecordsDataSource{}
)

func newDNSRecordsDataSource() datasource.DataSource {
	return &dnsRecordsDataSource{}
}

type dnsRecordsDataSource struct {
	client *muumuu.Client
}

type dnsRecordsDataSourceModel struct {
	DomainID types.String              `tfsdk:"domain_id"`
	FQDN     types.String              `tfsdk:"fqdn"`
	Type     types.String              `tfsdk:"type"`
	Records  []dnsRecordDataSourceItem `tfsdk:"records"`
}

type dnsRecordDataSourceItem struct {
	ID       types.String `tfsdk:"id"`
	FQDN     types.String `tfsdk:"fqdn"`
	Type     types.String `tfsdk:"type"`
	Value    types.String `tfsdk:"value"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Priority types.Int64  `tfsdk:"priority"`
}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the DNS records of a domain that uses Muumuu DNS.",
		Attributes: map[string]schema.Attribute{
			"domain_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the domain (e.g. `MU00000001`).",
				Required:            true,
			},
			"fqdn": schema.StringAttribute{
				MarkdownDescription: "Only return records with this FQDN.",
				Optional:            true,
				Validators:          []validator.String{fqdnValidator()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Only return records of this type.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf(dnsRecordTypes...)},
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "The DNS records.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":       schema.StringAttribute{MarkdownDescription: "The ID of the DNS record.", Computed: true},
						"fqdn":     schema.StringAttribute{MarkdownDescription: "The FQDN of the record without a trailing dot.", Computed: true},
						"type":     schema.StringAttribute{MarkdownDescription: "The record type.", Computed: true},
						"value":    schema.StringAttribute{MarkdownDescription: "The record value.", Computed: true},
						"ttl":      schema.Int64Attribute{MarkdownDescription: "The TTL in seconds.", Computed: true},
						"priority": schema.Int64Attribute{MarkdownDescription: "The priority of `MX` and `SRV` records.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dnsRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := config.DomainID.ValueString()
	input := &muumuu.InputGetDNSRecords{
		ListOptions: muumuu.ListOptions{Page: 1, PageSize: 100},
		FQDN:        config.FQDN.ValueString(),
		Type:        config.Type.ValueString(),
	}

	records := []dnsRecordDataSourceItem{}
	for {
		out, err := d.client.GetDNSRecords(ctx, domainID, input)
		if err != nil {
			resp.Diagnostics.AddError(
				"Reading DNS records",
				fmt.Sprintf("Unable to list DNS records (domain_id: %s), got error: %s", domainID, err),
			)
			return
		}
		for _, r := range out.DNSRecords {
			records = append(records, dnsRecordDataSourceItem{
				ID:       types.StringValue(strconv.FormatInt(r.ID, 10)),
				FQDN:     types.StringValue(normalizeFQDN(r.FQDN)),
				Type:     types.StringValue(r.Type),
				Value:    types.StringValue(r.Value),
				TTL:      types.Int64Value(int64(r.TTL)),
				Priority: intPtrToInt64(r.Priority),
			})
		}
		if !out.Meta.HasNext() || len(out.DNSRecords) == 0 {
			break
		}
		input.Page++
	}

	config.Records = records
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
