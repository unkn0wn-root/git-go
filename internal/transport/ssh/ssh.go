package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	defaultSSHPort    = "22"
	defaultSSHTimeout = 10 * time.Second
	defaultGitUser    = "git"
	sshProtocol       = "tcp"
	sshURLPrefix      = "ssh://"
	sshDirName        = ".ssh"
	sshCommand        = "ssh"

	// SSH key file names
	keyRSA     = "id_rsa"
	keyED25519 = "id_ed25519"
	keyECDSA   = "id_ecdsa"

	// env variables
	sshAuthSock = "SSH_AUTH_SOCK"
	unixNetwork = "unix"
)

type SSHClient struct {
	client  *ssh.Client
	host    string
	port    string
	user    string
	keyPath string
}

func NewSSHClient(host, port, user, keyPath string) *SSHClient {
	return &SSHClient{
		host:    host,
		port:    port,
		user:    user,
		keyPath: keyPath,
	}
}

func (c *SSHClient) Connect(ctx context.Context) (*SSHConnection, error) {
	var authMethods []ssh.AuthMethod
	if agentAuth := c.tryAgentAuth(); agentAuth != nil {
		authMethods = append(authMethods, agentAuth)
	}

	if c.keyPath != "" {
		keyAuth, err := c.keyFileAuth(c.keyPath)
		if err == nil {
			authMethods = append(authMethods, keyAuth)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKeys := []string{keyRSA, keyED25519, keyECDSA}
		for _, keyName := range defaultKeys {
			keyPath := filepath.Join(homeDir, sshDirName, keyName)
			if keyAuth, err := c.keyFileAuth(keyPath); err == nil {
				authMethods = append(authMethods, keyAuth)
			}
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no valid authentication methods found")
	}

	config := &ssh.ClientConfig{
		User:            c.user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // todo: Implement proper host key verification
		Timeout:         defaultSSHTimeout,
	}

	addr := net.JoinHostPort(c.host, c.port)
	client, err := ssh.Dial(sshProtocol, addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	return &SSHConnection{
		client: client,
	}, nil
}

func (c *SSHClient) tryAgentAuth() ssh.AuthMethod {
	agentConn, err := net.Dial(unixNetwork, os.Getenv(sshAuthSock))
	if err != nil {
		return nil
	}

	agentClient := agent.NewClient(agentConn)
	return ssh.PublicKeysCallback(agentClient.Signers)
}

func (c *SSHClient) keyFileAuth(keyPath string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

type SSHConnection struct {
	conn   net.Conn
	client *ssh.Client
}

func (conn *SSHConnection) ExecuteGitCommand(ctx context.Context, command string, args []string) (io.ReadWriteCloser, error) {
	cmdLine := fmt.Sprintf("%s %s", command, strings.Join(args, " "))

	session, err := conn.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := session.Start(cmdLine); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return &sshStream{
		session: session,
		stdin:   stdin,
		stdout:  stdout,
	}, nil
}

func (conn *SSHConnection) Close() error {
	if conn.client != nil {
		return conn.client.Close()
	}
	return nil
}

type sshStream struct {
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

func (s *sshStream) Read(p []byte) (n int, err error) {
	return s.stdout.Read(p)
}

func (s *sshStream) Write(p []byte) (n int, err error) {
	return s.stdin.Write(p)
}

func (s *sshStream) Close() error {
	s.stdin.Close()
	return s.session.Close()
}

// fallback to using ssh command if crypto/ssh doesn't work
func ExecuteSSHCommand(ctx context.Context, host, port, user, command string, args []string) (io.ReadWriteCloser, error) {
	sshArgs := []string{
		"-p", port,
		fmt.Sprintf("%s@%s", user, host),
		command,
	}
	sshArgs = append(sshArgs, args...)

	cmd := exec.CommandContext(ctx, sshCommand, sshArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start SSH command: %w", err)
	}

	return &cmdStream{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}, nil
}

type cmdStream struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
}

func (c *cmdStream) Read(p []byte) (n int, err error) {
	return c.stdout.Read(p)
}

func (c *cmdStream) Write(p []byte) (n int, err error) {
	return c.stdin.Write(p)
}

func (c *cmdStream) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

func ParseGitSSHURL(url string) (user, host, port, repo string, err error) {
	// handle git@host:repo format
	if strings.Contains(url, "@") && strings.Contains(url, ":") && !strings.HasPrefix(url, sshURLPrefix) {
		parts := strings.SplitN(url, "@", 2)
		if len(parts) != 2 {
			return "", "", "", "", fmt.Errorf("invalid SSH URL format")
		}
		user = parts[0]

		hostRepo := parts[1]
		colonIdx := strings.Index(hostRepo, ":")
		if colonIdx == -1 {
			return "", "", "", "", fmt.Errorf("invalid SSH URL format")
		}

		host = hostRepo[:colonIdx]
		repo = hostRepo[colonIdx+1:]
		port = defaultSSHPort
		return
	}

	// handle ssh://user@host:port/repo format
	if strings.HasPrefix(url, sshURLPrefix) {
		url = strings.TrimPrefix(url, sshURLPrefix)

		parts := strings.SplitN(url, "@", 2)
		if len(parts) == 2 {
			user = parts[0]
			url = parts[1]
		}

		parts = strings.SplitN(url, "/", 2)
		if len(parts) != 2 {
			return "", "", "", "", fmt.Errorf("invalid SSH URL format")
		}

		hostPort := parts[0]
		repo = parts[1]

		if strings.Contains(hostPort, ":") {
			hostPortParts := strings.SplitN(hostPort, ":", 2)
			host = hostPortParts[0]
			port = hostPortParts[1]
		} else {
			host = hostPort
			port = defaultSSHPort
		}

		if user == "" {
			user = defaultGitUser
		}

		return
	}

	return "", "", "", "", fmt.Errorf("unsupported SSH URL format: %s", url)
}
