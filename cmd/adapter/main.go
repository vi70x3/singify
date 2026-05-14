package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"vless-openvpn-adapter/pkg/network"
	"vless-openvpn-adapter/pkg/proxy"
	"vless-openvpn-adapter/pkg/subscription"
	"vless-openvpn-adapter/pkg/vpn"
)

func main() {
	subURL := flag.String("sub", "", "VLESS subscription URL")
	remoteHost := flag.String("host", "127.0.0.1", "Public host/IP for OpenVPN client config")
	verbose := flag.Bool("v", false, "Show verbose logs from sing-box and openvpn")
	flag.Parse()

	if *subURL == "" {
		log.Fatal("Subscription URL is required. Use -sub <url>")
	}

	fmt.Println("--- VLESS-OpenVPN Adapter ---")

	// 1. Fetch and Parse Subscription
	fmt.Println("[*] Fetching subscription...")
	data, err := subscription.FetchSubscription(*subURL)
	if err != nil {
		log.Fatalf("Failed to fetch subscription: %v", err)
	}

	nodes, err := subscription.ParseLinks(data)
	if err != nil {
		log.Fatalf("Failed to parse links: %v", err)
	}

	if len(nodes) == 0 {
		log.Fatal("No valid VLESS nodes found in subscription")
	}
	fmt.Printf("[+] Found %d nodes. Using first node: %s\n", len(nodes), nodes[0].Remark)

	// 2. Prepare Configurations
	fmt.Println("[*] Preparing configurations...")
	os.MkdirAll("temp", 0755)
	
	sbConfig := "temp/sing-box.json"
	ovpnConfig := "temp/openvpn-server.conf"
	ovpnKey := "temp/static.key"
	ovpnStatus := "temp/openvpn-status.log"
	clientConfig := "client.ovpn"

	if err := proxy.GenerateConfig(nodes, sbConfig); err != nil {
		log.Fatalf("Failed to generate sing-box config: %v", err)
	}

	if err := vpn.GenerateStaticKey(ovpnKey); err != nil {
		log.Fatalf("Failed to generate static key: %v", err)
	}

	if err := vpn.GenerateServerConfig(ovpnKey, ovpnConfig, ovpnStatus); err != nil {
		log.Fatalf("Failed to generate openvpn config: %v", err)
	}

	if err := vpn.GenerateClientConfig(*remoteHost, ovpnKey, clientConfig); err != nil {
		log.Fatalf("Failed to generate client config: %v", err)
	}
	absPath, _ := filepath.Abs(clientConfig)
	fmt.Printf("[+] Client config generated: %s\n", absPath)

	// 3. Network Setup
	fmt.Println("[*] Setting up network (requires sudo)...")
	if err := network.EnableIPForwarding(); err != nil {
		fmt.Printf("[!] Warning: Failed to enable IP forwarding: %v\n", err)
	}
	if err := network.SetupIPTables(); err != nil {
		fmt.Printf("[!] Warning: Failed to setup iptables: %v\n", err)
	}

	// 4. Run Processes
	fmt.Println("[*] Starting processes...")
	sbCmd, err := proxy.RunSingBox(sbConfig, *verbose)
	if err != nil {
		log.Fatalf("Failed to start sing-box: %v", err)
	}
	defer sbCmd.Process.Kill()

	ovpnCmd, err := vpn.RunOpenVPN(ovpnConfig, *verbose)
	if err != nil {
		log.Fatalf("Failed to start openvpn: %v", err)
	}
	defer ovpnCmd.Process.Kill()

	fmt.Println("[+] Adapter is running!")

	// Offer to connect
	go func() {
		fmt.Print("\n[?] Do you want to connect to this adapter now? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(response)) == "y" {
			connectToSelf(clientConfig)
		}
	}()

	fmt.Println("Press Ctrl+C to stop.")

	// 5. Wait for Signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 6. Cleanup
	fmt.Println("\n[*] Cleaning up...")
	network.CleanupIPTables()
	fmt.Println("[+] Done.")
}

func connectToSelf(clientConfig string) {
	absPath, _ := filepath.Abs(clientConfig)
	cmdStr := fmt.Sprintf("sudo openvpn --config %s --allow-deprecated-insecure-static-crypto", absPath)
	
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		sudoUser = "root"
	}

	fmt.Printf("[*] Attempting to spawn client as user %s: %s\n", sudoUser, cmdStr)
	
	terminalCmd := cmdStr + "; exec bash"

	// Try common terminal emulators
	display := os.Getenv("DISPLAY")
	xauth := os.Getenv("XAUTHORITY")
	
	prefix := []string{"sudo", "-u", sudoUser, "env", "DISPLAY=" + display, "XAUTHORITY=" + xauth}
	if sudoUser == "root" {
		prefix = []string{} 
	}

	terminals := [][]string{
		{"gnome-terminal", "--", "bash", "-c", terminalCmd},
		{"konsole", "-e", "bash", "-c", terminalCmd},
		{"xfce4-terminal", "-e", "bash", "-c", terminalCmd},
		{"kitty", "bash", "-c", terminalCmd},
		{"alacritty", "-e", "bash", "-c", terminalCmd},
		{"foot", "bash", "-c", terminalCmd},
		{"xterm", "-e", "bash", "-c", terminalCmd},
	}

	for _, t := range terminals {
		fullArgs := append(prefix, t...)
		cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
		if err := cmd.Start(); err == nil {
			fmt.Printf("[+] Spawned client in %s\n", t[0])
			return
		}
	}
	
	fmt.Println("[!] Could not find a supported terminal emulator or failed to spawn. Please run the command manually in a new window:")
	fmt.Printf("    %s\n", cmdStr)
}
