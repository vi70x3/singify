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
        subPath := flag.String("sub", "", "VLESS subscription URL or local JSON file path")
        verbose := flag.Bool("v", false, "Show verbose logs")
        vlessOnly := flag.Bool("vless-only", true, "Only include VLESS nodes")
        flag.Parse()

        if *subPath == "" {
                log.Fatal("Subscription URL or file path is required. Use -sub <url|path>")
        }

        fmt.Println("--- VLESS Native VPN ---")

        var data string
        var err error

        // 1. Fetch Subscription or Read File
        if _, err := os.Stat(*subPath); err == nil {
                content, err := os.ReadFile(*subPath)
                if err != nil {
                        log.Fatalf("Failed to read file: %v", err)
                }
                data = string(content)
        } else {
                fmt.Println("[*] Fetching subscription...")
                data, err = subscription.FetchSubscription(*subPath)
                if err != nil {
                        log.Fatalf("Failed to fetch subscription: %v", err)
                }
        }

        fmt.Println("[*] Parsing nodes...")
        nodes, err := subscription.ParseLinks(data, *vlessOnly)
        if err != nil || len(nodes) == 0 {
                log.Fatal("No valid nodes found")
        }
        fmt.Printf("[*] Found %d nodes (vless-only: %v)\n", len(nodes), *vlessOnly)

        node := subscription.SelectFastestNode(nodes)

	// 3. Routing Loop Prevention (Bypass VLESS Server)
	out, _ := exec.Command("ip", "route", "show", "default").Output()
	fields := strings.Fields(string(out))
	physDev, gwIP := "", ""
	for i, f := range fields {
	        if f == "dev" && i+1 < len(fields) { physDev = fields[i+1] }
	        if f == "via" && i+1 < len(fields) { gwIP = fields[i+1] }
	}

	// 2. Prepare Config
	os.MkdirAll("temp", 0755)
	sbConfig := "temp/sing-box.json"
	if err := proxy.GenerateConfig([]subscription.Node{node}, sbConfig, physDev); err != nil {
	        log.Fatalf("Failed to generate config: %v", err)
	}

	ips, err := net.LookupIP(node.Host)
	vlessIP := node.Host
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
