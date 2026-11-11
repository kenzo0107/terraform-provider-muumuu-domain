package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDomainDataSource_LooksUpByFQDN(t *testing.T) {
	newFakeAPI(t)
	name := "data.muumuu_domain.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "muumuu_domain" "test" { fqdn = "example.com" }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", fakeDomainID),
					resource.TestCheckResourceAttr(name, "sld", "example"),
					resource.TestCheckResourceAttr(name, "tld", "com"),
					resource.TestCheckResourceAttr(name, "whois_proxy_enabled", "true"),
					resource.TestCheckResourceAttr(name, "contract_end_date", "2027-01-01"),
				),
			},
		},
	})
}

func TestDomainDataSource_FailsWhenDomainIsNotInAccount(t *testing.T) {
	newFakeAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "muumuu_domain" "test" { fqdn = "unknown.example" }`,
				ExpectError: regexp.MustCompile(`domain unknown.example was not found`),
			},
			{
				Config:      `data "muumuu_domain" "test" {}`,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured`),
			},
		},
	})
}

func TestDNSRecordsDataSource_FiltersByType(t *testing.T) {
	newFakeAPI(t)
	name := "data.muumuu_dns_records.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "muumuu_dns_record" "a" {
  domain_id = %[1]q
  fqdn      = "www.example.com"
  type      = "A"
  value     = "192.0.2.1"
}

resource "muumuu_dns_record" "mx" {
  domain_id = %[1]q
  fqdn      = "example.com"
  type      = "MX"
  value     = "mail.example.com"
  priority  = 10
}

data "muumuu_dns_records" "test" {
  domain_id  = %[1]q
  type       = "MX"
  depends_on = [muumuu_dns_record.a, muumuu_dns_record.mx]
}
`, fakeDomainID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "records.#", "1"),
					resource.TestCheckResourceAttr(name, "records.0.fqdn", "example.com"),
					resource.TestCheckResourceAttr(name, "records.0.priority", "10"),
				),
			},
		},
	})
}

func TestNameserversDataSource_StripsTrailingDots(t *testing.T) {
	newFakeAPI(t)
	name := "data.muumuu_nameservers.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`data "muumuu_nameservers" "test" { domain_id = %q }`, fakeDomainID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "setup_type", "muumuu_dns"),
					resource.TestCheckResourceAttr(name, "nameservers.#", "2"),
					resource.TestCheckResourceAttr(name, "nameservers.1", "ns2.muumuu-domain.com"),
				),
			},
		},
	})
}
