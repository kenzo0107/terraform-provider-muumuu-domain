package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestDNSRecordResource_CRUD(t *testing.T) {
	newFakeAPI(t)
	name := "muumuu_dns_record.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordConfig(fakeDomainID, "www.example.com", "CNAME", "a.example.net", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "101"),
					resource.TestCheckResourceAttr(name, "fqdn", "www.example.com"),
					resource.TestCheckResourceAttr(name, "value", "a.example.net"),
					resource.TestCheckResourceAttr(name, "ttl", "3600"),
					resource.TestCheckNoResourceAttr(name, "priority"),
				),
			},
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateIdFunc:       importStateIDWithDomain(name),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
			{
				Config: testDNSRecordConfig(fakeDomainID, "www.example.com", "CNAME", "b.example.net", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr(name, "value", "b.example.net"),
			},
			{
				Config: testDNSRecordConfig(fakeDomainID, "www.example.com", "CNAME", "b.example.net", "ttl = 300"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "102"),
					resource.TestCheckResourceAttr(name, "ttl", "300"),
				),
			},
		},
	})
}

func TestDNSRecordResource_ImportedRecordWithTrailingDotsIsNotReplaced(t *testing.T) {
	api := newFakeAPI(t)
	api.records[1] = &fakeRecord{ID: 1, FQDN: "www.example.com.", Type: "CNAME", Value: "a.example.net.", TTL: 3600}
	name := "muumuu_dns_record.test"
	config := testDNSRecordConfig(fakeDomainID, "www.example.com", "CNAME", "a.example.net", "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       name,
				ImportState:        true,
				ImportStateId:      fakeDomainID + "/1",
				ImportStatePersist: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if got := states[0].Attributes["fqdn"]; got != "www.example.com" {
						return fmt.Errorf("fqdn = %q, want www.example.com", got)
					}
					return nil
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestDNSRecordResource_MXPriorityUpdatesInPlace(t *testing.T) {
	newFakeAPI(t)
	name := "muumuu_dns_record.test"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordConfig(fakeDomainID, "example.com", "MX", "mail.example.com", "priority = 10"),
				Check:  resource.TestCheckResourceAttr(name, "priority", "10"),
			},
			{
				Config: testDNSRecordConfig(fakeDomainID, "example.com", "MX", "mail.example.com", "priority = 20"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr(name, "priority", "20"),
			},
		},
	})
}

func TestDNSRecordResource_RemovedOutsideTerraformIsRecreated(t *testing.T) {
	api := newFakeAPI(t)
	name := "muumuu_dns_record.test"
	config := testDNSRecordConfig(fakeDomainID, "www.example.com", "A", "192.0.2.1", "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: api.deleteAllRecords,
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionCreate)},
				},
			},
		},
	})
}

func TestDNSRecordResource_RejectsInvalidArguments(t *testing.T) {
	newFakeAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testDNSRecordConfig(fakeDomainID, "www.example.com.", "A", "192.0.2.1", ""),
				ExpectError: regexp.MustCompile(`without a\s+trailing dot`),
			},
			{
				Config:      testDNSRecordConfig(fakeDomainID, "www", "A", "192.0.2.1", ""),
				ExpectError: regexp.MustCompile(`including the domain name`),
			},
			{
				Config:      testDNSRecordConfig(fakeDomainID, "www.example.com", "PTR", "192.0.2.1", ""),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestDNSRecordResource_RejectsMalformedImportID(t *testing.T) {
	newFakeAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        testDNSRecordConfig(fakeDomainID, "www.example.com", "A", "192.0.2.1", ""),
				ResourceName:  "muumuu_dns_record.test",
				ImportState:   true,
				ImportStateId: "101",
				ExpectError:   regexp.MustCompile(`<domain_id>/<record_id>`),
			},
		},
	})
}

func TestAccDNSRecordResource(t *testing.T) {
	name := "muumuu_dns_record.test"
	domainID := os.Getenv("MUUMUU_DOMAIN_TEST_DOMAIN_ID")
	fqdn := fmt.Sprintf("tf-acc-%s.%s", acctest.RandString(8), os.Getenv("MUUMUU_DOMAIN_TEST_DOMAIN_FQDN"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordConfig(domainID, fqdn, "TXT", "tf-acc", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "fqdn", fqdn),
					resource.TestCheckResourceAttr(name, "value", "tf-acc"),
				),
			},
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateIdFunc: importStateIDWithDomain(name),
				ImportStateVerify: true,
			},
			{
				Config: testDNSRecordConfig(domainID, fqdn, "TXT", "tf-acc-updated", ""),
				Check:  resource.TestCheckResourceAttr(name, "value", "tf-acc-updated"),
			},
		},
	})
}

func testDNSRecordConfig(domainID, fqdn, typ, value, extra string) string {
	return fmt.Sprintf(`
resource "muumuu_dns_record" "test" {
  domain_id = %q
  fqdn      = %q
  type      = %q
  value     = %q
  %s
}
`, domainID, fqdn, typ, value, extra)
}
