package log

import (
	"fmt"
	"strings"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/hash"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/repository"
)

type LogEntry struct {
	Hash      string
	Author    *objects.Signature
	Committer *objects.Signature
	Message   string
	Parents   []string
}

type LogOptions struct {
	MaxCount int
	Oneline  bool
	Graph    bool
}

func (le *LogEntry) String(options LogOptions) string {
	if options.Oneline {
		shortHash := hash.ShortHash(le.Hash, 7)
		messageLine := strings.Split(le.Message, "\n")[0]
		return fmt.Sprintf("%s %s", shortHash, messageLine)
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("commit %s\n", le.Hash))

	if le.Author.Name != le.Committer.Name || le.Author.Email != le.Committer.Email ||
	   le.Author.When.Unix() != le.Committer.When.Unix() {
		buf.WriteString(fmt.Sprintf("Author:     %s\n", le.Author.String()))
		buf.WriteString(fmt.Sprintf("AuthorDate: %s\n", le.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700")))
		buf.WriteString(fmt.Sprintf("Commit:     %s\n", le.Committer.String()))
		buf.WriteString(fmt.Sprintf("CommitDate: %s\n", le.Committer.When.Format("Mon Jan 2 15:04:05 2006 -0700")))
	} else {
		buf.WriteString(fmt.Sprintf("Author: %s\n", le.Author.String()))
		buf.WriteString(fmt.Sprintf("Date:   %s\n", le.Author.When.Format("Mon Jan 2 15:04:05 2006 -0700")))
	}

	buf.WriteString("\n")

	// Indent message
	messageLines := strings.Split(le.Message, "\n")
	for _, line := range messageLines {
		if line != "" {
			buf.WriteString(fmt.Sprintf("    %s\n", line))
		} else {
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

func GetLog(repo *repository.Repository, options LogOptions) ([]LogEntry, error) {
	if !repo.Exists() {
		return nil, errors.ErrNotGitRepository
	}

	headHash, err := repo.GetHead()
	if err != nil || headHash == "" {
		return []LogEntry{}, nil // No commits yet
	}

	var entries []LogEntry
	visited := make(map[string]bool)

	err = walkCommits(repo, headHash, &entries, visited, options)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func walkCommits(repo *repository.Repository, commitHash string, entries *[]LogEntry, visited map[string]bool, options LogOptions) error {
	if visited[commitHash] {
		return nil
	}

	if options.MaxCount > 0 && len(*entries) >= options.MaxCount {
		return nil
	}

	visited[commitHash] = true

	commitObj, err := repo.LoadObject(commitHash)
	if err != nil {
		return errors.NewGitError("log", "", fmt.Errorf("load commit %s: %w", commitHash, err))
	}

	commit, ok := commitObj.(*objects.Commit)
	if !ok {
		return errors.NewGitError("log", "", fmt.Errorf("object %s is not a commit", commitHash))
	}

	entry := LogEntry{
		Hash:      commitHash,
		Author:    commit.Author(),
		Committer: commit.Committer(),
		Message:   commit.Message(),
		Parents:   commit.Parents(),
	}

	*entries = append(*entries, entry)

	// Continue with parents
	for _, parentHash := range commit.Parents() {
		if options.MaxCount > 0 && len(*entries) >= options.MaxCount {
			break
		}
		err := walkCommits(repo, parentHash, entries, visited, options)
		if err != nil {
			return err
		}
	}

	return nil
}

func ShowLog(repo *repository.Repository, options LogOptions) error {
	entries, err := GetLog(repo, options)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No commits yet")
		return nil
	}

	for i, entry := range entries {
		fmt.Print(entry.String(options))

		// add separator between commits (except for last one and oneline format)
		if !options.Oneline && i < len(entries)-1 {
			fmt.Println()
		}
	}

	return nil
}
