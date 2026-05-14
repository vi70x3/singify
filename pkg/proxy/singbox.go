package proxy

import (
	"encoding/json"
	"os"
	"os/exec"
	"vless-openvpn-adapter/pkg/subscription"
)

type SingBoxConfig struct {
	Log       LogConfig       `json:"log"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
	Route     RouteConfig     `json:"route"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type InboundConfig struct {
	Type              string `json:"type"`
	Tag               string `json:"tag"`
	InterfaceName     string `json:"interface_name,omitempty"`
	Address           string `json:"address,omitempty"`
	AutoRoute         bool   `json:"auto_route,omitempty"`
	StrictRoute       bool   `json:"strict_route,omitempty"`
	Stack             string `json:"stack,omitempty"`
	Sniff             bool   `json:"sniff,omitempty"`
}

type OutboundConfig struct {
	Type         string `json:"type"`
	Tag          string `json:"tag"`
	Server       string `json:"server,omitempty"`
	ServerPort   int    `json:"server_port,omitempty"`
	UUID         string `json:"uuid,omitempty"`
	Flow         string `json:"flow,omitempty"`
	PacketEncoding string `json:"packet_encoding,omitempty"`
	TLS          *TLSConfig `json:"tls,omitempty"`
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
	Protocol []string `json:"protocol,omitempty"`
	Outbound string   `json:"outbound"`
}

func GenerateConfig(nodes []subscription.Node, configPath string) error {
	cfg := SingBoxConfig{
		Log: LogConfig{Level: "info"},
		Inbounds: []InboundConfig{
			{
				Type:          "tun",
				Tag:           "tun-in",
				InterfaceName: "tun_singbox",
				Address:       "172.16.0.1/30",
				AutoRoute:     false, // We will handle routing manually or via iptables
				Stack:         "system",
				Sniff:         true,
			},
		},
		Route: RouteConfig{
			Rules: []RuleConfig{
				{Outbound: "proxy"},
			},
		},
	}

	// For simplicity, we just use the first node as the proxy for now
	if len(nodes) > 0 {
		node := nodes[0]
		port := 443
		// Parse port logic would go here
		
		cfg.Outbounds = append(cfg.Outbounds, OutboundConfig{
			Type:       "vless",
			Tag:        "proxy",
			Server:     node.Host,
			ServerPort: port, // Needs proper parsing
			UUID:       node.UUID,
			TLS: &TLSConfig{
				Enabled:    true,
				ServerName: node.Host, // Simplified
			},
		})
	}

	cfg.Outbounds = append(cfg.Outbounds, OutboundConfig{
		Type: "direct",
		Tag:  "direct",
	})

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func RunSingBox(configPath string) (*exec.Cmd, error) {
	cmd := exec.Command("sing-box", "run", "-c", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, err
}
