package status

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

func TestFileStatus_String(t *testing.T) {
	tests := []struct {
		status   FileStatus
		expected string
	}{
		{StatusUntracked, "??"},
		{StatusAdded, "A "},
		{StatusModified, "M "},
		{StatusDeleted, "D "},
		{StatusRenamed, "R "},
		{StatusUnmodified, "  "},
	}

	for _, test := range tests {
		result := test.status.String()
		if result != test.expected {
			t.Errorf("Expected %q for status %d, got %q", test.expected, test.status, result)
		}
	}
}

func TestStatusResult_String(t *testing.T) {
	initialStatus := &StatusResult{
		Branch:     "main",
		Entries:    []StatusEntry{},
		HasChanges: false,
		IsInitial:  true,
	}

	result := initialStatus.String()
	expected := "On branch main\n\nNo commits yet\n\nnothing to commit, working tree clean\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}

	statusWithChanges := &StatusResult{
		Branch: "main",
		Entries: []StatusEntry{
			{Path: "staged.txt", IndexStatus: StatusAdded, WorkStatus: StatusUnmodified},
			{Path: "modified.txt", IndexStatus: StatusUnmodified, WorkStatus: StatusModified},
			{Path: "untracked.txt", IndexStatus: StatusUnmodified, WorkStatus: StatusUntracked},
		},
		HasChanges: true,
		IsInitial:  false,
	}

	result = statusWithChanges.String()
	if !containsSubstring(result, "Changes to be committed:") {
		t.Error("Expected staged changes section")
	}
	if !containsSubstring(result, "Changes not staged for commit:") {
		t.Error("Expected unstaged changes section")
	}
	if !containsSubstring(result, "Untracked files:") {
		t.Error("Expected untracked files section")
	}
}

func TestGetStatus_NotGitRepository(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	_, err := GetStatus(repo)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestGetStatus_InitialRepository(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	status, err := GetStatus(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !status.IsInitial {
		t.Error("Expected initial repository status")
	}

	if status.HasChanges {
		t.Error("Expected no changes in empty repository")
	}

	if len(status.Entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(status.Entries))
	}
}

func TestGetStatus_UntrackedFile(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	status, err := GetStatus(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected changes due to untracked file")
	}

	if len(status.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(status.Entries))
	}

	entry := status.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %q", entry.Path)
	}

	if entry.WorkStatus != StatusUntracked {
		t.Errorf("Expected untracked status, got %v", entry.WorkStatus)
	}

	if entry.IndexStatus != StatusUnmodified {
		t.Errorf("Expected unmodified index status, got %v", entry.IndexStatus)
	}
}

func TestGetStatus_StagedFile(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testFile := filepath.Join(tempDir, "staged.txt")
	content := []byte("staged content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	blob := objects.NewBlob(content)
	hash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}

	idx.Add("staged.txt", hash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	status, err := GetStatus(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected changes due to staged file")
	}

	if len(status.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(status.Entries))
	}

	entry := status.Entries[0]
	if entry.Path != "staged.txt" {
		t.Errorf("Expected path 'staged.txt', got %q", entry.Path)
	}

	if entry.IndexStatus != StatusAdded {
		t.Errorf("Expected added status, got %v", entry.IndexStatus)
	}
}

func TestGetStatus_ModifiedFile(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupRepoWithCommit(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	newContent := []byte("modified content")
	if err := os.WriteFile(testFile, newContent, 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	status, err := GetStatus(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected changes due to modified file")
	}

	if len(status.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(status.Entries))
	}

	entry := status.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %q", entry.Path)
	}

	if entry.WorkStatus != StatusModified {
		t.Errorf("Expected modified status, got %v", entry.WorkStatus)
	}
}

func TestGetStatus_DeletedFile(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupRepoWithCommit(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Failed to delete test file: %v", err)
	}

	status, err := GetStatus(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !status.HasChanges {
		t.Error("Expected changes due to deleted file")
	}

	if len(status.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(status.Entries))
	}

	entry := status.Entries[0]
	if entry.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %q", entry.Path)
	}

	if entry.WorkStatus != StatusDeleted {
		t.Errorf("Expected deleted status, got %v", entry.WorkStatus)
	}
}

func TestGetHeadFiles_Success(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupRepoWithCommit(t, tempDir)

	headHash, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	files, err := getHeadFiles(repo, headHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	if _, exists := files["test.txt"]; !exists {
		t.Error("Expected test.txt in HEAD files")
	}
}

func TestGetWorkingFiles_Success(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	files, err := getWorkingFiles(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	if _, exists := files["test.txt"]; !exists {
		t.Error("Expected test.txt in working files")
	}
}

func TestGetWorkingFiles_SkipsGitDir(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	gitFile := filepath.Join(tempDir, ".git", "config")
	if err := os.WriteFile(gitFile, []byte("test config"), 0644); err != nil {
		t.Fatalf("Failed to create git file: %v", err)
	}

	files, err := getWorkingFiles(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for path := range files {
		if filepath.HasPrefix(path, ".git") {
			t.Errorf("Should not include .git files, found: %s", path)
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s[1:], substr))
}

func setupRepoWithCommit(t *testing.T, tempDir string) *repository.Repository {
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.txt")
	content := []byte("initial content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	blob := objects.NewBlob(content)
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

	author := &objects.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Now(),
	}

	commit := objects.NewCommit(treeHash, []string{}, author, author, "Initial commit")
	commitHash, err := repo.StoreObject(commit)
	if err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	if err := repo.UpdateRef("refs/heads/main", commitHash); err != nil {
		t.Fatalf("Failed to update ref: %v", err)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}
	idx.Add("test.txt", blobHash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	return repo
}

func BenchmarkGetStatus(b *testing.B) {
	tempDir := b.TempDir()
	repo := setupRepoWithCommit(nil, tempDir) // Note: passing nil for testing.T in benchmark

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetStatus(repo)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkGetWorkingFiles(b *testing.B) {
	tempDir := b.TempDir()
	repo := repository.New(tempDir)
	if err := repo.Init(); err != nil {
		b.Fatalf("Failed to initialize repository: %v", err)
	}

	for i := 0; i < 100; i++ {
		testFile := filepath.Join(tempDir, fmt.Sprintf("test%d.txt", i))
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			b.Fatalf("Failed to create test file: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getWorkingFiles(repo)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
