package remote

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteConfig(t *testing.T) {
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	t.Run("NewRemoteConfig", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)
		assert.NotNil(t, rc)
		assert.Equal(t, gitDir, rc.gitDir)
		assert.Empty(t, rc.remotes)
	})

	t.Run("AddRemote", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)

		err := rc.AddRemote("origin", "https://github.com/user/repo.git")
		assert.NoError(t, err)

		remote, err := rc.GetRemote("origin")
		assert.NoError(t, err)
		assert.Equal(t, "origin", remote.Name)
		assert.Equal(t, "https://github.com/user/repo.git", remote.URL)
		assert.Equal(t, "https://github.com/user/repo.git", remote.FetchURL)
		assert.Equal(t, "https://github.com/user/repo.git", remote.PushURL)
	})

	t.Run("AddDuplicateRemote", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)

		err := rc.AddRemote("origin", "https://github.com/user/repo.git")
		assert.NoError(t, err)

		err = rc.AddRemote("origin", "https://github.com/user/other.git")
		assert.Error(t, err)
	})

	t.Run("RemoveRemote", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)

		err := rc.AddRemote("origin", "https://github.com/user/repo.git")
		assert.NoError(t, err)

		err = rc.RemoveRemote("origin")
		assert.NoError(t, err)

		_, err = rc.GetRemote("origin")
		assert.Error(t, err)
	})

	t.Run("RemoveNonexistentRemote", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)

		err := rc.RemoveRemote("nonexistent")
		assert.Error(t, err)
	})

	t.Run("ListRemotes", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)

		err := rc.AddRemote("origin", "https://github.com/user/repo.git")
		assert.NoError(t, err)

		err = rc.AddRemote("upstream", "https://github.com/upstream/repo.git")
		assert.NoError(t, err)

		remotes := rc.ListRemotes()
		assert.Len(t, remotes, 2)

		names := make(map[string]bool)
		for _, r := range remotes {
			names[r.Name] = true
		}

		assert.True(t, names["origin"])
		assert.True(t, names["upstream"])
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		rc := NewRemoteConfig(gitDir)

		err := rc.AddRemote("origin", "https://github.com/user/repo.git")
		assert.NoError(t, err)

		rc2 := NewRemoteConfig(gitDir)
		err = rc2.Load()
		assert.NoError(t, err)

		remote, err := rc2.GetRemote("origin")
		assert.NoError(t, err)
		assert.Equal(t, "origin", remote.Name)
		assert.Equal(t, "https://github.com/user/repo.git", remote.URL)
	})
}

func TestDetectProtocol(t *testing.T) {
	tests := []struct {
		url      string
		expected Protocol
	}{
		{"https://github.com/user/repo.git", ProtocolHTTPS},
		{"http://github.com/user/repo.git", ProtocolHTTP},
		{"git@github.com:user/repo.git", ProtocolSSH},
		{"ssh://git@github.com:user/repo.git", ProtocolSSH},
		{"git://github.com/user/repo.git", ProtocolGit},
		{"unknown://github.com/user/repo.git", ProtocolHTTPS},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			protocol := DetectProtocol(tt.url)
			assert.Equal(t, tt.expected, protocol)
		})
	}
}

func TestAuthConfig(t *testing.T) {
	t.Run("LoadAuthConfig", func(t *testing.T) {
		os.Setenv("GITHUB_TOKEN", "test-token")
		os.Setenv("GIT_USERNAME", "test-user")
		os.Setenv("GIT_PASSWORD", "test-password")

		defer func() {
			os.Unsetenv("GITHUB_TOKEN")
			os.Unsetenv("GIT_USERNAME")
			os.Unsetenv("GIT_PASSWORD")
		}()

		auth, err := LoadAuthConfig()
		assert.NoError(t, err)
		assert.Equal(t, "test-token", auth.Token)
		assert.Equal(t, "test-user", auth.Username)
		assert.Equal(t, "test-password", auth.Password)
	})
}

func TestNewHTTPTransport(t *testing.T) {
	t.Run("ValidURL", func(t *testing.T) {
		auth := &AuthConfig{
			Username: "test-user",
			Password: "test-password",
		}

		transport, err := NewHTTPTransport("https://github.com/user/repo.git", auth)
		assert.NoError(t, err)
		assert.NotNil(t, transport)
		assert.Equal(t, "test-user", transport.username)
		assert.Equal(t, "test-password", transport.password)
	})

	t.Run("InvalidURL", func(t *testing.T) {
		transport, err := NewHTTPTransport("://invalid-url", nil)
		assert.Error(t, err)
		assert.Nil(t, transport)
	})

	t.Run("TokenAuth", func(t *testing.T) {
		auth := &AuthConfig{
			Token: "test-token",
		}

		transport, err := NewHTTPTransport("https://github.com/user/repo.git", auth)
		assert.NoError(t, err)
		assert.NotNil(t, transport)
		assert.Equal(t, "test-token", transport.username)
		assert.Equal(t, "x-oauth-basic", transport.password)
	})
}

func TestNewSSHTransport(t *testing.T) {
	t.Run("ValidSSHURL", func(t *testing.T) {
		auth := &AuthConfig{
			SSHKey: "/path/to/key",
		}

		transport, err := NewSSHTransport("git@github.com:user/repo.git", auth)
		assert.NoError(t, err)
		assert.NotNil(t, transport)
		assert.Equal(t, "git", transport.user)
		assert.Equal(t, "github.com", transport.host)
		assert.Equal(t, "22", transport.port)
		assert.Equal(t, "/path/to/key", transport.key)
	})

	t.Run("SSHURLWithPort", func(t *testing.T) {
		transport, err := NewSSHTransport("ssh://git@github.com:2222/user/repo.git", nil)
		assert.NoError(t, err)
		assert.NotNil(t, transport)
		assert.Equal(t, "git", transport.user)
		assert.Equal(t, "github.com", transport.host)
		assert.Equal(t, "2222", transport.port)
	})

	t.Run("InvalidSSHURL", func(t *testing.T) {
		transport, err := NewSSHTransport("not-ssh-url", nil)
		assert.Error(t, err)
		assert.Nil(t, transport)
	})
}

// MockTransport implements the Transport interface for testing
type MockTransport struct {
	connectError   error
	refs           map[string]string
	packData       []byte
	sendPackError  error
	sendPackCalled bool
	lastRefs       map[string]RefUpdate
	lastPackData   []byte
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		refs: make(map[string]string),
	}
}

func (m *MockTransport) Connect(ctx context.Context, url string) error {
	return m.connectError
}

func (m *MockTransport) Disconnect() error {
	return nil
}

func (m *MockTransport) ListRefs(ctx context.Context) (map[string]string, error) {
	return m.refs, nil
}

func (m *MockTransport) FetchPack(ctx context.Context, wants, haves []string) (PackReader, error) {
	return &MockPackReader{data: m.packData}, nil
}

func (m *MockTransport) SendPack(ctx context.Context, refs map[string]RefUpdate, packData []byte) error {
	m.sendPackCalled = true
	m.lastRefs = refs
	m.lastPackData = packData
	return m.sendPackError
}

func (m *MockTransport) Close() error {
	return nil
}

type MockPackReader struct {
	data   []byte
	offset int
}

func (m *MockPackReader) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *MockPackReader) Close() error {
	return nil
}

func TestMockTransport(t *testing.T) {
	t.Run("MockTransportBasicFunctionality", func(t *testing.T) {
		mock := NewMockTransport()
		ctx := context.Background()

		// Test Connect
		err := mock.Connect(ctx, "test-url")
		assert.NoError(t, err)

		// Test ListRefs with empty refs
		refs, err := mock.ListRefs(ctx)
		assert.NoError(t, err)
		assert.Empty(t, refs)

		// Add some refs
		mock.refs["refs/heads/main"] = "abcdef1234567890"
		mock.refs["refs/heads/develop"] = "1234567890abcdef"

		refs, err = mock.ListRefs(ctx)
		assert.NoError(t, err)
		assert.Len(t, refs, 2)
		assert.Equal(t, "abcdef1234567890", refs["refs/heads/main"])

		// Test FetchPack
		mock.packData = []byte("test-pack-data")
		packReader, err := mock.FetchPack(ctx, []string{"abcdef1234567890"}, []string{})
		assert.NoError(t, err)
		assert.NotNil(t, packReader)

		data := make([]byte, 14)
		n, err := packReader.Read(data)
		assert.NoError(t, err)
		assert.Equal(t, 14, n)
		assert.Equal(t, "test-pack-data", string(data))

		// Test SendPack
		refUpdates := map[string]RefUpdate{
			"refs/heads/main": {
				RefName: "refs/heads/main",
				OldHash: "old-hash",
				NewHash: "new-hash",
			},
		}
		packData := []byte("pack-data-to-send")

		err = mock.SendPack(ctx, refUpdates, packData)
		assert.NoError(t, err)
		assert.True(t, mock.sendPackCalled)
		assert.Equal(t, refUpdates, mock.lastRefs)
		assert.Equal(t, packData, mock.lastPackData)
	})

	t.Run("MockTransportErrors", func(t *testing.T) {
		mock := NewMockTransport()
		mock.connectError = assert.AnError
		mock.sendPackError = assert.AnError

		ctx := context.Background()

		// Test Connect error
		err := mock.Connect(ctx, "test-url")
		assert.Error(t, err)

		// Test SendPack error
		err = mock.SendPack(ctx, map[string]RefUpdate{}, nil)
		assert.Error(t, err)
	})
}
