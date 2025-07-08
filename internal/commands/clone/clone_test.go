package clone

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneOptions(t *testing.T) {
	t.Run("DefaultCloneOptions", func(t *testing.T) {
		opts := DefaultCloneOptions()
		assert.Equal(t, "", opts.Branch)
		assert.Equal(t, 0, opts.Depth)
		assert.False(t, opts.Bare)
		assert.False(t, opts.Mirror)
		assert.False(t, opts.Shallow)
		assert.False(t, opts.SingleBranch)
		assert.True(t, opts.Progress)
		assert.Equal(t, 10*time.Minute, opts.Timeout)
	})
}

func TestCloner(t *testing.T) {
	t.Run("NewCloner", func(t *testing.T) {
		cloner := NewCloner()
		assert.NotNil(t, cloner)
		assert.NotNil(t, cloner.auth)
	})
}

func TestInferDirectoryName(t *testing.T) {
	cloner := NewCloner()

	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"git@github.com:user/repo.git", "repo"},
		{"ssh://git@github.com/user/repo.git", "repo"},
		{"https://gitlab.com/group/project.git", "project"},
		{"", "repository"},
		{"https://github.com/user/", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := cloner.inferDirectoryName(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineDefaultBranch(t *testing.T) {
	cloner := NewCloner()

	t.Run("PreferredBranchExists", func(t *testing.T) {
		refs := map[string]string{
			"refs/heads/main":    "abc123",
			"refs/heads/develop": "def456",
		}

		result := cloner.determineDefaultBranch(refs, "develop")
		assert.Equal(t, "develop", result)
	})

	t.Run("PreferredBranchNotExists", func(t *testing.T) {
		refs := map[string]string{
			"refs/heads/main":    "abc123",
			"refs/heads/develop": "def456",
		}

		result := cloner.determineDefaultBranch(refs, "nonexistent")
		assert.Equal(t, "", result)
	})

	t.Run("MainBranchExists", func(t *testing.T) {
		refs := map[string]string{
			"refs/heads/main":    "abc123",
			"refs/heads/develop": "def456",
		}

		result := cloner.determineDefaultBranch(refs, "")
		assert.Equal(t, "main", result)
	})

	t.Run("MasterBranchExists", func(t *testing.T) {
		refs := map[string]string{
			"refs/heads/master":  "abc123",
			"refs/heads/develop": "def456",
		}

		result := cloner.determineDefaultBranch(refs, "")
		assert.Equal(t, "master", result)
	})

	t.Run("FirstBranchFound", func(t *testing.T) {
		refs := map[string]string{
			"refs/heads/feature": "abc123",
			"refs/heads/develop": "def456",
		}

		result := cloner.determineDefaultBranch(refs, "")
		assert.Contains(t, []string{"feature", "develop"}, result)
	})

	t.Run("HEADSymbolicRef", func(t *testing.T) {
		refs := map[string]string{
			"HEAD":               "abc123",
			"refs/heads/main":    "abc123",
			"refs/heads/develop": "def456",
		}

		result := cloner.determineDefaultBranch(refs, "")
		assert.Equal(t, "main", result)
	})
}

func TestCloneValidation(t *testing.T) {
	cloner := NewCloner()
	ctx := context.Background()

	t.Run("EmptyURL", func(t *testing.T) {
		opts := DefaultCloneOptions()
		opts.URL = ""

		_, err := cloner.Clone(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository URL is required")
	})

	t.Run("ExistingNonEmptyDirectory", func(t *testing.T) {
		tempDir := t.TempDir()
		existingFile := filepath.Join(tempDir, "existing.txt")
		require.NoError(t, os.WriteFile(existingFile, []byte("content"), 0644))

		opts := DefaultCloneOptions()
		opts.URL = "https://github.com/user/repo.git"
		opts.Directory = tempDir

		_, err := cloner.Clone(ctx, opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists and is not an empty directory")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		tempDir := t.TempDir()
		targetDir := filepath.Join(tempDir, "newrepo")

		opts := DefaultCloneOptions()
		opts.URL = "invalid-url"
		opts.Directory = targetDir
		opts.Timeout = 1 * time.Second

		_, err := cloner.Clone(ctx, opts)
		assert.Error(t, err)
	})
}

func TestCloneResult(t *testing.T) {
	t.Run("EmptyCloneResult", func(t *testing.T) {
		result := &CloneResult{
			FetchedRefs: make(map[string]string),
		}

		assert.NotNil(t, result.FetchedRefs)
		assert.Empty(t, result.FetchedRefs)
		assert.False(t, result.CheckedOut)
		assert.Equal(t, 0, result.ObjectCount)
		assert.Equal(t, "", result.DefaultBranch)
		assert.Equal(t, "", result.ClonedCommit)
	})
}
