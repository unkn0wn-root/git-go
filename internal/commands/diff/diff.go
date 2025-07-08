package diff

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unkn0wn-root/git-go/pkg/display"
	"github.com/unkn0wn-root/git-go/pkg/errors"
	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
	"github.com/unkn0wn-root/git-go/utils"
)

const (
    maxLinesForMemory = 10000
    chunkSize         = 1000
)

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

type DiffLine struct {
	Type    LineType
	Content string
	OldLine int
	NewLine int
}

type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

type FileDiff struct {
	OldPath string
	NewPath string
	Lines   []DiffLine
	Hunks   []DiffHunk
}

func (fd *FileDiff) String() string {
	if len(fd.Hunks) > 0 {
		hunks := make([]display.DiffHunk, len(fd.Hunks))
		for i, hunk := range fd.Hunks {
			lines := make([]display.DiffLine, len(hunk.Lines))
			for j, line := range hunk.Lines {
				lines[j] = display.DiffLine{
					Type:    display.DiffLineType(line.Type),
					Content: line.Content,
					OldLine: line.OldLine,
					NewLine: line.NewLine,
				}
			}
			hunks[i] = display.DiffHunk{
				OldStart: hunk.OldStart,
				OldCount: hunk.OldCount,
				NewStart: hunk.NewStart,
				NewCount: hunk.NewCount,
				Lines:    lines,
			}
		}
		return display.FormatFileHunks(fd.OldPath, fd.NewPath, hunks)
	}

	// fallback to original line-based format if no hunks
	lines := make([]display.DiffLine, len(fd.Lines))
	for i, line := range fd.Lines {
		lines[i] = display.DiffLine{
			Type:    display.DiffLineType(line.Type),
			Content: line.Content,
			OldLine: line.OldLine,
			NewLine: line.NewLine,
		}
	}

	return display.FormatFileDiff(fd.OldPath, fd.NewPath, lines)
}

func ComputeFileDiff(oldContent, newContent []byte, oldPath, newPath string) *FileDiff {
	return ComputeFileDiffWithContext(oldContent, newContent, oldPath, newPath, 3)
}

func ComputeFileDiffWithContext(oldContent, newContent []byte, oldPath, newPath string, contextLines int) *FileDiff {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)
	// empty files
	if len(oldLines) == 0 && len(newLines) == 0 {
		return &FileDiff{
			OldPath: oldPath,
			NewPath: newPath,
			Lines:   []DiffLine{},
			Hunks:   []DiffHunk{},
		}
	}

	// streaming for large files
	if len(oldLines) > maxLinesForMemory || len(newLines) > maxLinesForMemory {
		return computeLargeFileDiff(oldLines, newLines, oldPath, newPath, contextLines)
	}

	// LCS algorithm to compute optimal diff
	lcs := longestCommonSubsequence(oldLines, newLines)
	diffLines := generateDiffLines(oldLines, newLines, lcs)
	hunks := createOptimizedHunks(diffLines, contextLines)

	return &FileDiff{
		OldPath: oldPath,
		NewPath: newPath,
		Lines:   diffLines,
		Hunks:   hunks,
	}
}

func computeLargeFileDiff(oldLines, newLines []string, oldPath, newPath string, contextLines int) *FileDiff {
	var diffLines []DiffLine

	// sliding window approach to compare chunks
	oldIdx, newIdx := 0, 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		oldChunk := getChunk(oldLines, oldIdx, chunkSize)
		newChunk := getChunk(newLines, newIdx, chunkSize)

		if len(oldChunk) == 0 {
			// only additions remain
			for i, line := range newChunk {
				diffLines = append(diffLines, DiffLine{
					Type:    LineAdded,
					Content: line,
					OldLine: 0,
					NewLine: newIdx + i + 1,
				})
			}
			newIdx += len(newChunk)
		} else if len(newChunk) == 0 {
			// only removals remain
			for i, line := range oldChunk {
				diffLines = append(diffLines, DiffLine{
					Type:    LineRemoved,
					Content: line,
					OldLine: oldIdx + i + 1,
					NewLine: 0,
				})
			}
			oldIdx += len(oldChunk)
		} else {
			// chunks with mini-LCS
			chunkLCS := longestCommonSubsequence(oldChunk, newChunk)
			chunkDiff := generateDiffLines(oldChunk, newChunk, chunkLCS)
			for _, line := range chunkDiff {
				adjustedLine := line
				if line.OldLine > 0 {
					adjustedLine.OldLine += oldIdx
				}
				if line.NewLine > 0 {
					adjustedLine.NewLine += newIdx
				}
				diffLines = append(diffLines, adjustedLine)
			}

			oldIdx += len(oldChunk)
			newIdx += len(newChunk)
		}
	}

	hunks := createOptimizedHunks(diffLines, contextLines)

	return &FileDiff{
		OldPath: oldPath,
		NewPath: newPath,
		Lines:   diffLines,
		Hunks:   hunks,
	}
}

func getChunk(lines []string, start, size int) []string {
	if start >= len(lines) {
		return []string{}
	}
	end := utils.Min(len(lines), start+size)
	return lines[start:end]
}

func ShowWorkingTreeDiff(repo *repository.Repository, paths []string) error {
	idx := index.New(repo.GitDir)
	if err := idx.Load(); err != nil {
		return errors.NewGitError("diff", "", err)
	}

	entries := idx.GetAll()

	for path, entry := range entries {
		if len(paths) > 0 && !utils.ContainsPath(paths, path) {
			continue
		}

		fullPath := filepath.Join(repo.WorkDir, path)
		workingContent, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		obj, err := repo.LoadObject(entry.Hash)
		if err != nil {
			continue
		}

		blob, ok := obj.(*objects.Blob)
		if !ok {
			continue
		}

		indexContent := blob.Content()

		if !bytes.Equal(indexContent, workingContent) {
			fileDiff := ComputeFileDiff(indexContent, workingContent, path, path)
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
			if len(paths) > 0 && !utils.ContainsPath(paths, path) {
				continue
			}
			fmt.Printf("%s\n", display.FormatNewFile(path))
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
		if len(paths) > 0 && !utils.ContainsPath(paths, path) {
			continue
		}

		headHash, existsInHead := headFiles[path]

		if !existsInHead {
			fmt.Printf("%s\n", display.FormatNewFile(path))
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
	// empty sequences
	if m == 0 || n == 0 {
		lcs := make([][]int, m+1)
		for i := range lcs {
			lcs[i] = make([]int, n+1)
		}
		return lcs
	}

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
				lcs[i][j] = utils.Max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}

	return lcs
}

func generateDiffLines(oldLines, newLines []string, lcs [][]int) []DiffLine {
	var result []DiffLine
	i, j := len(oldLines), len(newLines)
	oldLineNum, newLineNum := len(oldLines), len(newLines)

	if len(oldLines) == 0 && len(newLines) == 0 {
		return result
	}

	if len(oldLines) == 0 {
		// all lines are additions
		for idx, line := range newLines {
			result = append(result, DiffLine{
				Type:    LineAdded,
				Content: line,
				OldLine: 0,
				NewLine: idx + 1,
			})
		}
		return result
	}

	if len(newLines) == 0 {
		// All lines are removals
		for idx, line := range oldLines {
			result = append(result, DiffLine{
				Type:    LineRemoved,
				Content: line,
				OldLine: idx + 1,
				NewLine: 0,
			})
		}
		return result
	}

	// backtrack through LCS table to generate diff lines
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

func createOptimizedHunks(diffLines []DiffLine, contextLines int) []DiffHunk {
	if len(diffLines) == 0 {
		return []DiffHunk{}
	}

	// 1 pass: find all change regions
	changeRegions := findChangeRegions(diffLines)
	if len(changeRegions) == 0 {
		return []DiffHunk{}
	}

	// 2 pass: create hunks with context and merge overlapping ones
	hunks := createHunksFromRegions(diffLines, changeRegions, contextLines)

	// 3 pass: merge overlapping hunks and split oversized ones
	hunks = mergeOverlappingHunks(hunks)
	hunks = splitOversizedHunks(hunks, 100) // Max 100 lines per hunk

	return hunks
}

type ChangeRegion struct {
	Start int
	End   int
}

// identifies all continuous regions of changes with caching
func findChangeRegions(diffLines []DiffLine) []ChangeRegion {
	var regions []ChangeRegion
	var currentRegion *ChangeRegion

	changeStatus := make([]bool, len(diffLines))
	for i, line := range diffLines {
		changeStatus[i] = line.Type == LineAdded || line.Type == LineRemoved
	}

	for i, isChange := range changeStatus {
		if isChange {
			if currentRegion == nil {
				currentRegion = &ChangeRegion{Start: i, End: i}
			} else {
				currentRegion.End = i
			}
		} else {
			if currentRegion != nil {
				regions = append(regions, *currentRegion)
				currentRegion = nil
			}
		}
	}

	// handle case where file ends with changes
	if currentRegion != nil {
		regions = append(regions, *currentRegion)
	}

	return regions
}

func createHunksFromRegions(diffLines []DiffLine, regions []ChangeRegion, contextLines int) []DiffHunk {
	var hunks []DiffHunk

	for _, region := range regions {
		// calculate hunk boundaries with context
		hunkStart := utils.Max(0, region.Start-contextLines)
		hunkEnd := utils.Min(len(diffLines), region.End+contextLines+1)

		var hunkLines []DiffLine
		for i := hunkStart; i < hunkEnd; i++ {
			hunkLines = append(hunkLines, diffLines[i])
		}

		if len(hunkLines) > 0 {
			hunk := createHunkFromLines(hunkLines)
			hunks = append(hunks, hunk)
		}
	}

	return hunks
}

// ceate a hunk from a slice of diff lines
func createHunkFromLines(lines []DiffLine) DiffHunk {
	hunk := DiffHunk{Lines: lines}

	// find first line with valid line numbers for start positions
	for _, line := range lines {
		if line.OldLine > 0 || line.NewLine > 0 {
			if line.OldLine > 0 {
				hunk.OldStart = line.OldLine
			} else {
				hunk.OldStart = 1
			}
			if line.NewLine > 0 {
				hunk.NewStart = line.NewLine
			} else {
				hunk.NewStart = 1
			}
			break
		}
	}

	calculateHunkCounts(&hunk)
	return hunk
}

// merges hunks that have overlapping context
func mergeOverlappingHunks(hunks []DiffHunk) []DiffHunk {
	if len(hunks) <= 1 {
		return hunks
	}

	var merged []DiffHunk
	current := hunks[0]

	for i := 1; i < len(hunks); i++ {
		next := hunks[i]

		// check if hunks overlap or are very close
		currentEnd := current.OldStart + current.OldCount
		nextStart := next.OldStart

		if nextStart <= currentEnd + 6 { // Merge if within 6 lines
			current.Lines = append(current.Lines, next.Lines...)
			calculateHunkCounts(&current)
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	merged = append(merged, current)
	return merged
}

// splits hunks that are too large
func splitOversizedHunks(hunks []DiffHunk, maxLines int) []DiffHunk {
	var result []DiffHunk
	for _, hunk := range hunks {
		if len(hunk.Lines) <= maxLines {
			result = append(result, hunk)
			continue
		}

		split := splitHunk(hunk, maxLines)
		result = append(result, split...)
	}

	return result
}

// splits a single hunk into multiple smaller hunks
func splitHunk(hunk DiffHunk, maxLines int) []DiffHunk {
	var result []DiffHunk
	lines := hunk.Lines
	for i := 0; i < len(lines); i += maxLines {
		end := utils.Min(len(lines), i+maxLines)
		subLines := lines[i:end]

		if len(subLines) > 0 {
			subHunk := createHunkFromLines(subLines)
			result = append(result, subHunk)
		}
	}

	return result
}

func hasChangeInRange(diffLines []DiffLine, start, range_ int) bool {
	end := utils.Min(len(diffLines), start+range_)
	for i := start; i < end; i++ {
		if diffLines[i].Type == LineAdded || diffLines[i].Type == LineRemoved {
			return true
		}
	}
	return false
}

// calculateHunkCounts calculates the old and new line counts for a hunk
func calculateHunkCounts(hunk *DiffHunk) {
	oldCount := 0
	newCount := 0

	for _, line := range hunk.Lines {
		switch line.Type {
		case LineContext:
			oldCount++
			newCount++
		case LineRemoved:
			oldCount++
		case LineAdded:
			newCount++
		}
	}

	hunk.OldCount = oldCount
	hunk.NewCount = newCount
}

