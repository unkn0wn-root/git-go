package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRepository(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "src", "utils")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	gitDir := filepath.Join(projectDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name      string
		startPath string
		expected  string
		hasError  bool
	}{
		{
			name:      "from project root",
			startPath: projectDir,
			expected:  projectDir,
			hasError:  false,
		},
		{
			name:      "from subdirectory",
			startPath: subDir,
			expected:  projectDir,
			hasError:  false,
		},
		{
			name:      "from outside project",
			startPath: tmpDir,
			expected:  "",
			hasError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindRepository(tt.startPath)

			if tt.hasError {
				assert.Error(t, err)
				assert.Equal(t, "", result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFindRepositoryWithGitFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	worktreeDir := filepath.Join(tmpDir, "worktree")
	err = os.Mkdir(worktreeDir, 0755)
	require.NoError(t, err)

	gitFile := filepath.Join(worktreeDir, ".git")
	err = os.WriteFile(gitFile, []byte("gitdir: /path/to/real/git/dir"), 0644)
	require.NoError(t, err)

	result, err := FindRepository(worktreeDir)
	require.NoError(t, err)
	assert.Equal(t, worktreeDir, result)
}

func TestFindRepositoryFromCwd(t *testing.T) {
	result, err := FindRepositoryFromCwd()

	if err != nil {
		assert.Equal(t, "", result)
	} else {
		assert.NotEqual(t, "", result)
		assert.DirExists(t, result)
	}
}

func TestFindRepositoryNonExistentPath(t *testing.T) {
	nonExistentPath := "/this/path/should/not/exist/12345"

	_, err := FindRepository(nonExistentPath)
	assert.Error(t, err)
}

func TestFindRepositoryAtRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "sub", "deep", "path")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	_, err = FindRepository(subDir)
	assert.Error(t, err)
}
