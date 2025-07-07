package repository

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/hash"
	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
)

const (
	gitDirName          = ".git"
	defaultDirMode      = 0755
	defaultFileMode     = 0644
	executableFileMode  = 0755
	hashLength          = 40
	hashPrefixLength    = 2
	refPrefixLength     = 5
	headRefPrefixLength = 16

	objectsDir    = "objects"
	refsDir       = "refs"
	headsDir      = "heads"
	tagsDir       = "tags"
	headFile      = "HEAD"

	refPrefix     = "ref: "
	headsPrefix   = "ref: refs/heads/"

	defaultBranch = "main"
)

type Repository struct {
	WorkDir string
	GitDir  string
}

func New(workDir string) *Repository {
	return &Repository{
		WorkDir: workDir,
		GitDir:  filepath.Join(workDir, gitDirName),
	}
}

func (r *Repository) Init() error {
	if r.Exists() {
		return errors.NewGitError("init", r.WorkDir, fmt.Errorf("repository already exists"))
	}

	dirs := []string{
		r.GitDir,
		filepath.Join(r.GitDir, objectsDir),
		filepath.Join(r.GitDir, refsDir),
		filepath.Join(r.GitDir, refsDir, headsDir),
		filepath.Join(r.GitDir, refsDir, tagsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, defaultDirMode); err != nil {
			return errors.NewGitError("init", dir, err)
		}
	}

	headContent := fmt.Sprintf("%s%s/%s/%s\n", refPrefix, refsDir, headsDir, defaultBranch)
	headPath := filepath.Join(r.GitDir, headFile)
	if err := os.WriteFile(headPath, []byte(headContent), defaultFileMode); err != nil {
		return errors.NewGitError("init", headPath, err)
	}

	return nil
}

func (r *Repository) Exists() bool {
	_, err := os.Stat(r.GitDir)
	return !os.IsNotExist(err)
}

func (r *Repository) StoreObject(obj objects.Object) (string, error) {
	if !r.Exists() {
		return "", errors.ErrNotGitRepository
	}

	data := objects.SerializeObject(obj)
	objHash := hash.ComputeSHA1(data)
	objPath := r.objectPath(objHash)
	objDir := filepath.Dir(objPath)
	if err := os.MkdirAll(objDir, defaultDirMode); err != nil {
		return "", errors.NewObjectError(objHash, obj.Type().String(), err)
	}
	if _, err := os.Stat(objPath); err == nil {
		return objHash, nil
	}

	file, err := os.Create(objPath)
	if err != nil {
		return "", errors.NewObjectError(objHash, obj.Type().String(), err)
	}
	defer file.Close()

	writer := zlib.NewWriter(file)
	defer writer.Close()

	if _, err := writer.Write(data); err != nil {
		return "", errors.NewObjectError(objHash, obj.Type().String(), err)
	}

	switch o := obj.(type) {
	case *objects.Blob:
		o.SetHash(objHash)
	case *objects.Tree:
		o.SetHash(objHash)
	case *objects.Commit:
		o.SetHash(objHash)
	}

	return objHash, nil
}

func (r *Repository) LoadObject(hashStr string) (objects.Object, error) {
	if !r.Exists() {
		return nil, errors.ErrNotGitRepository
	}

	if !hash.ValidateHash(hashStr) {
		return nil, errors.ErrInvalidHash
	}

	objPath := r.objectPath(hashStr)
	file, err := os.Open(objPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.ErrObjectNotFound
		}
		return nil, errors.NewObjectError(hashStr, "unknown", err)
	}
	defer file.Close()

	reader, err := zlib.NewReader(file)
	if err != nil {
		return nil, errors.NewObjectError(hashStr, "unknown", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.NewObjectError(hashStr, "unknown", err)
	}

	objType, _, content, err := objects.ParseObjectHeader(data)
	if err != nil {
		return nil, errors.NewObjectError(hashStr, "unknown", err)
	}

	obj, err := objects.ParseObject(objType, content)
	if err != nil {
		return nil, errors.NewObjectError(hashStr, objType.String(), err)
	}

	switch o := obj.(type) {
	case *objects.Blob:
		o.SetHash(hashStr)
	case *objects.Tree:
		o.SetHash(hashStr)
	case *objects.Commit:
		o.SetHash(hashStr)
	}

	return obj, nil
}

func (r *Repository) objectPath(hash string) string {
	return filepath.Join(r.GitDir, objectsDir, hash[:hashPrefixLength], hash[hashPrefixLength:])
}

func (r *Repository) GetHead() (string, error) {
	headPath := filepath.Join(r.GitDir, headFile)
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", errors.NewGitError("head", headPath, err)
	}

	headContent := string(content)
	if len(headContent) > refPrefixLength && headContent[:refPrefixLength] == refPrefix {
		refPath := headContent[refPrefixLength : len(headContent)-1]
		refFullPath := filepath.Join(r.GitDir, refPath)

		refContent, err := os.ReadFile(refFullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", errors.NewGitError("head", refFullPath, err)
		}

		return string(refContent)[:hashLength], nil
	}

	return headContent[:hashLength], nil
}

func (r *Repository) UpdateRef(refName, hash string) error {
	refPath := filepath.Join(r.GitDir, refName)
	refDir := filepath.Dir(refPath)

	if err := os.MkdirAll(refDir, defaultDirMode); err != nil {
		return errors.NewGitError("update-ref", refName, err)
	}

	content := hash + "\n"
	return os.WriteFile(refPath, []byte(content), defaultFileMode)
}

func (r *Repository) GetCurrentBranch() (string, error) {
	headPath := filepath.Join(r.GitDir, headFile)
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", errors.NewGitError("current-branch", headPath, err)
	}

	headContent := string(content)
	if len(headContent) > headRefPrefixLength && headContent[:headRefPrefixLength] == headsPrefix {
		return headContent[headRefPrefixLength : len(headContent)-1], nil
	}

	return "", errors.ErrInvalidReference
}

func (r *Repository) CheckoutTreeWithIndex(tree *objects.Tree, idx *index.Index, prefix string) ([]string, error) {
	var updatedFiles []string
	for _, entry := range tree.Entries() {
		fullPath := filepath.Join(r.WorkDir, prefix, entry.Name)
		relativePath := filepath.Join(prefix, entry.Name)
		gitPath := filepath.ToSlash(relativePath)

		switch entry.Mode {
		case objects.FileModeTree:
			if err := os.MkdirAll(fullPath, defaultDirMode); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}

			subTreeObj, err := r.LoadObject(entry.Hash)
			if err != nil {
				return nil, fmt.Errorf("failed to load subtree %s for directory %s: %w", entry.Hash, gitPath, err)
			}

			subTree, ok := subTreeObj.(*objects.Tree)
			if !ok {
				return nil, fmt.Errorf("subtree object is not a tree")
			}

			subUpdated, err := r.CheckoutTreeWithIndex(subTree, idx, relativePath)
			if err != nil {
				return nil, err
			}
			updatedFiles = append(updatedFiles, subUpdated...)

		case objects.FileModeBlob, objects.FileModeExecutable:
			blobObj, err := r.LoadObject(entry.Hash)
			if err != nil {
				return nil, fmt.Errorf("failed to load blob %s for file %s: %w", entry.Hash, gitPath, err)
			}

			blob, ok := blobObj.(*objects.Blob)
			if !ok {
				return nil, fmt.Errorf("blob object is not a blob")
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), defaultDirMode); err != nil {
				return nil, fmt.Errorf("failed to create directory for %s: %w", fullPath, err)
			}

			mode := os.FileMode(defaultFileMode)
			if entry.Mode == objects.FileModeExecutable {
				mode = os.FileMode(executableFileMode)
			}

			if err := os.WriteFile(fullPath, blob.Content(), mode); err != nil {
				return nil, fmt.Errorf("failed to write file %s: %w", fullPath, err)
			}

			stat, err := os.Stat(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to stat file %s: %w", fullPath, err)
			}

			if err := idx.AddWithFileInfo(gitPath, entry.Hash, uint32(entry.Mode), stat); err != nil {
				return nil, fmt.Errorf("failed to add %s to index: %w", gitPath, err)
			}

			updatedFiles = append(updatedFiles, gitPath)
		}
	}

	return updatedFiles, nil
}
