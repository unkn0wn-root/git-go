package display

import (
	"fmt"
	"strings"
	"time"
)

type RefUpdateStatus int

const (
	RefUpdateUpToDate RefUpdateStatus = iota
	RefUpdateFastForward
	RefUpdateForced
	RefUpdateOK
	RefUpdateRejected
)

type RefUpdate struct {
	Status  RefUpdateStatus
	OldHash string
	NewHash string
	Reason  string
}

type CommandFormatter struct {
	*Formatter
}

func NewCommandFormatter(f *Formatter) *CommandFormatter {
	return &CommandFormatter{Formatter: f}
}

func (cf *CommandFormatter) FormatPushResult(remote string, updates map[string]RefUpdate, newBranch bool, upstreamSet bool, branch string) string {
	var buf strings.Builder

	if len(updates) == 0 {
		buf.WriteString(cf.Apply(InfoStyle, "Everything up-to-date"))
		return buf.String()
	}

	buf.WriteString(cf.Apply(InfoStyle, fmt.Sprintf("To %s", remote)))
	buf.WriteString("\n")

	for refName, update := range updates {
		branchName := cf.extractBranchName(refName)

		switch update.Status {
		case RefUpdateUpToDate:
			buf.WriteString(fmt.Sprintf("   = %s      %s\n",
				cf.Apply(SecondaryStyle, "[up to date]"),
				cf.Branch(branchName)))
		case RefUpdateFastForward:
			buf.WriteString(fmt.Sprintf("   %s..%s  %s\n",
				cf.Hash(update.OldHash),
				cf.Hash(update.NewHash),
				cf.Branch(branchName)))
		case RefUpdateForced:
			buf.WriteString(fmt.Sprintf(" + %s...%s %s %s\n",
				cf.Hash(update.OldHash),
				cf.Hash(update.NewHash),
				cf.Branch(branchName),
				cf.Apply(WarningStyle, "(forced update)")))
		case RefUpdateOK:
			if newBranch {
				buf.WriteString(fmt.Sprintf(" * %s      %s -> %s\n",
					cf.Apply(SuccessStyle, "[new branch]"),
					cf.Branch(branch),
					cf.Branch(branchName)))
			} else {
				buf.WriteString(fmt.Sprintf("   %s..%s  %s\n",
					cf.Hash(update.OldHash),
					cf.Hash(update.NewHash),
					cf.Branch(branchName)))
			}
		case RefUpdateRejected:
			buf.WriteString(fmt.Sprintf(" ! %s        %s %s\n",
				cf.Apply(ErrorStyle, "[rejected]"),
				cf.Branch(branchName),
				cf.Apply(ErrorStyle, fmt.Sprintf("(%s)", update.Reason))))
		}
	}

	if upstreamSet {
		buf.WriteString(fmt.Sprintf("Branch '%s' set up to track remote branch '%s' from '%s'.\n",
			cf.Branch(branch),
			cf.Branch(branch),
			cf.Apply(InfoStyle, remote)))
	}

	return buf.String()
}

func (cf *CommandFormatter) FormatCloneProgress(repo, progress string) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Cloning into %s...\n", cf.Apply(EmphasisStyle, repo)))
	if progress != "" {
		buf.WriteString(cf.Apply(ProgressStyle, progress))
		buf.WriteString("\n")
	}
	return buf.String()
}

func (cf *CommandFormatter) FormatCommitResult(hash, branch, message string, filesChanged, insertions, deletions int) string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("[%s %s] %s\n",
		cf.Branch(branch),
		cf.Hash(hash),
		message))

	if filesChanged > 0 {
		parts := []string{
			fmt.Sprintf("%d file%s changed", filesChanged, pluralS(filesChanged)),
		}

		if insertions > 0 {
			parts = append(parts, cf.Apply(DiffAddedStyle, fmt.Sprintf("%d insertion%s(+)", insertions, pluralS(insertions))))
		}

		if deletions > 0 {
			parts = append(parts, cf.Apply(DiffRemovedStyle, fmt.Sprintf("%d deletion%s(-)", deletions, pluralS(deletions))))
		}

		buf.WriteString(" ")
		buf.WriteString(strings.Join(parts, ", "))
		buf.WriteString("\n")
	}

	return buf.String()
}

func (cf *CommandFormatter) FormatResetResult(mode, target string, filesChanged int) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("HEAD is now at %s\n", cf.Hash(target)))
	if filesChanged > 0 {
		buf.WriteString(fmt.Sprintf("%d file%s changed", filesChanged, pluralS(filesChanged)))
	}
	return buf.String()
}

func (cf *CommandFormatter) FormatRemoteResult(operation, remote, url string) string {
	switch operation {
	case "add":
		return fmt.Sprintf("Added remote %s: %s", cf.Apply(SuccessStyle, remote), cf.Apply(InfoStyle, url))
	case "remove":
		return fmt.Sprintf("Removed remote %s", cf.Apply(SuccessStyle, remote))
	case "rename":
		return fmt.Sprintf("Renamed remote %s to %s", cf.Apply(InfoStyle, remote), cf.Apply(SuccessStyle, url))
	default:
		return ""
	}
}

func (cf *CommandFormatter) FormatRemoteList(remotes map[string]string, verbose bool) string {
	var buf strings.Builder
	for name, url := range remotes {
		if verbose {
			buf.WriteString(fmt.Sprintf("%s\t%s (fetch)\n", cf.Apply(SuccessStyle, name), cf.Apply(InfoStyle, url)))
			buf.WriteString(fmt.Sprintf("%s\t%s (push)\n", cf.Apply(SuccessStyle, name), cf.Apply(InfoStyle, url)))
		} else {
			buf.WriteString(fmt.Sprintf("%s\n", cf.Apply(SuccessStyle, name)))
		}
	}
	return buf.String()
}

func (cf *CommandFormatter) FormatInitResult(path string, bare bool) string {
	var msg string
	if bare {
		msg = fmt.Sprintf("Initialized empty Git repository in %s", cf.Path(path))
	} else {
		msg = fmt.Sprintf("Initialized empty Git repository in %s/.git/", cf.Path(path))
	}
	return cf.Apply(SuccessStyle, msg)
}

func (cf *CommandFormatter) FormatBranchList(branches []string, current string) string {
	var buf strings.Builder
	for _, branch := range branches {
		if branch == current {
			buf.WriteString(cf.Apply(SuccessStyle, "* "))
			buf.WriteString(cf.Branch(branch))
		} else {
			buf.WriteString("  ")
			buf.WriteString(cf.Apply(SecondaryStyle, branch))
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

func (cf *CommandFormatter) FormatProgressWithStats(message string, current, total int) string {
	percentage := float64(current) / float64(total) * 100
	return fmt.Sprintf("%s: %s (%d/%d, %.1f%%)",
		cf.Apply(ProgressStyle, message),
		cf.Apply(InfoStyle, "●"),
		current, total, percentage)
}

func (cf *CommandFormatter) FormatHintMessage(lines []string) string {
	var buf strings.Builder
	for _, line := range lines {
		buf.WriteString(cf.Hint(fmt.Sprintf("hint: %s", line)))
		buf.WriteString("\n")
	}
	return buf.String()
}

func (cf *CommandFormatter) extractBranchName(refName string) string {
	if strings.HasPrefix(refName, "refs/heads/") {
		return refName[11:]
	}
	if strings.HasPrefix(refName, "refs/tags/") {
		return refName[10:]
	}
	return refName
}

func pluralS(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (cf *CommandFormatter) FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return cf.Apply(InfoStyle, fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp]))
}

func (cf *CommandFormatter) FormatDuration(d time.Duration) string {
	if d < time.Second {
		return cf.Apply(InfoStyle, fmt.Sprintf("%dms", d.Nanoseconds()/1000000))
	}
	if d < time.Minute {
		return cf.Apply(InfoStyle, fmt.Sprintf("%.1fs", d.Seconds()))
	}
	return cf.Apply(InfoStyle, fmt.Sprintf("%.1fm", d.Minutes()))
}

var defaultCommandFormatter = NewCommandFormatter(defaultFormatter)

func FormatPushResult(remote string, updates map[string]RefUpdate, newBranch bool, upstreamSet bool, branch string) string {
	return defaultCommandFormatter.FormatPushResult(remote, updates, newBranch, upstreamSet, branch)
}
func FormatCloneProgress(repo, progress string) string {
	return defaultCommandFormatter.FormatCloneProgress(repo, progress)
}
func FormatCommitResult(hash, branch, message string, filesChanged, insertions, deletions int) string {
	return defaultCommandFormatter.FormatCommitResult(hash, branch, message, filesChanged, insertions, deletions)
}
func FormatResetResult(mode, target string, filesChanged int) string {
	return defaultCommandFormatter.FormatResetResult(mode, target, filesChanged)
}
func FormatRemoteResult(operation, remote, url string) string {
	return defaultCommandFormatter.FormatRemoteResult(operation, remote, url)
}
func FormatRemoteList(remotes map[string]string, verbose bool) string {
	return defaultCommandFormatter.FormatRemoteList(remotes, verbose)
}
func FormatInitResult(path string, bare bool) string {
	return defaultCommandFormatter.FormatInitResult(path, bare)
}
func FormatBranchList(branches []string, current string) string {
	return defaultCommandFormatter.FormatBranchList(branches, current)
}
func FormatProgressWithStats(message string, current, total int) string {
	return defaultCommandFormatter.FormatProgressWithStats(message, current, total)
}
func FormatHintMessage(lines []string) string {
	return defaultCommandFormatter.FormatHintMessage(lines)
}
func FormatBytes(bytes int64) string        { return defaultCommandFormatter.FormatBytes(bytes) }
func FormatDuration(d time.Duration) string { return defaultCommandFormatter.FormatDuration(d) }
