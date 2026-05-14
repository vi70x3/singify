package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"vless-openvpn-adapter/pkg/proxy"
	"vless-openvpn-adapter/pkg/subscription"
)

func main() {
	subURL := flag.String("sub", "", "VLESS subscription URL")
	verbose := flag.Bool("v", false, "Show verbose logs")
	flag.Parse()

	if *subURL == "" {
		log.Fatal("Subscription URL is required. Use -sub <url>")
	}

	fmt.Println("--- VLESS Native Adapter ---")

	// 1. Fetch Subscription
	fmt.Println("[*] Fetching subscription...")
	data, err := subscription.FetchSubscription(*subURL)
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}
	nodes, err := subscription.ParseLinks(data)
	if err != nil || len(nodes) == 0 {
		log.Fatal("No valid VLESS nodes found")
	}

	// 2. Prepare Config
	os.MkdirAll("temp", 0755)
	sbConfig := "temp/sing-box.json"
	if err := proxy.GenerateConfig(nodes, sbConfig); err != nil {
		log.Fatalf("Failed to generate config: %v", err)
	}

	// 3. Routing Loop Prevention (Bypass VLESS Server)
	out, _ := exec.Command("ip", "route", "show", "default").Output()
	fields := strings.Fields(string(out))
	physDev, gwIP := "", ""
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) { physDev = fields[i+1] }
		if f == "via" && i+1 < len(fields) { gwIP = fields[i+1] }
	}

	ips, err := net.LookupIP(nodes[0].Host)
	vlessIP := nodes[0].Host
	if err == nil && len(ips) > 0 { vlessIP = ips[0].String() }

	fmt.Printf("[*] Bypassing VPN for VLESS IP: %s\n", vlessIP)
	exec.Command("ip", "route", "add", vlessIP, "via", gwIP, "dev", physDev).Run()

	// 4. Start Sing-box
	fmt.Println("[*] Starting Sing-box...")
	sbCmd, err := proxy.RunSingBox(sbConfig, *verbose)
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}
	defer sbCmd.Process.Kill()

	fmt.Println("[+] Adapter is running!")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[*] Cleaning up...")
	exec.Command("ip", "route", "del", vlessIP, "via", gwIP, "dev", physDev).Run()
	fmt.Println("[+] Done.")
}
