package proxy

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"golang.org/x/crypto/ssh"
)

const defaultImage = "ghcr.io/lunar-bedrock/socks5-vpn:latest"

type Config struct {
	// SSH connection
	Host       string // e.g., "1.2.3.4:22"
	User       string
	PrivateKey []byte // PEM-encoded private key
	
	// Container settings
	Image         string // defaults to ghcr.io/lunar-bedrock/socks5-vpn:latest
	ContainerName string
	BindIP        string // public IP for SOCKS5 bind
	ListenPort    int    // SOCKS5 port (default 1080)
	HealthPort    int    // health check port (default 8080)
	
	// Auth (optional)
	Username string
	Password string
	
	// VPN config (raw OpenVPN config content)
	VPNConfig string
}

type Manager struct {
	client *ssh.Client
}

// Connect establishes SSH connection to the host.
func Connect(host, user string, privateKey []byte) (*Manager, error) {
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	
	return &Manager{client: client}, nil
}

// ConnectWithKeyFile connects using a private key file path.
func ConnectWithKeyFile(host, user, keyPath string) (*Manager, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	return Connect(host, user, key)
}

func (m *Manager) Close() error {
	return m.client.Close()
}

// Run executes a command and returns stdout.
func (m *Manager) Run(cmd string) (string, error) {
	session, err := m.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	
	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	
	return strings.TrimSpace(stdout.String()), nil
}

// StartProxy starts a SOCKS5 VPN proxy container.
func (m *Manager) StartProxy(cfg Config) error {
	if cfg.Image == "" {
		cfg.Image = defaultImage
	}
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 1080
	}
	if cfg.HealthPort == 0 {
		cfg.HealthPort = 8080
	}
	
	// Write VPN config to temp file on remote
	if cfg.VPNConfig != "" {
		configPath := fmt.Sprintf("/tmp/%s.ovpn", cfg.ContainerName)
		escaped := strings.ReplaceAll(cfg.VPNConfig, "'", "'\\''")
		if _, err := m.Run(fmt.Sprintf("cat > %s << 'VPNEOF'\n%s\nVPNEOF", configPath, escaped)); err != nil {
			return fmt.Errorf("write vpn config: %w", err)
		}
	}
	
	// Build docker run command
	tmpl := template.Must(template.New("docker").Parse(dockerRunTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return err
	}
	
	_, err := m.Run(buf.String())
	return err
}

// StopProxy stops and removes a proxy container.
func (m *Manager) StopProxy(name string) error {
	_, _ = m.Run(fmt.Sprintf("docker stop %s 2>/dev/null", name))
	_, _ = m.Run(fmt.Sprintf("docker rm %s 2>/dev/null", name))
	return nil
}

// ProxyStatus returns the status of a proxy container.
func (m *Manager) ProxyStatus(name string) (string, error) {
	return m.Run(fmt.Sprintf("docker inspect -f '{{.State.Status}}' %s 2>/dev/null || echo 'not found'", name))
}

// ListProxies lists all socks5-vpn containers.
func (m *Manager) ListProxies() ([]string, error) {
	out, err := m.Run("docker ps -a --filter ancestor=" + defaultImage + " --format '{{.Names}}'")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

const dockerRunTemplate = `docker run -d \
  --name {{.ContainerName}} \
  --cap-add=NET_ADMIN \
  --device /dev/net/tun \
  -e SOCKS5_BIND_IP={{.BindIP}} \
  -e SOCKS5_LISTEN_ADDR=0.0.0.0:{{.ListenPort}} \
{{- if .Username}}
  -e SOCKS5_USERNAME={{.Username}} \
{{- end}}
{{- if .Password}}
  -e SOCKS5_PASSWORD={{.Password}} \
{{- end}}
{{- if .VPNConfig}}
  -v /tmp/{{.ContainerName}}.ovpn:/vpn/config.ovpn:ro \
{{- else}}
  -e DISABLE_VPN=1 \
{{- end}}
  -p {{.ListenPort}}:{{.ListenPort}}/tcp \
  -p {{.ListenPort}}:{{.ListenPort}}/udp \
  -p {{.HealthPort}}:8080/tcp \
  {{.Image}}`
