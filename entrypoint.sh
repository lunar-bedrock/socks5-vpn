#!/bin/sh
set -eu

VPN_INTERFACE="${VPN_INTERFACE:-tun0}"
VPN_CONFIG_FILE="${VPN_CONFIG_FILE:-/vpn/config.ovpn}"
VPN_AUTH_FILE="${VPN_AUTH_FILE:-}"
VPN_READY_TIMEOUT="${VPN_READY_TIMEOUT:-60}"
DISABLE_VPN="${DISABLE_VPN:-0}"

# Setup policy routing to handle asymmetric routing when VPN is enabled.
# Ensures that incoming connections via eth0 get replies via eth0, not via VPN.
setup_policy_routing() {
  # Get eth0 gateway
  ETH0_GW=$(ip route show default dev eth0 2>/dev/null | awk '{print $3}')
  if [ -z "$ETH0_GW" ]; then
    ETH0_GW="172.17.0.1"
    echo "Warning: Could not determine eth0 gateway, using default: $ETH0_GW"
  fi

  echo "Setting up policy routing with eth0 gateway: $ETH0_GW"

  # Add default route to table 100 (using numeric ID to avoid rt_tables)
  ip route add default via "$ETH0_GW" dev eth0 table 100 2>/dev/null || true

  # Mark incoming connections on eth0 and use connmark to persist
  iptables -t mangle -A PREROUTING -i eth0 -j CONNMARK --set-mark 1
  iptables -t mangle -A OUTPUT -m connmark --mark 1 -j MARK --set-mark 1

  # Use marked packets with table 100
  ip rule add fwmark 1 table 100 priority 100 2>/dev/null || true

  # Verify
  echo "IP rules:"
  ip rule list
  echo "Table 100 routes:"
  ip route show table 100

  echo "Policy routing configured"
}

wait_for_vpn() {
  i=0
  while [ "$i" -lt "$VPN_READY_TIMEOUT" ]; do
    if ip link show "$VPN_INTERFACE" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  return 1
}

cleanup() {
  if [ -n "${SOCKS5_PID:-}" ]; then
    kill "$SOCKS5_PID" 2>/dev/null || true
    wait "$SOCKS5_PID" 2>/dev/null || true
  fi
  if [ -n "${OPENVPN_PID:-}" ]; then
    kill "$OPENVPN_PID" 2>/dev/null || true
    wait "$OPENVPN_PID" 2>/dev/null || true
  fi
}

trap cleanup INT TERM

if [ "$DISABLE_VPN" != "1" ]; then
  if [ ! -f "$VPN_CONFIG_FILE" ]; then
    echo "VPN config file not found: $VPN_CONFIG_FILE" >&2
    exit 1
  fi

  OPENVPN_ARGS="--config $VPN_CONFIG_FILE --dev $VPN_INTERFACE --auth-nocache"
  if [ -n "$VPN_AUTH_FILE" ]; then
    if [ ! -f "$VPN_AUTH_FILE" ]; then
      echo "VPN auth file not found: $VPN_AUTH_FILE" >&2
      exit 1
    fi
    OPENVPN_ARGS="$OPENVPN_ARGS --auth-user-pass $VPN_AUTH_FILE"
  fi

  # shellcheck disable=SC2086
  openvpn $OPENVPN_ARGS &
  OPENVPN_PID=$!

  if ! wait_for_vpn; then
    echo "VPN interface $VPN_INTERFACE did not become ready within ${VPN_READY_TIMEOUT}s" >&2
    cleanup
    exit 1
  fi

  # Setup policy routing after VPN is connected
  setup_policy_routing

  # Fix DNS to use public DNS (Docker DNS not reachable through VPN)
  echo "nameserver 8.8.8.8" > /etc/resolv.conf
  echo "nameserver 8.8.4.4" >> /etc/resolv.conf
fi

/usr/local/bin/socks5-vpn &
SOCKS5_PID=$!
wait "$SOCKS5_PID"
