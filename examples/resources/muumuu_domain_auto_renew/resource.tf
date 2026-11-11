data "muumuu_domain" "example" {
  fqdn = "example.com"
}

resource "muumuu_domain_auto_renew" "example" {
  domain_id = data.muumuu_domain.example.id
  enabled   = true
}
