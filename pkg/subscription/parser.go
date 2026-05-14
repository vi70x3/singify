package subscription

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Node struct {
	Protocol string
	UUID     string
	Host     string
	Port     string
	Query    string
	Remark   string
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

func ParseLinks(data string) ([]Node, error) {
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
