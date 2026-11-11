[![Tests](https://github.com/kenzo0107/terraform-provider-muumuu-domain/actions/workflows/test.yml/badge.svg)](https://github.com/kenzo0107/terraform-provider-muumuu-domain/actions/workflows/test.yml)

# Terraform Provider for Muumuu Domain

Terraform provider for [Muumuu Domain](https://muumuu-domain.com/), built with the Terraform Plugin Framework on top of [kenzo0107/muumuu-domain](https://github.com/kenzo0107/muumuu-domain).

## Usage

```hcl
terraform {
  required_providers {
    muumuu = {
      source = "kenzo0107/muumuu-domain"
    }
  }
}

provider "muumuu" {} # MUUMUU_DOMAIN_TOKEN

data "muumuu_domain" "example" {
  fqdn = "example.com"
}

resource "muumuu_dns_record" "www" {
  domain_id = data.muumuu_domain.example.id
  fqdn      = "www.example.com"
  type      = "A"
  value     = "192.0.2.1"
}
```

The provider's local name is `muumuu`, so resources are named `muumuu_*`.

| Kind | Name |
| --- | --- |
| Resource | `muumuu_dns_record`, `muumuu_nameservers`, `muumuu_domain_auto_renew` |
| Data source | `muumuu_domain`, `muumuu_dns_records`, `muumuu_nameservers` |

See [docs](docs/) for details.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.11
- [Go](https://go.dev/doc/install) >= 1.26

## Development

```sh
mise run fmt
mise run lint
mise run test       # runs Terraform against an in-process fake API
mise run generate   # formats examples and regenerates docs/

# acceptance tests against the sandbox
export MUUMUU_DOMAIN_TOKEN=muu_pat_sandbox_xxx
export MUUMUU_DOMAIN_BASE_URL=https://api-sandbox.muumuu-domain.com/api/v2
# optional: the sandbox has no domains, so DNS record tests are skipped without these
export MUUMUU_DOMAIN_TEST_DOMAIN_ID=MU00000001
export MUUMUU_DOMAIN_TEST_DOMAIN_FQDN=example.com
mise run testacc
```

## License

[MIT License](LICENSE)
