# fetch-vpn-config.sh

Fetches the best OpenVPN configs from [VPN Gate](https://www.vpngate.net/) (free academic VPN project by University of Tsukuba).

## Usage

```bash
./fetch-vpn-config.sh [REGION] [COUNT] [OUTPUT_DIR]
```

| Argument | Default | Description |
|----------|---------|-------------|
| REGION | US | Two-letter country code |
| COUNT | 10 | Number of top servers to fetch |
| OUTPUT_DIR | . | Directory to save configs |

## Examples

```bash
# Top 10 US servers (default)
./fetch-vpn-config.sh

# Top 5 Japan servers
./fetch-vpn-config.sh JP 5 ./configs

# Top 3 Germany servers to /etc/openvpn
./fetch-vpn-config.sh DE 3 /etc/openvpn
```

## Available Regions

| Code | Country |
|------|---------|
| US | United States |
| CA | Canada |
| JP | Japan |
| KR | South Korea |
| DE | Germany |
| GB | United Kingdom |
| FR | France |
| NL | Netherlands |
| AU | Australia |
| SG | Singapore |

Full list at: https://www.vpngate.net/en/

## Output

Configs are saved as `{region}-{ip}.ovpn` with the `data-ciphers` fix pre-applied for OpenVPN 2.6+ compatibility.

```
$ ./fetch-vpn-config.sh US 3 ./vpn-configs
Fetching top 3 VPN Gate servers for region: US
[50.159.130.186] Score: 520541, Speed: 103Mbps -> ./vpn-configs/us-50.159.130.186.ovpn
[24.253.99.103] Score: 754158, Speed: 37Mbps -> ./vpn-configs/us-24.253.99.103.ovpn
[67.170.115.255] Score: 488835, Speed: 89Mbps -> ./vpn-configs/us-67.170.115.255.ovpn
Done. Configs saved to ./vpn-configs/
```

## Notes

- Servers are sorted by VPN Gate score (combination of uptime, speed, and reliability)
- VPN Gate is volunteer-run; servers may go offline unexpectedly
- Configs auto-include `data-ciphers` fix for AES-128-CBC negotiation
- For production use, consider refreshing configs periodically
