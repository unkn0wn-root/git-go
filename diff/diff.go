package diff

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

type DiffLine struct {
	Type    LineType
	Content string
	OldLine int
	NewLine int
}

type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

func (t LineType) String() string {
	switch t {
	case LineContext:
		return " "
	case LineAdded:
		return "+"
	case LineRemoved:
		return "-"
	default:
		return " "
	}
}

type FileDiff struct {
	OldPath string
	NewPath string
	Lines   []DiffLine
}

func (fd *FileDiff) String() string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("--- a/%s\n", fd.OldPath))
	buf.WriteString(fmt.Sprintf("+++ b/%s\n", fd.NewPath))

	for _, line := range fd.Lines {
		buf.WriteString(fmt.Sprintf("%s%s\n", line.Type.String(), line.Content))
	}

	return buf.String()
}

func ComputeFileDiff(oldContent, newContent []byte, oldPath, newPath string) *FileDiff {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	// Use LCS algorithm to compute optimal diff
	lcs := longestCommonSubsequence(oldLines, newLines)
	diffLines := generateDiffLines(oldLines, newLines, lcs)

	return &FileDiff{
		OldPath: oldPath,
		NewPath: newPath,
		Lines:   diffLines,
	}
}

func ShowWorkingTreeDiff(repo *repository.Repository, paths []string) error {
	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return errors.NewGitError("diff", "", err)
	}

	entries := idx.GetAll()

	for path, entry := range entries {
		if len(paths) > 0 && !containsPath(paths, path) {
			continue
		}

		fullPath := filepath.Join(repo.WorkDir, path)
		workingContent, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		obj, err := repo.LoadObject(entry.Hash)
		if err != nil {
			return errors.NewGitError("diff", path, fmt.Errorf("%s: %w", entry.Hash, err))
		}

		blob, ok := obj.(*objects.Blob)
		if !ok {
			continue
		}

		indexContent := blob.Content()

		if !bytes.Equal(indexContent, workingContent) {
			fileDiff := ComputeFileDiff(indexContent, workingContent, path, path)
			fmt.Printf("diff --git a/%s b/%s\n", path, path)
			fmt.Print(fileDiff.String())
		}
	}

	return nil
}

func ShowStagedDiff(repo *repository.Repository, paths []string) error {
	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return errors.NewGitError("diff", "", err)
	}

	headHash, err := repo.GetHead()
	if err != nil {
		return errors.NewGitError("diff", "", err)
	}

	if headHash == "" {
		entries := idx.GetAll()
		for path := range entries {
			if len(paths) > 0 && !containsPath(paths, path) {
				continue
			}
			fmt.Printf("new file: %s\n", path)
		}
		return nil
	}

	headCommit, err := repo.LoadObject(headHash)
	if err != nil {
		return errors.NewGitError("diff", "", fmt.Errorf("load HEAD commit: %w", err))
	}

	commit, ok := headCommit.(*objects.Commit)
	if !ok {
		return errors.NewGitError("diff", "", fmt.Errorf("HEAD is not a commit"))
	}

	headTree, err := repo.LoadObject(commit.Tree())
	if err != nil {
		return errors.NewGitError("diff", "", fmt.Errorf("load HEAD tree: %w", err))
	}

	tree, ok := headTree.(*objects.Tree)
	if !ok {
		return errors.NewGitError("diff", "", fmt.Errorf("HEAD tree is not a tree object"))
	}

	headFiles := make(map[string]string)
	for _, entry := range tree.Entries() {
		headFiles[entry.Name] = entry.Hash
	}

	entries := idx.GetAll()
	for path, entry := range entries {
		if len(paths) > 0 && !containsPath(paths, path) {
			continue
		}

		headHash, existsInHead := headFiles[path]

		if !existsInHead {
			fmt.Printf("new file: %s\n", path)
			continue
		}

		if headHash == entry.Hash {
			continue
		}

		headObj, err := repo.LoadObject(headHash)
		if err != nil {
			return errors.NewGitError("diff", path, fmt.Errorf("load HEAD object: %w", err))
		}

		indexObj, err := repo.LoadObject(entry.Hash)
		if err != nil {
			return errors.NewGitError("diff", path, fmt.Errorf("load index object: %w", err))
		}

		headBlob, ok := headObj.(*objects.Blob)
		if !ok {
			continue
		}

		indexBlob, ok := indexObj.(*objects.Blob)
		if !ok {
			continue
		}

		fileDiff := ComputeFileDiff(headBlob.Content(), indexBlob.Content(), path, path)
		fmt.Printf("diff --git a/%s b/%s\n", path, path)
		fmt.Print(fileDiff.String())
	}

	return nil
}

func splitLines(content []byte) []string {
	if len(content) == 0 {
		return []string{}
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

func longestCommonSubsequence(a, b []string) [][]int {
	m, n := len(a), len(b)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	// longest common subsequence
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				lcs[i][j] = max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}

	return lcs
}

func generateDiffLines(oldLines, newLines []string, lcs [][]int) []DiffLine {
	var result []DiffLine
	i, j := len(oldLines), len(newLines)
	oldLineNum, newLineNum := len(oldLines), len(newLines)

	// Backtrack through LCS table to generate diff lines
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			result = append([]DiffLine{{
				Type:    LineContext,
				Content: oldLines[i-1],
				OldLine: oldLineNum,
				NewLine: newLineNum,
			}}, result...)
			i--
			j--
			oldLineNum--
			newLineNum--
		} else if i > 0 && (j == 0 || lcs[i-1][j] >= lcs[i][j-1]) {
			result = append([]DiffLine{{
				Type:    LineRemoved,
				Content: oldLines[i-1],
				OldLine: oldLineNum,
				NewLine: 0,
			}}, result...)
			i--
			oldLineNum--
		} else {
			result = append([]DiffLine{{
				Type:    LineAdded,
				Content: newLines[j-1],
				OldLine: 0,
				NewLine: newLineNum,
			}}, result...)
			j--
			newLineNum--
		}
	}

	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}
