package repository

import (
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/unkn0wn-root/git-go/internal/core/hash"
	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/pkg/errors"
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

	objectsDir = "objects"
	refsDir    = "refs"
	headsDir   = "heads"
	tagsDir    = "tags"
	headFile   = "HEAD"

	refPrefix   = "ref: "
	headsPrefix = "ref: refs/heads/"

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
	if err == nil {
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

	// if not found in loose objects, try pack files
	if os.IsNotExist(err) {
		obj, err := r.loadObjectFromPack(hashStr)
		if err != nil {
			return nil, errors.ErrObjectNotFound
		}
		return obj, nil
	}

	return nil, errors.NewObjectError(hashStr, "unknown", err)
}

func (r *Repository) objectPath(hash string) string {
	return filepath.Join(r.GitDir, objectsDir, hash[:hashPrefixLength], hash[hashPrefixLength:])
}

func (r *Repository) loadObjectFromPack(hashStr string) (objects.Object, error) {
	packDir := filepath.Join(r.GitDir, objectsDir, "pack")
	if _, err := os.Stat(packDir); os.IsNotExist(err) {
		return nil, errors.ErrObjectNotFound
	}

	// Find all pack index files
	files, err := os.ReadDir(packDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".idx") {
			idxPath := filepath.Join(packDir, file.Name())
			packPath := strings.TrimSuffix(idxPath, ".idx") + ".pack"

			// Try to find object in this pack
			if obj, err := r.loadObjectFromSpecificPack(hashStr, idxPath, packPath); err == nil {
				return obj, nil
			}
		}
	}

	return nil, errors.ErrObjectNotFound
}

func (r *Repository) loadObjectFromSpecificPack(hashStr, idxPath, packPath string) (objects.Object, error) {
	// read pack index to find object offset
	offset, err := r.findObjectInPackIndex(hashStr, idxPath)
	if err != nil {
		return nil, err
	}

	// read object from pack at the given offset
	return r.readObjectFromPack(hashStr, packPath, offset)
}

func (r *Repository) findObjectInPackIndex(hashStr, idxPath string) (int64, error) {
	idxFile, err := os.Open(idxPath)
	if err != nil {
		return 0, err
	}
	defer idxFile.Close()

	// read pack index format
	magic := make([]byte, 4)
	if _, err := idxFile.Read(magic); err != nil {
		return 0, err
	}

	// check for version 2 index (starts with 0xff744f63 magic)
	if binary.BigEndian.Uint32(magic) == 0xff744f63 {
		return r.findObjectInPackIndexV2(hashStr, idxFile)
	} else {
		idxFile.Seek(0, 0) // reset to beginning
		return r.findObjectInPackIndexV1(hashStr, idxFile)
	}
}

// V1 - We have to support both versions of pack index
func (r *Repository) findObjectInPackIndexV1(hashStr string, idxFile *os.File) (int64, error) {
	// skip fanout table (256 * 4 bytes)
	if _, err := idxFile.Seek(256*4, 0); err != nil {
		return 0, err
	}

	// read number of objects from last fanout entry
	if _, err := idxFile.Seek(255*4, 0); err != nil {
		return 0, err
	}
	numObjectsBytes := make([]byte, 4)
	if _, err := idxFile.Read(numObjectsBytes); err != nil {
		return 0, err
	}
	numObjects := binary.BigEndian.Uint32(numObjectsBytes)

	if _, err := idxFile.Seek(256*4, 0); err != nil {
		return 0, err
	}

	// each entry is 24 bytes: 4-byte offset + 20-byte hash
	for i := uint32(0); i < numObjects; i++ {
		entry := make([]byte, 24)
		if _, err := idxFile.Read(entry); err != nil {
			return 0, err
		}

		offset := binary.BigEndian.Uint32(entry[:4])
		hash := fmt.Sprintf("%x", entry[4:24])

		if hash == hashStr {
			return int64(offset), nil
		}
	}

	return 0, errors.ErrObjectNotFound
}

func (r *Repository) findObjectInPackIndexV2(hashStr string, idxFile *os.File) (int64, error) {
	// skip magic (4 bytes) and version (4 bytes)
	if _, err := idxFile.Seek(8, 0); err != nil {
		return 0, err
	}

	// read fanout table to get number of objects
	fanout := make([]byte, 256*4)
	if _, err := idxFile.Read(fanout); err != nil {
		return 0, err
	}
	numObjects := binary.BigEndian.Uint32(fanout[255*4 : 256*4])

	// calculate the first byte of the hash to use as fanout index
	// convert first hex character to its numeric value
	firstChar := hashStr[0]
	var fanoutIdx int
	if firstChar >= '0' && firstChar <= '9' {
		fanoutIdx = int(firstChar - '0')
	} else if firstChar >= 'a' && firstChar <= 'f' {
		fanoutIdx = int(firstChar-'a') + 10
	} else {
		return 0, errors.ErrInvalidHash
	}

	// convert second hex character and combine for full byte value
	secondChar := hashStr[1]
	var secondNibble int
	if secondChar >= '0' && secondChar <= '9' {
		secondNibble = int(secondChar - '0')
	} else if secondChar >= 'a' && secondChar <= 'f' {
		secondNibble = int(secondChar-'a') + 10
	} else {
		return 0, errors.ErrInvalidHash
	}

	fanoutIdx = fanoutIdx*16 + secondNibble

	// get start and end positions for bin search
	startCount := uint32(0)
	if fanoutIdx > 0 {
		startCount = binary.BigEndian.Uint32(fanout[(fanoutIdx-1)*4 : fanoutIdx*4])
	}
	endCount := binary.BigEndian.Uint32(fanout[fanoutIdx*4 : (fanoutIdx+1)*4])

	// search in hash table
	hashTableOffset := int64(8 + 256*4) // after header and fanout
	for i := startCount; i < endCount; i++ {
		hashOffset := hashTableOffset + int64(i)*20
		if _, err := idxFile.Seek(hashOffset, 0); err != nil {
			return 0, err
		}

		hashBytes := make([]byte, 20)
		if _, err := idxFile.Read(hashBytes); err != nil {
			return 0, err
		}

		hash := fmt.Sprintf("%x", hashBytes)
		if hash == hashStr {
			// get the offset from offset table
			offsetTableOffset := int64(8 + 256*4 + int(numObjects)*20 + int(numObjects)*4) // After header, fanout, hashes, and CRCs
			offsetPos := offsetTableOffset + int64(i)*4

			if _, err := idxFile.Seek(offsetPos, 0); err != nil {
				return 0, err
			}

			offsetBytes := make([]byte, 4)
			if _, err := idxFile.Read(offsetBytes); err != nil {
				return 0, err
			}

			offset := binary.BigEndian.Uint32(offsetBytes)
			return int64(offset), nil
		}
	}

	return 0, errors.ErrObjectNotFound
}

func (r *Repository) readObjectFromPack(hashStr, packPath string, offset int64) (objects.Object, error) {
	packFile, err := os.Open(packPath)
	if err != nil {
		return nil, err
	}
	defer packFile.Close()

	// seek to object offset
	if _, err := packFile.Seek(offset, 0); err != nil {
		return nil, err
	}

	// read object header to get type and size
	objType, size, dataOffset, err := r.readPackObjectHeader(packFile, offset)
	if err != nil {
		return nil, err
	}

	// only handle simple objects (not deltas for now)
	if objType < 1 || objType > 4 {
		return nil, fmt.Errorf("unsupported pack object type: %d", objType)
	}

	// seek to compressed data
	if _, err := packFile.Seek(dataOffset, 0); err != nil {
		return nil, err
	}

	// read and decompress object data
	reader, err := zlib.NewReader(packFile)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data := make([]byte, size)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}

	// convert pack object type to Git object type
	var gitObjType objects.ObjectType
	switch objType {
	case 1: // OBJ_COMMIT
		gitObjType = objects.ObjectTypeCommit
	case 2: // OBJ_TREE
		gitObjType = objects.ObjectTypeTree
	case 3: // OBJ_BLOB
		gitObjType = objects.ObjectTypeBlob
	case 4: // OBJ_TAG
		gitObjType = objects.ObjectTypeTag
	default:
		return nil, fmt.Errorf("unknown object type: %d", objType)
	}

	obj, err := objects.ParseObject(gitObjType, data)
	if err != nil {
		return nil, err
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

func (r *Repository) readPackObjectHeader(packFile *os.File, offset int64) (int, int64, int64, error) {
	if _, err := packFile.Seek(offset, 0); err != nil {
		return 0, 0, 0, err
	}

	// read first byte to get type and start of size
	firstByte := make([]byte, 1)
	if _, err := packFile.Read(firstByte); err != nil {
		return 0, 0, 0, err
	}

	b := firstByte[0]
	objType := int((b >> 4) & 7)
	size := int64(b & 15)
	currentOffset := offset + 1

	// variable-length size encoding
	shift := 4
	for (b & 0x80) != 0 {
		if _, err := packFile.Seek(currentOffset, 0); err != nil {
			return 0, 0, 0, err
		}
		if _, err := packFile.Read(firstByte); err != nil {
			return 0, 0, 0, err
		}
		b = firstByte[0]
		size |= int64(b&0x7f) << shift
		shift += 7
		currentOffset++
	}

	return objType, size, currentOffset, nil
}

func (r *Repository) GetHead() (string, error) {
	headPath := filepath.Join(r.GitDir, headFile)
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", errors.NewGitError("head", headPath, err)
	}

	headContent := strings.TrimSpace(string(content))
	if len(headContent) > refPrefixLength && headContent[:refPrefixLength] == refPrefix {
		refPath := headContent[refPrefixLength:]
		refFullPath := filepath.Join(r.GitDir, refPath)

		refContent, err := os.ReadFile(refFullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", errors.NewGitError("head", refFullPath, err)
		}

		return strings.TrimSpace(string(refContent)), nil
	}

	return strings.TrimSpace(headContent), nil
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

	headContent := strings.TrimSpace(string(content))
	if len(headContent) > headRefPrefixLength && headContent[:headRefPrefixLength] == headsPrefix {
		return headContent[headRefPrefixLength:], nil
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
