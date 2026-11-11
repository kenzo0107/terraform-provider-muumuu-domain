data "muumuu_domain" "example" {
  fqdn = "example.com"
}

resource "muumuu_dns_record" "www" {
  domain_id = data.muumuu_domain.example.id
  fqdn      = "www.example.com"
  type      = "A"
  value     = "192.0.2.1"
}

resource "muumuu_dns_record" "mx" {
  domain_id = data.muumuu_domain.example.id
  fqdn      = "example.com"
  type      = "MX"
  value     = "mail.example.com."
  priority  = 10
}
