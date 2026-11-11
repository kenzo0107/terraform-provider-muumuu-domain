terraform {
  required_providers {
    muumuu = {
      source = "kenzo0107/muumuu-domain"
    }
  }
}

# NOTE: token は環境変数 MUUMUU_DOMAIN_TOKEN で渡すことを推奨する
provider "muumuu" {}
