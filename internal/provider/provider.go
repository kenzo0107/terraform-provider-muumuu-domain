package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	muumuu "github.com/kenzo0107/muumuu-domain"
)

const (
	envToken   = "MUUMUU_DOMAIN_TOKEN"
	envBaseURL = "MUUMUU_DOMAIN_BASE_URL"
)

var _ provider.Provider = &muumuuProvider{}

type muumuuProvider struct {
	version string
}

type muumuuProviderModel struct {
	Token   types.String `tfsdk:"token"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &muumuuProvider{version: version}
	}
}

func (p *muumuuProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "muumuu"
	resp.Version = p.version
}

func (p *muumuuProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
The Muumuu Domain provider manages domains of a [Muumuu Domain](https://muumuu-domain.com/) account through the [Muumuu Domain API v2](https://muumuu-domain.com/developers/openapi-me.html).

## Rate limits

Every request is retried automatically when the API answers with HTTP 429. The provider waits for the ` + "`Retry-After`" + ` header
(up to 5 retries, at most 60 seconds per wait) before failing. Only 429 responses are retried, so a request is never applied twice.
`,
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				MarkdownDescription: "Personal access token for the Muumuu Domain API. May also be provided via the `" + envToken + "` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Muumuu Domain API. May also be provided via the `" + envBaseURL + "` environment variable. Defaults to `" + muumuu.BaseURLProduction + "`. Use `" + muumuu.BaseURLSandbox + "` for the sandbox.",
				Optional:            true,
			},
		},
	}
}

func (p *muumuuProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config muumuuProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Muumuu Domain API token",
			"The provider cannot create the Muumuu Domain API client as there is an unknown configuration value for the token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the "+envToken+" environment variable.",
		)
	}
	if config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Unknown Muumuu Domain API base URL",
			"The provider cannot create the Muumuu Domain API client as there is an unknown configuration value for the base URL.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	token := os.Getenv(envToken)
	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}
	baseURL := os.Getenv(envBaseURL)
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Muumuu Domain API token",
			"Set the token value in the configuration or use the "+envToken+" environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
		return
	}

	opts := []muumuu.Option{
		muumuu.OptionHTTPClient(newHTTPClient()),
		muumuu.OptionUserAgent(fmt.Sprintf("terraform-provider-muumuu-domain/%s", p.version)),
	}
	if baseURL != "" {
		opts = append(opts, muumuu.OptionBaseURL(baseURL))
	}

	client, err := muumuu.New(token, opts...)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Muumuu Domain API client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *muumuuProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newDNSRecordResource,
		newNameserversResource,
		newDomainAutoRenewResource,
	}
}

func (p *muumuuProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newDomainDataSource,
		newDNSRecordsDataSource,
		newNameserversDataSource,
	}
}

func clientFromProviderData(v any) (*muumuu.Client, error) {
	c, ok := v.(*muumuu.Client)
	if !ok || c == nil {
		return nil, fmt.Errorf("expected *muumuu.Client, got: %T. Please report this issue to the provider developers", v)
	}
	return c, nil
}
