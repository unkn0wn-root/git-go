package display

import (
	"fmt"
	"strings"
)

type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdded
	DiffLineRemoved
)

type DiffLine struct {
	Type    DiffLineType
	Content string
	OldLine int
	NewLine int
}

type DiffFormatter struct {
	*Formatter
}

func NewDiffFormatter(f *Formatter) *DiffFormatter {
	return &DiffFormatter{Formatter: f}
}

func (df *DiffFormatter) FormatDiffLine(line DiffLine) string {
	var prefix string
	var style Style

	switch line.Type {
	case DiffLineContext:
		prefix = " "
		style = DiffContextStyle
	case DiffLineAdded:
		prefix = "+"
		style = DiffAddedStyle
	case DiffLineRemoved:
		prefix = "-"
		style = DiffRemovedStyle
	}

	return df.Apply(style, prefix+line.Content)
}

func (df *DiffFormatter) FormatDiffHeader(oldPath, newPath string) string {
	var buf strings.Builder
	buf.WriteString(df.Apply(DiffHeaderStyle, fmt.Sprintf("diff --git a/%s b/%s", oldPath, newPath)))
	buf.WriteString("\n")
	buf.WriteString(df.Apply(DiffRemovedStyle, fmt.Sprintf("--- a/%s", oldPath)))
	buf.WriteString("\n")
	buf.WriteString(df.Apply(DiffAddedStyle, fmt.Sprintf("+++ b/%s", newPath)))
	buf.WriteString("\n")
	return buf.String()
}

func (df *DiffFormatter) FormatFileDiff(oldPath, newPath string, lines []DiffLine) string {
	var buf strings.Builder
	buf.WriteString(df.FormatDiffHeader(oldPath, newPath))
	for _, line := range lines {
		buf.WriteString(df.FormatDiffLine(line))
		buf.WriteString("\n")
	}
	return buf.String()
}

func (df *DiffFormatter) FormatNewFile(path string) string { return df.Apply(AddedStyle, fmt.Sprintf("new file: %s", path)) }
func (df *DiffFormatter) FormatDeletedFile(path string) string { return df.Apply(DeletedStyle, fmt.Sprintf("deleted file: %s", path)) }
func (df *DiffFormatter) FormatModifiedFile(path string) string { return df.Apply(ModifiedStyle, fmt.Sprintf("modified: %s", path)) }
func (df *DiffFormatter) FormatRenamedFile(oldPath, newPath string) string { return df.Apply(RenamedStyle, fmt.Sprintf("renamed: %s -> %s", oldPath, newPath)) }
func (df *DiffFormatter) FormatHunkHeader(oldStart, oldCount, newStart, newCount int) string { return df.Apply(DiffHeaderStyle, fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)) }
func (df *DiffFormatter) FormatBinaryDiff(path string) string { return df.Apply(InfoStyle, fmt.Sprintf("Binary file %s differs", path)) }
func (df *DiffFormatter) FormatNoNewlineWarning() string { return df.Apply(WarningStyle, "\\ No newline at end of file") }

func (df *DiffFormatter) FormatDiffSummary(filesChanged, insertions, deletions int) string {
	var parts []string

	if filesChanged > 0 {
		files := "file"
		if filesChanged > 1 {
			files = "files"
		}
		parts = append(parts, df.Apply(InfoStyle, fmt.Sprintf("%d %s changed", filesChanged, files)))
	}

	if insertions > 0 {
		insertion := "insertion"
		if insertions > 1 {
			insertion = "insertions"
		}
		parts = append(parts, df.Apply(DiffAddedStyle, fmt.Sprintf("%d %s(+)", insertions, insertion)))
	}

	if deletions > 0 {
		deletion := "deletion"
		if deletions > 1 {
			deletion = "deletions"
		}
		parts = append(parts, df.Apply(DiffRemovedStyle, fmt.Sprintf("%d %s(-)", deletions, deletion)))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, ", ")
}

func (df *DiffFormatter) FormatCompactDiff(path string, additions, deletions int) string {
	var buf strings.Builder
	buf.WriteString(df.Path(path))
	buf.WriteString(" | ")

	total := additions + deletions
	if total > 0 {
		buf.WriteString(fmt.Sprintf("%d ", total))

		const maxWidth = 20
		if total <= maxWidth {
			buf.WriteString(strings.Repeat(df.Apply(DiffAddedStyle, "+"), additions))
			buf.WriteString(strings.Repeat(df.Apply(DiffRemovedStyle, "-"), deletions))
		} else {
			scale := float64(maxWidth) / float64(total)
			addedWidth := int(float64(additions) * scale)
			deletedWidth := maxWidth - addedWidth

			buf.WriteString(strings.Repeat(df.Apply(DiffAddedStyle, "+"), addedWidth))
			buf.WriteString(strings.Repeat(df.Apply(DiffRemovedStyle, "-"), deletedWidth))
		}
	}

	return buf.String()
}

var defaultDiffFormatter = NewDiffFormatter(defaultFormatter)

func FormatDiffLine(line DiffLine) string { return defaultDiffFormatter.FormatDiffLine(line) }
func FormatDiffHeader(oldPath, newPath string) string { return defaultDiffFormatter.FormatDiffHeader(oldPath, newPath) }
func FormatFileDiff(oldPath, newPath string, lines []DiffLine) string { return defaultDiffFormatter.FormatFileDiff(oldPath, newPath, lines) }
func FormatNewFile(path string) string { return defaultDiffFormatter.FormatNewFile(path) }
func FormatDeletedFile(path string) string { return defaultDiffFormatter.FormatDeletedFile(path) }
func FormatModifiedFile(path string) string { return defaultDiffFormatter.FormatModifiedFile(path) }
func FormatRenamedFile(oldPath, newPath string) string { return defaultDiffFormatter.FormatRenamedFile(oldPath, newPath) }
func FormatHunkHeader(oldStart, oldCount, newStart, newCount int) string { return defaultDiffFormatter.FormatHunkHeader(oldStart, oldCount, newStart, newCount) }
func FormatBinaryDiff(path string) string { return defaultDiffFormatter.FormatBinaryDiff(path) }
func FormatNoNewlineWarning() string { return defaultDiffFormatter.FormatNoNewlineWarning() }
func FormatDiffSummary(filesChanged, insertions, deletions int) string { return defaultDiffFormatter.FormatDiffSummary(filesChanged, insertions, deletions) }
func FormatCompactDiff(path string, additions, deletions int) string { return defaultDiffFormatter.FormatCompactDiff(path, additions, deletions) }
