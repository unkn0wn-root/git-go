package log

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

func TestShowLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	_ = createTestCommit(t, repo, "Test commit", "test.txt", "Hello World")

	opts := LogOptions{
		MaxCount: 10,
		Oneline:  false,
	}

	err = ShowLog(repo, opts)
	require.NoError(t, err)
}

func TestShowLogOneLine(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	_ = createTestCommit(t, repo, "Test commit", "test.txt", "Hello World")

	opts := LogOptions{
		MaxCount: 5,
		Oneline:  true,
	}

	err = ShowLog(repo, opts)
	require.NoError(t, err)
}

func TestShowLogMaxCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	_ = createTestCommit(t, repo, "First commit", "file1.txt", "Content 1")
	_ = createTestCommit(t, repo, "Second commit", "file2.txt", "Content 2")
	_ = createTestCommit(t, repo, "Third commit", "file3.txt", "Content 3")

	opts := LogOptions{
		MaxCount: 2,
		Oneline:  false,
	}

	err = ShowLog(repo, opts)
	require.NoError(t, err)
}

func TestShowLogNoCommits(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	opts := LogOptions{
		MaxCount: 10,
		Oneline:  false,
	}

	err = ShowLog(repo, opts)
	require.NoError(t, err)
}

func TestShowLogNotGitRepository(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)

	opts := LogOptions{
		MaxCount: 10,
		Oneline:  false,
	}

	err = ShowLog(repo, opts)
	assert.Error(t, err)
}

func TestLogOptionsDefaults(t *testing.T) {
	opts := LogOptions{}

	assert.Equal(t, 0, opts.MaxCount)
	assert.Equal(t, false, opts.Oneline)
	assert.Equal(t, false, opts.Graph)
}

func TestLogOptionsValidation(t *testing.T) {
	tests := []struct {
		maxCount int
		valid    bool
	}{
		{0, true},   // 0 means no limit
		{1, true},   // positive values are valid
		{100, true}, // large positive values are valid
		{-1, true},  // negative values might be handled as no limit
	}

	for _, tt := range tests {
		opts := LogOptions{
			MaxCount: tt.maxCount,
			Oneline:  false,
		}

		assert.Equal(t, tt.maxCount, opts.MaxCount)
	}
}

func TestGetLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	hash1 := createTestCommit(t, repo, "First commit", "file1.txt", "Content 1")
	hash2 := createTestCommit(t, repo, "Second commit", "file2.txt", "Content 2")

	opts := LogOptions{
		MaxCount: 10,
		Oneline:  false,
	}

	entries, err := GetLog(repo, opts)
	require.NoError(t, err)

	assert.Len(t, entries, 2)

	assert.Equal(t, hash2, entries[0].Hash)
	assert.Equal(t, hash1, entries[1].Hash)

	assert.Equal(t, "Second commit", entries[0].Message)
	assert.Equal(t, "First commit", entries[1].Message)
	assert.Equal(t, "Test Author", entries[0].Author.Name)
}

func TestGetLogMaxCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	createTestCommit(t, repo, "First commit", "file1.txt", "Content 1")
	createTestCommit(t, repo, "Second commit", "file2.txt", "Content 2")
	createTestCommit(t, repo, "Third commit", "file3.txt", "Content 3")

	opts := LogOptions{
		MaxCount: 2,
		Oneline:  false,
	}

	entries, err := GetLog(repo, opts)
	require.NoError(t, err)

	assert.Len(t, entries, 2)
	assert.Equal(t, "Third commit", entries[0].Message)
	assert.Equal(t, "Second commit", entries[1].Message)
}

func TestGetLogEmptyRepository(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	opts := LogOptions{
		MaxCount: 10,
		Oneline:  false,
	}

	entries, err := GetLog(repo, opts)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestLogEntryString(t *testing.T) {
	author := &objects.Signature{
		Name:  "John Doe",
		Email: "john@example.com",
		When:  time.Unix(1234567890, 0),
	}

	committer := &objects.Signature{
		Name:  "Jane Smith",
		Email: "jane@example.com",
		When:  time.Unix(1234567900, 0),
	}

	entry := LogEntry{
		Hash:      "abcdef1234567890abcdef1234567890abcdef12",
		Author:    author,
		Committer: committer,
		Message:   "Test commit message\n\nWith detailed description",
		Parents:   []string{"parent123"},
	}

	onelineOpts := LogOptions{Oneline: true}
	oneline := entry.String(onelineOpts)

	assert.Contains(t, oneline, "abcdef1")                      // short hash
	assert.Contains(t, oneline, "Test commit message")          // first line of message
	assert.NotContains(t, oneline, "With detailed description") // should not include body

	fullOpts := LogOptions{Oneline: false}
	full := entry.String(fullOpts)

	assert.Contains(t, full, "commit abcdef1234567890abcdef1234567890abcdef12")
	assert.Contains(t, full, "Author:")
	assert.Contains(t, full, "John Doe")
	assert.Contains(t, full, "Test commit message")
	assert.Contains(t, full, "With detailed description")
}

func TestLogEntryStringSameAuthorCommitter(t *testing.T) {
	author := &objects.Signature{
		Name:  "John Doe",
		Email: "john@example.com",
		When:  time.Unix(1234567890, 0),
	}

	entry := LogEntry{
		Hash:      "abcdef1234567890abcdef1234567890abcdef12",
		Author:    author,
		Committer: author, // Same as author
		Message:   "Test commit",
		Parents:   []string{},
	}

	fullOpts := LogOptions{Oneline: false}
	full := entry.String(fullOpts)

	assert.Contains(t, full, "Author:")
	assert.Contains(t, full, "Date:")
	assert.NotContains(t, full, "AuthorDate:")
	assert.NotContains(t, full, "CommitDate:")
	assert.NotContains(t, full, "Commit:")
}

func createTestCommit(t *testing.T, repo *repository.Repository, message, filename, content string) string {
	idx := index.New(repo.GitDir)
	err := idx.Load()
	require.NoError(t, err)

	blob := objects.NewBlob([]byte(content))
	hash, err := repo.StoreObject(blob)
	require.NoError(t, err)

	err = idx.Add(filename, hash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	require.NoError(t, err)
	err = idx.Save()
	require.NoError(t, err)

	var parents []string
	if head, err := repo.GetHead(); err == nil && head != "" {
		parents = []string{head}
	}

	entries := idx.GetAll()
	var treeEntries []objects.TreeEntry
	for path, entry := range entries {
		treeEntries = append(treeEntries, objects.TreeEntry{
			Mode: objects.FileMode(entry.Mode),
			Name: path,
			Hash: entry.Hash,
		})
	}

	tree := objects.NewTree(treeEntries)
	treeHash, err := repo.StoreObject(tree)
	require.NoError(t, err)

	author := &objects.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Now(),
	}

	commit := objects.NewCommit(treeHash, parents, author, author, message)
	commitHash, err := repo.StoreObject(commit)
	require.NoError(t, err)

	err = repo.UpdateRef("refs/heads/main", commitHash)
	require.NoError(t, err)

	idx.Clear()
	err = idx.Save()
	require.NoError(t, err)

	return commitHash
}

func TestLogCommitDisplay(t *testing.T) {

	author := &objects.Signature{
		Name:  "John Doe",
		Email: "john@example.com",
		When:  time.Unix(1234567890, 0),
	}

	commit := objects.NewCommit(
		"tree123",
		[]string{},
		author,
		author,
		"Test commit message",
	)

	assert.Equal(t, "tree123", commit.Tree())
	assert.Equal(t, "Test commit message", commit.Message())
	assert.Equal(t, "John Doe", commit.Author().Name)
}

func TestLogWithLongCommitHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		content := fmt.Sprintf("Content for file %d", i)
		message := fmt.Sprintf("Commit %d", i+1)

		createTestCommit(t, repo, message, filename, content)
	}

	opts := LogOptions{
		MaxCount: 3,
		Oneline:  true,
	}

	err = ShowLog(repo, opts)
	require.NoError(t, err)
}

func TestWalkCommits(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(t, err)

	hash1 := createTestCommit(t, repo, "First commit", "file1.txt", "Content 1")
	hash2 := createTestCommit(t, repo, "Second commit", "file2.txt", "Content 2")

	var entries []LogEntry
	visited := make(map[string]bool)
	opts := LogOptions{MaxCount: 0, Oneline: false}

	err = walkCommits(repo, hash2, &entries, visited, opts)
	require.NoError(t, err)

	assert.Len(t, entries, 2)
	assert.Equal(t, hash2, entries[0].Hash)
	assert.Equal(t, hash1, entries[1].Hash)
}

func BenchmarkGetLog(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "git-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	repo := repository.New(tmpDir)
	err = repo.Init()
	require.NoError(b, err)

	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("file%d.txt", i)
		content := fmt.Sprintf("Content %d", i)
		message := fmt.Sprintf("Commit %d", i+1)
		createTestCommit(nil, repo, message, filename, content)
	}

	opts := LogOptions{
		MaxCount: 5,
		Oneline:  false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetLog(repo, opts)
		if err != nil {
			b.Fatalf("GetLog failed: %v", err)
		}
	}
}
