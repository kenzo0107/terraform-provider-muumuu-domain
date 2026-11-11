package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestNameserversResource_SwitchesBetweenCustomAndMuumuuDNS(t *testing.T) {
	newFakeAPI(t)
	name := "muumuu_nameservers.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testNameserversConfig(`setup_type = "custom"
  nameservers = ["ns-1.awsdns-01.org", "ns-2.awsdns-02.net"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "setup_type", "custom"),
					resource.TestCheckResourceAttr(name, "nameservers.#", "2"),
					resource.TestCheckResourceAttr(name, "nameservers.0", "ns-1.awsdns-01.org"),
				),
			},
			{
				ResourceName:                         name,
				ImportState:                          true,
				ImportStateId:                        fakeDomainID,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "domain_id",
			},
			{
				Config: testNameserversConfig(`setup_type = "muumuu_dns"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectUnknownValue(name, tfjsonPath("nameservers"))},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "setup_type", "muumuu_dns"),
					resource.TestCheckResourceAttr(name, "nameservers.0", "ns1.muumuu-domain.com"),
				),
			},
		},
	})
}

func TestNameserversResource_ConfirmExistingRecordsAloneDoesNotCallAPI(t *testing.T) {
	api := newFakeAPI(t)
	name := "muumuu_nameservers.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testNameserversConfig(`setup_type = "muumuu_dns"`)},
			{
				PreConfig: func() { api.nameserverPatches = 0 },
				Config: testNameserversConfig(`setup_type = "muumuu_dns"
  confirm_existing_records = true`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "nameservers.#", "2"),
					func(*terraform.State) error {
						if api.nameserverPatches != 0 {
							return fmt.Errorf("nameserver PATCH called %d times, want 0", api.nameserverPatches)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestNameserversResource_ValidatesNameserversAgainstSetupType(t *testing.T) {
	newFakeAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testNameserversConfig(`setup_type = "custom"`),
				ExpectError: regexp.MustCompile(`nameservers is required when setup_type is custom`),
			},
			{
				Config: testNameserversConfig(`setup_type = "muumuu_dns"
  nameservers = ["ns1.example.com"]`),
				ExpectError: regexp.MustCompile(`nameservers can only be set when setup_type is custom`),
			},
		},
	})
}

func testNameserversConfig(body string) string {
	return fmt.Sprintf(`
resource "muumuu_nameservers" "test" {
  domain_id = %q
  %s
}
`, fakeDomainID, body)
}
