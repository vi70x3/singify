package network

import (
	"os/exec"
)

func EnableIPForwarding() error {
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	return cmd.Run()
}

func SetupIPTables() error {
	// Enable IP Forwarding
	exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()

	// Force MTU on the interfaces
	exec.Command("ip", "link", "set", "dev", "tun_ovpn", "mtu", "1300").Run()
	exec.Command("ip", "link", "set", "dev", "tun_singbox", "mtu", "1300").Run()

	rules := [][]string{
		{"-t", "nat", "-A", "POSTROUTING", "-s", "10.8.0.0/24", "-o", "tun_singbox", "-j", "MASQUERADE"},
		{"-A", "FORWARD", "-i", "tun_ovpn", "-o", "tun_singbox", "-j", "ACCEPT"},
		{"-A", "FORWARD", "-i", "tun_singbox", "-o", "tun_ovpn", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"-A", "INPUT", "-p", "tcp", "--dport", "1194", "-j", "ACCEPT"},
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
		{"-D", "INPUT", "-p", "tcp", "--dport", "1194", "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		cmd := exec.Command("iptables", rule...)
		_ = cmd.Run() // Ignore errors during cleanup
	}

	return nil
}

func AddRoute(dest, dev string) error {
	cmd := exec.Command("ip", "route", "add", dest, "dev", dev)
	return cmd.Run()
}

func DelRoute(dest, dev string) error {
	cmd := exec.Command("ip", "route", "del", dest, "dev", dev)
	return cmd.Run()
}
