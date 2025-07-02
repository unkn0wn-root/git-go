package blame

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

type BlameLine struct {
	LineNumber int
	Content    string
	CommitHash string
	Author     string
	AuthorTime time.Time
}

type BlameResult struct {
	Path  string
	Lines []BlameLine
}

func (br *BlameResult) String() string {
	var buf strings.Builder

	for _, line := range br.Lines {
		buf.WriteString(fmt.Sprintf("%s (%s %s %d) %s\n",
			line.CommitHash[:8],
			line.Author,
			line.AuthorTime.Format("2006-01-02 15:04:05"),
			line.LineNumber,
			line.Content,
		))
	}

	return buf.String()
}

func BlameFile(repo *repository.Repository, filePath, commitHash string) (*BlameResult, error) {
	if commitHash == "" {
		head, err := repo.GetHead()
		if err != nil {
			return nil, errors.NewGitError("blame", filePath, err)
		}
		commitHash = head
	}

	if commitHash == "" {
		return nil, errors.NewGitError("blame", filePath, fmt.Errorf("no commits found"))
	}

	content, err := getFileContentAtCommit(repo, commitHash, filePath)
	if err != nil {
		return nil, errors.NewGitError("blame", filePath, err)
	}

	lines := splitLines(content)
	blameLines := make([]BlameLine, len(lines))

	for i, line := range lines {
		commit, err := findCommitForLine(repo, commitHash, filePath, i+1)
		if err != nil {
			blameLines[i] = BlameLine{
				LineNumber: i + 1,
				Content:    line,
				CommitHash: commitHash,
				Author:     "Unknown",
				AuthorTime: time.Now(),
			}
			continue
		}

		blameLines[i] = BlameLine{
			LineNumber: i + 1,
			Content:    line,
			CommitHash: commit.Hash(),
			Author:     commit.Author().Name,
			AuthorTime: commit.Author().When,
		}
	}

	return &BlameResult{
		Path:  filePath,
		Lines: blameLines,
	}, nil
}

func getFileContentAtCommit(repo *repository.Repository, commitHash, filePath string) ([]byte, error) {
	commitObj, err := repo.LoadObject(commitHash)
	if err != nil {
		return nil, err
	}

	commit, ok := commitObj.(*objects.Commit)
	if !ok {
		return nil, errors.NewGitError("blame", filePath, fmt.Errorf("object is not a commit"))
	}

	treeObj, err := repo.LoadObject(commit.Tree())
	if err != nil {
		return nil, err
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return nil, errors.NewGitError("blame", filePath, fmt.Errorf("object is not a tree"))
	}

	for _, entry := range tree.Entries() {
		if entry.Name == filePath {
			blobObj, err := repo.LoadObject(entry.Hash)
			if err != nil {
				return nil, err
			}

			blob, ok := blobObj.(*objects.Blob)
			if !ok {
				return nil, errors.NewGitError("blame", filePath, fmt.Errorf("object is not a blob"))
			}

			return blob.Content(), nil
		}
	}

	return nil, errors.NewGitError("blame", filePath, fmt.Errorf("file not found in commit"))
}

func findCommitForLine(repo *repository.Repository, commitHash, filePath string, lineNumber int) (*objects.Commit, error) {
	return findCommitForLineRecursive(repo, commitHash, filePath, lineNumber, make(map[string]bool))
}

func findCommitForLineRecursive(repo *repository.Repository, commitHash, filePath string, lineNumber int, visited map[string]bool) (*objects.Commit, error) {
	// Prevent infinite loops in commit history
	if visited[commitHash] {
		return nil, fmt.Errorf("circular reference detected")
	}
	visited[commitHash] = true

	commitObj, err := repo.LoadObject(commitHash)
	if err != nil {
		return nil, err
	}

	commit, ok := commitObj.(*objects.Commit)
	if !ok {
		return nil, errors.NewGitError("blame", filePath, fmt.Errorf("object is not a commit"))
	}

	parents := commit.Parents()
	if len(parents) == 0 {
		// This is the initial commit
		return commit, nil
	}

	currentContent, err := getFileContentAtCommit(repo, commitHash, filePath)
	if err != nil {
		// File doesn't exist at this commit, try parent
		if len(parents) > 0 {
			return findCommitForLineRecursive(repo, parents[0], filePath, lineNumber, visited)
		}
		return commit, nil
	}

	currentLines := splitLines(currentContent)
	if lineNumber > len(currentLines) {
		// Line doesn't exist in current version
		return commit, nil
	}

	// Track line changes across parent commits
	for _, parentHash := range parents {
		parentContent, err := getFileContentAtCommit(repo, parentHash, filePath)
		if err != nil {
			// File didn't exist in parent, this commit introduced it
			continue
		}

		parentLines := splitLines(parentContent)

		// Map line numbers between current and parent versions
		mappedLine := findLineInParent(currentLines, parentLines, lineNumber)
		if mappedLine > 0 && mappedLine <= len(parentLines) {
			// Line exists in parent, check if it's the same
			if currentLines[lineNumber-1] == parentLines[mappedLine-1] {
				// Line unchanged, continue tracking in parent
				return findCommitForLineRecursive(repo, parentHash, filePath, mappedLine, visited)
			}
		}
	}

	// Line was introduced or modified in this commit
	return commit, nil
}

// findLineInParent maps line numbers between file versions using simple content matching
func findLineInParent(currentLines, parentLines []string, currentLineNum int) int {
	if currentLineNum > len(currentLines) {
		return 0
	}

	currentLine := currentLines[currentLineNum-1]

	// Simple line matching approach (could be enhanced with LCS diff)
	for i, parentLine := range parentLines {
		if parentLine == currentLine {
			// Found matching line, adjust for surrounding context
			offset := findBestOffset(currentLines, parentLines, currentLineNum-1, i)
			return i + 1 + offset
		}
	}

	// Line not found in parent (newly added)
	return 0
}

// findBestOffset improves line mapping accuracy by checking surrounding context
func findBestOffset(currentLines, parentLines []string, currentIdx, parentIdx int) int {
	// Use small context window to find best line alignment
	windowSize := 3
	maxScore := -1
	bestOffset := 0

	for offset := -windowSize; offset <= windowSize; offset++ {
		score := 0
		newParentIdx := parentIdx + offset

		if newParentIdx < 0 || newParentIdx >= len(parentLines) {
			continue
		}

		// Score based on matching surrounding lines
		for i := -windowSize; i <= windowSize; i++ {
			curIdx := currentIdx + i
			parIdx := newParentIdx + i

			if curIdx >= 0 && curIdx < len(currentLines) &&
			   parIdx >= 0 && parIdx < len(parentLines) {
				if currentLines[curIdx] == parentLines[parIdx] {
					score++
				}
			}
		}

		if score > maxScore {
			maxScore = score
			bestOffset = offset
		}
	}

	return bestOffset
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
