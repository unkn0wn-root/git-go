package objects

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/pkg/errors"
)

type ObjectType string

const (
	ObjectTypeBlob   ObjectType = "blob"
	ObjectTypeTree   ObjectType = "tree"
	ObjectTypeCommit ObjectType = "commit"
	ObjectTypeTag    ObjectType = "tag"
)

func (t ObjectType) String() string {
	return string(t)
}

func ParseObjectType(s string) (ObjectType, error) {
	switch s {
	case "blob":
		return ObjectTypeBlob, nil
	case "tree":
		return ObjectTypeTree, nil
	case "commit":
		return ObjectTypeCommit, nil
	case "tag":
		return ObjectTypeTag, nil
	default:
		return "", errors.ErrInvalidObjectType
	}
}

type Object interface {
	Type() ObjectType
	Size() int64
	Data() []byte
	Hash() string
}

type Blob struct {
	hash    string
	content []byte
}

func NewBlob(content []byte) *Blob {
	return &Blob{
		content: content,
	}
}

func (b *Blob) Type() ObjectType {
	return ObjectTypeBlob
}

func (b *Blob) Size() int64 {
	return int64(len(b.content))
}

func (b *Blob) Data() []byte {
	return b.content
}

func (b *Blob) Hash() string {
	return b.hash
}

func (b *Blob) SetHash(hash string) {
	b.hash = hash
}

func (b *Blob) Content() []byte {
	return b.content
}

type TreeEntry struct {
	Mode FileMode
	Name string
	Hash string
}

type FileMode uint32

const (
	FileModeBlob       FileMode = 0o100644
	FileModeExecutable FileMode = 0o100755
	FileModeSymlink    FileMode = 0o120000
	FileModeTree       FileMode = 0o040000
)

func (m FileMode) String() string {
	return fmt.Sprintf("%06o", uint32(m))
}

func ParseFileMode(s string) (FileMode, error) {
	mode, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid file mode: %w", err)
	}
	return FileMode(mode), nil
}

type Tree struct {
	hash    string
	entries []TreeEntry
}

func NewTree(entries []TreeEntry) *Tree {
	return &Tree{
		entries: entries,
	}
}

func (t *Tree) Type() ObjectType {
	return ObjectTypeTree
}

func (t *Tree) Size() int64 {
	var size int64
	for _, entry := range t.entries {
		size += int64(len(entry.Mode.String()) + 1 + len(entry.Name) + 1 + 20)
	}
	return size
}

func (t *Tree) Data() []byte {
	var buf bytes.Buffer
	for _, entry := range t.entries {
		buf.WriteString(entry.Mode.String())
		buf.WriteByte(' ')
		buf.WriteString(entry.Name)
		buf.WriteByte(0)
		// Convert hex hash string to 20-byte binary format for Git tree objects
		hashBytes := make([]byte, 20)
		for i := 0; i < 20; i++ {
			var b byte
			fmt.Sscanf(entry.Hash[i*2:i*2+2], "%02x", &b)
			hashBytes[i] = b
		}
		buf.Write(hashBytes)
	}
	return buf.Bytes()
}

func (t *Tree) Hash() string {
	return t.hash
}

func (t *Tree) SetHash(hash string) {
	t.hash = hash
}

func (t *Tree) Entries() []TreeEntry {
	return t.entries
}

type Signature struct {
	Name  string
	Email string
	When  time.Time
}

func (s *Signature) String() string {
	return fmt.Sprintf("%s <%s> %d %s", s.Name, s.Email, s.When.Unix(), s.When.Format("-0700"))
}

func ParseSignature(data string) (*Signature, error) {
	if data == "" {
		return nil, errors.NewGitError("parse-signature", "", errors.ErrInvalidCommit)
	}

	// Parse Git signature format: "Name <email> timestamp timezone"
	emailStart := strings.Index(data, "<")
	emailEnd := strings.Index(data, ">")
	if emailStart == -1 || emailEnd == -1 || emailEnd <= emailStart {
		return nil, errors.NewGitError("parse-signature", "", fmt.Errorf("invalid email format in signature"))
	}

	name := strings.TrimSpace(data[:emailStart])
	email := data[emailStart+1 : emailEnd]

	timeStr := strings.TrimSpace(data[emailEnd+1:])
	timeParts := strings.Split(timeStr, " ")
	if len(timeParts) < 2 {
		return nil, errors.NewGitError("parse-signature", "", fmt.Errorf("invalid time format in signature"))
	}

	timestamp, err := strconv.ParseInt(timeParts[0], 10, 64)
	if err != nil {
		return nil, errors.NewGitError("parse-signature", "", fmt.Errorf("invalid timestamp: %w", err))
	}

	return &Signature{
		Name:  name,
		Email: email,
		When:  time.Unix(timestamp, 0),
	}, nil
}

type Commit struct {
	hash      string
	tree      string
	parents   []string
	author    *Signature
	committer *Signature
	message   string
}

func NewCommit(tree string, parents []string, author, committer *Signature, message string) *Commit {
	return &Commit{
		tree:      tree,
		parents:   parents,
		author:    author,
		committer: committer,
		message:   message,
	}
}

func (c *Commit) Type() ObjectType {
	return ObjectTypeCommit
}

func (c *Commit) Size() int64 {
	var size int64
	size += int64(len("tree ") + len(c.tree) + 1)
	for _, parent := range c.parents {
		size += int64(len("parent ") + len(parent) + 1)
	}
	size += int64(len("author ") + len(c.author.String()) + 1)
	size += int64(len("committer ") + len(c.committer.String()) + 1)
	size += int64(1 + len(c.message))
	return size
}

func (c *Commit) Data() []byte {
	var buf bytes.Buffer
	buf.WriteString("tree ")
	buf.WriteString(c.tree)
	buf.WriteByte('\n')

	for _, parent := range c.parents {
		buf.WriteString("parent ")
		buf.WriteString(parent)
		buf.WriteByte('\n')
	}

	buf.WriteString("author ")
	buf.WriteString(c.author.String())
	buf.WriteByte('\n')

	buf.WriteString("committer ")
	buf.WriteString(c.committer.String())
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	buf.WriteString(c.message)

	return buf.Bytes()
}

func (c *Commit) Hash() string {
	return c.hash
}

func (c *Commit) SetHash(hash string) {
	c.hash = hash
}

func (c *Commit) Tree() string {
	return c.tree
}

func (c *Commit) Parents() []string {
	return c.parents
}

func (c *Commit) Author() *Signature {
	return c.author
}

func (c *Commit) Committer() *Signature {
	return c.committer
}

func (c *Commit) Message() string {
	return c.message
}
