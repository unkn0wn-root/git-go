package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type GitIgnore struct {
	patterns []*pattern
}

type pattern struct {
	regex     *regexp.Regexp
	negate    bool
	directory bool
}

func NewGitIgnore(repoRoot string) (*GitIgnore, error) {
	gi := &GitIgnore{}

	// global gitignore patterns first (common ones)
	gi.addGlobalPatterns()

	// .gitignore from repository root (can override global patterns)
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	if err := gi.loadFromFile(gitignorePath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return gi, nil
}

func (gi *GitIgnore) loadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if err := gi.addPattern(line); err != nil {
			continue
		}
	}

	return scanner.Err()
}

func (gi *GitIgnore) addPattern(patternStr string) error {
	p := &pattern{}

	if strings.HasPrefix(patternStr, "!") {
		p.negate = true
		patternStr = patternStr[1:]
	}

	if strings.HasSuffix(patternStr, "/") {
		p.directory = true
		patternStr = patternStr[:len(patternStr)-1]
	}

	regexPattern := gitignoreToRegex(patternStr)
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return err
	}

	p.regex = regex
	gi.patterns = append(gi.patterns, p)

	return nil
}

func (gi *GitIgnore) addGlobalPatterns() {
	// I got an idea to have common ones. Why? Don't know but i thought it would be nice
	commonPatterns := []string{
		".git/",
		"*.tmp",
		"*.swp",
		"*.swo",
		".DS_Store",
		"Thumbs.db",
	}

	for _, pat := range commonPatterns {
		gi.addPattern(pat)
	}
}

func (gi *GitIgnore) IsIgnored(path string, isDir bool) bool {
	matched := false

	for _, p := range gi.patterns {
		if p.directory && !isDir {
			continue
		}

		if p.regex.MatchString(path) || p.regex.MatchString(filepath.Base(path)) {
			if p.negate {
				matched = false
			} else {
				matched = true
			}
		}
	}

	return matched
}

func gitignoreToRegex(patternStr string) string {
	// Escape special regex characters except * and ?
	patternStr = regexp.QuoteMeta(patternStr)

	// Convert gitignore wildcards to regex - order matters
	// **/ first (directory traversal)
	patternStr = strings.ReplaceAll(patternStr, `\*\*/`, `〈DOUBLESTAR_SLASH〉`)
	// /**/ (middle directory traversal)
	patternStr = strings.ReplaceAll(patternStr, `/\*\*/`, `/〈DOUBLESTAR_SLASH〉`)
	// remaining ** (end of path)
	patternStr = strings.ReplaceAll(patternStr, `\*\*`, `〈DOUBLESTAR〉`)
	// single * (filename wildcards)
	patternStr = strings.ReplaceAll(patternStr, `\*`, `[^/]*`)
	// replace placeholders
	patternStr = strings.ReplaceAll(patternStr, `〈DOUBLESTAR_SLASH〉`, `.*`)
	patternStr = strings.ReplaceAll(patternStr, `〈DOUBLESTAR〉`, `.*`)
	// handle ?
	patternStr = strings.ReplaceAll(patternStr, `\?`, `.`)

	// handle leading slash (absolute from repo root)
	if strings.HasPrefix(patternStr, "/") {
		patternStr = "^" + patternStr[1:] + "$"
	} else {
		// can match anywhere in path
		patternStr = "(^|/)" + patternStr + "$"
	}

	return patternStr
}
