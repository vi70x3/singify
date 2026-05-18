# singify

A high-performance, native Sing-box based VPN tool that routes system traffic through VLESS proxy nodes. 

This tool automates the process of fetching VLESS subscriptions, configuring Sing-box as a system-wide TUN provider, and managing necessary routing bypasses to ensure a stable, loop-free connection.

## Architecture
`vless-vpn` uses Sing-box's native `tun` inbound with the `gvisor` stack to provide system-wide proxying.

## Features
- **Native TUN Engine**: Direct system traffic handling for maximum performance.
- **Latency-Based Selection**: Automatically pings all available nodes from your subscription and selects the one with the lowest latency for the fastest connection.
- **Auto-Routing**: Automatically detects the default gateway and routes transport traffic through the physical interface, preventing routing loops.
- **Stability**: Integrated keepalives and MTU optimization to ensure persistent, reset-free connections.
- **Self-Contained**: Manages its own routing table and DNS hijacking (via DoH over VLESS) to eliminate dependency on OS-level DNS setups.

## Requirements
- `sing-box` binary must be in your `PATH`.
- Must be run with `sudo` to manage network interfaces and routing tables.

## Usage

1. Build the binary:
   ```bash
   go build -o vless-vpn cmd/vless-vpn/main.go
   ```

2. Run the VPN using the provided helper script:
   ```bash
   chmod +x run_vpn.sh
   sudo ./run_vpn.sh "YOUR_SUBSCRIPTION_URL"
   ```

The script automatically sets the required environment variables:
```bash
sudo ENABLE_DEPRECATED_LEGACY_DNS_SERVERS=true \
     ENABLE_DEPRECATED_OUTBOUND_DNS_RULE_ITEM=true \
     ENABLE_DEPRECATED_MISSING_DOMAIN_RESOLVER=true \
     ./vless-vpn -sub "YOUR_SUBSCRIPTION_URL" -v
```

3. Stop the VPN:
   Press `Ctrl+C`. The tool will automatically clean up the routing table and restore network state.
