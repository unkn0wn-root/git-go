package push

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

func TestPushOptions(t *testing.T) {
	t.Run("DefaultPushOptions", func(t *testing.T) {
		opts := DefaultPushOptions()
		assert.Equal(t, "origin", opts.Remote)
		assert.False(t, opts.Force)
		assert.False(t, opts.SetUpstream)
		assert.False(t, opts.PushAll)
		assert.False(t, opts.PushTags)
		assert.False(t, opts.DryRun)
		assert.Equal(t, 2*time.Minute, opts.Timeout)
	})
}

func TestRefUpdateStatus(t *testing.T) {
	tests := []struct {
		status   RefUpdateStatus
		expected string
	}{
		{RefUpdateOK, "ok"},
		{RefUpdateRejected, "rejected"},
		{RefUpdateError, "error"},
		{RefUpdateUpToDate, "up-to-date"},
		{RefUpdateFastForward, "fast-forward"},
		{RefUpdateForced, "forced"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestPusher(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("NewPusher", func(t *testing.T) {
		repo := repository.New(tempDir)
		pusher := NewPusher(repo)

		assert.NotNil(t, pusher)
		assert.Equal(t, repo, pusher.repo)
		assert.NotNil(t, pusher.auth)
	})

	t.Run("PushWithoutRepository", func(t *testing.T) {
		repo := repository.New(tempDir)
		pusher := NewPusher(repo)

		ctx := context.Background()
		opts := DefaultPushOptions()

		_, err := pusher.Push(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote 'origin' not found")
	})
}

func TestPushResult(t *testing.T) {
	t.Run("EmptyPushResult", func(t *testing.T) {
		result := &PushResult{
			UpdatedRefs:  make(map[string]RefUpdateResult),
			RejectedRefs: make(map[string]string),
		}

		assert.Empty(t, result.UpdatedRefs)
		assert.Empty(t, result.RejectedRefs)
		assert.False(t, result.FastForward)
		assert.False(t, result.Forced)
		assert.False(t, result.NewBranch)
		assert.False(t, result.UpstreamSet)
		assert.Equal(t, 0, result.PushedObjects)
		assert.Equal(t, int64(0), result.PushedSize)
	})
}

func setupTestRepositoryForPush(t *testing.T) (*repository.Repository, string) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	require.NoError(t, repo.Init())

	gitDir := filepath.Join(tempDir, ".git")
	configPath := filepath.Join(gitDir, "config")

	config := `[remote "origin"]
	url = https://github.com/test/repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`

	require.NoError(t, os.WriteFile(configPath, []byte(config), 0644))

	branchPath := filepath.Join(gitDir, "refs", "heads", "main")
	require.NoError(t, os.MkdirAll(filepath.Dir(branchPath), 0755))
	require.NoError(t, os.WriteFile(branchPath, []byte("abcdef1234567890abcdef1234567890abcdef12\n"), 0644))

	return repo, tempDir
}

func TestPusherIntegration(t *testing.T) {
	t.Run("PushToNonexistentRemote", func(t *testing.T) {
		repo, _ := setupTestRepositoryForPush(t)
		pusher := NewPusher(repo)

		ctx := context.Background()
		opts := DefaultPushOptions()
		opts.Remote = "nonexistent"

		_, err := pusher.Push(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote 'nonexistent' not found")
	})

	t.Run("PushWithTimeout", func(t *testing.T) {
		repo, _ := setupTestRepositoryForPush(t)
		pusher := NewPusher(repo)

		ctx := context.Background()
		opts := DefaultPushOptions()
		opts.Timeout = 1 * time.Millisecond

		_, err := pusher.Push(ctx, opts)
		assert.Error(t, err)
	})

	t.Run("DryRunPush", func(t *testing.T) {
		repo, _ := setupTestRepositoryForPush(t)
		pusher := NewPusher(repo)

		ctx := context.Background()
		opts := DefaultPushOptions()
		opts.DryRun = true

		_, err := pusher.Push(ctx, opts)
		assert.Error(t, err)
	})
}

func TestGetAllBranches(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)
	require.NoError(t, repo.Init())

	pusher := NewPusher(repo)

	branches, err := pusher.getAllBranches()
	assert.NoError(t, err)
	assert.Empty(t, branches)

	refsDir := filepath.Join(repo.GitDir, "refs", "heads")
	require.NoError(t, os.WriteFile(filepath.Join(refsDir, "main"), []byte("hash1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(refsDir, "develop"), []byte("hash2\n"), 0644))

	branches, err = pusher.getAllBranches()
	assert.NoError(t, err)
	assert.Len(t, branches, 2)
	assert.Contains(t, branches, "main")
	assert.Contains(t, branches, "develop")
}

func TestGetAllTags(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)
	require.NoError(t, repo.Init())

	pusher := NewPusher(repo)

	tags, err := pusher.getAllTags()
	assert.NoError(t, err)
	assert.Empty(t, tags)

	tagsDir := filepath.Join(repo.GitDir, "refs", "tags")
	require.NoError(t, os.MkdirAll(tagsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tagsDir, "v1.0.0"), []byte("hash1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tagsDir, "v2.0.0"), []byte("hash2\n"), 0644))

	tags, err = pusher.getAllTags()
	assert.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.Contains(t, tags, "v1.0.0")
	assert.Contains(t, tags, "v2.0.0")
}
