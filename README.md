# singify

A high-performance, native Sing-box based VPN tool that routes system traffic through VLESS proxy nodes. 

This tool automates the process of fetching VLESS subscriptions, configuring Sing-box, and managing necessary routing bypasses to ensure a stable, loop-free connection.

## Architecture
`vless-vpn` uses Sing-box's native `tun` inbound with the `gvisor` stack to provide system-wide proxying, or a `mixed` inbound for local SOCKS5+HTTP proxy mode.

## Features
- **Native TUN Engine**: Direct system traffic handling for maximum performance.
- **Proxy Mode**: Local SOCKS5+HTTP proxy for selective application routing (no sudo required).
- **Auto Port-Jump**: In proxy mode, automatically finds a free port if the requested one is busy.
- **Concurrent Latency Selection**: Simultaneously pings all available nodes from your subscription/file and selects the one with the lowest latency instantly.
- **Local & Remote Support**: Works with both subscription URLs and local Sing-box JSON configuration files.
- **Protocol Filtering**: Optional VLESS-only filtering (enabled by default) to ensure compatibility.
- **Auto-Routing**: Automatically detects the default gateway and routes transport traffic through the physical interface, preventing routing loops (TUN mode).
- **Stability**: Integrated keepalives and MTU optimization to ensure persistent, reset-free connections.
- **Self-Contained**: Manages its own routing table and DNS hijacking (via DoH over VLESS) to eliminate dependency on OS-level DNS setups.

## Requirements
- `sing-box` binary must be in your `PATH`.
- TUN mode must be run with `sudo` to manage network interfaces and routing tables.
- Proxy mode does NOT require `sudo`.

## Usage

1. Build the binary:
   ```bash
   go build -o vless-vpn cmd/vless-vpn/main.go
   ```

2. Run using the provided helper script:
   ```bash
   chmod +x run_vpn.sh

   # TUN mode (system-wide VPN, requires sudo)
   sudo ./run_vpn.sh "YOUR_SUBSCRIPTION_URL"
   sudo ./run_vpn.sh "config.json"

   # Proxy mode (local proxy, no sudo)
   ./run_vpn.sh "YOUR_SUBSCRIPTION_URL" proxy
   ./run_vpn.sh "YOUR_SUBSCRIPTION_URL" proxy 8080
   ```

   Or run the binary directly:
   ```bash
   # TUN mode
   sudo ./vless-vpn -sub "config.json" -mode tun -v

   # Proxy mode (auto-selects fastest node, auto-jumps port if busy)
   ./vless-vpn -sub "config.json" -mode proxy -port 1080 -v
   ```

3. Stop:
   Press `Ctrl+C`. The tool will automatically clean up the routing table and restore network state (TUN mode) or simply stop the proxy (proxy mode).

## Modes

### TUN Mode (`-mode tun`)
System-wide VPN that captures all traffic through a virtual `tun0` interface. Requires `sudo`. All applications are proxied automatically with no per-app configuration needed.

### Proxy Mode (`-mode proxy`)
Local SOCKS5+HTTP proxy on `127.0.0.1:1080` (configurable with `-port` and `-listen`). Does NOT require `sudo`. Only applications explicitly configured to use the proxy will be routed through the VLESS node.

**Auto Port-Jump**: If the requested port is already in use, the tool automatically increments until a free port is found and prints the actual port to stdout.

**Configuring applications to use the proxy:**
- **curl**: `curl --proxy socks5h://127.0.0.1:1080 https://example.com`
- **wget**: `wget -e use_proxy=yes -e https_proxy=127.0.0.1:1080 https://example.com`
- **SSH**: `ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:1080 %h %p" user@host`
- **Environment variables**: `export ALL_PROXY=socks5h://127.0.0.1:1080`
- **Browser**: Configure SOCKS5 proxy in browser settings (127.0.0.1:1080)

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-sub` | (required) | Subscription URL or local JSON file path |
| `-mode` | `tun` | Operating mode: `tun` or `proxy` |
| `-port` | `1080` | Proxy listen port (proxy mode only) |
| `-listen` | `127.0.0.1` | Proxy listen address (proxy mode only) |
| `-vless-only` | `true` | Filter to VLESS nodes only |
| `-v` | `false` | Verbose output (show sing-box logs) |
