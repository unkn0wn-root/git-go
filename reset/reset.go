package reset

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

const (
	defaultDirMode       = 0755
	directoryMode        = 0o040000
	gitHashLength        = 40
	minShortHashLength   = 4
	hashPrefixLength     = 2
	headRef              = "HEAD"
	headsPrefix          = "refs/heads/"
	objectsDir           = "objects"
)

type ResetMode int

const (
	ResetModeDefault ResetMode = iota // Mixed mode
	ResetModeSoft
	ResetModeMixed
	ResetModeHard
)

func (m ResetMode) String() string {
	switch m {
	case ResetModeSoft:
		return "soft"
	case ResetModeMixed:
		return "mixed"
	case ResetModeHard:
		return "hard"
	default:
		return "mixed"
	}
}

func Reset(repo *repository.Repository, target string, mode ResetMode, paths []string) error {
	if !repo.Exists() {
		return errors.ErrNotGitRepository
	}

	// if paths are specified, do a pathspec reset (mixed mode only)
	if len(paths) > 0 {
		return resetPaths(repo, target, paths)
	}

	targetHash, err := resolveTarget(repo, target)
	if err != nil {
		return errors.NewGitError("reset", target, fmt.Errorf("failed to resolve target '%s': %w", target, err))
	}

	targetObj, err := repo.LoadObject(targetHash)
	if err != nil {
		return errors.NewObjectError(targetHash, "commit", err)
	}

	targetCommit, ok := targetObj.(*objects.Commit)
	if !ok {
		return errors.NewObjectError(targetHash, "commit", errors.ErrInvalidCommit)
	}

	// update HEAD reference
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return errors.NewGitError("reset", "", err)
	}

	refPath := fmt.Sprintf("%s%s", headsPrefix, currentBranch)
	if err := repo.UpdateRef(refPath, targetHash); err != nil {
		return errors.NewGitError("reset", refPath, err)
	}

	if mode != ResetModeSoft {
		if err := resetIndex(repo, targetCommit.Tree()); err != nil {
			return errors.NewIndexError("", err)
		}
	}

	if mode == ResetModeHard {
		if err := resetWorkingTree(repo, targetCommit.Tree()); err != nil {
			return errors.NewGitError("reset", "", err)
		}
	}

	return nil
}

func resetPaths(repo *repository.Repository, target string, paths []string) error {
	// Resolve target (defaults to HEAD if empty)
	targetHash, err := resolveTarget(repo, target)
	if err != nil {
		return errors.NewGitError("reset", target, fmt.Errorf("'%s': %w", target, err))
	}

	targetObj, err := repo.LoadObject(targetHash)
	if err != nil {
		return errors.NewObjectError(targetHash, "commit", fmt.Errorf("load target commit: %w", err))
	}

	targetCommit, ok := targetObj.(*objects.Commit)
	if !ok {
		return errors.NewObjectError(targetHash, "commit", errors.ErrInvalidCommit)
	}

	treeObj, err := repo.LoadObject(targetCommit.Tree())
	if err != nil {
		return errors.NewObjectError(targetCommit.Tree(), "tree", fmt.Errorf("load target tree: %w", err))
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return errors.NewObjectError(targetCommit.Tree(), "tree", errors.ErrInvalidTree)
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return errors.NewIndexError("", fmt.Errorf("load index: %w", err))
	}

	for _, path := range paths {
		if err := resetPathInIndex(idx, tree, path); err != nil {
			return errors.NewIndexError(path, fmt.Errorf("'%s': %w", path, err))
		}
	}

	if err := idx.Save(); err != nil {
		return errors.NewIndexError("", fmt.Errorf("save index: %w", err))
	}

	return nil
}

func resetIndex(repo *repository.Repository, treeHash string) error {
	idx := index.New(repo.GitDir)
	idx.Clear()

	treeObj, err := repo.LoadObject(treeHash)
	if err != nil {
		return errors.NewObjectError(treeHash, "tree", fmt.Errorf("load tree: %w", err))
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return errors.NewObjectError(treeHash, "tree", errors.ErrInvalidTree)
	}

	if err := addTreeToIndex(repo, idx, tree, ""); err != nil {
		return err
	}

	return idx.Save()
}

func resetWorkingTree(repo *repository.Repository, treeHash string) error {
	treeObj, err := repo.LoadObject(treeHash)
	if err != nil {
		return errors.NewObjectError(treeHash, "tree", fmt.Errorf("load tree: %w", err))
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return errors.NewObjectError(treeHash, "tree", errors.ErrInvalidTree)
	}

	// remove all tracked files from working tree
	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return errors.NewIndexError("", fmt.Errorf("load index: %w", err))
	}

	for path := range idx.GetAll() {
		fullPath := filepath.Join(repo.WorkDir, path)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return errors.NewGitError("reset", path, fmt.Errorf("remove file '%s': %w", path, err))
		}
	}

	return restoreTreeToWorkingDir(repo, tree, "")
}

func addTreeToIndex(repo *repository.Repository, idx *index.Index, tree *objects.Tree, basePath string) error {
	for _, entry := range tree.Entries() {
		entryPath := entry.Name
		if basePath != "" {
			entryPath = filepath.Join(basePath, entry.Name)
		}

		if entry.Mode == directoryMode { // Directory
			subtreeObj, err := repo.LoadObject(entry.Hash)
			if err != nil {
				return errors.NewObjectError(entry.Hash, "tree", fmt.Errorf("load subtree: %w", err))
			}

			subtree, ok := subtreeObj.(*objects.Tree)
			if !ok {
				return errors.NewObjectError(entry.Hash, "tree", errors.ErrInvalidTree)
			}

			if err := addTreeToIndex(repo, idx, subtree, entryPath); err != nil {
				return err
			}
		} else { // File
			blobObj, err := repo.LoadObject(entry.Hash)
			if err != nil {
				return errors.NewObjectError(entry.Hash, "blob", fmt.Errorf("load blob: %w", err))
			}

			blob, ok := blobObj.(*objects.Blob)
			if !ok {
				return errors.NewObjectError(entry.Hash, "blob", errors.ErrInvalidBlob)
			}

			// index with current time as modification time
			if err := idx.Add(entryPath, entry.Hash, uint32(entry.Mode), blob.Size(), time.Now()); err != nil {
				return errors.NewIndexError(entryPath, fmt.Errorf("failed to add file to index: %w", err))
			}
		}
	}

	return nil
}

func restoreTreeToWorkingDir(repo *repository.Repository, tree *objects.Tree, basePath string) error {
	for _, entry := range tree.Entries() {
		entryPath := entry.Name
		if basePath != "" {
			entryPath = filepath.Join(basePath, entry.Name)
		}

		fullPath := filepath.Join(repo.WorkDir, entryPath)

		if entry.Mode == directoryMode { // Directory
			if err := os.MkdirAll(fullPath, defaultDirMode); err != nil {
				return errors.NewGitError("reset", entryPath, fmt.Errorf("create directory '%s': %w", entryPath, err))
			}

			subtreeObj, err := repo.LoadObject(entry.Hash)
			if err != nil {
				return errors.NewObjectError(entry.Hash, "tree", fmt.Errorf("load subtree: %w", err))
			}

			subtree, ok := subtreeObj.(*objects.Tree)
			if !ok {
				return errors.NewObjectError(entry.Hash, "tree", errors.ErrInvalidTree)
			}

			if err := restoreTreeToWorkingDir(repo, subtree, entryPath); err != nil {
				return err
			}
		} else { // File
			// Load blob
			blobObj, err := repo.LoadObject(entry.Hash)
			if err != nil {
				return errors.NewObjectError(entry.Hash, "blob", fmt.Errorf("load blob: %w", err))
			}

			blob, ok := blobObj.(*objects.Blob)
			if !ok {
				return errors.NewObjectError(entry.Hash, "blob", errors.ErrInvalidBlob)
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), defaultDirMode); err != nil {
				return errors.NewGitError("reset", entryPath, fmt.Errorf("create parent directory for '%s': %w", entryPath, err))
			}

			if err := os.WriteFile(fullPath, blob.Content(), os.FileMode(entry.Mode)); err != nil {
				return errors.NewGitError("reset", entryPath, fmt.Errorf("write file '%s': %w", entryPath, err))
			}
		}
	}

	return nil
}

func resetPathInIndex(idx *index.Index, tree *objects.Tree, path string) error {
	for _, entry := range tree.Entries() {
		if entry.Name == path && entry.Mode != directoryMode {
			idx.Remove(path)

			if err := idx.Add(path, entry.Hash, uint32(entry.Mode), 0, time.Now()); err != nil {
				return errors.NewIndexError(path, fmt.Errorf("failed to add path to index: %w", err))
			}
			return nil
		}
	}

	// file not found in tree, remove from index
	return idx.Remove(path)
}

// resolveTarget resolves a target reference to a commit hash
func resolveTarget(repo *repository.Repository, target string) (string, error) {
	if target == "" || target == headRef {
		return repo.GetHead()
	}

	// full hash first
	if len(target) == gitHashLength {
		return target, nil
	}

	// short hash - expand to full hash by finding matching object
	if len(target) >= minShortHashLength && len(target) <= gitHashLength {
		fullHash, err := expandShortHash(repo, target)
		if err == nil {
			return fullHash, nil
		}
	}

	// branch reference
	branchRef := fmt.Sprintf("%s%s", headsPrefix, target)
	if hash, err := readRef(repo, branchRef); err == nil {
		return hash, nil
	}

	return "", errors.NewGitError("reset", target, fmt.Errorf("unable to resolve target '%s'", target))
}

// expandShortHash finds the full hash for a short hash
func expandShortHash(repo *repository.Repository, shortHash string) (string, error) {
	objectsDirPath := filepath.Join(repo.GitDir, objectsDir)

	// short hash format: first 2 chars as directory, rest as filename prefix
	if len(shortHash) < minShortHashLength {
		return "", errors.NewGitError("reset", shortHash, fmt.Errorf("short hash too short"))
	}

	prefix := shortHash[:hashPrefixLength]
	suffix := shortHash[hashPrefixLength:]

	dirPath := filepath.Join(objectsDirPath, prefix)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", errors.NewGitError("reset", dirPath, fmt.Errorf("read objects directory: %w", err))
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) >= len(suffix) && entry.Name()[:len(suffix)] == suffix {
			fullHash := prefix + entry.Name()
			matches = append(matches, fullHash)
		}
	}

	if len(matches) == 0 {
		return "", errors.NewGitError("reset", shortHash, fmt.Errorf("no objects found matching short hash"))
	}

	if len(matches) > 1 {
		return "", errors.NewGitError("reset", shortHash, fmt.Errorf("short hash is ambiguous"))
	}

	return matches[0], nil
}

func readRef(repo *repository.Repository, refPath string) (string, error) {
	fullPath := filepath.Join(repo.GitDir, refPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	hash := string(content)
	if len(hash) > gitHashLength {
		hash = hash[:gitHashLength]
	}

	return hash, nil
}
