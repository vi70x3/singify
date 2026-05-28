package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"vless-openvpn-adapter/pkg/subscription"
)

func createTestNode() subscription.Node {
	raw := json.RawMessage(`{
		"type": "vless",
		"tag": "test-node",
		"server": "1.2.3.4",
		"server_port": 443,
		"uuid": "00000000-0000-0000-0000-000000000000"
	}`)
	return subscription.Node{
		Protocol: "vless",
		Remark:   "test-node",
		Host:     "1.2.3.4",
		Port:     "443",
		Raw:      raw,
	}
}

func TestFindFreePort(t *testing.T) {
	// FindFreePort should return a usable port
	port, err := FindFreePort(1080)
	if err != nil {
		t.Fatalf("FindFreePort failed: %v", err)
	}
	if port < 1080 || port >= 1180 {
		t.Errorf("Expected port in range 1080-1179, got %d", port)
	}

	// Verify the port is actually free by listening on it
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Port %d returned by FindFreePort is not actually free: %v", port, err)
	}
	ln.Close()
}

func TestFindFreePortWithBusyPort(t *testing.T) {
	// Occupy port 10800 to force auto-jump
	ln, err := net.Listen("tcp", "127.0.0.1:10800")
	if err != nil {
		t.Skipf("Could not occupy port 10800: %v", err)
	}
	defer ln.Close()

	port, err := FindFreePort(10800)
	if err != nil {
		t.Fatalf("FindFreePort failed with busy starting port: %v", err)
	}

	// Should have jumped to at least 10801
	if port <= 10800 {
		t.Errorf("Expected port > 10800 due to auto-jump, got %d", port)
	}
}

func TestGenerateProxyConfig(t *testing.T) {
	nodes := []subscription.Node{createTestNode()}
	configPath := "temp/test-proxy-config.json"

	os.MkdirAll("temp", 0755)

	err := GenerateProxyConfig(nodes, configPath, "127.0.0.1", 1080)
	if err != nil {
		t.Fatalf("GenerateProxyConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse generated config JSON: %v", err)
	}

	// Verify inbound is mixed type
	inbounds, ok := cfg["inbounds"].([]interface{})
	if !ok {
		t.Fatal("Config has no inbounds array")
	}
	inbound := inbounds[0].(map[string]interface{})
	if inbound["type"] != "mixed" {
		t.Errorf("Expected inbound type 'mixed', got '%v'", inbound["type"])
	}
	if inbound["listen"] != "127.0.0.1" {
		t.Errorf("Expected listen '127.0.0.1', got '%v'", inbound["listen"])
	}
	if inbound["listen_port"] != float64(1080) {
		t.Errorf("Expected listen_port 1080, got '%v'", inbound["listen_port"])
	}

	// Verify outbound does NOT have bind_interface
	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok {
		t.Fatal("Config has no outbounds array")
	}
	vlessOut := outbounds[0].(map[string]interface{})
	if _, exists := vlessOut["bind_interface"]; exists {
		t.Error("Proxy mode VLESS outbound should NOT have bind_interface")
	}
	if vlessOut["tag"] != "proxy" {
		t.Errorf("Expected outbound tag 'proxy', got '%v'", vlessOut["tag"])
	}

	directOut := outbounds[1].(map[string]interface{})
	if _, exists := directOut["bind_interface"]; exists {
		t.Error("Proxy mode direct outbound should NOT have bind_interface")
	}
}

func TestGenerateTUNConfig(t *testing.T) {
	nodes := []subscription.Node{createTestNode()}
	configPath := "temp/test-tun-config.json"

	os.MkdirAll("temp", 0755)

	err := GenerateTUNConfig(nodes, configPath, "eth0")
	if err != nil {
		t.Fatalf("GenerateTUNConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse generated config JSON: %v", err)
	}

	// Verify inbound is tun type
	inbounds, ok := cfg["inbounds"].([]interface{})
	if !ok {
		t.Fatal("Config has no inbounds array")
	}
	inbound := inbounds[0].(map[string]interface{})
	if inbound["type"] != "tun" {
		t.Errorf("Expected inbound type 'tun', got '%v'", inbound["type"])
	}

	// Verify outbound HAS bind_interface
	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok {
		t.Fatal("Config has no outbounds array")
	}
	vlessOut := outbounds[0].(map[string]interface{})
	if vlessOut["bind_interface"] != "eth0" {
		t.Errorf("Expected bind_interface 'eth0', got '%v'", vlessOut["bind_interface"])
	}
}