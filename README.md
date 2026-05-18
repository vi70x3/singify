# singify

A high-performance, native Sing-box based VPN tool that routes system traffic through VLESS proxy nodes. 

This tool automates the process of fetching VLESS subscriptions, configuring Sing-box as a system-wide TUN provider, and managing necessary routing bypasses to ensure a stable, loop-free connection.

## Architecture
`vless-vpn` uses Sing-box's native `tun` inbound with the `gvisor` stack to provide system-wide proxying.

## Features
- **Native TUN Engine**: Direct system traffic handling for maximum performance.
- **Concurrent Latency Selection**: Simultaneously pings all available nodes from your subscription/file and selects the one with the lowest latency instantly.
- **Local & Remote Support**: Works with both subscription URLs and local Sing-box JSON configuration files.
- **Protocol Filtering**: Optional VLESS-only filtering (enabled by default) to ensure compatibility.
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
   # Use a subscription URL
   sudo ./run_vpn.sh "YOUR_SUBSCRIPTION_URL"
   # OR use a local JSON file
   sudo ./run_vpn.sh "config.json"
   ```

The script automatically sets the required environment variables. You can also run the binary directly with additional flags:
```bash
sudo ./vless-vpn -sub "config.json" -vless-only=true -v
```

3. Stop the VPN:
   Press `Ctrl+C`. The tool will automatically clean up the routing table and restore network state.
