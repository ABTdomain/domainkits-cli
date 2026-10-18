# domainkits-cli

Command-line client for the [DomainKits](https://domainkits.com) REST API.

This is the official command-line client for the DomainKits API, published and maintained by the DomainKits team. DomainKits is built and operated by Lyalpha GmbH, with domain data and infrastructure provided by [ABTdomain](https://abtdomain.com), our domain intelligence and data aggregation platform. This repository is hosted under the ABTdomain GitHub organisation. Learn more about the relationship at [domainkits.com/about](https://domainkits.com/about).

Every parameter, response field and current limit is documented in the [API reference](https://domainkits.com/dev/api-docs) and the [OpenAPI spec](https://domainkits.com/dev/openapi.yaml). This README only lists what the CLI covers. One binary, no dependencies.

## Requirements

The REST API is for Premium and Platinum accounts; unauthenticated requests are rejected with 401. Keys start with `dk_` and come from [domainkits.com](https://domainkits.com/pricing). The same key works for the REST API, the SDKs, the MCP server and this CLI.

## Install

Download the archive for your platform from [Releases](https://github.com/ABTdomain/domainkits-cli/releases), or install with Go:

```bash
go install github.com/ABTdomain/domainkits-cli/cmd/dk@latest
```

## Usage

```bash
export DOMAINKITS_API_KEY=dk_...      # PowerShell: $env:DOMAINKITS_API_KEY = "dk_..."
dk search nrds query=ai tld=com limit=20
dk whois example.com
dk ct subdomains github.com
dk help
```

Parameters after the command go to the API unchanged, as `key=value`, `--key=value` or `--key value`. The response body is printed as returned. `export=csv`, `export=txt` and `download` write a file instead. Exit code 0 is success, 1 an API error, 2 a usage or network error.

## Commands

| Command | Endpoint |
|---|---|
| `search <type>` | `/search/nrds` · `/search/aged` · `/search/active` · `/search/expired` · `/search/deleted` · `/search/market` · `/search/hostname` · `/search/status` |
| `whois <domain>` | `/whois` |
| `dns <domain>` | `/dns` |
| `bulk whois <domains>` | `/bulk/whois` |
| `bulk dns <domains>` | `/bulk/dns` |
| `ns-reverse <nameservers>` | `/ns-reverse` |
| `ip-lookup <ip or domain>` | `/ip-lookup` |
| `registrar <name or IANA ID>` | `/registrar` |
| `status-guide <code>` | `/status-guide` |
| `tld-check <prefix>` | `/tld-check` |
| `typosquat <domain>` | `/typosquat` |
| `ct subdomains <domain>` | `/ct/subdomains` |
| `ct certs <domain>` | `/ct/certs` |
| `trends tlds <newly or active>` | `/trends/tlds/*` |
| `trends keywords <hot, emerging or prefix>` | `/trends/keywords/*` |
| `monitor` | `/monitor/changes` |
| `download` | `/nrds/download` |
| `usage` | `/usage` |
| `health` | `/health` |
| `get <path>`, `post <path> <json>` | any endpoint |

## Resources

- [API reference and key management](https://domainkits.com/dev)
- [OpenAPI 3.0 spec](https://domainkits.com/dev/openapi.yaml)
- [About DomainKits and ABTdomain](https://domainkits.com/about)

## License

[MIT](LICENSE.md)
