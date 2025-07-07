package blame

import (
	"bytes"
	"testing"
	"time"

	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

type MockRepository struct {
	objects map[string]objects.Object
	head    string
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		objects: make(map[string]objects.Object),
	}
}

func (m *MockRepository) LoadObject(hash string) (objects.Object, error) {
	if obj, exists := m.objects[hash]; exists {
		return obj, nil
	}
	return nil, &MockError{message: "object not found: " + hash}
}

func (m *MockRepository) GetHead() (string, error) {
	if m.head == "" {
		return "", &MockError{message: "no HEAD found"}
	}
	return m.head, nil
}

func (m *MockRepository) AddObject(hash string, obj objects.Object) {
	m.objects[hash] = obj
}

func (m *MockRepository) SetHead(hash string) {
	m.head = hash
}

type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}

type MockCommit struct {
	hash    string
	tree    string
	parents []string
	author  *objects.Signature
	data    []byte
}

func (c *MockCommit) Hash() string                  { return c.hash }
func (c *MockCommit) SetHash(h string)              { c.hash = h }
func (c *MockCommit) Tree() string                  { return c.tree }
func (c *MockCommit) Parents() []string             { return c.parents }
func (c *MockCommit) Author() *objects.Signature    { return c.author }
func (c *MockCommit) Committer() *objects.Signature { return c.author } // Use same as author for simplicity
func (c *MockCommit) Message() string               { return "Test commit message" }
func (c *MockCommit) Type() objects.ObjectType      { return objects.ObjectTypeCommit }
func (c *MockCommit) Size() int64                   { return int64(len(c.data)) }
func (c *MockCommit) Data() []byte                  { return c.data }

type MockTree struct {
	entries []objects.TreeEntry
	hash    string
	data    []byte
}

func (t *MockTree) Entries() []objects.TreeEntry { return t.entries }
func (t *MockTree) Hash() string                 { return t.hash }
func (t *MockTree) SetHash(h string)             { t.hash = h }
func (t *MockTree) Type() objects.ObjectType     { return objects.ObjectTypeTree }
func (t *MockTree) Size() int64                  { return int64(len(t.data)) }
func (t *MockTree) Data() []byte                 { return t.data }

type MockBlob struct {
	content []byte
	hash    string
	data    []byte
}

func (b *MockBlob) Content() []byte          { return b.content }
func (b *MockBlob) Hash() string             { return b.hash }
func (b *MockBlob) SetHash(h string)         { b.hash = h }
func (b *MockBlob) Type() objects.ObjectType { return objects.ObjectTypeBlob }
func (b *MockBlob) Size() int64              { return int64(len(b.content)) }
func (b *MockBlob) Data() []byte             { return b.data }

func createRepositoryWrapper(mock *MockRepository) repositoryInterface {
	return &repositoryWrapper{mock: mock}
}

type repositoryInterface interface {
	LoadObject(hash string) (objects.Object, error)
	GetHead() (string, error)
}

type repositoryWrapper struct {
	mock *MockRepository
}

func (w *repositoryWrapper) LoadObject(hash string) (objects.Object, error) {
	return w.mock.LoadObject(hash)
}

func (w *repositoryWrapper) GetHead() (string, error) {
	return w.mock.GetHead()
}

func TestBlameLine_String(t *testing.T) {
	line := BlameLine{
		LineNumber: 1,
		Content:    "hello world",
		CommitHash: "abc123def456",
		Author:     "John Doe",
		AuthorTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	expected := "abc123de (John Doe 2023-01-01 12:00:00 1) hello world\n"
	result := (&BlameResult{Lines: []BlameLine{line}}).String()

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestBlameFile_NoCommits(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize test repository: %v", err)
	}

	result, err := BlameFile(repo, "test.txt", "")

	if err == nil {
		t.Error("Expected error for repository with no commits")
	}
	if result != nil {
		t.Error("Expected nil result for repository with no commits")
	}
}

func TestBlameFile_FileNotFound(t *testing.T) {
	mock := setupBasicMockRepo(t)
	repo := setupTestRepository(t, mock)

	commitHash, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	result, err := BlameFile(repo, "nonexistent.txt", commitHash)

	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if result != nil {
		t.Error("Expected nil result for non-existent file")
	}
}

func TestBlameFile_SingleCommit(t *testing.T) {
	mock := setupBasicMockRepo(t)
	repo := setupTestRepository(t, mock)

	commitHash, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	result, err := BlameFile(repo, "test.txt", commitHash)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %q", result.Path)
	}

	if len(result.Lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(result.Lines))
	}

	line := result.Lines[0]
	if line.LineNumber != 1 {
		t.Errorf("Expected line number 1, got %d", line.LineNumber)
	}
	if line.Content != "line 1" {
		t.Errorf("Expected content 'line 1', got %q", line.Content)
	}
	if line.CommitHash != commitHash {
		t.Errorf("Expected commit hash %q, got %q", commitHash, line.CommitHash)
	}
	if line.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %q", line.Author)
	}
}

func TestBlameFile_UseHead(t *testing.T) {
	mock := setupBasicMockRepo(t)
	repo := setupTestRepository(t, mock)

	result, err := BlameFile(repo, "test.txt", "")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(result.Lines))
	}
}

func TestGetFileContentAtCommit_Success(t *testing.T) {
	mock := setupBasicMockRepo(t)
	repo := setupTestRepository(t, mock)

	commitHash, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	content, err := getFileContentAtCommit(repo, commitHash, "test.txt")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []byte("line 1\nline 2\nline 3")
	if !bytes.Equal(content, expected) {
		t.Errorf("Expected content %q, got %q", expected, content)
	}
}

func TestGetFileContentAtCommit_InvalidCommit(t *testing.T) {
	mock := setupBasicMockRepo(t)
	repo := setupTestRepository(t, mock)

	content, err := getFileContentAtCommit(repo, "invalid", "test.txt")

	if err == nil {
		t.Error("Expected error for invalid commit")
	}
	if content != nil {
		t.Error("Expected nil content for invalid commit")
	}
}

func TestFindLineInParent_ExactMatch(t *testing.T) {
	currentLines := []string{"line 1", "line 2", "line 3"}
	parentLines := []string{"line 1", "line 2", "line 3"}

	result := findLineInParent(currentLines, parentLines, 2)

	if result != 2 {
		t.Errorf("Expected line 2, got %d", result)
	}
}

func TestFindLineInParent_LineNotFound(t *testing.T) {
	currentLines := []string{"line 1", "new line", "line 3"}
	parentLines := []string{"line 1", "line 3"}

	result := findLineInParent(currentLines, parentLines, 2)

	if result != 0 {
		t.Errorf("Expected 0 (not found), got %d", result)
	}
}

func TestFindLineInParent_InvalidLineNumber(t *testing.T) {
	currentLines := []string{"line 1", "line 2"}
	parentLines := []string{"line 1", "line 2"}

	result := findLineInParent(currentLines, parentLines, 5)

	if result != 0 {
		t.Errorf("Expected 0 for invalid line number, got %d", result)
	}
}

func TestFindBestOffset_NoOffset(t *testing.T) {
	currentLines := []string{"line 1", "line 2", "line 3"}
	parentLines := []string{"line 1", "line 2", "line 3"}

	offset := findBestOffset(currentLines, parentLines, 1, 1)

	if offset != 0 {
		t.Errorf("Expected offset 0, got %d", offset)
	}
}

func TestFindBestOffset_WithOffset(t *testing.T) {
	currentLines := []string{"line 1", "line 2", "line 3", "line 4"}
	parentLines := []string{"new line", "line 1", "line 2", "line 3", "line 4"}

	offset := findBestOffset(currentLines, parentLines, 1, 2)

	if offset != 0 {
		t.Errorf("Expected offset 0, got %d", offset)
	}
}

func TestSplitLines_EmptyContent(t *testing.T) {
	lines := splitLines([]byte(""))

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for empty content, got %d", len(lines))
	}
}

func TestSplitLines_MultipleLines(t *testing.T) {
	content := []byte("line 1\nline 2\nline 3")
	lines := splitLines(content)

	expected := []string{"line 1", "line 2", "line 3"}

	if len(lines) != len(expected) {
		t.Errorf("Expected %d lines, got %d", len(expected), len(lines))
	}

	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Expected line %d to be %q, got %q", i, expected[i], line)
		}
	}
}

func TestSplitLines_SingleLine(t *testing.T) {
	content := []byte("single line")
	lines := splitLines(content)

	if len(lines) != 1 {
		t.Errorf("Expected 1 line, got %d", len(lines))
	}

	if lines[0] != "single line" {
		t.Errorf("Expected 'single line', got %q", lines[0])
	}
}

func TestFindCommitForLineRecursive_CircularReference(t *testing.T) {
	mock := NewMockRepository()

	commit := &MockCommit{
		hash:    "commit1",
		tree:    "tree1",
		parents: []string{"commit1"}, // Self-reference for testing
		author: &objects.Signature{
			Name: "Test Author",
			When: time.Now(),
		},
		data: []byte("commit data"),
	}

	mock.AddObject("commit1", commit)
	repo := setupTestRepository(t, mock)

	visited := make(map[string]bool)
	result, err := findCommitForLineRecursive(repo, "commit1", "test.txt", 1, visited)

	if err == nil {
		t.Error("Expected error for circular reference")
	}
	if result != nil {
		t.Error("Expected nil result for circular reference")
	}
}

func setupBasicMockRepo(t *testing.T) *MockRepository {
	mock := NewMockRepository()
	return mock
}

func setupTestRepository(t *testing.T, mock *MockRepository) *repository.Repository {
	tempDir := t.TempDir()

	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize test repository: %v", err)
	}

	// Create real objects and store them
	blob := objects.NewBlob([]byte("line 1\nline 2\nline 3"))
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	tree := objects.NewTree([]objects.TreeEntry{
		{
			Mode: objects.FileModeBlob,
			Name: "test.txt",
			Hash: blobHash,
		},
	})
	treeHash, err := repo.StoreObject(tree)
	if err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	commit := objects.NewCommit(
		treeHash,
		[]string{},
		&objects.Signature{
			Name: "Test Author",
			When: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		&objects.Signature{
			Name: "Test Author",
			When: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		"Test commit message",
	)
	commitHash, err := repo.StoreObject(commit)
	if err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	// Update HEAD to point to the commit
	if err := repo.UpdateRef("refs/heads/main", commitHash); err != nil {
		t.Fatalf("Failed to update ref: %v", err)
	}

	// Store the commit hash in mock for tests that need it
	mock.SetHead(commitHash)

	return repo
}

func BenchmarkBlameFile(b *testing.B) {
	mock := setupBasicMockRepo(nil)
	repo := setupTestRepository(nil, mock) // Note: passing nil for testing.T in benchmark

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		commitHash, err := repo.GetHead()
		if err != nil {
			b.Fatalf("Failed to get HEAD: %v", err)
		}

		_, err = BlameFile(repo, "test.txt", commitHash)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkSplitLines(b *testing.B) {
	content := bytes.Repeat([]byte("test line\n"), 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		splitLines(content)
	}
}

func BenchmarkFindLineInParent(b *testing.B) {
	currentLines := make([]string, 1000)
	parentLines := make([]string, 1000)

	for i := 0; i < 1000; i++ {
		currentLines[i] = "line " + string(rune(i))
		parentLines[i] = "line " + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findLineInParent(currentLines, parentLines, 500)
	}
}
