package add

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/gitignore"
	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

func AddFiles(repo *repository.Repository, pathspecs []string) error {
	if !repo.Exists() {
		return errors.ErrNotGitRepository
	}

	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return errors.NewGitError("add", "", fmt.Errorf("load index: %w", err))
	}

	// Load gitignore patterns
	gi, err := gitignore.NewGitIgnore(repo.WorkDir)
	if err != nil {
		return errors.NewGitError("add", "", fmt.Errorf("load gitignore: %w", err))
	}

	for _, pathspec := range pathspecs {
		if pathspec == "." {
			if err := addDirectory(repo, idx, repo.WorkDir, gi); err != nil {
				return err
			}
		} else {
			fullPath := filepath.Join(repo.WorkDir, pathspec)
			info, err := os.Stat(fullPath)
			if err != nil {
				return errors.NewGitError("add", pathspec, fmt.Errorf("pathspec did not match any files"))
			}

			if info.IsDir() {
				if err := addDirectory(repo, idx, fullPath, gi); err != nil {
					return err
				}
			} else {
				if err := addFile(repo, idx, fullPath, gi); err != nil {
					return err
				}
			}
		}
	}

	if err := idx.Save(); err != nil {
		return errors.NewGitError("add", "", fmt.Errorf("failed to save index: %w", err))
	}

	return nil
}

func addFile(repo *repository.Repository, idx *index.Index, filePath string, gi *gitignore.GitIgnore) error {
	relPath, err := filepath.Rel(repo.WorkDir, filePath)
	if err != nil {
		return errors.NewGitError("add", filePath, err)
	}

	// Check if file should be ignored
	if gi.IsIgnored(relPath, false) {
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return errors.NewGitError("add", filePath, err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return errors.NewGitError("add", filePath, err)
	}

	blob := objects.NewBlob(content)
	hash, err := repo.StoreObject(blob)
	if err != nil {
		return errors.NewGitError("add", filePath, err)
	}

	mode := uint32(0o100644)
	if info.Mode()&0o111 != 0 {
		mode = uint32(0o100755)
	}

	if err := idx.Add(relPath, hash, mode, info.Size(), info.ModTime()); err != nil {
		return errors.NewGitError("add", filePath, err)
	}

	return nil
}

func addDirectory(repo *repository.Repository, idx *index.Index, dirPath string, gi *gitignore.GitIgnore) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(repo.WorkDir, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			if gi.IsIgnored(relPath, true) {
				return filepath.SkipDir
			}

			if d.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		// Skip hidden files except .gitignore (unless explicitly not ignored)
		if strings.HasPrefix(d.Name(), ".") && d.Name() != ".gitignore" {
			if gi.IsIgnored(relPath, false) {
				return nil
			}
		}

		return addFile(repo, idx, path, gi)
	})
}
