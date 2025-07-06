package errors

import (
	stderrors "errors"
	"fmt"
)

var (
	ErrNotGitRepository     = stderrors.New("not a git repository")
	ErrObjectNotFound       = stderrors.New("object not found")
	ErrInvalidObjectType    = stderrors.New("invalid object type")
	ErrInvalidHash          = stderrors.New("invalid hash")
	ErrFileNotFound         = stderrors.New("file not found")
	ErrInvalidCommit        = stderrors.New("invalid commit object")
	ErrInvalidTree          = stderrors.New("invalid tree object")
	ErrInvalidBlob          = stderrors.New("invalid blob object")
	ErrInvalidIndex         = stderrors.New("invalid index")
	ErrFileAlreadyStaged    = stderrors.New("file already staged")
	ErrFileNotStaged        = stderrors.New("file not staged")
	ErrNothingToCommit      = stderrors.New("nothing to commit")
	ErrInvalidReference     = stderrors.New("invalid reference")
	ErrReferenceNotFound    = stderrors.New("reference not found")
	ErrCorruptedRepository  = stderrors.New("corrupted repository")
	ErrInvalidObjectFormat  = stderrors.New("invalid object format")
	ErrPermissionDenied     = stderrors.New("permission denied")
	ErrDirectoryNotEmpty    = stderrors.New("directory not empty")
	ErrRemoteNotFound       = stderrors.New("remote not found")
	ErrRemoteAlreadyExists  = stderrors.New("remote already exists")
	ErrNetworkTimeout       = stderrors.New("network timeout")
	ErrAuthenticationFailed = stderrors.New("authentication failed")
	ErrPushRejected         = stderrors.New("push rejected")
	ErrNonFastForward       = stderrors.New("non-fast-forward")
	ErrUnrelatedHistories   = stderrors.New("unrelated histories")
	ErrMergeConflict        = stderrors.New("merge conflict")
	ErrInvalidURL           = stderrors.New("invalid URL")
	ErrUnsupportedProtocol  = stderrors.New("unsupported protocol")
)

type GitError struct {
	Op   string
	Path string
	Err  error
}

func (e *GitError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("git %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("git %s %s: %v", e.Op, e.Path, e.Err)
}

func (e *GitError) Unwrap() error {
	return e.Err
}

func NewGitError(op, path string, err error) *GitError {
	return &GitError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}

type ObjectError struct {
	Hash string
	Type string
	Err  error
}

func (e *ObjectError) Error() string {
	return fmt.Sprintf("object %s (%s): %v", e.Hash, e.Type, e.Err)
}

func (e *ObjectError) Unwrap() error {
	return e.Err
}

func NewObjectError(hash, objType string, err error) *ObjectError {
	return &ObjectError{
		Hash: hash,
		Type: objType,
		Err:  err,
	}
}

type IndexError struct {
	Path string
	Err  error
}

func (e *IndexError) Error() string {
	return fmt.Sprintf("index %s: %v", e.Path, e.Err)
}

func (e *IndexError) Unwrap() error {
	return e.Err
}

func NewIndexError(path string, err error) *IndexError {
	return &IndexError{
		Path: path,
		Err:  err,
	}
}