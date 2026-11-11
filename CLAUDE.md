# CLAUDE.md

## コマンド

- `mise run test` - httptest のフェイク API に対して実際の Terraform CLI でプロバイダーを動かすユニットテスト
- `mise run testacc` - サンドボックス API に対するアクセプタンステスト（`MUUMUU_DOMAIN_TOKEN` / `MUUMUU_DOMAIN_BASE_URL` / `MUUMUU_DOMAIN_TEST_DOMAIN_ID` / `MUUMUU_DOMAIN_TEST_DOMAIN_FQDN` が必要）
- `mise run lint` - golangci-lint
- `mise run generate` - `examples/` の fmt と `docs/` の再生成（CI で差分チェックあり）

## アーキテクチャ

Terraform Plugin Framework 製の Muumuu Domain 用プロバイダー。API クライアントは `github.com/kenzo0107/muumuu-domain`。

- `main.go` - エントリーポイント（registry アドレス `kenzo0107/muumuu-domain`）
- `internal/provider/provider.go` - プロバイダー設定（`token` / `base_url`）、リソースとデータソースの登録
- `internal/provider/http_client.go` - HTTP 429 のみを `Retry-After` に従ってリトライ
- `internal/provider/dns.go` - FQDN・レコード値の末尾ドット正規化
- `internal/provider/*_resource.go` / `*_data_source.go` - リソース・データソース
- `internal/provider/provider_test.go` - テスト用フェイク API

## 主要パターン

- プロバイダーの TypeName は `muumuu`。リソースは `muumuu_<name>`
- API は FQDN を末尾ドット付きで返す。`fqdn` は末尾ドットなし小文字に正規化し、`value` は設定値と末尾ドット以外一致すれば設定値を保持する
- `muumuu_nameservers` / `muumuu_domain_auto_renew` は削除 API が無いため destroy は state から外すだけ
- 新しいリソースを追加したら `examples/` を追加し `mise run generate` を実行する
