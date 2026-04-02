# SOCKS5 VPN Gateway

This service exposes a SOCKS5 proxy with UDP ASSOCIATE support and is intended to run behind a VPN tunnel inside the same container.

The intended flow is:

1. OpenVPN brings up `tun0` from a mounted `.ovpn` config.
2. The Go process starts a SOCKS5 server on `:1080` for both TCP and UDP.
3. Lumine production points its upstream RakNet dialer at one or more of these endpoints.

## Why this exists

It is difficult to find third-party SOCKS5 providers with reliable UDP ASSOCIATE support. Running our own SOCKS5 server inside a VPN-connected container keeps the proxy surface simple and lets Lumine reuse its existing upstream SOCKS5 support.

## Environment

- `SOCKS5_LISTEN_ADDR`: listen address for the SOCKS5 server. Default: `:1080`
- `SOCKS5_BIND_IP`: IP advertised in the UDP ASSOCIATE reply. Required when listening on `0.0.0.0`, `::`, or an empty host.
- `SOCKS5_USERNAME`: optional username
- `SOCKS5_PASSWORD`: optional password
- `SOCKS5_TCP_TIMEOUT_SECONDS`: TCP timeout. Default: `30`
- `SOCKS5_UDP_TIMEOUT_SECONDS`: UDP timeout. Default: `60`
- `SOCKS5_HEALTH_ADDR`: HTTP health server. Default: `:8080`
- `VPN_CONFIG_FILE`: mounted OpenVPN config path. Default: `/vpn/config.ovpn`
- `VPN_AUTH_FILE`: optional OpenVPN auth file path
- `VPN_INTERFACE`: VPN device name. Default: `tun0`
- `VPN_READY_TIMEOUT`: seconds to wait for the VPN interface. Default: `60`
- `DISABLE_VPN=1`: skip OpenVPN startup for local testing

## Docker run

```bash
docker run \
  --cap-add=NET_ADMIN \
  --device /dev/net/tun \
  -e SOCKS5_BIND_IP=10.42.0.15 \
  -v "$PWD/vpn/config.ovpn:/vpn/config.ovpn:ro" \
  -p 1080:1080/tcp \
  -p 1080:1080/udp \
  -p 8080:8080/tcp \
  socks5-vpn:latest
```

## Kubernetes notes

- Prefer stable per-pod addresses. A headless Service or explicit pod DNS names are safer than a single shared Service VIP because SOCKS5 UDP ASSOCIATE returns a concrete relay IP/port.
- Set `SOCKS5_BIND_IP` from `status.podIP`.
- The pod needs `NET_ADMIN` and access to `/dev/net/tun`.
- If you run many replicas, spread them across different VPN endpoints if you actually need egress diversity. Multiple replicas against the same VPN endpoint still share the same outward IP.

## Lumine integration

`lumine/production` now supports:

- `PRODUCTION_UPSTREAM_SOCKS5_ENDPOINTS`
- `PRODUCTION_UPSTREAM_SOCKS5_PROXY_LIST_URLS`
- `PRODUCTION_UPSTREAM_SOCKS5_USERNAME`
- `PRODUCTION_UPSTREAM_SOCKS5_PASSWORD`
- `PRODUCTION_UPSTREAM_SOCKS5_DIAL_TIMEOUT`
- `PRODUCTION_UPSTREAM_ROUTE_SELECTOR_JSON`

If endpoint lists are configured and no explicit route-selector JSON is provided, Lumine routes all `raknet` upstream dials through SOCKS5 automatically.

## VPN Gate caveat

VPN Gate is fine for experimentation, but it is a volunteer/academic network, not a production-grade commercial backbone. The official site lists OpenVPN as a supported protocol and the anti-abuse policy says connection logs are retained and disclosures can occur for abuse/legal requests:

- https://www.vpngate.net/en/about_faq.aspx
- https://www.vpngate.net/en/about_abuse.aspx

Use a controlled VPN provider if you need predictable uptime, stable egress IP assignment, or stronger operational guarantees.
