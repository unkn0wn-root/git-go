package commit

import (
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

type CommitOptions struct {
	Message     string
	AuthorName  string
	AuthorEmail string
}

func CreateCommit(repo *repository.Repository, opts CommitOptions) (string, error) {
	if !repo.Exists() {
		return "", errors.ErrNotGitRepository
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	if !idx.HasChanges() {
		return "", errors.ErrNothingToCommit
	}

	if opts.Message == "" {
		return "", errors.NewGitError("commit", "", fmt.Errorf("commit message is required"))
	}

	treeHash, err := createTreeFromIndex(repo, idx)
	if err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	parentHash, err := repo.GetHead()
	if err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	author, committer, err := getSignatures(opts.AuthorName, opts.AuthorEmail)
	if err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	var parents []string
	if parentHash != "" {
		parents = []string{parentHash}
	}

	commit := objects.NewCommit(treeHash, parents, author, committer, opts.Message)
	commitHash, err := repo.StoreObject(commit)
	if err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	branch, err := repo.GetCurrentBranch()
	if err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	if branch == "" {
		branch = "main"
	}

	refPath := fmt.Sprintf("refs/heads/%s", branch)
	if err := repo.UpdateRef(refPath, commitHash); err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	idx.Clear()
	if err := idx.Save(); err != nil {
		return "", errors.NewGitError("commit", "", err)
	}

	return commitHash, nil
}

func createTreeFromIndex(repo *repository.Repository, idx *index.Index) (string, error) {
	entries := idx.GetAll()
	if len(entries) == 0 {
		return "", errors.NewGitError("commit", "", fmt.Errorf("no files in index"))
	}

	var treeEntries []objects.TreeEntry
	for path, entry := range entries {
		treeEntries = append(treeEntries, objects.TreeEntry{
			Mode: objects.FileMode(entry.Mode),
			Name: path,
			Hash: entry.Hash,
		})
	}

	tree := objects.NewTree(treeEntries)
	hash, err := repo.StoreObject(tree)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func getSignatures(authorName, authorEmail string) (*objects.Signature, *objects.Signature, error) {
	name := authorName
	email := authorEmail

	if name == "" {
		if envName := os.Getenv("GIT_AUTHOR_NAME"); envName != "" {
			name = envName
		} else if u, err := user.Current(); err == nil {
			name = u.Username
		} else {
			name = "Unknown"
		}
	}

	if email == "" {
		if envEmail := os.Getenv("GIT_AUTHOR_EMAIL"); envEmail != "" {
			email = envEmail
		} else {
			email = "local@localhost.local"
		}
	}

	now := time.Now()
	author := &objects.Signature{
		Name:  name,
		Email: email,
		When:  now,
	}

	committer := &objects.Signature{
		Name:  name,
		Email: email,
		When:  now,
	}

	return author, committer, nil
}
