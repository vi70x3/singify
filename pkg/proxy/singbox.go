package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"vless-openvpn-adapter/pkg/subscription"
)

type SingBoxConfig struct {
	Log       LogConfig         `json:"log"`
	DNS       *DNSConfig        `json:"dns,omitempty"`
	Inbounds  []InboundConfig    `json:"inbounds"`
	Outbounds []json.RawMessage `json:"outbounds"`
	Route     RouteConfig       `json:"route"`
}

type DNSConfig struct {
	Servers []DNSServerConfig `json:"servers"`
	Rules   []DNSRuleConfig   `json:"rules"`
}

type DNSServerConfig struct {
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	Detour   string `json:"detour,omitempty"`
}

type DNSRuleConfig struct {
	Server string   `json:"server,omitempty"`
	Domain []string `json:"domain,omitempty"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type InboundConfig struct {
	Type          string   `json:"type"`
	Tag           string   `json:"tag"`
	InterfaceName string   `json:"interface_name,omitempty"`
	Address       []string `json:"address,omitempty"`
	AutoRoute     bool     `json:"auto_route,omitempty"`
	StrictRoute   bool     `json:"strict_route,omitempty"`
	Stack         string   `json:"stack,omitempty"`
}

type OutboundConfig struct {
	Type           string     `json:"type"`
	Tag            string     `json:"tag"`
	Server         string     `json:"server,omitempty"`
	ServerPort     int        `json:"server_port,omitempty"`
	UUID           string     `json:"uuid,omitempty"`
	Flow           string     `json:"flow,omitempty"`
	PacketEncoding string     `json:"packet_encoding,omitempty"`
	TLS            *TLSConfig `json:"tls,omitempty"`
	BindInterface  string     `json:"bind_interface,omitempty"`
}

type TLSConfig struct {
	Enabled    bool   `json:"enabled"`
	ServerName string `json:"server_name,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"`
}

type RouteConfig struct {
	Rules []RuleConfig `json:"rules"`
}

type RuleConfig struct {
	Action   string   `json:"action,omitempty"`
	Protocol []string `json:"protocol,omitempty"`
	Outbound string   `json:"outbound,omitempty"`
	Domain   []string `json:"domain,omitempty"`
}

func GenerateConfig(nodes []subscription.Node, configPath, physDev string) error {
	cfg := SingBoxConfig{
		Log: LogConfig{Level: "info"},
		DNS: &DNSConfig{
			Servers: []DNSServerConfig{
				{
					Tag:     "dns-direct",
					Address: "8.8.8.8",
					Detour:  "direct",
				},
			},
			Rules: []DNSRuleConfig{
				{
					Server: "dns-direct",
					Domain: []string{},
				},
			},
		},
		Inbounds: []InboundConfig{
			{
				Type:          "tun",
				Tag:           "tun-in",
				InterfaceName: "tun_singbox",
				Address:       []string{"172.16.0.2/30"},
				AutoRoute:     false,
				Stack:         "gvisor",
			},
		},
		Route: RouteConfig{
			Rules: []RuleConfig{
				{Action: "sniff"},
				{
					Protocol: []string{"dns"},
					Action:   "hijack-dns",
				},
				{
					Outbound: "direct",
					Domain:   []string{},
				},
				{
					Outbound: "proxy",
				},
			},
		},
	}

	// Use the first node as the proxy
	if len(nodes) > 0 {
		node := nodes[0]
		var outboundRaw json.RawMessage

		if node.Host != "" {
			cfg.DNS.Rules[0].Domain = append(cfg.DNS.Rules[0].Domain, node.Host)
			cfg.Route.Rules[2].Domain = append(cfg.Route.Rules[2].Domain, node.Host)
		}

		if node.Raw != nil {
			// Override tag to "proxy" so our routing rule works
			var m map[string]interface{}
			if err := json.Unmarshal(node.Raw, &m); err == nil {
				m["tag"] = "proxy"
				m["bind_interface"] = physDev
				outboundRaw, _ = json.Marshal(m)
			} else {
				outboundRaw = node.Raw
			}
		} else {
			// Fallback to manual construction (for vless:// links)
			port := 443
			manual := OutboundConfig{
				Type:          "vless",
				Tag:           "proxy",
				Server:        node.Host,
				ServerPort:    port,
				UUID:          node.UUID,
				TLS:           &TLSConfig{Enabled: true, ServerName: node.Host},
				BindInterface: physDev,
			}
			outboundRaw, _ = json.Marshal(manual)
		}
		cfg.Outbounds = append(cfg.Outbounds, outboundRaw)
	}

	direct := OutboundConfig{
		Type:          "direct",
		Tag:           "direct",
		BindInterface: physDev,
	}
	directRaw, _ := json.Marshal(direct)
	cfg.Outbounds = append(cfg.Outbounds, directRaw)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func RunSingBox(configPath string, verbose bool) (*exec.Cmd, error) {
	fmt.Printf("[debug] Running sing-box with config: %s\n", configPath)
	cmd := exec.Command("sing-box", "run", "-c", configPath)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		logFile, _ := os.OpenFile("temp/sing-box.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		fmt.Println("[*] Sing-box logs redirected to temp/sing-box.log")
	}
	err := cmd.Start()
	return cmd, err
}
