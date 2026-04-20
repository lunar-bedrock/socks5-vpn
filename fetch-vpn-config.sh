#!/bin/bash
set -e

# Usage: ./fetch-vpn-config.sh [region] [count] [output_dir]
# Regions: US, CA, JP, KR, DE, GB, FR, NL, etc.

REGION="${1:-US}"
COUNT="${2:-10}"
OUTPUT_DIR="${3:-.}"

echo "Fetching top $COUNT VPN Gate servers for region: $REGION"

mkdir -p "$OUTPUT_DIR"

# Fetch and parse server list
curl -s "https://www.vpngate.net/api/iphone/" | \
  grep ",${REGION}," | \
  sort -t',' -k3 -nr | \
  head -n "$COUNT" | \
while IFS=',' read -r hostname ip score ping speed country_long country_short sessions uptime users traffic logtype operator message config_b64; do
  if [ -z "$config_b64" ] || [ "$config_b64" = "" ]; then
    echo "Skipping $ip - no config"
    continue
  fi

  # Clean and decode base64
  output_file="$OUTPUT_DIR/${REGION,,}-${ip}.ovpn"
  echo "$config_b64" | tr -d '\r\n ' | base64 -d > "$output_file" 2>/dev/null || {
    echo "Skipping $ip - invalid base64"
    continue
  }

  # Add data-ciphers fix
  if grep -q "^cipher AES-128-CBC" "$output_file" 2>/dev/null; then
    sed -i '/^cipher AES-128-CBC/a data-ciphers AES-128-CBC:AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305' "$output_file"
  fi

  speed_mbps=$((speed / 1000000))
  echo "[$ip] Score: $score, Speed: ${speed_mbps}Mbps -> $output_file"
done

echo "Done. Configs saved to $OUTPUT_DIR/"
