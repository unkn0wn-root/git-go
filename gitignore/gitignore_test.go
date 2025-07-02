package gitignore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitIgnore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	gitignoreContent := "*.log\n*.tmp\n/build/\n"
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	require.NoError(t, err)

	gi, err := NewGitIgnore(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, gi)
	assert.NotEmpty(t, gi.patterns)
}

func TestNewGitIgnoreNoFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	gi, err := NewGitIgnore(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, gi)
	assert.NotEmpty(t, gi.patterns)
}

func TestIsIgnored(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	gitignoreContent := "*.log\n*.tmp\n/build/\n!important.log\n"
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	require.NoError(t, err)

	gi, err := NewGitIgnore(tmpDir)
	require.NoError(t, err)

	tests := []struct {
		path     string
		isDir    bool
		expected bool
	}{
		{"file.log", false, true},     // Matches *.log
		{"file.tmp", false, true},     // Matches *.tmp
		{"file.txt", false, false},    // No match
		{"build", true, true},         // Matches /build/ (directory)
		{"build/file.txt", false, false}, // Directory pattern doesn't match files
		{"important.log", false, false},   // Negated by !important.log
		{"src/file.log", false, true},     // *.log matches anywhere
		{".DS_Store", false, true},        // Global pattern
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := gi.IsIgnored(tt.path, tt.isDir)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitignoreToRegex(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{"*.log", "(^|/)[^/]*\\.log$"},
		{"file.txt", "(^|/)file\\.txt$"},
		{"/build/", "^build/$"},
		{"**/*.js", "(^|/).*[^/]*\\.js$"},
		{"src/*", "(^|/)src/[^/]*$"},
		{"test?", "(^|/)test.$"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := gitignoreToRegex(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddPattern(t *testing.T) {
	gi := &GitIgnore{}

	err := gi.addPattern("*.log")
	require.NoError(t, err)
	assert.Len(t, gi.patterns, 1)
	assert.False(t, gi.patterns[0].negate)
	assert.False(t, gi.patterns[0].directory)

	err = gi.addPattern("!important.log")
	require.NoError(t, err)
	assert.Len(t, gi.patterns, 2)
	assert.True(t, gi.patterns[1].negate)
	assert.False(t, gi.patterns[1].directory)

	err = gi.addPattern("build/")
	require.NoError(t, err)
	assert.Len(t, gi.patterns, 3)
	assert.False(t, gi.patterns[2].negate)
	assert.True(t, gi.patterns[2].directory)
}

func TestLoadFromFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	gitignoreContent := `# Comment line
*.log
!important.log
/build/

# Another comment
*.tmp
`
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	require.NoError(t, err)

	gi := &GitIgnore{}
	err = gi.loadFromFile(gitignorePath)
	require.NoError(t, err)

	assert.Len(t, gi.patterns, 4)
}
