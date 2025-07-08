package pull

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

func TestPullOptions(t *testing.T) {
	t.Run("DefaultPullOptions", func(t *testing.T) {
		opts := DefaultPullOptions()
		assert.Equal(t, "origin", opts.Remote)
		assert.Equal(t, PullMerge, opts.Strategy)
		assert.False(t, opts.AllowUnrelated)
		assert.False(t, opts.Force)
		assert.False(t, opts.Prune)
		assert.Equal(t, 0, opts.Depth)
		assert.Equal(t, 5*time.Minute, opts.Timeout)
	})
}

func TestPuller(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("NewPuller", func(t *testing.T) {
		repo := repository.New(tempDir)
		puller := NewPuller(repo)

		assert.NotNil(t, puller)
		assert.Equal(t, repo, puller.repo)
		assert.NotNil(t, puller.auth)
	})

	t.Run("PullWithoutRepository", func(t *testing.T) {
		repo := repository.New(tempDir)
		puller := NewPuller(repo)

		ctx := context.Background()
		opts := DefaultPullOptions()

		_, err := puller.Pull(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote 'origin' not found")
	})
}

func TestPullStrategy(t *testing.T) {
	tests := []struct {
		strategy PullStrategy
		name     string
	}{
		{PullMerge, "merge"},
		{PullRebase, "rebase"},
		{PullFastForward, "fast-forward"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultPullOptions()
			opts.Strategy = tt.strategy
			assert.Equal(t, tt.strategy, opts.Strategy)
		})
	}
}

func TestPullResult(t *testing.T) {
	t.Run("EmptyPullResult", func(t *testing.T) {
		result := &PullResult{
			Strategy:      PullMerge,
			UpdatedRefs:   make(map[string]string),
			ConflictFiles: []string{},
			UpdatedFiles:  []string{},
			DeletedFiles:  []string{},
			AddedFiles:    []string{},
		}

		assert.Equal(t, PullMerge, result.Strategy)
		assert.Empty(t, result.UpdatedRefs)
		assert.Empty(t, result.ConflictFiles)
		assert.Empty(t, result.UpdatedFiles)
		assert.Empty(t, result.DeletedFiles)
		assert.Empty(t, result.AddedFiles)
		assert.Equal(t, 0, result.CommitsAhead)
		assert.Equal(t, 0, result.CommitsBehind)
		assert.False(t, result.FastForward)
	})
}

func setupTestRepository(t *testing.T) (*repository.Repository, string) {
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

	return repo, tempDir
}

func TestPullerIntegration(t *testing.T) {
	t.Run("PullFromNonexistentRemote", func(t *testing.T) {
		repo, _ := setupTestRepository(t)
		puller := NewPuller(repo)

		ctx := context.Background()
		opts := DefaultPullOptions()
		opts.Remote = "nonexistent"

		_, err := puller.Pull(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote 'nonexistent' not found")
	})

	t.Run("PullWithTimeout", func(t *testing.T) {
		repo, _ := setupTestRepository(t)
		puller := NewPuller(repo)

		ctx := context.Background()
		opts := DefaultPullOptions()
		opts.Timeout = 1 * time.Millisecond

		_, err := puller.Pull(ctx, opts)
		assert.Error(t, err)
	})
}
