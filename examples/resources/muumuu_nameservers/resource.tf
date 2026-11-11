data "muumuu_domain" "example" {
  fqdn = "example.com"
}

resource "aws_route53_zone" "example" {
  name = "example.com"
}

# Route 53 にネームサーバーを委譲する
resource "muumuu_nameservers" "example" {
  domain_id   = data.muumuu_domain.example.id
  setup_type  = "custom"
  nameservers = aws_route53_zone.example.name_servers
}
