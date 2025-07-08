package reset

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	giterrors "github.com/unkn0wn-root/git-go/pkg/errors"
	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

func setupTestRepo(t *testing.T) (*repository.Repository, string, string) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	blob := objects.NewBlob([]byte("initial content"))
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	tree := objects.NewTree([]objects.TreeEntry{
		{Mode: objects.FileModeBlob, Name: "test.txt", Hash: blobHash},
	})
	treeHash, err := repo.StoreObject(tree)
	if err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	author := &objects.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	commit := objects.NewCommit(treeHash, []string{}, author, author, "Initial commit")
	commitHash, err := repo.StoreObject(commit)
	if err != nil {
		t.Fatalf("Failed to store commit: %v", err)
	}

	err = repo.UpdateRef("refs/heads/main", commitHash)
	if err != nil {
		t.Fatalf("Failed to update main ref: %v", err)
	}

	idx := index.New(repo.GitDir)
	err = idx.Add("test.txt", blobHash, uint32(objects.FileModeBlob), blob.Size(), time.Now())
	if err != nil {
		t.Fatalf("Failed to add file to index: %v", err)
	}
	err = idx.Save()
	if err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	return repo, commitHash, treeHash
}

func createSecondCommit(t *testing.T, repo *repository.Repository, parentHash string) (string, string) {
	blob := objects.NewBlob([]byte("modified content"))
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store second blob: %v", err)
	}

	tree := objects.NewTree([]objects.TreeEntry{
		{Mode: objects.FileModeBlob, Name: "test.txt", Hash: blobHash},
	})
	treeHash, err := repo.StoreObject(tree)
	if err != nil {
		t.Fatalf("Failed to store second tree: %v", err)
	}

	author := &objects.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
	}
	commit := objects.NewCommit(treeHash, []string{parentHash}, author, author, "Second commit")
	commitHash, err := repo.StoreObject(commit)
	if err != nil {
		t.Fatalf("Failed to store second commit: %v", err)
	}

	err = repo.UpdateRef("refs/heads/main", commitHash)
	if err != nil {
		t.Fatalf("Failed to update main ref for second commit: %v", err)
	}

	return commitHash, treeHash
}

func TestResetMode_String(t *testing.T) {
	tests := []struct {
		mode     ResetMode
		expected string
	}{
		{ResetModeSoft, "soft"},
		{ResetModeMixed, "mixed"},
		{ResetModeHard, "hard"},
		{ResetModeDefault, "mixed"},
		{ResetMode(999), "mixed"}, // Unknown mode defaults to mixed
	}

	for _, test := range tests {
		result := test.mode.String()
		if result != test.expected {
			t.Errorf("Expected %q for mode %d, got %q", test.expected, test.mode, result)
		}
	}
}

func TestReset_NonExistentRepository(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir)

	err := Reset(repo, "HEAD", ResetModeMixed, nil)
	if err != giterrors.ErrNotGitRepository {
		t.Errorf("Expected ErrNotGitRepository, got %v", err)
	}
}

func TestReset_SoftMode(t *testing.T) {
	repo, commit1Hash, _ := setupTestRepo(t)
	commit2Hash, _ := createSecondCommit(t, repo, commit1Hash)

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}
	if head != commit2Hash {
		t.Fatalf("Expected HEAD to be %q, got %q", commit2Hash, head)
	}

	err = Reset(repo, commit1Hash, ResetModeSoft, nil)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	head, err = repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD after reset: %v", err)
	}
	if head != commit1Hash {
		t.Errorf("Expected HEAD to be %q after soft reset, got %q", commit1Hash, head)
	}

	idx := index.New(repo.GitDir)
	err = idx.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetAll()
	if len(entries) != 1 {
		t.Errorf("Expected 1 index entry, got %d", len(entries))
	}

	content, err := os.ReadFile(filepath.Join(repo.WorkDir, "test.txt"))
	if err != nil {
		t.Fatalf("Failed to read working file: %v", err)
	}
	if string(content) != "initial content" {
		t.Errorf("Expected working file content to be unchanged")
	}
}

func TestReset_MixedMode(t *testing.T) {
	repo, commit1Hash, tree1Hash := setupTestRepo(t)
	commit2Hash, _ := createSecondCommit(t, repo, commit1Hash)

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}
	if head != commit2Hash {
		t.Fatalf("Expected HEAD to be %q, got %q", commit2Hash, head)
	}

	err = Reset(repo, commit1Hash, ResetModeMixed, nil)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	head, err = repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD after reset: %v", err)
	}
	if head != commit1Hash {
		t.Errorf("Expected HEAD to be %q after mixed reset, got %q", commit1Hash, head)
	}

	idx := index.New(repo.GitDir)
	err = idx.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetAll()
	if len(entries) != 1 {
		t.Errorf("Expected 1 index entry, got %d", len(entries))
	}

	for path, entry := range entries {
		if path != "test.txt" {
			t.Errorf("Expected index entry for 'test.txt', got %q", path)
		}

		treeObj, err := repo.LoadObject(tree1Hash)
		if err != nil {
			t.Fatalf("Failed to load tree: %v", err)
		}
		tree := treeObj.(*objects.Tree)
		expectedBlobHash := tree.Entries()[0].Hash

		if entry.Hash != expectedBlobHash {
			t.Errorf("Expected index entry hash %q, got %q", expectedBlobHash, entry.Hash)
		}
	}
}

func TestReset_HardMode(t *testing.T) {
	repo, commit1Hash, _ := setupTestRepo(t)
	_, _ = createSecondCommit(t, repo, commit1Hash)

	testFile := filepath.Join(repo.WorkDir, "test.txt")
	err := os.WriteFile(testFile, []byte("working directory change"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify working file: %v", err)
	}

	err = Reset(repo, commit1Hash, ResetModeHard, nil)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD after reset: %v", err)
	}
	if head != commit1Hash {
		t.Errorf("Expected HEAD to be %q after hard reset, got %q", commit1Hash, head)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read working file after hard reset: %v", err)
	}
	if string(content) != "initial content" {
		t.Errorf("Expected working file content 'initial content', got %q", string(content))
	}

	idx := index.New(repo.GitDir)
	err = idx.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetAll()
	if len(entries) != 1 {
		t.Errorf("Expected 1 index entry, got %d", len(entries))
	}
}

func TestReset_DefaultMode(t *testing.T) {
	repo, commit1Hash, _ := setupTestRepo(t)
	createSecondCommit(t, repo, commit1Hash)

	err := Reset(repo, commit1Hash, ResetModeDefault, nil)
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD after reset: %v", err)
	}
	if head != commit1Hash {
		t.Errorf("Expected HEAD to be %q after default reset, got %q", commit1Hash, head)
	}
}

func TestReset_WithPaths(t *testing.T) {
	repo, commit1Hash, _ := setupTestRepo(t)

	blob1 := objects.NewBlob([]byte("modified test.txt"))
	blob1Hash, err := repo.StoreObject(blob1)
	if err != nil {
		t.Fatalf("Failed to store modified blob: %v", err)
	}

	blob2 := objects.NewBlob([]byte("new file content"))
	blob2Hash, err := repo.StoreObject(blob2)
	if err != nil {
		t.Fatalf("Failed to store new blob: %v", err)
	}

	tree := objects.NewTree([]objects.TreeEntry{
		{Mode: objects.FileModeBlob, Name: "test.txt", Hash: blob1Hash},
		{Mode: objects.FileModeBlob, Name: "new.txt", Hash: blob2Hash},
	})
	treeHash, err := repo.StoreObject(tree)
	if err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	author := &objects.Signature{
		Name:  "Test Author",
		Email: "test@example.com",
		When:  time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
	}
	commit := objects.NewCommit(treeHash, []string{commit1Hash}, author, author, "Second commit with multiple files")
	commit2Hash, err := repo.StoreObject(commit)
	if err != nil {
		t.Fatalf("Failed to store second commit: %v", err)
	}

	err = repo.UpdateRef("refs/heads/main", commit2Hash)
	if err != nil {
		t.Fatalf("Failed to update main ref: %v", err)
	}

	idx := index.New(repo.GitDir)
	idx.Clear()
	err = idx.Add("test.txt", blob1Hash, uint32(objects.FileModeBlob), blob1.Size(), time.Now())
	if err != nil {
		t.Fatalf("Failed to add test.txt to index: %v", err)
	}
	err = idx.Add("new.txt", blob2Hash, uint32(objects.FileModeBlob), blob2.Size(), time.Now())
	if err != nil {
		t.Fatalf("Failed to add new.txt to index: %v", err)
	}
	err = idx.Save()
	if err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	err = Reset(repo, commit1Hash, ResetModeMixed, []string{"test.txt"})
	if err != nil {
		t.Fatalf("Path reset failed: %v", err)
	}

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}
	if head != commit2Hash {
		t.Errorf("Expected HEAD to remain %q after path reset, got %q", commit2Hash, head)
	}

	idx = index.New(repo.GitDir)
	err = idx.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetAll()
	if len(entries) != 2 {
		t.Errorf("Expected 2 index entries, got %d", len(entries))
	}

	commit1Obj, err := repo.LoadObject(commit1Hash)
	if err != nil {
		t.Fatalf("Failed to load commit1: %v", err)
	}
	commit1Tree := commit1Obj.(*objects.Commit).Tree()
	tree1Obj, err := repo.LoadObject(commit1Tree)
	if err != nil {
		t.Fatalf("Failed to load commit1 tree: %v", err)
	}
	expectedBlob1Hash := tree1Obj.(*objects.Tree).Entries()[0].Hash

	if entry, exists := entries["test.txt"]; exists {
		if entry.Hash != expectedBlob1Hash {
			t.Errorf("Expected test.txt hash %q, got %q", expectedBlob1Hash, entry.Hash)
		}
	} else {
		t.Error("Expected test.txt in index")
	}

	if entry, exists := entries["new.txt"]; exists {
		if entry.Hash != blob2Hash {
			t.Errorf("Expected new.txt hash %q, got %q", blob2Hash, entry.Hash)
		}
	} else {
		t.Error("Expected new.txt in index")
	}
}

func TestResolveTarget_EmptyAndHEAD(t *testing.T) {
	repo, commitHash, _ := setupTestRepo(t)

	resolved, err := resolveTarget(repo, "")
	if err != nil {
		t.Fatalf("Failed to resolve empty target: %v", err)
	}
	if resolved != commitHash {
		t.Errorf("Expected empty target to resolve to %q, got %q", commitHash, resolved)
	}

	resolved, err = resolveTarget(repo, "HEAD")
	if err != nil {
		t.Fatalf("Failed to resolve HEAD: %v", err)
	}
	if resolved != commitHash {
		t.Errorf("Expected HEAD to resolve to %q, got %q", commitHash, resolved)
	}
}

func TestResolveTarget_FullHash(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	testHash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	resolved, err := resolveTarget(repo, testHash)
	if err != nil {
		t.Fatalf("Failed to resolve full hash: %v", err)
	}
	if resolved != testHash {
		t.Errorf("Expected full hash to resolve to itself %q, got %q", testHash, resolved)
	}
}

func TestResolveTarget_BranchName(t *testing.T) {
	repo, commitHash, _ := setupTestRepo(t)

	err := repo.UpdateRef("refs/heads/feature", commitHash)
	if err != nil {
		t.Fatalf("Failed to create feature branch: %v", err)
	}

	resolved, err := resolveTarget(repo, "feature")
	if err != nil {
		t.Fatalf("Failed to resolve branch name: %v", err)
	}
	if resolved != commitHash {
		t.Errorf("Expected branch 'feature' to resolve to %q, got %q", commitHash, resolved)
	}
}

func TestResolveTarget_InvalidTarget(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := resolveTarget(repo, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent target")
	}
}

func TestExpandShortHash(t *testing.T) {
	repo, commitHash, _ := setupTestRepo(t)

	shortHash := commitHash[:8]
	expanded, err := expandShortHash(repo, shortHash)
	if err != nil {
		t.Fatalf("Failed to expand short hash: %v", err)
	}
	if expanded != commitHash {
		t.Errorf("Expected short hash %q to expand to %q, got %q", shortHash, commitHash, expanded)
	}
}

func TestExpandShortHash_TooShort(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := expandShortHash(repo, "ab")
	if err == nil {
		t.Error("Expected error for hash that's too short")
	}
}

func TestExpandShortHash_NotFound(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	_, err := expandShortHash(repo, "zzzzzzzz")
	if err == nil {
		t.Error("Expected error for non-existent short hash")
	}
}

func TestReset_InvalidTarget(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	err := Reset(repo, "nonexistent", ResetModeMixed, nil)
	if err == nil {
		t.Error("Expected error for invalid target")
	}
}

func TestReset_InvalidCommitObject(t *testing.T) {
	repo, _, _ := setupTestRepo(t)

	blob := objects.NewBlob([]byte("not a commit"))
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	err = Reset(repo, blobHash, ResetModeMixed, nil)
	if err == nil {
		t.Error("Expected error when resetting to non-commit object")
	}
}

func TestResetPaths_NonExistentPath(t *testing.T) {
	repo, commitHash, _ := setupTestRepo(t)

	err := Reset(repo, commitHash, ResetModeMixed, []string{"nonexistent.txt"})
	if err == nil {
		t.Error("Expected error when resetting non-existent path")
	}

	if !errors.Is(err, giterrors.ErrFileNotStaged) {
		t.Errorf("Expected ErrFileNotStaged, got %v", err)
	}
}

func TestResetPaths_RemoveFromIndex(t *testing.T) {
	repo, commitHash, _ := setupTestRepo(t)

	idx := index.New(repo.GitDir)
	err := idx.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	extraBlob := objects.NewBlob([]byte("extra content"))
	extraBlobHash, err := repo.StoreObject(extraBlob)
	if err != nil {
		t.Fatalf("Failed to store extra blob: %v", err)
	}

	err = idx.Add("extra.txt", extraBlobHash, uint32(objects.FileModeBlob), extraBlob.Size(), time.Now())
	if err != nil {
		t.Fatalf("Failed to add extra file to index: %v", err)
	}
	err = idx.Save()
	if err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	err = Reset(repo, commitHash, ResetModeMixed, []string{"extra.txt"})
	if err != nil {
		t.Fatalf("Path reset failed: %v", err)
	}

	idx = index.New(repo.GitDir)
	err = idx.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	entries := idx.GetAll()
	if _, exists := entries["extra.txt"]; exists {
		t.Error("extra.txt should have been removed from index")
	}

	if _, exists := entries["test.txt"]; !exists {
		t.Error("test.txt should still be in index")
	}
}
