package network

import (
	"os/exec"
)

func EnableIPForwarding() error {
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	return cmd.Run()
}

func SetupIPTables() error {
	// 1. Flush existing rules (be careful in production, but here we want a clean state for our bridge)
	// For now, just add the necessary rules.
	
	// 2. Allow forwarding between OpenVPN and Sing-box TUN
	// Assuming OpenVPN is 10.8.0.0/24 and Sing-box TUN is 172.16.0.1/30
	
	rules := [][]string{
		{"-t", "nat", "-A", "POSTROUTING", "-s", "10.8.0.0/24", "-o", "tun_singbox", "-j", "MASQUERADE"},
		{"-A", "FORWARD", "-i", "tun_ovpn", "-o", "tun_singbox", "-j", "ACCEPT"},
		{"-A", "FORWARD", "-i", "tun_singbox", "-o", "tun_ovpn", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		cmd := exec.Command("iptables", rule...)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func CleanupIPTables() error {
	rules := [][]string{
		{"-t", "nat", "-D", "POSTROUTING", "-s", "10.8.0.0/24", "-o", "tun_singbox", "-j", "MASQUERADE"},
		{"-D", "FORWARD", "-i", "tun_ovpn", "-o", "tun_singbox", "-j", "ACCEPT"},
		{"-D", "FORWARD", "-i", "tun_singbox", "-o", "tun_ovpn", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		cmd := exec.Command("iptables", rule...)
		_ = cmd.Run() // Ignore errors during cleanup
	}

	return nil
}
