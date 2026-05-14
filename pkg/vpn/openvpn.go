package vpn

import (
	"fmt"
	"os"
	"os/exec"
)

const OpenVPNServerTemplate = `
dev tun
ifconfig 10.8.0.1 10.8.0.2
secret %s
status %s
proto tcp
port 1194
cipher AES-256-CBC
data-ciphers AES-256-CBC
data-ciphers-fallback AES-256-CBC
verb 3
`

const OpenVPNClientTemplate = `
client
dev tun
proto tcp
remote %s 1194
nobind
resolv-retry infinite
persist-key
persist-tun
cipher AES-256-CBC
data-ciphers AES-256-CBC
data-ciphers-fallback AES-256-CBC
redirect-gateway def1

# Bypass the VPN for the VLESS server and DNS
route %s 255.255.255.255 net_gateway
route 8.8.8.8 255.255.255.255 net_gateway

<secret>
%s
</secret>
`


func GenerateServerConfig(keyPath, configPath, statusPath string) error {
	content := fmt.Sprintf(OpenVPNServerTemplate, keyPath, statusPath)
	return os.WriteFile(configPath, []byte(content), 0644)
}

func GenerateStaticKey(keyPath string) error {
	cmd := exec.Command("openvpn", "--genkey", "secret", keyPath)
	return cmd.Run()
}

func GenerateClientConfig(remoteHost, proxyHost, keyPath, configPath string) error {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	content := fmt.Sprintf(OpenVPNClientTemplate, remoteHost, proxyHost, string(keyData))
	return os.WriteFile(configPath, []byte(content), 0644)
}

func RunOpenVPN(configPath string, verbose bool) (*exec.Cmd, error) {
	fmt.Printf("[debug] Running openvpn with config: %s\n", configPath)
	cmd := exec.Command("openvpn", "--config", configPath, "--allow-deprecated-insecure-static-crypto")
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		logFile, _ := os.OpenFile("temp/openvpn.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		fmt.Println("[*] OpenVPN logs redirected to temp/openvpn.log")
	}
	err := cmd.Start()
	return cmd, err
}
