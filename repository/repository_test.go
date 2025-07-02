package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/objects"
)

func TestNew(t *testing.T) {
	workDir := "/tmp/test-repo"
	repo := New(workDir)

	if repo.WorkDir != workDir {
		t.Errorf("Expected WorkDir %q, got %q", workDir, repo.WorkDir)
	}

	expectedGitDir := filepath.Join(workDir, ".git")
	if repo.GitDir != expectedGitDir {
		t.Errorf("Expected GitDir %q, got %q", expectedGitDir, repo.GitDir)
	}
}

func TestRepository_Init(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedDirs := []string{
		repo.GitDir,
		filepath.Join(repo.GitDir, "objects"),
		filepath.Join(repo.GitDir, "refs"),
		filepath.Join(repo.GitDir, "refs", "heads"),
		filepath.Join(repo.GitDir, "refs", "tags"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %q to exist", dir)
		}
	}

	headPath := filepath.Join(repo.GitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("Failed to read HEAD file: %v", err)
	}

	expectedHead := "ref: refs/heads/main\n"
	if string(content) != expectedHead {
		t.Errorf("Expected HEAD content %q, got %q", expectedHead, string(content))
	}

	err = repo.Init()
	if err == nil {
		t.Error("Expected error when re-initializing existing repository")
	}
}

func TestRepository_Exists(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	if repo.Exists() {
		t.Error("Expected repository to not exist")
	}

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	if !repo.Exists() {
		t.Error("Expected repository to exist after initialization")
	}
}

func TestRepository_StoreAndLoadObject_Blob(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	content := []byte("Hello, World!")
	blob := objects.NewBlob(content)

	hash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	if len(hash) != 40 {
		t.Errorf("Expected hash length 40, got %d", len(hash))
	}

	if blob.Hash() != hash {
		t.Errorf("Expected blob hash to be set to %q, got %q", hash, blob.Hash())
	}

	loadedObj, err := repo.LoadObject(hash)
	if err != nil {
		t.Fatalf("Failed to load blob: %v", err)
	}

	loadedBlob, ok := loadedObj.(*objects.Blob)
	if !ok {
		t.Fatalf("Expected loaded object to be *objects.Blob, got %T", loadedObj)
	}

	if string(loadedBlob.Content()) != string(content) {
		t.Errorf("Expected loaded blob content %q, got %q", content, loadedBlob.Content())
	}

	if loadedBlob.Hash() != hash {
		t.Errorf("Expected loaded blob hash %q, got %q", hash, loadedBlob.Hash())
	}
}

func TestRepository_StoreAndLoadObject_Tree(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	blob := objects.NewBlob([]byte("test content"))
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	entries := []objects.TreeEntry{
		{
			Mode: objects.FileModeBlob,
			Name: "test.txt",
			Hash: blobHash,
		},
	}
	tree := objects.NewTree(entries)

	treeHash, err := repo.StoreObject(tree)
	if err != nil {
		t.Fatalf("Failed to store tree: %v", err)
	}

	loadedObj, err := repo.LoadObject(treeHash)
	if err != nil {
		t.Fatalf("Failed to load tree: %v", err)
	}

	loadedTree, ok := loadedObj.(*objects.Tree)
	if !ok {
		t.Fatalf("Expected loaded object to be *objects.Tree, got %T", loadedObj)
	}

	loadedEntries := loadedTree.Entries()
	if len(loadedEntries) != 1 {
		t.Errorf("Expected 1 tree entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]
	if entry.Name != "test.txt" {
		t.Errorf("Expected entry name 'test.txt', got %q", entry.Name)
	}
	if entry.Hash != blobHash {
		t.Errorf("Expected entry hash %q, got %q", blobHash, entry.Hash)
	}
}

func TestRepository_StoreAndLoadObject_Commit(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	blob := objects.NewBlob([]byte("test content"))
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

	loadedObj, err := repo.LoadObject(commitHash)
	if err != nil {
		t.Fatalf("Failed to load commit: %v", err)
	}

	loadedCommit, ok := loadedObj.(*objects.Commit)
	if !ok {
		t.Fatalf("Expected loaded object to be *objects.Commit, got %T", loadedObj)
	}

	if loadedCommit.Tree() != treeHash {
		t.Errorf("Expected commit tree %q, got %q", treeHash, loadedCommit.Tree())
	}
	if loadedCommit.Message() != "Initial commit" {
		t.Errorf("Expected commit message 'Initial commit', got %q", loadedCommit.Message())
	}
	if loadedCommit.Author().Name != "Test Author" {
		t.Errorf("Expected author name 'Test Author', got %q", loadedCommit.Author().Name)
	}
}

func TestRepository_StoreObject_Errors(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	blob := objects.NewBlob([]byte("test"))
	_, err := repo.StoreObject(blob)
	if err != errors.ErrNotGitRepository {
		t.Errorf("Expected ErrNotGitRepository, got %v", err)
	}
}

func TestRepository_LoadObject_Errors(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	_, err := repo.LoadObject("a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != errors.ErrNotGitRepository {
		t.Errorf("Expected ErrNotGitRepository, got %v", err)
	}

	err = repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	_, err = repo.LoadObject("invalid-hash")
	if err != errors.ErrInvalidHash {
		t.Errorf("Expected ErrInvalidHash, got %v", err)
	}

	_, err = repo.LoadObject("a94a8fe5ccb19ba61c4c0873d391e987982fbbd3")
	if err != errors.ErrObjectNotFound {
		t.Errorf("Expected ErrObjectNotFound, got %v", err)
	}
}

func TestRepository_StoreObject_DuplicateStorage(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	blob := objects.NewBlob([]byte("test content"))
	hash1, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob first time: %v", err)
	}

	hash2, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Expected identical hashes for same content, got %q and %q", hash1, hash2)
	}
}

func TestRepository_UpdateRef(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testHash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	err = repo.UpdateRef("refs/heads/test-branch", testHash)
	if err != nil {
		t.Fatalf("Failed to update ref: %v", err)
	}

	refPath := filepath.Join(repo.GitDir, "refs", "heads", "test-branch")
	content, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("Failed to read ref file: %v", err)
	}

	expectedContent := testHash + "\n"
	if string(content) != expectedContent {
		t.Errorf("Expected ref content %q, got %q", expectedContent, string(content))
	}
}

func TestRepository_GetHead_WithRef(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("GetHead failed: %v", err)
	}
	if head != "" {
		t.Errorf("Expected empty head for non-existent main branch, got %q", head)
	}

	testHash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	err = repo.UpdateRef("refs/heads/main", testHash)
	if err != nil {
		t.Fatalf("Failed to update main ref: %v", err)
	}

	head, err = repo.GetHead()
	if err != nil {
		t.Fatalf("GetHead failed: %v", err)
	}
	if head != testHash {
		t.Errorf("Expected HEAD %q, got %q", testHash, head)
	}
}

func TestRepository_GetHead_DirectHash(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testHash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	headPath := filepath.Join(repo.GitDir, "HEAD")
	err = os.WriteFile(headPath, []byte(testHash+"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write HEAD file: %v", err)
	}

	head, err := repo.GetHead()
	if err != nil {
		t.Fatalf("GetHead failed: %v", err)
	}
	if head != testHash {
		t.Errorf("Expected HEAD %q, got %q", testHash, head)
	}
}

func TestRepository_GetCurrentBranch(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	branch, err := repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "main" {
		t.Errorf("Expected current branch 'main', got %q", branch)
	}

	headPath := filepath.Join(repo.GitDir, "HEAD")
	err = os.WriteFile(headPath, []byte("ref: refs/heads/feature\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write HEAD file: %v", err)
	}

	branch, err = repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "feature" {
		t.Errorf("Expected current branch 'feature', got %q", branch)
	}
}

func TestRepository_GetCurrentBranch_DetachedHead(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	err := repo.Init()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	testHash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	headPath := filepath.Join(repo.GitDir, "HEAD")
	err = os.WriteFile(headPath, []byte(testHash+"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write HEAD file: %v", err)
	}

	_, err = repo.GetCurrentBranch()
	if err != errors.ErrInvalidReference {
		t.Errorf("Expected ErrInvalidReference for detached HEAD, got %v", err)
	}
}

func TestRepository_ObjectPath(t *testing.T) {
	tempDir := t.TempDir()
	repo := New(tempDir)

	hash := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"
	expectedPath := filepath.Join(repo.GitDir, "objects", "a9", "4a8fe5ccb19ba61c4c0873d391e987982fbbd3")

	actualPath := repo.objectPath(hash)
	if actualPath != expectedPath {
		t.Errorf("Expected object path %q, got %q", expectedPath, actualPath)
	}
}
