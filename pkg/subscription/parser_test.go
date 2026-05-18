package subscription

import (
	"testing"
)

func TestParseLinks_SingBoxJSON(t *testing.T) {
	jsonData := `{
		"outbounds": [
			{
				"type": "vless",
				"tag": "test-node",
				"server": "1.2.3.4",
				"server_port": 443,
				"uuid": "00000000-0000-0000-0000-000000000000"
			}
		]
	}`

	nodes, err := ParseLinks(jsonData, true)
	if err != nil {
		t.Fatalf("ParseLinks failed: %v", err)
	}

	if len(nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(nodes))
	}

	if nodes[0].Protocol != "vless" {
		t.Errorf("Expected protocol vless, got %s", nodes[0].Protocol)
	}

	if nodes[0].Remark != "test-node" {
		t.Errorf("Expected remark test-node, got %s", nodes[0].Remark)
	}

	if nodes[0].Host != "1.2.3.4" {
		t.Errorf("Expected host 1.2.3.4, got %s", nodes[0].Host)
	}
}

func TestFetchSubscriptionTimeout(t *testing.T) {
	// Use a non-routable IP to simulate a timeout
	_, err := FetchSubscription("http://10.255.255.1")
	if err == nil {
		t.Error("Expected error due to timeout, got nil")
	}
}
