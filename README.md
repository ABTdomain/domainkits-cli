[![Version](https://img.shields.io/github/v/tag/ABTdomain/domainkits-cli?style=flat-square&label=version)](https://github.com/ABTdomain/domainkits-cli/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/ABTdomain/domainkits-cli?style=flat-square&logo=go&logoColor=white)](go.mod)
[![License](https://img.shields.io/github/license/ABTdomain/domainkits-cli?style=flat-square)](LICENSE.md)

# domainkits-cli

Command-line client for the [DomainKits](https://domainkits.com) domain data API. `dk` is written in Go and ships as one binary with no dependencies.

## What it does

It puts domain data in your terminal, so AI agents and scripts can work with it one command at a time: each command returns the data as the API sends it, and the exit code says how the call went. Pick the commands you need and chain them into your own workflow:

- **Lifecycle**: [newly registered domains](https://github.com/ABTdomain/newly-registered-domains) as a search or as the daily list, [expired domains](https://github.com/ABTdomain/expired-domains) as a search or as an exported file, and deleted, aged, active and for-sale domains
- **Registration**: WHOIS data, the registrar, and what each EPP status code means
- **DNS and hosting**: DNS records, the network behind an IP, and other domains on the same nameserver
- **Certificates**: subdomains, certificates and newly seen hostnames from CT logs
- **Footprint**: the same name across TLDs, and typosquat variants around a brand
- **Trends**: keyword and TLD registration trends
- **Changes**: transfers, drops, renewals and nameserver updates on high-value domains

▶ [See how it is used](#use-it-for)

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
| `get <path>`, `post <path> <json>` | other endpoints |

Parameters for each command are in the [CLI guide](https://domainkits.com/dev/cli).

## Install

Download the archive for your platform from [Releases](https://github.com/ABTdomain/domainkits-cli/releases), or install with Go:

```bash
go install github.com/ABTdomain/domainkits-cli/cmd/dk@latest
```

Set your key and run a command:

```bash
export DOMAINKITS_API_KEY=dk_...      # PowerShell: $env:DOMAINKITS_API_KEY = "dk_..."
dk search nrds query=ai tld=com limit=20
dk whois example.com
dk ct subdomains github.com
dk help
```

Parameters after the command go to the API unchanged, as `key=value`, `--key=value` or `--key value`. The response body is printed as returned. `export=csv`, `export=txt` and `download` write a file instead. Exit code 0 is success, 1 an API error, 2 a usage or network error.

The same key works for the REST API, the SDKs, the MCP server and this CLI.

Sign up at [domainkits.com](https://domainkits.com/register) and create a key on your [dashboard](https://domainkits.com/dashboard). `dk download` works on a free account; the other commands need a plan that includes the API, see the [CLI guide](https://domainkits.com/dev/cli).

## Use it for

### [Newly registered domains](https://github.com/ABTdomain/newly-registered-domains)

```bash
dk search nrds query=ai tld=com limit=20   # search by keyword
dk download tld=com type=domains_only      # pull the daily list
```

<p align="center">
<a href="https://www.youtube.com/watch?v=l5L919wRKOI"><img src="https://img.youtube.com/vi/l5L919wRKOI/maxresdefault.jpg" alt="Video: How to find newly registered domains from your terminal with the DomainKits CLI" width="640"></a><br>
<a href="https://www.youtube.com/watch?v=l5L919wRKOI">▶ Watch on YouTube: How to Find Newly Registered Domains From Your Terminal</a>
</p>

### [Expired domains](https://github.com/ABTdomain/expired-domains)

```bash
dk search expired query=shop status=pending_delete age_min=10   # pending delete names with a long history
dk search expired tld=app status=redemption export=csv          # one TLD's redemption stage as a CSV file
```

## Related

- [ABTdomain/domainkits-mcp](https://github.com/ABTdomain/domainkits-mcp): hosted MCP server for domain data
- [ABTdomain/domainkits-sdk-py](https://github.com/ABTdomain/domainkits-sdk-py): Python client for the DomainKits domain data API
- [ABTdomain/domainkits-sdk-go](https://github.com/ABTdomain/domainkits-sdk-go): Go client for the DomainKits domain data API
- [ABTdomain/domainkits-sdk-js](https://github.com/ABTdomain/domainkits-sdk-js): TypeScript client for the DomainKits domain data API

## Resources

- [CLI guide](https://domainkits.com/dev/cli)
- [API reference and key management](https://domainkits.com/dev)
- [OpenAPI 3.0 spec](https://domainkits.com/dev/openapi.yaml)
- [About DomainKits and ABTdomain](https://domainkits.com/about)

## License

[MIT](LICENSE.md)
