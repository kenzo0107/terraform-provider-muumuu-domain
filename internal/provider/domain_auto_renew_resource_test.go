package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDomainAutoRenewResource_TogglesSetting(t *testing.T) {
	newFakeAPI(t)
	name := "muumuu_domain_auto_renew.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDomainAutoRenewConfig(true),
				Check:  resource.TestCheckResourceAttr(name, "enabled", "true"),
			},
			{
				ResourceName:                         name,
				ImportState:                          true,
				ImportStateId:                        fakeDomainID,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "domain_id",
			},
			{
				Config: testDomainAutoRenewConfig(false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "enabled", "false"),
					resource.TestCheckResourceAttr("data.muumuu_domain.test", "auto_renew_enabled", "false"),
				),
			},
		},
	})
}

func testDomainAutoRenewConfig(enabled bool) string {
	return fmt.Sprintf(`
resource "muumuu_domain_auto_renew" "test" {
  domain_id = %q
  enabled   = %t
}

data "muumuu_domain" "test" {
  id = muumuu_domain_auto_renew.test.domain_id
}
`, fakeDomainID, enabled)
}
