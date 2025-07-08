package display

import (
	"fmt"
	"strings"
)

type FileStatus int

const (
	FileStatusUntracked FileStatus = iota
	FileStatusAdded
	FileStatusModified
	FileStatusDeleted
	FileStatusRenamed
	FileStatusUnmodified
)

type StatusFormatter struct {
	*Formatter
}

func NewStatusFormatter(f *Formatter) *StatusFormatter {
	return &StatusFormatter{Formatter: f}
}

func (sf *StatusFormatter) FormatFileStatus(status FileStatus) string {
	switch status {
	case FileStatusUntracked:
		return sf.Apply(UntrackedStyle, "??")
	case FileStatusAdded:
		return sf.Apply(AddedStyle, "A ")
	case FileStatusModified:
		return sf.Apply(ModifiedStyle, "M ")
	case FileStatusDeleted:
		return sf.Apply(DeletedStyle, "D ")
	case FileStatusRenamed:
		return sf.Apply(RenamedStyle, "R ")
	default:
		return "  "
	}
}

func (sf *StatusFormatter) FormatBranchHeader(branch string, isInitial bool) string {
	var buf strings.Builder
	buf.WriteString("On branch ")
	buf.WriteString(sf.Branch(branch))
	if isInitial {
		buf.WriteString("\n\n")
		buf.WriteString(sf.Apply(InfoStyle, "No commits yet"))
		buf.WriteString("\n")
	}
	return buf.String()
}

func (sf *StatusFormatter) FormatStagedSection(entries []StatusEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("\n")
	buf.WriteString(sf.Apply(StagedStyle, "Changes to be committed:"))
	buf.WriteString("\n")
	buf.WriteString(sf.Hint("  (use \"git reset HEAD <file>...\" to unstage)"))
	buf.WriteString("\n\n")
	for _, entry := range entries {
		buf.WriteString(fmt.Sprintf("  %s %s\n",
			sf.FormatFileStatus(entry.IndexStatus),
			sf.Path(entry.Path)))
	}
	return buf.String()
}

func (sf *StatusFormatter) FormatUnstagedSection(entries []StatusEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("\n")
	buf.WriteString(sf.Apply(UnstagedStyle, "Changes not staged for commit:"))
	buf.WriteString("\n")
	buf.WriteString(sf.Hint("  (use \"git add <file>...\" to update what will be committed)"))
	buf.WriteString("\n")
	buf.WriteString(sf.Hint("  (use \"git checkout -- <file>...\" to discard changes in working directory)"))
	buf.WriteString("\n\n")
	for _, entry := range entries {
		buf.WriteString(fmt.Sprintf("  %s %s\n",
			sf.FormatFileStatus(entry.WorkStatus),
			sf.Path(entry.Path)))
	}
	return buf.String()
}

func (sf *StatusFormatter) FormatUntrackedSection(entries []StatusEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString("\n")
	buf.WriteString(sf.Apply(UntrackedStyle, "Untracked files:"))
	buf.WriteString("\n")
	buf.WriteString(sf.Hint("  (use \"git add <file>...\" to include in what will be committed)"))
	buf.WriteString("\n\n")
	for _, entry := range entries {
		buf.WriteString(fmt.Sprintf("  %s\n", sf.Apply(UntrackedStyle, entry.Path)))
	}
	return buf.String()
}

func (sf *StatusFormatter) FormatCleanMessage() string {
	return sf.Apply(SuccessStyle, "nothing to commit, working tree clean")
}

type StatusEntry struct {
	Path        string
	IndexStatus FileStatus
	WorkStatus  FileStatus
}

func (sf *StatusFormatter) FormatStatusResult(branch string, entries []StatusEntry, isInitial bool) string {
	var buf strings.Builder
	buf.WriteString(sf.FormatBranchHeader(branch, isInitial))

	var staged, unstaged, untracked []StatusEntry
	for _, entry := range entries {
		if entry.IndexStatus != FileStatusUnmodified {
			staged = append(staged, entry)
		}
		if entry.WorkStatus == FileStatusUntracked {
			untracked = append(untracked, entry)
		} else if entry.WorkStatus != FileStatusUnmodified {
			unstaged = append(unstaged, entry)
		}
	}

	buf.WriteString(sf.FormatStagedSection(staged))
	buf.WriteString(sf.FormatUnstagedSection(unstaged))
	buf.WriteString(sf.FormatUntrackedSection(untracked))

	if len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0 {
		buf.WriteString("\n")
		buf.WriteString(sf.FormatCleanMessage())
	}

	buf.WriteString("\n")
	return buf.String()
}

var defaultStatusFormatter = NewStatusFormatter(defaultFormatter)

func FormatFileStatus(status FileStatus) string { return defaultStatusFormatter.FormatFileStatus(status) }
func FormatBranchHeader(branch string, isInitial bool) string { return defaultStatusFormatter.FormatBranchHeader(branch, isInitial) }
func FormatStagedSection(entries []StatusEntry) string { return defaultStatusFormatter.FormatStagedSection(entries) }
func FormatUnstagedSection(entries []StatusEntry) string { return defaultStatusFormatter.FormatUnstagedSection(entries) }
func FormatUntrackedSection(entries []StatusEntry) string { return defaultStatusFormatter.FormatUntrackedSection(entries) }
func FormatCleanMessage() string { return defaultStatusFormatter.FormatCleanMessage() }
func FormatStatusResult(branch string, entries []StatusEntry, isInitial bool) string { return defaultStatusFormatter.FormatStatusResult(branch, entries, isInitial) }
