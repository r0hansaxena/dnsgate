# dnsgate

A lightweight DNS proxy and filtering server written in Go. Sits between your devices and upstream resolvers, giving you control over what gets resolved, cached, and blocked.

## What it does

dnsgate intercepts DNS queries and handles them through a layered pipeline:

1. Checks the in-memory cache for a recent answer
2. Checks your local hosts file for custom mappings
3. Checks if the domain is on the blocklist
4. Forwards to upstream resolvers if none of the above match

Blocked domains are either returned as `0.0.0.0` (null-routed) or as `NXDOMAIN`, depending on your config. Resolved answers are cached with their original TTL to reduce repeat lookups.

## Features

- In-memory DNS cache with TTL expiration
- Domain blocklist loaded from local files or remote URLs
- Hosts file support with wildcard matching
- DNS-over-HTTPS (DoH) with fallback to standard nameservers
- Parallel upstream resolution with configurable timeout and interval
- Live query statistics with QPS tracking
- REST API for cache inspection and server control
- Web dashboard

## Installation

```bash
git clone https://github.com/r0hansaxena/dnsgate
cd dnsgate
make build
```

## Usage

```bash
sudo ./dnsgate dnsproxy.yaml
```

Needs to bind to port 53, so run with sudo or set the appropriate capability.

## Configuration

All configuration lives in `dnsproxy.yaml`. Key sections:

**DNS server**
```yaml
DNSServer:
  BindAddr: "0.0.0.0:53"
```

**Upstream resolvers**
```yaml
Resolver:
  Nameservers:
    - "1.1.1.1:53"
    - "1.0.0.1:53"
  Timeout: 5
  Interval: 200
  TTL: 600
```

**DNS-over-HTTPS**
```yaml
Resolver:
  DoH:
    Enable: true
    Endpoint: "https://cloudflare-dns.com/dns-query"
```

**Blocklist**
```yaml
Blocker:
  Enable: true
  SourceURLs:
    - Name: someads
      URL: "https://example.com/ads.txt"
  Whitelist:
    - "example.com"
```

**API server**
```yaml
APIServer:
  Enable: true
  BindAddr: "127.0.0.1:8080"
```

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/cache` | Dump full cache |
| GET | `/cache/:key` | Look up a single entry |
| DELETE | `/cache/:key` | Remove a cache entry |
| GET | `/cache/length` | Number of cached entries |
| GET | `/query/:key` | Resolve a domain and show cache state |
| GET | `/stats` | Query and domain statistics |
| GET | `/application/active` | Check if filtering is active |
| PUT | `/application/active` | Toggle filtering on or off |

## License

MIT
