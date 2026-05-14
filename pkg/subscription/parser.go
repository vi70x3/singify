package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Node struct {
	Protocol string
	UUID     string
	Host     string
	Port     string
	Query    string
	Remark   string
	Raw      json.RawMessage
}

func FetchSubscription(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// Ping calculates the latency to a host in milliseconds.
// Returns a large number (1 hour) if unreachable.
func (n *Node) Ping() time.Duration {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(n.Host, n.Port), 2*time.Second)
	if err != nil {
		return time.Hour
	}
	defer conn.Close()
	return time.Since(start)
}

// SelectFastestNode picks the node with the lowest latency.
func SelectFastestNode(nodes []Node) Node {
	type nodeLatency struct {
		node    Node
		latency time.Duration
	}

	var results []nodeLatency
	for _, node := range nodes {
		latency := node.Ping()
		results = append(results, nodeLatency{node, latency})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].latency < results[j].latency
	})

	fmt.Printf("[*] Selected node: %s (latency: %v)\n", results[0].node.Remark, results[0].latency)
	return results[0].node
}

type singBoxConfig struct {
	Outbounds []json.RawMessage `json:"outbounds"`
}

type outboundFull struct {
	Type   string `json:"type"`
	Tag    string `json:"tag"`
	Server string `json:"server"`
}

func ParseLinks(data string) ([]Node, error) {
	// Try parsing as Sing-box JSON first
	var sbCfg singBoxConfig
	if err := json.Unmarshal([]byte(data), &sbCfg); err == nil && len(sbCfg.Outbounds) > 0 {
		var nodes []Node
		for _, raw := range sbCfg.Outbounds {
			var out outboundFull
			if err := json.Unmarshal(raw, &out); err == nil {
				// Only include actual proxy protocols
				if out.Type == "vless" || out.Type == "trojan" || out.Type == "hysteria2" || out.Type == "vmess" {
					nodes = append(nodes, Node{
						Protocol: out.Type,
						Remark:   out.Tag,
						Host:     out.Server,
						Raw:      raw,
					})
				}
			}
		}
		if len(nodes) > 0 {
			return nodes, nil
		}
	}

	// Try to decode Base64
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		// If not base64, assume it's raw links
		decoded = []byte(data)
	}

	links := strings.Split(string(decoded), "\n")
	var nodes []Node

	for _, link := range links {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}

		node, err := parseLink(link)
		if err == nil {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func parseLink(link string) (Node, error) {
	if strings.HasPrefix(link, "vless://") {
		return parseVLess(link)
	}
	// Add more protocols if needed (vmess, trojan, etc.)
	return Node{}, fmt.Errorf("unsupported protocol")
}

func parseVLess(link string) (Node, error) {
	// vless://uuid@host:port?query#remark
	u := strings.TrimPrefix(link, "vless://")
	
	remarkParts := strings.SplitN(u, "#", 2)
	remark := ""
	if len(remarkParts) > 1 {
		remark = remarkParts[1]
	}
	
	mainPart := remarkParts[0]
	queryParts := strings.SplitN(mainPart, "?", 2)
	query := ""
	if len(queryParts) > 1 {
		query = queryParts[1]
	}
	
	addrPart := queryParts[0]
	authParts := strings.SplitN(addrPart, "@", 2)
	if len(authParts) != 2 {
		return Node{}, fmt.Errorf("invalid vless link")
	}
	
	uuid := authParts[0]
	hostPort := authParts[1]
	
	hpParts := strings.SplitN(hostPort, ":", 2)
	if len(hpParts) != 2 {
		return Node{}, fmt.Errorf("invalid host:port")
	}
	
	return Node{
		Protocol: "vless",
		UUID:     uuid,
		Host:     hpParts[0],
		Port:     hpParts[1],
		Query:    query,
		Remark:   remark,
	}, nil
}
