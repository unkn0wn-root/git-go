package remote

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/hash"
	"github.com/unkn0wn-root/git-go/repository"
	"github.com/unkn0wn-root/git-go/ssh"
)

type Remote struct {
	Name     string
	URL      string
	FetchURL string
	PushURL  string
}

type RemoteConfig struct {
	remotes map[string]*Remote
	gitDir  string
}

type Protocol int

const (
	ProtocolHTTP Protocol = iota
	ProtocolHTTPS
	ProtocolSSH
	ProtocolGit
)

type Transport interface {
	Connect(ctx context.Context, url string) error
	Disconnect() error
	ListRefs(ctx context.Context) (map[string]string, error)
	FetchPack(ctx context.Context, wants, haves []string) (PackReader, error)
	SendPack(ctx context.Context, refs map[string]RefUpdate, packData []byte) error
	Close() error
}

type PackReader interface {
	Read(p []byte) (n int, err error)
	Close() error
}

type HTTPTransport struct {
	client   *http.Client
	baseURL  *url.URL
	username string
	password string
}

type SSHTransport struct {
	sshClient *ssh.SSHClient
	conn      io.ReadWriteCloser
	host      string
	port      string
	user      string
	repo      string
	key       string
}

type AuthConfig struct {
	Username string
	Password string
	Token    string
	SSHKey   string
}

func NewRemoteConfig(gitDir string) *RemoteConfig {
	return &RemoteConfig{
		remotes: make(map[string]*Remote),
		gitDir:  gitDir,
	}
}

func (rc *RemoteConfig) Load() error {
	configPath := filepath.Join(rc.gitDir, "config")

	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.NewGitError("config", configPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentRemote *Remote

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[remote \"") && strings.HasSuffix(line, "\"]") {
			remoteName := line[9 : len(line)-2]
			currentRemote = &Remote{Name: remoteName}
			rc.remotes[remoteName] = currentRemote
		} else if currentRemote != nil {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "url":
					currentRemote.URL = value
					currentRemote.FetchURL = value
					currentRemote.PushURL = value
				case "fetch":
					currentRemote.FetchURL = value
				case "pushurl":
					currentRemote.PushURL = value
				}
			}
		}
	}

	return scanner.Err()
}

func (rc *RemoteConfig) Save() error {
	configPath := filepath.Join(rc.gitDir, "config")

	file, err := os.Create(configPath)
	if err != nil {
		return errors.NewGitError("config", configPath, err)
	}
	defer file.Close()

	for _, remote := range rc.remotes {
		fmt.Fprintf(file, "[remote \"%s\"]\n", remote.Name)
		fmt.Fprintf(file, "\turl = %s\n", remote.URL)
		if remote.FetchURL != remote.URL {
			fmt.Fprintf(file, "\tfetch = %s\n", remote.FetchURL)
		}
		if remote.PushURL != remote.URL {
			fmt.Fprintf(file, "\tpushurl = %s\n", remote.PushURL)
		}
		fmt.Fprintf(file, "\tfetch = +refs/heads/*:refs/remotes/%s/*\n", remote.Name)
		fmt.Fprintln(file)
	}

	return nil
}

func (rc *RemoteConfig) AddRemote(name, url string) error {
	if _, exists := rc.remotes[name]; exists {
		return errors.NewGitError("remote", name, fmt.Errorf("remote already exists"))
	}

	rc.remotes[name] = &Remote{
		Name:     name,
		URL:      url,
		FetchURL: url,
		PushURL:  url,
	}

	return rc.Save()
}

func (rc *RemoteConfig) RemoveRemote(name string) error {
	if _, exists := rc.remotes[name]; !exists {
		return errors.NewGitError("remote", name, fmt.Errorf("remote not found"))
	}

	delete(rc.remotes, name)
	return rc.Save()
}

func (rc *RemoteConfig) GetRemote(name string) (*Remote, error) {
	remote, exists := rc.remotes[name]
	if !exists {
		return nil, errors.NewGitError("remote", name, fmt.Errorf("remote not found"))
	}
	return remote, nil
}

func (rc *RemoteConfig) ListRemotes() []*Remote {
	var remotes []*Remote
	for _, remote := range rc.remotes {
		remotes = append(remotes, remote)
	}

	sort.Slice(remotes, func(i, j int) bool {
		return remotes[i].Name < remotes[j].Name
	})

	return remotes
}

func DetectProtocol(url string) Protocol {
	if strings.HasPrefix(url, "https://") {
		return ProtocolHTTPS
	} else if strings.HasPrefix(url, "http://") {
		return ProtocolHTTP
	} else if strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://") {
		return ProtocolSSH
	} else if strings.HasPrefix(url, "git://") {
		return ProtocolGit
	}
	return ProtocolHTTPS
}

func CreateTransport(remoteURL string, auth *AuthConfig) (Transport, error) {
	protocol := DetectProtocol(remoteURL)

	switch protocol {
	case ProtocolHTTP, ProtocolHTTPS:
		return NewHTTPTransport(remoteURL, auth)
	case ProtocolSSH:
		return NewSSHTransport(remoteURL, auth)
	default:
		return nil, fmt.Errorf("unsupported protocol for URL: %s", remoteURL)
	}
}

func NewHTTPTransport(remoteURL string, auth *AuthConfig) (*HTTPTransport, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	transport := &HTTPTransport{
		client:  client,
		baseURL: parsedURL,
	}

	if auth != nil {
		if auth.Token != "" {
			transport.username = auth.Token
			transport.password = "x-oauth-basic"
		} else if auth.Username != "" && auth.Password != "" {
			transport.username = auth.Username
			transport.password = auth.Password
		}
	}

	return transport, nil
}

func NewSSHTransport(remoteURL string, auth *AuthConfig) (*SSHTransport, error) {
	user, host, port, repo, err := ssh.ParseGitSSHURL(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH URL format: %w", err)
	}

	keyPath := ""
	if auth != nil && auth.SSHKey != "" {
		keyPath = auth.SSHKey
	}

	sshClient := ssh.NewSSHClient(host, port, user, keyPath)

	transport := &SSHTransport{
		sshClient: sshClient,
		host:      host,
		port:      port,
		user:      user,
		repo:      repo,
		key:       keyPath,
	}

	return transport, nil
}

func (t *HTTPTransport) Connect(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/info/refs?service=git-upload-pack", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if t.username != "" && t.password != "" {
		req.SetBasicAuth(t.username, t.password)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return nil
}

func (t *HTTPTransport) Disconnect() error {
	return nil
}

func (t *HTTPTransport) ListRefs(ctx context.Context) (map[string]string, error) {
	url := fmt.Sprintf("%s/info/refs?service=git-upload-pack", t.baseURL.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if t.username != "" && t.password != "" {
		req.SetBasicAuth(t.username, t.password)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list refs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return parseGitRefs(resp.Body)
}

func (t *HTTPTransport) FetchPack(ctx context.Context, wants, haves []string) (PackReader, error) {
	url := fmt.Sprintf("%s/git-upload-pack", t.baseURL.String())

	packRequest := buildPackRequest(wants, haves)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(packRequest))
	if err != nil {
		return nil, fmt.Errorf("failed to create pack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	if t.username != "" && t.password != "" {
		req.SetBasicAuth(t.username, t.password)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pack: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return resp.Body, nil
}

func (t *HTTPTransport) SendPack(ctx context.Context, refs map[string]RefUpdate, packData []byte) error {
	url := fmt.Sprintf("%s/git-receive-pack", t.baseURL.String())

	// Build complete request with refs and pack data
	var requestData bytes.Buffer

	// Add ref updates
	refData := buildPushRequest(refs)
	requestData.Write(refData)

	// Add pack data if provided
	if packData != nil {
		requestData.Write(packData)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, &requestData)
	if err != nil {
		return fmt.Errorf("failed to create push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	if t.username != "" && t.password != "" {
		req.SetBasicAuth(t.username, t.password)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return nil
}

func (t *HTTPTransport) Close() error {
	return nil
}

func (t *SSHTransport) Connect(ctx context.Context, url string) error {
	conn, err := t.sshClient.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %w", err)
	}

	// For now, just establish the connection
	_ = conn
	return nil
}

func (t *SSHTransport) Disconnect() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

func (t *SSHTransport) ListRefs(ctx context.Context) (map[string]string, error) {
	conn, err := ssh.ExecuteSSHCommand(ctx, t.host, t.port, t.user, "git-upload-pack", []string{t.repo})
	if err != nil {
		return nil, fmt.Errorf("failed to execute git-upload-pack: %w", err)
	}
	defer conn.Close()

	// Send initial packet to get refs
	_, err = conn.Write([]byte("0014command=ls-refs0001"))
	if err != nil {
		return nil, fmt.Errorf("failed to send ls-refs command: %w", err)
	}

	return parseGitRefs(conn)
}

func (t *SSHTransport) FetchPack(ctx context.Context, wants, haves []string) (PackReader, error) {
	conn, err := ssh.ExecuteSSHCommand(ctx, t.host, t.port, t.user, "git-upload-pack", []string{t.repo})
	if err != nil {
		return nil, fmt.Errorf("failed to execute git-upload-pack: %w", err)
	}

	packRequest := buildPackRequest(wants, haves)
	_, err = conn.Write(packRequest)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send pack request: %w", err)
	}

	return conn, nil
}

func (t *SSHTransport) SendPack(ctx context.Context, refs map[string]RefUpdate, packData []byte) error {
	conn, err := ssh.ExecuteSSHCommand(ctx, t.host, t.port, t.user, "git-receive-pack", []string{t.repo})
	if err != nil {
		return fmt.Errorf("failed to execute git-receive-pack: %w", err)
	}
	defer conn.Close()

	// Send ref updates
	refData := buildPushRequest(refs)
	_, err = conn.Write(refData)
	if err != nil {
		return fmt.Errorf("failed to send ref updates: %w", err)
	}

	// Send pack data if provided
	if packData != nil {
		_, err = conn.Write(packData)
		if err != nil {
			return fmt.Errorf("failed to send pack data: %w", err)
		}
	}

	return nil
}

func (t *SSHTransport) Close() error {
	return t.Disconnect()
}

func parseGitRefs(reader io.Reader) (map[string]string, error) {
	refs := make(map[string]string)

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read refs data: %w", err)
	}

	offset := 0
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}

		// Parse packet length
		lengthStr := string(data[offset : offset+4])
		length, err := strconv.ParseInt(lengthStr, 16, 32)
		if err != nil {
			break
		}

		if length == 0 {
			// Flush packet - skip and continue
			offset += 4
			continue
		}

		if offset+int(length) > len(data) {
			break
		}

		// Extract packet payload
		payload := data[offset+4 : offset+int(length)]

		// Skip service announcement
		if bytes.HasPrefix(payload, []byte("# service=")) {
			offset += int(length)
			continue
		}

		// Parse ref line: "hash refname\0capabilities" or "hash refname"
		payloadStr := string(payload)
		if strings.Contains(payloadStr, "\x00") {
			// Remove capabilities part
			payloadStr = payloadStr[:strings.Index(payloadStr, "\x00")]
		}

		// Remove trailing newline if present
		payloadStr = strings.TrimSuffix(payloadStr, "\n")

		parts := strings.SplitN(payloadStr, " ", 2)
		if len(parts) == 2 {
			hashStr := parts[0]
			refName := strings.TrimSpace(parts[1])

			if hash.ValidateHash(hashStr) {
				refs[refName] = hashStr
			}
		}

		offset += int(length)
	}

	return refs, nil
}

func buildPackRequest(wants, haves []string) []byte {
	var buf bytes.Buffer

	// Send want lines with capabilities on first want
	for i, want := range wants {
		if i == 0 {
			// First want includes capabilities
			line := fmt.Sprintf("want %s multi_ack_detailed no-done side-band-64k thin-pack ofs-delta\n", want)
			pktLine := fmt.Sprintf("%04x%s", len(line)+4, line)
			buf.WriteString(pktLine)
		} else {
			line := fmt.Sprintf("want %s\n", want)
			pktLine := fmt.Sprintf("%04x%s", len(line)+4, line)
			buf.WriteString(pktLine)
		}
	}

	// Flush packet
	buf.WriteString("0000")

	// Send have lines
	for _, have := range haves {
		line := fmt.Sprintf("have %s\n", have)
		pktLine := fmt.Sprintf("%04x%s", len(line)+4, line)
		buf.WriteString(pktLine)
	}

	// Send done
	buf.WriteString("0009done\n")

	return buf.Bytes()
}

func buildPushRequest(updates map[string]RefUpdate) []byte {
	var buf bytes.Buffer

	first := true
	for _, update := range updates {
		var line string
		if first {
			// Include capabilities on first command
			line = fmt.Sprintf("%s %s %s\x00report-status side-band-64k\n",
				update.OldHash, update.NewHash, update.RefName)
			first = false
		} else {
			line = fmt.Sprintf("%s %s %s\n",
				update.OldHash, update.NewHash, update.RefName)
		}

		pktLine := fmt.Sprintf("%04x%s", len(line)+4, line)
		buf.WriteString(pktLine)
	}

	// Flush packet
	buf.WriteString("0000")

	return buf.Bytes()
}

type RefUpdate struct {
	RefName string
	OldHash string
	NewHash string
}

func GetDefaultRemote(repo *repository.Repository) (*Remote, error) {
	rc := NewRemoteConfig(repo.GitDir)
	if err := rc.Load(); err != nil {
		return nil, err
	}

	remotes := rc.ListRemotes()
	if len(remotes) == 0 {
		return nil, errors.NewGitError("remote", "", fmt.Errorf("no remotes configured"))
	}

	for _, remote := range remotes {
		if remote.Name == "origin" {
			return remote, nil
		}
	}

	return remotes[0], nil
}

func LoadAuthConfig() (*AuthConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	auth := &AuthConfig{}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		auth.Token = token
	} else if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		auth.Token = token
	}

	if username := os.Getenv("GIT_USERNAME"); username != "" {
		auth.Username = username
	}

	if password := os.Getenv("GIT_PASSWORD"); password != "" {
		auth.Password = password
	}

	sshKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
	if _, err := os.Stat(sshKeyPath); err == nil {
		auth.SSHKey = sshKeyPath
	}

	return auth, nil
}
