#!/bin/bash

# Required environment variables for legacy compatibility
export ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true
export ENABLE_DEPRECATED_OUTBOUND_DNS_RULE_ITEM=true
export ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true

if [ -z "$1" ]; then
    echo "Usage: $0 <subscription_url | local_json_file>"
    exit 1
fi

# Run the binary
sudo -E ./vless-vpn -sub "$1" -v
