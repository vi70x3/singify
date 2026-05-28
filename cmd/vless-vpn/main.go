package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"vless-openvpn-adapter/pkg/proxy"
	"vless-openvpn-adapter/pkg/subscription"
)

func main() {
	subSource := flag.String("sub", "", "Subscription URL or local JSON file path (required)")
	mode := flag.String("mode", "tun", "Operating mode: 'tun' (system-wide VPN) or 'proxy' (local SOCKS5+HTTP proxy)")
	port := flag.Int("port", 1080, "Proxy listen port (proxy mode only, auto-jumps if busy)")
	listen := flag.String("listen", "127.0.0.1", "Proxy listen address (proxy mode only)")
	vlessOnly := flag.Bool("vless-only", true, "Filter to VLESS nodes only")
	verbose := flag.Bool("v", false, "Verbose output (show sing-box logs)")

	flag.Parse()

	if *subSource == "" {
		fmt.Println("[!] Error: -sub flag is required. Provide a subscription URL or local JSON file path.")
		flag.Usage()
		os.Exit(1)
	}

	if *mode != "tun" && *mode != "proxy" {
		fmt.Printf("[!] Error: invalid mode '%s'. Must be 'tun' or 'proxy'.\n", *mode)
		flag.Usage()
		os.Exit(1)
	}

	// Step 1: Fetch or read subscription data
	fmt.Printf("[*] Loading subscription from: %s\n", *subSource)
	var data string
	var err error

	if isURL(*subSource) {
		data, err = subscription.FetchSubscription(*subSource)
	} else {
		raw, err := os.ReadFile(*subSource)
		if err != nil {
			fmt.Printf("[!] Error reading local file: %v\n", err)
			os.Exit(1)
		}
		data = string(raw)
	}

	if err != nil {
		fmt.Printf("[!] Error fetching subscription: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Parse nodes
	nodes, err := subscription.ParseLinks(data, *vlessOnly)
	if err != nil {
		fmt.Printf("[!] Error parsing nodes: %v\n", err)
		os.Exit(1)
	}

	if len(nodes) == 0 {
		fmt.Println("[!] No usable nodes found in the subscription.")
		os.Exit(1)
	}

	fmt.Printf("[*] Found %d nodes\n", len(nodes))

	// Step 3: Select fastest node
	bestNode := subscription.SelectFastestNode(nodes)
	nodes = []subscription.Node{bestNode}

	// Step 4: Create temp directory under os.TempDir() (always writable by current user)
	tempDir := filepath.Join(os.TempDir(), "singify")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Printf("[!] Error creating temp directory: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(tempDir, "sing-box-config.json")

	// Step 5: Generate config based on mode
	switch *mode {
	case "tun":
		runTUNMode(nodes, configPath, *verbose)
	case "proxy":
		runProxyMode(nodes, configPath, *listen, *port, *verbose)
	}
}

func runTUNMode(nodes []subscription.Node, configPath string, verbose bool) {
	// Detect physical device (default route interface)
	physDev, gateway, err := detectPhysicalDevice()
	if err != nil {
		fmt.Printf("[!] Error detecting physical device: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[*] Physical device: %s (gateway: %s)\n", physDev, gateway)

	// Generate TUN config
	err = proxy.GenerateTUNConfig(nodes, configPath, physDev)
	if err != nil {
		fmt.Printf("[!] Error generating TUN config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[*] Starting sing-box in TUN mode (system-wide VPN)...")

	cmd, err := proxy.RunSingBox(configPath, verbose)
	if err != nil {
		fmt.Printf("[!] Error starting sing-box: %v\n", err)
		os.Exit(1)
	}

	// Wait for Ctrl+C
	waitForInterrupt(cmd)
}

func runProxyMode(nodes []subscription.Node, configPath string, listenAddr string, requestedPort int, verbose bool) {
	// Auto-detect free port starting from requested port
	actualPort, err := proxy.FindFreePort(requestedPort)
	if err != nil {
		fmt.Printf("[!] Error finding free port: %v\n", err)
		os.Exit(1)
	}

	if actualPort != requestedPort {
		fmt.Printf("[*] Port %d is busy, auto-jumped to port %d\n", requestedPort, actualPort)
	}

	// Generate proxy config
	err = proxy.GenerateProxyConfig(nodes, configPath, listenAddr, actualPort)
	if err != nil {
		fmt.Printf("[!] Error generating proxy config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[*] Proxy listening on %s:%d (SOCKS5 + HTTP)\n", listenAddr, actualPort)
	fmt.Println("[*] Configure your applications to use this proxy address")
	fmt.Println("[*] Starting sing-box in proxy mode...")

	cmd, err := proxy.RunSingBox(configPath, verbose)
	if err != nil {
		fmt.Printf("[!] Error starting sing-box: %v\n", err)
		os.Exit(1)
	}

	// Wait for Ctrl+C
	waitForInterrupt(cmd)
}

func waitForInterrupt(cmd *exec.Cmd) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Monitor sing-box process exit in background
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- cmd.Wait()
	}()

	fmt.Println("[*] Press Ctrl+C to stop...")

	select {
	case sig := <-sigChan:
		fmt.Printf("\n[*] Received signal: %v. Shutting down...\n", sig)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
		fmt.Println("[*] sing-box stopped. Clean exit.")
	case err := <-doneChan:
		if err != nil {
			fmt.Printf("[!] sing-box exited unexpectedly: %v\n", err)
		} else {
			fmt.Println("[!] sing-box exited unexpectedly (no error).")
		}
	}
}

// detectPhysicalDevice finds the network interface and gateway for the default route.
func detectPhysicalDevice() (string, string, error) {
	if runtime.GOOS == "linux" {
		// Parse `ip route show default` output
		out, err := exec.Command("ip", "route", "show", "default").Output()
		if err != nil {
			return "", "", fmt.Errorf("failed to run 'ip route show default': %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 0 {
			return "", "", fmt.Errorf("no default route found")
		}

		// Example output: "default via 192.168.1.1 dev eth0"
		fields := strings.Fields(lines[0])
		var gateway, dev string
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "via" {
				gateway = fields[i+1]
			}
			if fields[i] == "dev" {
				dev = fields[i+1]
			}
		}

		if dev == "" {
			return "", "", fmt.Errorf("could not parse device from route output")
		}

		return dev, gateway, nil
	}

	// macOS fallback
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to detect default route: %v", err)
	}

	var gateway, dev string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if strings.HasPrefix(line, "gateway:") && len(fields) >= 2 {
			gateway = fields[1]
		}
		if strings.HasPrefix(line, "interface:") && len(fields) >= 2 {
			dev = fields[1]
		}
	}

	if dev == "" {
		return "", "", fmt.Errorf("could not parse device from route output")
	}

	return dev, gateway, nil
}

// isURL checks if the given string looks like a URL (starts with http:// or https://).
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}