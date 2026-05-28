# Proxy Mode Architecture Plan

## Overview

Add a **proxy mode** to `vless-vpn` that runs sing-box as a local SOCKS5/HTTP proxy instead of a system-wide TUN VPN. This mode:
- Does NOT require `sudo` or routing table changes
- Exposes a local proxy endpoint (default `127.0.0.1:1080`)
- Applications connect explicitly by configuring their proxy settings
- Auto-selects the fastest node using the existing `SelectFastestNode()` logic

## Current vs. Proposed Architecture

```mermaid
flowchart TD
    subgraph Current TUN Mode
        A1[Subscription URL] --> B1[ParseLinks]
        B1 --> C1[SelectFastestNode]
        C1 --> D1[GenerateConfig - TUN inbound]
        D1 --> E1[sing-box run - requires sudo]
        E1 --> F1[System-wide VPN via tun0]
    end

    subgraph New Proxy Mode
        A2[Subscription URL] --> B2[ParseLinks]
        B2 --> C2[SelectFastestNode]
        C2 --> D2[GenerateProxyConfig - mixed inbound]
        D2 --> E2[sing-box run - no sudo needed]
        E2 --> F2[Local proxy on 127.0.0.1:1080]
    end
```

## Sing-box Config Differences

### TUN Mode Config (current)
- **Inbound**: `tun` type → creates `tun0` interface, `auto_route`, `strict_route`
- **Outbounds**: VLESS with `bind_interface=physDev`, direct with `bind_interface=physDev`
- **DNS**: DoH over proxy + local DNS fallback
- **Route**: sniff, DNS hijack, direct route for proxy server domain
- **Requires**: sudo, routing table manipulation

### Proxy Mode Config (new)
- **Inbound**: `mixed` type → SOCKS5 + HTTP proxy on `127.0.0.1:1080`
- **Outbounds**: VLESS without `bind_interface`, direct without `bind_interface`
- **DNS**: DoH over proxy for DNS resolution through the proxy tunnel
- **Route**: sniff, DNS hijack, direct route for proxy server domain
- **Requires**: no sudo, no routing changes

### Proxy Mode sing-box JSON structure
```json
{
  "log": { "level": "info" },
  "dns": {
    "servers": [
      { "tag": "proxy-dns", "address": "https://8.8.8.8/dns-query", "detour": "proxy" },
      { "tag": "local-dns", "address": "8.8.8.8", "detour": "direct" }
    ],
    "rules": [
      { "outbound": "any", "server": "local-dns" }
    ]
  },
  "inbounds": [
    {
      "type": "mixed",
      "tag": "mixed-in",
      "listen": "127.0.0.1",
      "listen_port": 1080
    }
  ],
  "outbounds": [
    { ... VLESS outbound with tag=proxy, NO bind_interface ... },
    { "type": "direct", "tag": "direct" }
  ],
  "route": {
    "auto_detect_interface": true,
    "rules": [
      { "action": "sniff" },
      { "protocol": "dns", "action": "hijack-dns" },
      { "port": 53, "action": "hijack-dns" },
      { "domain": ["proxy-server-host"], "action": "route", "outbound": "direct" }
    ]
  }
}
```

## Implementation Plan

### 1. `pkg/proxy/singbox.go` — Add `GenerateProxyConfig()`

Create a new function `GenerateProxyConfig(nodes []subscription.Node, configPath string, listenAddr string, listenPort int) error` that:
- Takes the fastest node from `nodes[0]`
- Builds a sing-box config with `mixed` inbound instead of `tun`
- Does NOT inject `bind_interface` on outbounds
- Accepts configurable listen address and port (defaults: `127.0.0.1`, `1080`)
- Writes the config JSON to `configPath`

Also add a helper `FindFreePort(startPort int) int` in `pkg/proxy/` that:
- Attempts to listen on `startPort` via `net.Listen`
- If it fails (port busy), increments and tries `startPort+1`, `startPort+2`, etc.
- Returns the first available port
- Used by `main.go` before calling `GenerateProxyConfig()` so the config always uses a free port

Also rename the existing `GenerateConfig()` to `GenerateTUNConfig()` for clarity, or keep both names.

### 2. `cmd/vless-vpn/main.go` — Create with mode flag

Create the main entry point with these flags:
- `-sub string` — Subscription URL or local JSON file path (required)
- `-mode string` — Operating mode: `tun` (default) or `proxy`
- `-port int` — Proxy listen port (default `1080`, only used in proxy mode)
- `-listen string` — Proxy listen address (default `127.0.0.1`, only used in proxy mode)
- `-vless-only bool` — Filter to VLESS nodes only (default `true`)
- `-v bool` — Verbose output

Main flow:
1. Parse flags
2. Fetch or read subscription data
3. Parse nodes with `ParseLinks()`
4. Select fastest node with `SelectFastestNode()`
5. If mode=`tun`: detect physical device, call `GenerateTUNConfig()`, run with sudo requirements
6. If mode=`proxy`: call `FindFreePort()` to auto-detect an available port starting from the requested port, then call `GenerateProxyConfig()` with the resolved port, print the actual port to stdout, run sing-box (no sudo needed)
7. Wait for Ctrl+C, cleanup

### 3. `run_vpn.sh` — Update to support proxy mode

Add a second argument or flag for mode selection:
```bash
# Usage examples:
sudo ./run_vpn.sh "SUBSCRIPTION_URL"              # TUN mode (default)
./run_vpn.sh "SUBSCRIPTION_URL" proxy              # Proxy mode (no sudo)
./run_vpn.sh "SUBSCRIPTION_URL" proxy 1080         # Proxy mode with custom port
```

### 4. `README.md` — Document proxy mode

Add a section explaining:
- What proxy mode is and when to use it
- How to run it
- How to configure applications to use the proxy
- Differences from TUN mode

### 5. Tests — Add test for `GenerateProxyConfig()`

Add a test in `pkg/proxy/singbox_test.go` that:
- Creates mock nodes
- Calls `GenerateProxyConfig()`
- Verifies the generated JSON has `mixed` inbound, no `bind_interface`, correct port

## Key Design Decisions

1. **Default mode is TUN** — preserves backward compatibility; existing users unaffected
2. **Proxy mode uses `mixed` inbound** — supports both SOCKS5 and HTTP CONNECT in one listener
3. **No `bind_interface` in proxy mode** — system routing is untouched; traffic to the proxy server goes through normal system routes
4. **DNS still routes through proxy** — ensures DNS queries don't leak when applications use the proxy
5. **Configurable port/address** — allows users to change the proxy endpoint if needed
6. **`SelectFastestNode()` is reused** — same auto-selection logic works for both modes
7. **Auto port-jump** — if the default listen port is occupied, automatically increments until a free port is found; prints the actual port being used to stdout