data "muumuu_dns_records" "mx" {
  domain_id = data.muumuu_domain.example.id
  type      = "MX"
}
