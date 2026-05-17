# dnsgate

A DNS proxy and filter written in Go.

## Features

- In-memory DNS cache
- Configurable upstream nameservers
- DNS-over-HTTPS (DoH) support
- Domain blocklist with automatic updates
- Hosts file integration with wildcard support
- Live statistics and query tracking
- Web UI dashboard
- REST API

## Usage

```bash
./dnsgate dnsproxy.yaml
```

## Configuration

Edit `dnsproxy.yaml` to set your upstream nameservers, blocklist sources, bind address, and API settings.
