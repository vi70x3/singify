package vpn

import (
	"fmt"
	"os"
	"os/exec"
)

const OpenVPNServerTemplate = `
dev tun_ovpn
proto tcp-server
lport 1194
ifconfig 10.8.0.1 10.8.0.2
secret %s
status %s
cipher AES-256-CBC
data-ciphers AES-256-CBC
data-ciphers-fallback AES-256-CBC
verb 3
`

const OpenVPNClientTemplate = `
dev tun
proto tcp-client
remote 127.0.0.1 1194
ifconfig 10.8.0.2 10.8.0.1
nobind
persist-key
persist-tun
cipher AES-256-CBC
data-ciphers AES-256-CBC
data-ciphers-fallback AES-256-CBC
redirect-gateway def1

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

func GenerateClientConfig(keyPath, configPath string) error {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	content := fmt.Sprintf(OpenVPNClientTemplate, string(keyData))
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
