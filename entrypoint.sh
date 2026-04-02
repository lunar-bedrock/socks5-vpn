#!/bin/sh
set -eu

VPN_INTERFACE="${VPN_INTERFACE:-tun0}"
VPN_CONFIG_FILE="${VPN_CONFIG_FILE:-/vpn/config.ovpn}"
VPN_AUTH_FILE="${VPN_AUTH_FILE:-}"
VPN_READY_TIMEOUT="${VPN_READY_TIMEOUT:-60}"
DISABLE_VPN="${DISABLE_VPN:-0}"

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
fi

/usr/local/bin/socks5-vpn &
SOCKS5_PID=$!
wait "$SOCKS5_PID"
