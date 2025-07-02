package objects

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/unkn0wn-root/git-go/errors"
)

func ParseObject(objType ObjectType, data []byte) (Object, error) {
	switch objType {
	case ObjectTypeBlob:
		return parseBlob(data)
	case ObjectTypeTree:
		return parseTree(data)
	case ObjectTypeCommit:
		return parseCommit(data)
	default:
		return nil, errors.ErrInvalidObjectType
	}
}

func parseBlob(data []byte) (*Blob, error) {
	return NewBlob(data), nil
}

func parseTree(data []byte) (*Tree, error) {
	var entries []TreeEntry
	pos := 0

	// Parse Git tree format: "mode name\0<20-byte-hash>"
	for pos < len(data) {
		spaceIdx := bytes.IndexByte(data[pos:], ' ')
		if spaceIdx == -1 {
			return nil, errors.NewGitError("parse-tree", "", fmt.Errorf("failed to find space separator"))
		}
		spaceIdx += pos

		modeStr := string(data[pos:spaceIdx])
		mode, err := ParseFileMode(modeStr)
		if err != nil {
			return nil, errors.NewGitError("parse-tree", "", fmt.Errorf("invalid file mode %s: %w", modeStr, err))
		}

		pos = spaceIdx + 1
		nullIdx := bytes.IndexByte(data[pos:], 0)
		if nullIdx == -1 {
			return nil, errors.NewGitError("parse-tree", "", fmt.Errorf("failed to find null separator"))
		}
		nullIdx += pos

		name := string(data[pos:nullIdx])
		pos = nullIdx + 1

		// Each hash is exactly 20 bytes in binary format
		if pos+20 > len(data) {
			return nil, errors.NewGitError("parse-tree", "", fmt.Errorf("insufficient data for hash"))
		}

		hashBytes := data[pos : pos+20]
		hash := fmt.Sprintf("%x", hashBytes)
		pos += 20

		entries = append(entries, TreeEntry{
			Mode: mode,
			Name: name,
			Hash: hash,
		})
	}

	return NewTree(entries), nil
}

func parseCommit(data []byte) (*Commit, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var tree string
	var parents []string
	var author *Signature
	var committer *Signature
	var messageLines []string
	inMessage := false

	// Parse Git commit format: headers followed by blank line and message
	for scanner.Scan() {
		line := scanner.Text()

		if inMessage {
			messageLines = append(messageLines, line)
			continue
		}

		if line == "" {
			inMessage = true
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, errors.NewGitError("parse-commit", "", fmt.Errorf("invalid commit line: %s", line))
		}

		key, value := parts[0], parts[1]

		switch key {
		case "tree":
			tree = value
		case "parent":
			parents = append(parents, value)
		case "author":
			var err error
			author, err = ParseSignature(value)
			if err != nil {
				return nil, errors.NewGitError("parse-commit", "", fmt.Errorf("invalid author signature: %w", err))
			}
		case "committer":
			var err error
			committer, err = ParseSignature(value)
			if err != nil {
				return nil, errors.NewGitError("parse-commit", "", fmt.Errorf("invalid committer signature: %w", err))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.NewGitError("parse-commit", "", fmt.Errorf("failed to parse commit: %w", err))
	}

	if tree == "" {
		return nil, errors.NewGitError("parse-commit", "", errors.ErrInvalidCommit)
	}

	if author == nil || committer == nil {
		return nil, errors.NewGitError("parse-commit", "", errors.ErrInvalidCommit)
	}

	message := strings.Join(messageLines, "\n")

	return NewCommit(tree, parents, author, committer, message), nil
}

func SerializeObject(obj Object) []byte {
	header := fmt.Sprintf("%s %d\x00", obj.Type(), obj.Size())
	return append([]byte(header), obj.Data()...)
}

func ParseObjectHeader(data []byte) (ObjectType, int64, []byte, error) {
	nullIdx := bytes.IndexByte(data, 0)
	if nullIdx == -1 {
		return "", 0, nil, errors.ErrInvalidObjectFormat
	}

	header := string(data[:nullIdx])
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return "", 0, nil, errors.ErrInvalidObjectFormat
	}

	objType, err := ParseObjectType(parts[0])
	if err != nil {
		return "", 0, nil, err
	}

	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, nil, errors.NewGitError("parse-header", "", fmt.Errorf("invalid object size: %w", err))
	}

	content := data[nullIdx+1:]
	if int64(len(content)) != size {
		return "", 0, nil, errors.NewGitError("parse-header", "", fmt.Errorf("object size mismatch: expected %d, got %d", size, len(content)))
	}

	return objType, size, content, nil
}
