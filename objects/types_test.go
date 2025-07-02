package objects

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlob(t *testing.T) {
	content := []byte("Hello, World!")
	blob := NewBlob(content)

	assert.Equal(t, ObjectTypeBlob, blob.Type())
	assert.Equal(t, int64(len(content)), blob.Size())
	assert.Equal(t, content, blob.Data())
	assert.Equal(t, content, blob.Content())
}

func TestTree(t *testing.T) {
	entries := []TreeEntry{
		{
			Mode: FileModeBlob,
			Name: "file1.txt",
			Hash: "abc123",
		},
		{
			Mode: FileModeExecutable,
			Name: "script.sh",
			Hash: "def456",
		},
	}

	tree := NewTree(entries)

	assert.Equal(t, ObjectTypeTree, tree.Type())
	assert.Equal(t, entries, tree.Entries())
	assert.True(t, tree.Size() > 0)
}

func TestCommit(t *testing.T) {
	treeHash := "abc123"
	parents := []string{"parent1", "parent2"}

	author := &Signature{
		Name:  "John Doe",
		Email: "john@example.com",
		When:  time.Unix(1234567890, 0),
	}

	committer := &Signature{
		Name:  "Jane Doe",
		Email: "jane@example.com",
		When:  time.Unix(1234567900, 0),
	}

	message := "Initial commit"

	commit := NewCommit(treeHash, parents, author, committer, message)

	assert.Equal(t, ObjectTypeCommit, commit.Type())
	assert.Equal(t, treeHash, commit.Tree())
	assert.Equal(t, parents, commit.Parents())
	assert.Equal(t, author, commit.Author())
	assert.Equal(t, committer, commit.Committer())
	assert.Equal(t, message, commit.Message())
	assert.True(t, commit.Size() > 0)
}

func TestParseSignature(t *testing.T) {
	tests := []struct {
		input    string
		expected *Signature
		hasError bool
	}{
		{
			input: "John Doe <john@example.com> 1234567890 +0000",
			expected: &Signature{
				Name:  "John Doe",
				Email: "john@example.com",
				When:  time.Unix(1234567890, 0),
			},
			hasError: false,
		},
		{
			input:    "Invalid signature",
			expected: nil,
			hasError: true,
		},
		{
			input:    "John Doe john@example.com 1234567890 +0000",
			expected: nil,
			hasError: true,
		},
		{
			input:    "",
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sig, err := ParseSignature(tt.input)

			if tt.hasError {
				assert.Error(t, err)
				assert.Nil(t, sig)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Name, sig.Name)
				assert.Equal(t, tt.expected.Email, sig.Email)
				assert.Equal(t, tt.expected.When.Unix(), sig.When.Unix())
			}
		})
	}
}

func TestFileMode(t *testing.T) {
	tests := []struct {
		mode     FileMode
		expected string
	}{
		{FileModeBlob, "100644"},
		{FileModeExecutable, "100755"},
		{FileModeSymlink, "120000"},
		{FileModeTree, "040000"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())

			parsed, err := ParseFileMode(tt.expected)
			require.NoError(t, err)
			assert.Equal(t, tt.mode, parsed)
		})
	}
}

func TestObjectType(t *testing.T) {
	tests := []struct {
		objType  ObjectType
		expected string
	}{
		{ObjectTypeBlob, "blob"},
		{ObjectTypeTree, "tree"},
		{ObjectTypeCommit, "commit"},
		{ObjectTypeTag, "tag"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.objType.String())

			parsed, err := ParseObjectType(tt.expected)
			require.NoError(t, err)
			assert.Equal(t, tt.objType, parsed)
		})
	}

	_, err := ParseObjectType("invalid")
	assert.Error(t, err)
}
