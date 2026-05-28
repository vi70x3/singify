#!/bin/bash

# Required environment variables for legacy compatibility
export ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true
export ENABLE_DEPRECATED_OUTBOUND_DNS_RULE_ITEM=true
export ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true

if [ -z "$1" ]; then
    echo "Usage: $0 <subscription_url | local_json_file> [mode] [port]"
    echo ""
    echo "Modes:"
    echo "  tun   - System-wide VPN via TUN interface (default, requires sudo)"
    echo "  proxy - Local SOCKS5+HTTP proxy (no sudo required)"
    echo ""
    echo "Examples:"
    echo "  sudo $0 \"YOUR_SUBSCRIPTION_URL\"              # TUN mode (default)"
    echo "  $0 \"YOUR_SUBSCRIPTION_URL\" proxy             # Proxy mode on port 1080"
    echo "  $0 \"YOUR_SUBSCRIPTION_URL\" proxy 8080        # Proxy mode on port 8080"
    exit 1
fi

SUB="$1"
MODE="${2:-tun}"
PORT="${3:-1080}"

if [ "$MODE" = "tun" ]; then
    # TUN mode requires sudo
    sudo -E ./vless-vpn -sub "$SUB" -mode tun -v
elif [ "$MODE" = "proxy" ]; then
    # Proxy mode does not require sudo
    ./vless-vpn -sub "$SUB" -mode proxy -port "$PORT" -v
else
    echo "Unknown mode: $MODE. Use 'tun' or 'proxy'."
    exit 1
fi