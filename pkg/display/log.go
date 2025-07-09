package display

import (
	"fmt"
	"strings"
	"time"
)

type AuthorStats struct {
	Commits      int
	LinesAdded   int
	LinesRemoved int
	FirstCommit  time.Time
	LastCommit   time.Time
}

type LogEntry struct {
	Hash    string
	Author  string
	Email   string
	Date    time.Time
	Message string
	Parents []string
	IsMerge bool
}

type LogOptions struct {
	Oneline    bool
	ShowGraph  bool
	ShowStats  bool
	MaxEntries int
	Since      *time.Time
	Until      *time.Time
}

type LogFormatter struct {
	*Formatter
}

func NewLogFormatter(f *Formatter) *LogFormatter {
	return &LogFormatter{Formatter: f}
}

func (lf *LogFormatter) FormatLogEntry(entry LogEntry, options LogOptions) string {
	if options.Oneline {
		return lf.formatOnelineEntry(entry)
	}
	return lf.formatFullEntry(entry)
}

func (lf *LogFormatter) formatOnelineEntry(entry LogEntry) string {
	var buf strings.Builder
	buf.WriteString(lf.Hash(entry.Hash))
	buf.WriteString(" ")
	firstLine := strings.Split(entry.Message, "\n")[0]
	buf.WriteString(firstLine)
	return buf.String()
}

func (lf *LogFormatter) formatFullEntry(entry LogEntry) string {
	var buf strings.Builder

	buf.WriteString(lf.Apply(HashStyle, fmt.Sprintf("commit %s", entry.Hash)))
	buf.WriteString("\n")

	if entry.IsMerge && len(entry.Parents) > 1 {
		buf.WriteString(fmt.Sprintf("Merge: %s",
			strings.Join(lf.formatParentHashes(entry.Parents), " ")))
		buf.WriteString("\n")
	}

	buf.WriteString(fmt.Sprintf("Author: %s <%s>",
		lf.Apply(EmphasisStyle, entry.Author),
		lf.Apply(InfoStyle, entry.Email)))
	buf.WriteString("\n")

	buf.WriteString(fmt.Sprintf("Date:   %s",
		lf.formatDate(entry.Date)))
	buf.WriteString("\n")

	buf.WriteString("\n")
	buf.WriteString(lf.formatCommitMessage(entry.Message))
	buf.WriteString("\n")

	return buf.String()
}

func (lf *LogFormatter) formatParentHashes(parents []string) []string {
	var formatted []string
	for _, parent := range parents {
		formatted = append(formatted, lf.Hash(parent))
	}
	return formatted
}

func (lf *LogFormatter) formatDate(date time.Time) string {
	return lf.Apply(InfoStyle, date.Format("Mon Jan 2 15:04:05 2006 -0700"))
}

func (lf *LogFormatter) formatCommitMessage(message string) string {
	lines := strings.Split(message, "\n")
	var buf strings.Builder

	for i, line := range lines {
		if i == 0 {
			buf.WriteString(fmt.Sprintf("    %s", lf.Apply(EmphasisStyle, line)))
		} else if strings.TrimSpace(line) == "" {
			buf.WriteString("    ")
		} else {
			buf.WriteString(fmt.Sprintf("    %s", line))
		}

		if i < len(lines)-1 {
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

func (lf *LogFormatter) FormatLogGraph(entries []LogEntry, options LogOptions) string {
	var buf strings.Builder

	for i, entry := range entries {
		if i == 0 {
			buf.WriteString(lf.Apply(SuccessStyle, "● "))
		} else {
			buf.WriteString(lf.Apply(SecondaryStyle, "│ "))
		}

		buf.WriteString(lf.FormatLogEntry(entry, options))

		if i < len(entries)-1 {
			buf.WriteString("\n")
			if !options.Oneline {
				buf.WriteString(lf.Apply(SecondaryStyle, "│"))
				buf.WriteString("\n")
			}
		}
	}

	return buf.String()
}

func (lf *LogFormatter) FormatLogStats(totalCommits int, authors map[string]int, dateRange string) string {
	var buf strings.Builder

	buf.WriteString(lf.Apply(InfoStyle, fmt.Sprintf("Total commits: %d", totalCommits)))
	buf.WriteString("\n")

	if dateRange != "" {
		buf.WriteString(lf.Apply(InfoStyle, fmt.Sprintf("Date range: %s", dateRange)))
		buf.WriteString("\n")
	}

	if len(authors) > 0 {
		buf.WriteString(lf.Apply(InfoStyle, "Authors:"))
		buf.WriteString("\n")

		for author, count := range authors {
			buf.WriteString(fmt.Sprintf("  %s: %s",
				lf.Apply(EmphasisStyle, author),
				lf.Apply(SuccessStyle, fmt.Sprintf("%d commits", count))))
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

func (lf *LogFormatter) FormatShortLog(entries []LogEntry, maxWidth int) string {
	var buf strings.Builder

	for _, entry := range entries {
		buf.WriteString(lf.Hash(entry.Hash))
		buf.WriteString(" ")

		author := entry.Author
		if len(author) > 15 {
			author = author[:12] + "..."
		}
		buf.WriteString(fmt.Sprintf("(%s) ", lf.Apply(SecondaryStyle, author)))

		message := strings.Split(entry.Message, "\n")[0]
		remaining := maxWidth - len(entry.Hash) - len(author) - 10
		if remaining > 0 && len(message) > remaining {
			message = message[:remaining-3] + "..."
		}
		buf.WriteString(message)
		buf.WriteString("\n")
	}

	return buf.String()
}

func (lf *LogFormatter) FormatBranchLog(branch string, entries []LogEntry, options LogOptions) string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("Commits on branch %s:\n\n", lf.Branch(branch)))

	if options.ShowGraph {
		buf.WriteString(lf.FormatLogGraph(entries, options))
	} else {
		for i, entry := range entries {
			buf.WriteString(lf.FormatLogEntry(entry, options))
			if i < len(entries)-1 {
				buf.WriteString("\n")
				if !options.Oneline {
					buf.WriteString("\n")
				}
			}
		}
	}

	return buf.String()
}

func (lf *LogFormatter) FormatMergeCommit(entry LogEntry) string {
	var buf strings.Builder
	buf.WriteString(lf.Apply(MergeStyle, "●"))
	buf.WriteString(" ")
	buf.WriteString(lf.Hash(entry.Hash))
	buf.WriteString(" ")
	firstLine := strings.Split(entry.Message, "\n")[0]
	buf.WriteString(lf.Apply(EmphasisStyle, firstLine))
	return buf.String()
}

func (lf *LogFormatter) FormatCommitRange(from, to string, count int) string {
	return fmt.Sprintf("Showing %d commits from %s to %s",
		count,
		lf.Hash(from),
		lf.Hash(to))
}

func (lf *LogFormatter) FormatAuthorStats(stats map[string]AuthorStats) string {
	var buf strings.Builder

	buf.WriteString(lf.Apply(InfoStyle, "Author Statistics:"))
	buf.WriteString("\n\n")

	for author, stat := range stats {
		buf.WriteString(fmt.Sprintf("%s:\n", lf.Apply(EmphasisStyle, author)))
		buf.WriteString(fmt.Sprintf("  Commits: %s\n",
			lf.Apply(SuccessStyle, fmt.Sprintf("%d", stat.Commits))))
		buf.WriteString(fmt.Sprintf("  Lines added: %s\n",
			lf.Apply(DiffAddedStyle, fmt.Sprintf("+%d", stat.LinesAdded))))
		buf.WriteString(fmt.Sprintf("  Lines removed: %s\n",
			lf.Apply(DiffRemovedStyle, fmt.Sprintf("-%d", stat.LinesRemoved))))
		buf.WriteString(fmt.Sprintf("  First commit: %s\n",
			lf.Apply(InfoStyle, stat.FirstCommit.Format("2006-01-02"))))
		buf.WriteString(fmt.Sprintf("  Last commit: %s\n",
			lf.Apply(InfoStyle, stat.LastCommit.Format("2006-01-02"))))
		buf.WriteString("\n")
	}

	return buf.String()
}

var MergeStyle = Style{color: Magenta, bold: true}
var defaultLogFormatter = NewLogFormatter(defaultFormatter)

func FormatLogEntry(entry LogEntry, options LogOptions) string {
	return defaultLogFormatter.FormatLogEntry(entry, options)
}
func FormatLogGraph(entries []LogEntry, options LogOptions) string {
	return defaultLogFormatter.FormatLogGraph(entries, options)
}
func FormatLogStats(totalCommits int, authors map[string]int, dateRange string) string {
	return defaultLogFormatter.FormatLogStats(totalCommits, authors, dateRange)
}
func FormatShortLog(entries []LogEntry, maxWidth int) string {
	return defaultLogFormatter.FormatShortLog(entries, maxWidth)
}
func FormatBranchLog(branch string, entries []LogEntry, options LogOptions) string {
	return defaultLogFormatter.FormatBranchLog(branch, entries, options)
}
func FormatMergeCommit(entry LogEntry) string { return defaultLogFormatter.FormatMergeCommit(entry) }
func FormatCommitRange(from, to string, count int) string {
	return defaultLogFormatter.FormatCommitRange(from, to, count)
}
func FormatAuthorStats(stats map[string]AuthorStats) string {
	return defaultLogFormatter.FormatAuthorStats(stats)
}
