package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeFileDiff(t *testing.T) {
	oldContent := []byte("line1\nline2\nline3\n")
	newContent := []byte("line1\nmodified line2\nline3\nline4\n")

	fileDiff := ComputeFileDiff(oldContent, newContent, "test.txt", "test.txt")

	assert.Equal(t, "test.txt", fileDiff.OldPath)
	assert.Equal(t, "test.txt", fileDiff.NewPath)
	assert.NotEmpty(t, fileDiff.Lines)

	hasAddedLine := false
	hasRemovedLine := false
	hasContextLine := false

	for _, line := range fileDiff.Lines {
		switch line.Type {
		case LineAdded:
			hasAddedLine = true
		case LineRemoved:
			hasRemovedLine = true
		case LineContext:
			hasContextLine = true
		}
	}

	assert.True(t, hasAddedLine, "Should have added lines")
	assert.True(t, hasRemovedLine, "Should have removed lines")
	assert.True(t, hasContextLine, "Should have context lines")
}

func TestEmptyFileDiff(t *testing.T) {
	oldContent := []byte("")
	newContent := []byte("new line\n")

	fileDiff := ComputeFileDiff(oldContent, newContent, "test.txt", "test.txt")

	assert.Equal(t, 1, len(fileDiff.Lines))
	assert.Equal(t, LineAdded, fileDiff.Lines[0].Type)
	assert.Equal(t, "new line", fileDiff.Lines[0].Content)
}

func TestIdenticalFiles(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")

	fileDiff := ComputeFileDiff(content, content, "test.txt", "test.txt")

	for _, line := range fileDiff.Lines {
		assert.Equal(t, LineContext, line.Type)
	}
}

func TestLineTypeString(t *testing.T) {
	tests := []struct {
		lineType LineType
		expected string
	}{
		{LineContext, " "},
		{LineAdded, "+"},
		{LineRemoved, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.lineType.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileDiffString(t *testing.T) {
	fileDiff := &FileDiff{
		OldPath: "old.txt",
		NewPath: "new.txt",
		Lines: []DiffLine{
			{Type: LineContext, Content: "unchanged line"},
			{Type: LineRemoved, Content: "removed line"},
			{Type: LineAdded, Content: "added line"},
		},
	}

	result := fileDiff.String()

	assert.Contains(t, result, "--- a/old.txt")
	assert.Contains(t, result, "+++ b/new.txt")
	assert.Contains(t, result, " unchanged line")
	assert.Contains(t, result, "-removed line")
	assert.Contains(t, result, "+added line")
}
