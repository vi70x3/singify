package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"vless-openvpn-adapter/pkg/subscription"
)

// FindFreePort attempts to find an available TCP port starting from startPort.
// If startPort is occupied, it increments until a free port is found (up to 100 attempts, capped at 65535).
// Returns the available port number, or an error if no port is found in range.
func FindFreePort(startPort int) (int, error) {
	maxPort := startPort + 100
	if maxPort > 65535 {
		maxPort = 65535
	}
	for port := startPort; port < maxPort; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d-%d", startPort, maxPort)
}

// GenerateTUNConfig creates a sing-box configuration for TUN (system-wide VPN) mode.
func GenerateTUNConfig(nodes []subscription.Node, configPath, physDev string) error {
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes available to generate TUN config")
	}
	node := nodes[0]

	// Prepare the VLESS outbound by injecting the bind_interface
	var vlessOutbound map[string]interface{}
	if err := json.Unmarshal(node.Raw, &vlessOutbound); err != nil {
		return fmt.Errorf("failed to parse node outbound JSON: %w", err)
	}
	vlessOutbound["tag"] = "proxy"
	vlessOutbound["bind_interface"] = physDev

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"tag":     "proxy-dns",
					"address": "https://8.8.8.8/dns-query",
					"detour":  "proxy",
				},
				{
					"tag":     "local-dns",
					"address": "8.8.8.8",
					"detour":  "direct",
				},
			},
			"rules": []map[string]interface{}{
				{
					"outbound": "any",
					"server":   "local-dns",
				},
			},
		},
		"inbounds": []map[string]interface{}{
			{
				"type":           "tun",
				"tag":            "tun-in",
				"interface_name": "tun0",
				"address":        []string{"172.19.0.1/30"},
				"auto_route":     true,
				"strict_route":   true,
				"stack":          "gvisor",
			},
		},
		"outbounds": []interface{}{
			vlessOutbound,
			map[string]interface{}{
				"type":           "direct",
				"tag":            "direct",
				"bind_interface": physDev,
			},
		},
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"rules": []map[string]interface{}{
				{
					"action": "sniff", // NEW: Sniffing is now a rule action
				},
				{
					"protocol": "dns",
					"action":   "hijack-dns",
				},
				{
					"port":   53,
					"action": "hijack-dns",
				},
				{
					"domain":   []string{node.Host},
					"action":   "route",
					"outbound": "direct",
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal TUN config: %w", err)
	}
	return os.WriteFile(configPath, data, 0600)
}

// GenerateProxyConfig creates a sing-box configuration for local proxy mode.
// It uses a mixed (SOCKS5+HTTP) inbound instead of TUN, and does not bind interfaces
// since system routing is untouched.
func GenerateProxyConfig(nodes []subscription.Node, configPath string, listenAddr string, listenPort int) error {
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes available to generate proxy config")
	}
	node := nodes[0]

	// Prepare the VLESS outbound — no bind_interface in proxy mode
	var vlessOutbound map[string]interface{}
	if err := json.Unmarshal(node.Raw, &vlessOutbound); err != nil {
		return fmt.Errorf("failed to parse node outbound JSON: %w", err)
	}
	vlessOutbound["tag"] = "proxy"

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level": "info",
		},
		"dns": map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"tag":     "proxy-dns",
					"address": "https://8.8.8.8/dns-query",
					"detour":  "proxy",
				},
				{
					"tag":     "local-dns",
					"address": "8.8.8.8",
					"detour":  "direct",
				},
			},
			"rules": []map[string]interface{}{
				{
					"outbound": "any",
					"server":   "local-dns",
				},
			},
		},
		"inbounds": []map[string]interface{}{
			{
				"type":        "mixed",
				"tag":         "mixed-in",
				"listen":      listenAddr,
				"listen_port": listenPort,
			},
		},
		"outbounds": []interface{}{
			vlessOutbound,
			map[string]interface{}{
				"type": "direct",
				"tag":  "direct",
			},
		},
		"route": map[string]interface{}{
			"auto_detect_interface": true,
			"rules": []map[string]interface{}{
				{
					"action": "sniff",
				},
				{
					"protocol": "dns",
					"action":   "hijack-dns",
				},
				{
					"port":   53,
					"action": "hijack-dns",
				},
				{
					"domain":   []string{node.Host},
					"action":   "route",
					"outbound": "direct",
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy config: %w", err)
	}
	return os.WriteFile(configPath, data, 0600)
}

// GenerateConfig is a backward-compatible alias for GenerateTUNConfig.
func GenerateConfig(nodes []subscription.Node, configPath, physDev string) error {
	return GenerateTUNConfig(nodes, configPath, physDev)
}

func RunSingBox(configPath string, verbose bool) (*exec.Cmd, error) {
	fmt.Printf("[debug] Running sing-box: %s\n", configPath)
	cmd := exec.Command("sing-box", "run", "-c", configPath)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		logFile, err := os.OpenFile("temp/sing-box.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	err := cmd.Start()
	return cmd, err
}
