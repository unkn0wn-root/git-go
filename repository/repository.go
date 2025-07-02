package repository

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/hash"
	"github.com/unkn0wn-root/git-go/objects"
)

type Repository struct {
	WorkDir string
	GitDir  string
}

func New(workDir string) *Repository {
	return &Repository{
		WorkDir: workDir,
		GitDir:  filepath.Join(workDir, ".git"),
	}
}

func (r *Repository) Init() error {
	if r.Exists() {
		return errors.NewGitError("init", r.WorkDir, fmt.Errorf("repository already exists"))
	}

	dirs := []string{
		r.GitDir,
		filepath.Join(r.GitDir, "objects"),
		filepath.Join(r.GitDir, "refs"),
		filepath.Join(r.GitDir, "refs", "heads"),
		filepath.Join(r.GitDir, "refs", "tags"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.NewGitError("init", dir, err)
		}
	}

	headContent := "ref: refs/heads/main\n"
	headPath := filepath.Join(r.GitDir, "HEAD")
	if err := os.WriteFile(headPath, []byte(headContent), 0644); err != nil {
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

	if err := os.MkdirAll(objDir, 0755); err != nil {
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
	return filepath.Join(r.GitDir, "objects", hash[:2], hash[2:])
}

func (r *Repository) GetHead() (string, error) {
	headPath := filepath.Join(r.GitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", errors.NewGitError("head", headPath, err)
	}

	headContent := string(content)
	if len(headContent) > 5 && headContent[:5] == "ref: " {
		refPath := headContent[5 : len(headContent)-1]
		refFullPath := filepath.Join(r.GitDir, refPath)

		refContent, err := os.ReadFile(refFullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", errors.NewGitError("head", refFullPath, err)
		}

		return string(refContent)[:40], nil
	}

	return headContent[:40], nil
}

func (r *Repository) UpdateRef(refName, hash string) error {
	refPath := filepath.Join(r.GitDir, refName)
	refDir := filepath.Dir(refPath)

	if err := os.MkdirAll(refDir, 0755); err != nil {
		return errors.NewGitError("update-ref", refName, err)
	}

	content := hash + "\n"
	return os.WriteFile(refPath, []byte(content), 0644)
}

func (r *Repository) GetCurrentBranch() (string, error) {
	headPath := filepath.Join(r.GitDir, "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", errors.NewGitError("current-branch", headPath, err)
	}

	headContent := string(content)
	if len(headContent) > 16 && headContent[:16] == "ref: refs/heads/" {
		return headContent[16 : len(headContent)-1], nil
	}

	return "", errors.ErrInvalidReference
}
