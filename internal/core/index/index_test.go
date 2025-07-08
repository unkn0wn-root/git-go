package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	gitDir := "/tmp/test/.git"
	idx := New(gitDir)

	assert.Equal(t, gitDir, idx.gitDir)
	assert.NotNil(t, idx.entries)
	assert.Empty(t, idx.entries)
}

func TestAdd(t *testing.T) {
	idx := New("/tmp/test/.git")

	path := "test.txt"
	hash := "abc123def456789012345678901234567890abcd"
	mode := uint32(0o100644)
	size := int64(100)
	modTime := time.Now()

	err := idx.Add(path, hash, mode, size, modTime)
	require.NoError(t, err)

	entry, exists := idx.Get(path)
	require.True(t, exists)
	assert.Equal(t, path, entry.Path)
	assert.Equal(t, hash, entry.Hash)
	assert.Equal(t, mode, entry.Mode)
	assert.Equal(t, size, entry.Size)
	assert.True(t, entry.Staged)
}

func TestAddInvalidHash(t *testing.T) {
	idx := New("/tmp/test/.git")

	err := idx.Add("test.txt", "invalid", 0o100644, 100, time.Now())
	assert.Error(t, err)
}

func TestGet(t *testing.T) {
	idx := New("/tmp/test/.git")

	_, exists := idx.Get("nonexistent.txt")
	assert.False(t, exists)

	path := "test.txt"
	hash := "abc123def456789012345678901234567890abcd"
	err := idx.Add(path, hash, 0o100644, 100, time.Now())
	require.NoError(t, err)

	entry, exists := idx.Get(path)
	assert.True(t, exists)
	assert.Equal(t, path, entry.Path)
	assert.Equal(t, hash, entry.Hash)
}

func TestRemove(t *testing.T) {
	idx := New("/tmp/test/.git")

	err := idx.Remove("nonexistent.txt")
	assert.Error(t, err)

	path := "test.txt"
	hash := "abc123def456789012345678901234567890abcd"
	err = idx.Add(path, hash, 0o100644, 100, time.Now())
	require.NoError(t, err)

	err = idx.Remove(path)
	require.NoError(t, err)

	_, exists := idx.Get(path)
	assert.False(t, exists)
}

func TestIsStaged(t *testing.T) {
	idx := New("/tmp/test/.git")

	path := "test.txt"

	assert.False(t, idx.IsStaged(path))

	hash := "abc123def456789012345678901234567890abcd"
	err := idx.Add(path, hash, 0o100644, 100, time.Now())
	require.NoError(t, err)

	assert.True(t, idx.IsStaged(path))
}

func TestHasChanges(t *testing.T) {
	idx := New("/tmp/test/.git")

	assert.False(t, idx.HasChanges())

	hash := "abc123def456789012345678901234567890abcd"
	err := idx.Add("test.txt", hash, 0o100644, 100, time.Now())
	require.NoError(t, err)

	assert.True(t, idx.HasChanges())
}

func TestClear(t *testing.T) {
	idx := New("/tmp/test/.git")

	hash := "abc123def456789012345678901234567890abcd"
	err := idx.Add("test.txt", hash, 0o100644, 100, time.Now())
	require.NoError(t, err)

	assert.True(t, idx.HasChanges())

	idx.Clear()

	assert.False(t, idx.HasChanges())
	assert.Empty(t, idx.entries)
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	gitDir := filepath.Join(tmpDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)

	idx := New(gitDir)

	hash1 := "abc123def456789012345678901234567890abcd"
	hash2 := "def456abc7890123456789012345678901abcdef"

	err = idx.Add("file1.txt", hash1, 0o100644, 100, time.Now())
	require.NoError(t, err)

	err = idx.Add("file2.sh", hash2, 0o100755, 200, time.Now())
	require.NoError(t, err)

	err = idx.Save()
	require.NoError(t, err)

	idx2 := New(gitDir)
	err = idx2.Load()
	require.NoError(t, err)

	entry1, exists := idx2.Get("file1.txt")
	assert.True(t, exists)
	assert.Equal(t, hash1, entry1.Hash)
	assert.Equal(t, uint32(0o100644), entry1.Mode)

	entry2, exists := idx2.Get("file2.sh")
	assert.True(t, exists)
	assert.Equal(t, hash2, entry2.Hash)
	assert.Equal(t, uint32(0o100755), entry2.Mode)
}
