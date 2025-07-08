package status

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/unkn0wn-root/git-go/pkg/display"
	"github.com/unkn0wn-root/git-go/pkg/errors"
	"github.com/unkn0wn-root/git-go/internal/core/hash"
	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

type FileStatus int

const (
	StatusUntracked FileStatus = iota
	StatusAdded
	StatusModified
	StatusDeleted
	StatusRenamed
	StatusUnmodified
)

func (s FileStatus) String() string {
	switch s {
	case StatusUntracked:
		return "??"
	case StatusAdded:
		return "A "
	case StatusModified:
		return "M "
	case StatusDeleted:
		return "D "
	case StatusRenamed:
		return "R "
	default:
		return "  "
	}
}

// DisplayStatus returns the formatted status with colors
func (s FileStatus) DisplayStatus() string {
	return display.FormatFileStatus(display.FileStatus(s))
}

type StatusEntry struct {
	Path        string
	IndexStatus FileStatus
	WorkStatus  FileStatus
}

type StatusResult struct {
	Branch     string
	Entries    []StatusEntry
	HasChanges bool
	IsInitial  bool
}

func (sr *StatusResult) String() string {
	// Convert to display format for colored output
	entries := make([]display.StatusEntry, len(sr.Entries))
	for i, entry := range sr.Entries {
		entries[i] = display.StatusEntry{
			Path:        entry.Path,
			IndexStatus: display.FileStatus(entry.IndexStatus),
			WorkStatus:  display.FileStatus(entry.WorkStatus),
		}
	}

	return display.FormatStatusResult(sr.Branch, entries, sr.IsInitial)
}

func GetStatus(repo *repository.Repository) (*StatusResult, error) {
	if !repo.Exists() {
		return nil, errors.ErrNotGitRepository
	}

	branch, err := repo.GetCurrentBranch()
	if err != nil {
		branch = "main" // default
	}

	headHash, err := repo.GetHead()
	isInitial := (err != nil || headHash == "")

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return nil, errors.NewGitError("status", "", fmt.Errorf("load index: %w", err))
	}

	var headFiles map[string]string
	if !isInitial {
		headFiles, err = getHeadFiles(repo, headHash)
		if err != nil {
			return nil, err
		}
	} else {
		headFiles = make(map[string]string)
	}

	workingFiles, err := getWorkingFiles(repo)
	if err != nil {
		return nil, err
	}

	indexFiles := idx.GetAll()

	allFiles := make(map[string]bool)
	for path := range headFiles {
		allFiles[path] = true
	}
	for path := range indexFiles {
		allFiles[path] = true
	}
	for path := range workingFiles {
		allFiles[path] = true
	}

	var entries []StatusEntry

	for path := range allFiles {
		entry := StatusEntry{Path: path}

		headHash, inHead := headFiles[path]
		indexEntry, inIndex := indexFiles[path]
		workingHash, inWorking := workingFiles[path]

		// Determine index status (HEAD vs Index)
		if !inHead && inIndex {
			entry.IndexStatus = StatusAdded
		} else if inHead && !inIndex {
			entry.IndexStatus = StatusDeleted
		} else if inHead && inIndex && headHash != indexEntry.Hash {
			entry.IndexStatus = StatusModified
		} else {
			entry.IndexStatus = StatusUnmodified
		}

		// Determine working status (Index vs Working)
		if !inIndex && inWorking {
			entry.WorkStatus = StatusUntracked
		} else if inIndex && !inWorking {
			entry.WorkStatus = StatusDeleted
		} else if inIndex && inWorking && indexEntry.Hash != workingHash {
			entry.WorkStatus = StatusModified
		} else {
			entry.WorkStatus = StatusUnmodified
		}

		if entry.IndexStatus != StatusUnmodified || entry.WorkStatus != StatusUnmodified {
			entries = append(entries, entry)
		}
	}

	return &StatusResult{
		Branch:     branch,
		Entries:    entries,
		HasChanges: len(entries) > 0,
		IsInitial:  isInitial,
	}, nil
}

func getHeadFiles(repo *repository.Repository, headHash string) (map[string]string, error) {
	commitObj, err := repo.LoadObject(headHash)
	if err != nil {
		return nil, err
	}

	commit, ok := commitObj.(*objects.Commit)
	if !ok {
		return nil, errors.NewGitError("status", "", fmt.Errorf("HEAD is not a commit"))
	}

	treeObj, err := repo.LoadObject(commit.Tree())
	if err != nil {
		return nil, err
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return nil, errors.NewGitError("status", "", fmt.Errorf("commit tree is not a tree object"))
	}

	files := make(map[string]string)
	if err := walkTree(repo, tree, "", files); err != nil {
		return nil, err
	}

	return files, nil
}

func walkTree(repo *repository.Repository, tree *objects.Tree, prefix string, files map[string]string) error {
	for _, entry := range tree.Entries() {
		path := entry.Name
		if prefix != "" {
			path = filepath.Join(prefix, entry.Name)
		}

		switch entry.Mode {
		case objects.FileModeTree:
			subtreeObj, err := repo.LoadObject(entry.Hash)
			if err != nil {
				return err
			}
			subtree, ok := subtreeObj.(*objects.Tree)
			if !ok {
				return fmt.Errorf("object %s is not a tree", entry.Hash)
			}
			if err := walkTree(repo, subtree, path, files); err != nil {
				return err
			}
		case objects.FileModeBlob, objects.FileModeExecutable:
			gitPath := filepath.ToSlash(path)
			files[gitPath] = entry.Hash
		}
	}
	return nil
}

func getWorkingFiles(repo *repository.Repository) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.WalkDir(repo.WorkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(repo.WorkDir, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		objHash := hash.ComputeObjectHash("blob", content)
		gitPath := filepath.ToSlash(relPath)
		files[gitPath] = objHash

		return nil
	})

	return files, err
}
