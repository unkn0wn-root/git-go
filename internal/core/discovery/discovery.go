package discovery

import (
	"os"
	"path/filepath"

	"github.com/unkn0wn-root/git-go/pkg/errors"
)

// FindRepository walks up the directory tree to find a .git directory
func FindRepository(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	current := absPath

	for {
		gitDir := filepath.Join(current, ".git")

		// Check if .git exists and is a directory
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return current, nil
		}

		// Check if .git is a file (for git worktrees)
		if info, err := os.Stat(gitDir); err == nil && !info.IsDir() {
			// Read the .git file to get the actual git directory
			// For now, just return the directory containing the .git file
			return current, nil
		}

		parent := filepath.Dir(current)

		// reached the root directory, stop
		if parent == current {
			break
		}

		current = parent
	}

	return "", errors.ErrNotGitRepository
}

// FindRepositoryFromCwd finds the repository starting from current working directory
func FindRepositoryFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return FindRepository(cwd)
}
