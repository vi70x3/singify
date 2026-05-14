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
	DNS       DNSConfig         `json:"dns"`
	Inbounds  []InboundConfig   `json:"inbounds"`
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
	Outbound      string   `json:"outbound,omitempty"`
	Server        string   `json:"server,omitempty"`
	DisableCache  bool     `json:"disable_cache,omitempty"`
	QueryType     []string `json:"query_type,omitempty"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type InboundConfig struct {
	Type          string   `json:"type"`
	Tag           string   `json:"tag"`
	InterfaceName string   `json:"interface_name"`
	Address       []string `json:"address"`
	AutoRoute     bool     `json:"auto_route"`
	StrictRoute   bool     `json:"strict_route"`
	Stack         string   `json:"stack"`
	Sniff         bool     `json:"sniff"`
}

type OutboundConfig struct {
	Type           string     `json:"type"`
	Tag            string     `json:"tag"`
	Server         string     `json:"server,omitempty"`
	ServerPort     int        `json:"server_port,omitempty"`
	UUID           string     `json:"uuid,omitempty"`
	TLS            *TLSConfig `json:"tls,omitempty"`
}

type TLSConfig struct {
	Enabled    bool   `json:"enabled"`
	ServerName string `json:"server_name,omitempty"`
}

type RouteConfig struct {
	AutoDetectInterface bool         `json:"auto_detect_interface"`
	Rules               []RuleConfig `json:"rules"`
}

type RuleConfig struct {
	Action   string   `json:"action"`
	Protocol []string `json:"protocol,omitempty"`
	Port     int      `json:"port,omitempty"`
}

func GenerateConfig(nodes []subscription.Node, configPath string) error {
	cfg := SingBoxConfig{
		Log: LogConfig{Level: "info"},
		DNS: DNSConfig{
			Servers: []DNSServerConfig{
				{Tag: "proxy-dns", Address: "https://8.8.8.8/dns-query", Detour: "proxy"},
				{Tag: "local-dns", Address: "8.8.8.8", Detour: "direct"},
			},
			Rules: []DNSRuleConfig{
				{Outbound: "any", Server: "local-dns", DisableCache: true},
				{QueryType: []string{"A", "AAAA"}, Server: "proxy-dns"},
			},
		},
		Inbounds: []InboundConfig{
			{
				Type:          "tun",
				Tag:           "tun-in",
				InterfaceName: "tun0",
				Address:       []string{"172.19.0.1/30"},
				AutoRoute:     true,
				StrictRoute:   true,
				Stack:         "system",
				Sniff:         true,
			},
		},
		Route: RouteConfig{
			AutoDetectInterface: true,
			Rules: []RuleConfig{
				{Action: "hijack-dns", Protocol: []string{"dns"}},
				{Action: "hijack-dns", Port: 53},
			},
		},
	}

	if len(nodes) > 0 {
		node := nodes[0]
		var outboundRaw json.RawMessage

		if node.Raw != nil {
			var m map[string]interface{}
			if err := json.Unmarshal(node.Raw, &m); err == nil {
				m["tag"] = "proxy"
				outboundRaw, _ = json.Marshal(m)
			} else {
				outboundRaw = node.Raw
			}
		} else {
			manual := OutboundConfig{
				Type:       "vless",
				Tag:        "proxy",
				Server:     node.Host,
				ServerPort: 443,
				UUID:       node.UUID,
				TLS:        &TLSConfig{Enabled: true, ServerName: node.Host},
			}
			outboundRaw, _ = json.Marshal(manual)
		}
		cfg.Outbounds = append(cfg.Outbounds, outboundRaw)
	}

	cfg.Outbounds = append(cfg.Outbounds, json.RawMessage(`{"type": "direct", "tag": "direct"}`))

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func RunSingBox(configPath string, verbose bool) (*exec.Cmd, error) {
	fmt.Printf("[debug] Running sing-box: %s\n", configPath)
	cmd := exec.Command("sing-box", "run", "-c", configPath)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		logFile, _ := os.OpenFile("temp/sing-box.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	err := cmd.Start()
	return cmd, err
}
