package vpn

import (
	"fmt"
	"os"
	"os/exec"
)

const OpenVPNServerTemplate = `
dev tun_ovpn
ifconfig 10.8.0.1 10.8.0.2
secret %s
port 1194
proto udp
status %s
verb 3
`

const OpenVPNClientTemplate = `
remote %s 1194
dev tun
ifconfig 10.8.0.2 10.8.0.1
secret %s
proto udp
redirect-gateway def1
`

func GenerateServerConfig(keyPath, configPath, statusPath string) error {
	content := fmt.Sprintf(OpenVPNServerTemplate, keyPath, statusPath)
	return os.WriteFile(configPath, []byte(content), 0644)
}

func GenerateStaticKey(keyPath string) error {
	cmd := exec.Command("openvpn", "--genkey", "secret", keyPath)
	return cmd.Run()
}

func GenerateClientConfig(remoteHost, keyPath, configPath string) error {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	content := fmt.Sprintf(OpenVPNClientTemplate, remoteHost, string(keyData))
	return os.WriteFile(configPath, []byte(content), 0644)
}

func RunOpenVPN(configPath string) (*exec.Cmd, error) {
	cmd := exec.Command("openvpn", "--config", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, err
}
