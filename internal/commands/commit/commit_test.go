package commit

import (
	"testing"
	"time"

	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

func TestCreateCommit_Success(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupTestRepository(t, tempDir)

	content := []byte("test content")
	blob := objects.NewBlob(content)
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}
	idx.Add("test.txt", blobHash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	options := CommitOptions{
		Message:     "Test commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	commitHash, err := CreateCommit(repo, options)
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	if commitHash == "" {
		t.Error("Expected non-empty commit hash")
	}

	obj, err := repo.LoadObject(commitHash)
	if err != nil {
		t.Fatalf("Failed to load commit object: %v", err)
	}

	commit, ok := obj.(*objects.Commit)
	if !ok {
		t.Fatalf("Expected commit object, got %T", obj)
	}

	if commit.Message() != "Test commit" {
		t.Errorf("Expected message 'Test commit', got %q", commit.Message())
	}

	if commit.Author().Name != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %q", commit.Author().Name)
	}

	if commit.Committer().Name != "Test Author" {
		t.Errorf("Expected committer 'Test Author', got %q", commit.Committer().Name)
	}

	if commit.Author().Email != "test@example.com" {
		t.Errorf("Expected author email 'test@example.com', got %q", commit.Author().Email)
	}
}

func TestCreateCommit_EmptyIndex(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupTestRepository(t, tempDir)

	options := CommitOptions{
		Message:     "Empty commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	_, err := CreateCommit(repo, options)
	if err == nil {
		t.Error("Expected error for empty index")
	}
}

func TestCreateCommit_NotGitRepository(t *testing.T) {
	tempDir := t.TempDir()
	repo := repository.New(tempDir) // Don't initialize

	options := CommitOptions{
		Message:     "Test commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	_, err := CreateCommit(repo, options)
	if err == nil {
		t.Error("Expected error for non-git repository")
	}
}

func TestCreateCommit_WithParent(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupTestRepository(t, tempDir)

	content1 := []byte("first content")
	blob1 := objects.NewBlob(content1)
	blobHash1, err := repo.StoreObject(blob1)
	if err != nil {
		t.Fatalf("Failed to store first blob: %v", err)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}
	idx.Add("file1.txt", blobHash1, uint32(objects.FileModeBlob), int64(len(content1)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	options1 := CommitOptions{
		Message:     "First commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	firstCommitHash, err := CreateCommit(repo, options1)
	if err != nil {
		t.Fatalf("Failed to create first commit: %v", err)
	}

	if err := repo.UpdateRef("refs/heads/main", firstCommitHash); err != nil {
		t.Fatalf("Failed to update ref: %v", err)
	}

	content2 := []byte("second content")
	blob2 := objects.NewBlob(content2)
	blobHash2, err := repo.StoreObject(blob2)
	if err != nil {
		t.Fatalf("Failed to store second blob: %v", err)
	}

	idx.Add("file2.txt", blobHash2, uint32(objects.FileModeBlob), int64(len(content2)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	options2 := CommitOptions{
		Message:     "Second commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	secondCommitHash, err := CreateCommit(repo, options2)
	if err != nil {
		t.Fatalf("Failed to create second commit: %v", err)
	}

	obj, err := repo.LoadObject(secondCommitHash)
	if err != nil {
		t.Fatalf("Failed to load second commit: %v", err)
	}

	commit, ok := obj.(*objects.Commit)
	if !ok {
		t.Fatalf("Expected commit object, got %T", obj)
	}

	parents := commit.Parents()
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	} else if parents[0] != firstCommitHash {
		t.Errorf("Expected parent %s, got %s", firstCommitHash, parents[0])
	}
}

func TestCreateCommit_UpdatesHead(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupTestRepository(t, tempDir)

	content := []byte("test content")
	blob := objects.NewBlob(content)
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}
	idx.Add("test.txt", blobHash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	options := CommitOptions{
		Message:     "Test commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	commitHash, err := CreateCommit(repo, options)
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	headHash, err := repo.GetHead()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	if headHash != commitHash {
		t.Errorf("Expected HEAD to be %s, got %s", commitHash, headHash)
	}
}

func TestCreateCommit_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupTestRepository(t, tempDir)

	files := map[string][]byte{
		"file1.txt": []byte("content 1"),
		"file2.txt": []byte("content 2"),
		"file3.txt": []byte("content 3"),
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}

	for filename, content := range files {
		blob := objects.NewBlob(content)
		blobHash, err := repo.StoreObject(blob)
		if err != nil {
			t.Fatalf("Failed to store blob for %s: %v", filename, err)
		}

		idx.Add(filename, blobHash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	}

	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	options := CommitOptions{
		Message:     "Multi-file commit",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	commitHash, err := CreateCommit(repo, options)
	if err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	obj, err := repo.LoadObject(commitHash)
	if err != nil {
		t.Fatalf("Failed to load commit: %v", err)
	}

	commit, ok := obj.(*objects.Commit)
	if !ok {
		t.Fatalf("Expected commit object, got %T", obj)
	}

	treeObj, err := repo.LoadObject(commit.Tree())
	if err != nil {
		t.Fatalf("Failed to load tree: %v", err)
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		t.Fatalf("Expected tree object, got %T", treeObj)
	}

	entries := tree.Entries()
	if len(entries) != len(files) {
		t.Errorf("Expected %d tree entries, got %d", len(files), len(entries))
	}

	entryNames := make(map[string]bool)
	for _, entry := range entries {
		entryNames[entry.Name] = true
	}

	for filename := range files {
		if !entryNames[filename] {
			t.Errorf("Expected file %s in tree", filename)
		}
	}
}

func TestCreateCommit_InvalidOptions(t *testing.T) {
	tempDir := t.TempDir()
	repo := setupTestRepository(t, tempDir)

	options := CommitOptions{
		Message:     "",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
	}

	_, err := CreateCommit(repo, options)
	if err == nil {
		t.Error("Expected error for empty message")
	}

	options = CommitOptions{
		Message:     "Test commit",
		AuthorName:  "",
		AuthorEmail: "test@example.com",
	}

	content := []byte("test content")
	blob := objects.NewBlob(content)
	blobHash, err := repo.StoreObject(blob)
	if err != nil {
		t.Fatalf("Failed to store blob: %v", err)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
	}
	idx.Add("test.txt", blobHash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
	if err := idx.Save(); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	_, err = CreateCommit(repo, options)
	if err != nil {
		t.Errorf("Expected success with default author name, got error: %v", err)
	}
}

func setupTestRepository(t *testing.T, tempDir string) *repository.Repository {
	repo := repository.New(tempDir)

	if err := repo.Init(); err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	return repo
}

func BenchmarkCreateCommit(b *testing.B) {
	tempDir := b.TempDir()
	repo := setupTestRepository(nil, tempDir) // Note: passing nil for testing.T in benchmark

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content := []byte("benchmark content " + string(rune(i)))
		blob := objects.NewBlob(content)
		blobHash, err := repo.StoreObject(blob)
		if err != nil {
			b.Fatalf("Failed to store blob: %v", err)
		}

		idx := index.New(repo.GitDir)
		if err := idx.Load(); err != nil {
		}
		idx.Add("benchmark.txt", blobHash, uint32(objects.FileModeBlob), int64(len(content)), time.Now())
		if err := idx.Save(); err != nil {
			b.Fatalf("Failed to save index: %v", err)
		}

		options := CommitOptions{
			Message:     "Benchmark commit",
			AuthorName:  "Benchmark Author",
			AuthorEmail: "benchmark@example.com",
		}

		_, err = CreateCommit(repo, options)
		if err != nil {
			b.Fatalf("Failed to create commit: %v", err)
		}
	}
}
