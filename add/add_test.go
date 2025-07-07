package add

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unkn0wn-root/git-go/gitignore"
	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/repository"
)

func TestAddFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("Hello World"), 0644)
	require.NoError(t, err)

	err = AddFiles(repo, []string{"test.txt"})
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	err = idx.Load()
	require.NoError(t, err)

	assert.True(t, idx.IsStaged("test.txt"))
}

func TestAddFilesNotGitRepository(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)

	err = AddFiles(repo, []string{"test.txt"})
	assert.Error(t, err)
}

func TestAddFilesNonexistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	err = AddFiles(repo, []string{"nonexistent.txt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pathspec did not match any files")
}

func TestAddDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	subDir := filepath.Join(tmpDir, "src")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	file1 := filepath.Join(subDir, "file1.txt")
	file2 := filepath.Join(subDir, "file2.txt")
	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)

	err = AddFiles(repo, []string{"src"})
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	err = idx.Load()
	require.NoError(t, err)

	assert.True(t, idx.IsStaged("src/file1.txt"))
	assert.True(t, idx.IsStaged("src/file2.txt"))
}

func TestAddCurrentDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)

	err = AddFiles(repo, []string{"."})
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	err = idx.Load()
	require.NoError(t, err)

	assert.True(t, idx.IsStaged("file1.txt"))
	assert.True(t, idx.IsStaged("file2.txt"))
}

func TestAddFileWithGitignore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err = os.WriteFile(gitignorePath, []byte("*.log\n*.tmp\n"), 0644)
	require.NoError(t, err)

	normalFile := filepath.Join(tmpDir, "normal.txt")
	logFile := filepath.Join(tmpDir, "test.log")
	tmpFile := filepath.Join(tmpDir, "temp.tmp")

	err = os.WriteFile(normalFile, []byte("normal"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(logFile, []byte("log content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(tmpFile, []byte("temp content"), 0644)
	require.NoError(t, err)

	err = AddFiles(repo, []string{"."})
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	err = idx.Load()
	require.NoError(t, err)

	assert.True(t, idx.IsStaged("normal.txt"))
	assert.False(t, idx.IsStaged("test.log"))
	assert.False(t, idx.IsStaged("temp.tmp"))
	assert.True(t, idx.IsStaged(".gitignore")) // .gitignore itself should be added
}

func TestAddFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello World")
	err = os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	gi, err := gitignore.NewGitIgnore(tmpDir)
	require.NoError(t, err)

	err = addFile(repo, idx, testFile, gi)
	require.NoError(t, err)

	assert.True(t, idx.IsStaged("test.txt"))

	entry, exists := idx.Get("test.txt")
	require.True(t, exists)
	assert.Equal(t, "test.txt", entry.Path)
	assert.Equal(t, uint32(0o100644), entry.Mode)
	assert.Equal(t, int64(len(content)), entry.Size)
}

func TestAddExecutableFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	scriptFile := filepath.Join(tmpDir, "script.sh")
	content := []byte("#!/bin/bash\necho hello")
	err = os.WriteFile(scriptFile, content, 0755)
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	gi, err := gitignore.NewGitIgnore(tmpDir)
	require.NoError(t, err)

	err = addFile(repo, idx, scriptFile, gi)
	require.NoError(t, err)

	entry, exists := idx.Get("script.sh")
	require.True(t, exists)
	assert.Equal(t, uint32(0o100755), entry.Mode)
}

func TestAddIgnoredFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err = os.WriteFile(gitignorePath, []byte("*.log\n"), 0644)
	require.NoError(t, err)

	logFile := filepath.Join(tmpDir, "test.log")
	err = os.WriteFile(logFile, []byte("log content"), 0644)
	require.NoError(t, err)

	idx := index.New(repo.GitDir)
	gi, err := gitignore.NewGitIgnore(tmpDir)
	require.NoError(t, err)

	err = addFile(repo, idx, logFile, gi)
	require.NoError(t, err)

	assert.False(t, idx.IsStaged("test.log"))
}
