package index

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/errors"
	"github.com/unkn0wn-root/git-go/hash"
)

type IndexEntry struct {
	Path        string
	Hash        string
	Mode        uint32
	Size        int64
	ModTime     time.Time
	Staged      bool
	StageNumber int
}

type Index struct {
	entries map[string]*IndexEntry
	gitDir  string
}

func New(gitDir string) *Index {
	return &Index{
		entries: make(map[string]*IndexEntry),
		gitDir:  gitDir,
	}
}

func (idx *Index) Load() error {
	indexPath := filepath.Join(idx.gitDir, "index")

	file, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.NewIndexError(indexPath, err)
	}
	defer file.Close()

	// Git index format: 12-byte header + entries + checksum
	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		if err == io.EOF {
			return nil // Empty index
		}
		return errors.NewIndexError(indexPath, fmt.Errorf("failed to read header: %w", err))
	}

	// Git index signature is "DIRC" (DirCache)
	if string(header[:4]) != "DIRC" {
		return errors.NewIndexError(indexPath, fmt.Errorf("invalid index signature"))
	}

	// Only support index version 2
	version := binary.BigEndian.Uint32(header[4:8])
	if version != 2 {
		return errors.NewIndexError(indexPath, fmt.Errorf("unsupported index version: %d", version))
	}

	entryCount := binary.BigEndian.Uint32(header[8:12])
	for i := uint32(0); i < entryCount; i++ {
		entry, err := idx.readIndexEntry(file)
		if err != nil {
			return errors.NewIndexError(indexPath, fmt.Errorf("%d: %w", i, err))
		}
		idx.entries[entry.Path] = entry
	}

	return nil
}

func (idx *Index) Save() error {
	indexPath := filepath.Join(idx.gitDir, "index")

	file, err := os.Create(indexPath)
	if err != nil {
		return errors.NewIndexError(indexPath, err)
	}
	defer file.Close()

	// get all entries (both staged and committed) and sort them
	var allEntries []*IndexEntry
	for _, entry := range idx.entries {
		allEntries = append(allEntries, entry)
	}

	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Path < allEntries[j].Path
	})

	var buf bytes.Buffer
	buf.WriteString("DIRC")                                       // Signature
	binary.Write(&buf, binary.BigEndian, uint32(2))               // Version
	binary.Write(&buf, binary.BigEndian, uint32(len(allEntries))) // Entry count

	for _, entry := range allEntries {
		if err := idx.writeIndexEntry(&buf, entry); err != nil {
			return errors.NewIndexError(indexPath, err)
		}
	}

	hash := sha1.Sum(buf.Bytes())
	buf.Write(hash[:])

	if _, err := file.Write(buf.Bytes()); err != nil {
		return errors.NewIndexError(indexPath, err)
	}

	return nil
}

func (idx *Index) Add(path, objHash string, mode uint32, size int64, modTime time.Time) error {
	if !hash.ValidateHash(objHash) {
		return errors.NewIndexError(path, errors.ErrInvalidHash)
	}

	idx.entries[path] = &IndexEntry{
		Path:    path,
		Hash:    objHash,
		Mode:    mode,
		Size:    size,
		ModTime: modTime,
		Staged:  true,
	}
	return nil
}

func (idx *Index) Remove(path string) error {
	if _, exists := idx.entries[path]; !exists {
		return errors.ErrFileNotStaged
	}

	delete(idx.entries, path)
	return nil
}

func (idx *Index) Get(path string) (*IndexEntry, bool) {
	entry, exists := idx.entries[path]
	return entry, exists
}

func (idx *Index) GetAll() map[string]*IndexEntry {
	result := make(map[string]*IndexEntry)
	for k, v := range idx.entries {
		if v.Staged {
			result[k] = v
		}
	}
	return result
}

func (idx *Index) IsStaged(path string) bool {
	entry, exists := idx.entries[path]
	return exists && entry.Staged
}

func (idx *Index) HasChanges() bool {
	for _, entry := range idx.entries {
		if entry.Staged {
			return true
		}
	}
	return false
}

func (idx *Index) Clear() {
	idx.entries = make(map[string]*IndexEntry)
}

func (idx *Index) WriteTree() (string, error) {
	if !idx.HasChanges() {
		return "", errors.ErrNothingToCommit
	}

	// Build hierarchical directory structure from flat file paths
	root := &dirNode{
		name:     "",
		children: make(map[string]*dirNode),
		files:    make(map[string]*IndexEntry),
	}

	// convert flat file paths to nested directory structure
	for _, entry := range idx.entries {
		if !entry.Staged {
			continue
		}

		parts := strings.Split(entry.Path, string(filepath.Separator))
		current := root

		for i := 0; i < len(parts)-1; i++ {
			dirName := parts[i]
			if current.children[dirName] == nil {
				current.children[dirName] = &dirNode{
					name:     dirName,
					children: make(map[string]*dirNode),
					files:    make(map[string]*IndexEntry),
				}
			}
			current = current.children[dirName]
		}

		fileName := parts[len(parts)-1]
		current.files[fileName] = entry
	}

	return idx.writeTreeRecursive(root)
}

type dirNode struct {
	name     string
	children map[string]*dirNode
	files    map[string]*IndexEntry
}

func (idx *Index) readIndexEntry(file io.Reader) (*IndexEntry, error) {
	// Git index entry: 62-byte fixed header + variable-length path
	header := make([]byte, 62)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, err
	}

	// Parse Git index entry fields (skip unused filesystem metadata)
	_ = binary.BigEndian.Uint32(header[0:4]) // ctime
	_ = binary.BigEndian.Uint32(header[4:8]) // cmtime
	mtime := binary.BigEndian.Uint32(header[8:12])
	mmtime := binary.BigEndian.Uint32(header[12:16])
	_ = binary.BigEndian.Uint32(header[16:20]) // dev
	_ = binary.BigEndian.Uint32(header[20:24]) // ino
	mode := binary.BigEndian.Uint32(header[24:28])
	_ = binary.BigEndian.Uint32(header[28:32]) // uid
	_ = binary.BigEndian.Uint32(header[32:36]) // gid
	size := binary.BigEndian.Uint32(header[36:40])
	hashBytes := header[40:60]
	flags := binary.BigEndian.Uint16(header[60:62])

	hashStr := hex.EncodeToString(hashBytes)

	// Path length is stored in lower 12 bits of flags
	pathLen := flags & 0xFFF
	var pathBytes []byte

	if pathLen == 0xFFF {
		// Path >= 4095 chars: read until null terminator
		var pathBuf bytes.Buffer
		buf := make([]byte, 1)
		for {
			if _, err := io.ReadFull(file, buf); err != nil {
				return nil, err
			}
			if buf[0] == 0 {
				break
			}
			pathBuf.WriteByte(buf[0])
		}
		pathBytes = pathBuf.Bytes()
		pathLen = uint16(len(pathBytes))
	} else {
		pathBytes = make([]byte, pathLen)
		if _, err := io.ReadFull(file, pathBytes); err != nil {
			return nil, err
		}
	}

	// Git index entries are padded to 8-byte alignment
	entrySize := 62 + int(pathLen)
	if pathLen == 0xFFF {
		entrySize++ // For null terminator
	}
	padding := (8 - (entrySize % 8)) % 8
	if padding > 0 {
		padBytes := make([]byte, padding)
		io.ReadFull(file, padBytes)
	}

	modTime := time.Unix(int64(mtime), int64(mmtime))

	return &IndexEntry{
		Path:    string(pathBytes),
		Hash:    hashStr,
		Mode:    mode,
		Size:    int64(size),
		ModTime: modTime,
		Staged:  true, // Files in the index are staged for commit
	}, nil
}

func (idx *Index) writeIndexEntry(buf *bytes.Buffer, entry *IndexEntry) error {
	hashBytes, err := hex.DecodeString(entry.Hash)
	if err != nil {
		return fmt.Errorf("invalid hash %s: %w", entry.Hash, err)
	}

	// fixed-size header (62 bytes)
	// this should use actual filesystem values instead of fixed to zero
	// but leave it at it is for now
	ctime := uint32(entry.ModTime.Unix())
	cmtime := uint32(0) // Nanoseconds
	mtime := uint32(entry.ModTime.Unix())
	mmtime := uint32(0) // Nanoseconds
	dev := uint32(0)    // Device
	ino := uint32(0)    // Inode
	uid := uint32(0)    // User ID
	gid := uint32(0)    // Group ID
	size := uint32(entry.Size)

	binary.Write(buf, binary.BigEndian, ctime)
	binary.Write(buf, binary.BigEndian, cmtime)
	binary.Write(buf, binary.BigEndian, mtime)
	binary.Write(buf, binary.BigEndian, mmtime)
	binary.Write(buf, binary.BigEndian, dev)
	binary.Write(buf, binary.BigEndian, ino)
	binary.Write(buf, binary.BigEndian, entry.Mode)
	binary.Write(buf, binary.BigEndian, uid)
	binary.Write(buf, binary.BigEndian, gid)
	binary.Write(buf, binary.BigEndian, size)
	buf.Write(hashBytes)

	// flags (path length)
	pathLen := len(entry.Path)
	flags := uint16(pathLen)
	if pathLen >= 0xFFF {
		flags = 0xFFF
	}
	binary.Write(buf, binary.BigEndian, flags)

	// write path
	buf.WriteString(entry.Path)
	if pathLen >= 0xFFF {
		buf.WriteByte(0) // Null terminator for long paths
	}

	// add padding to align to 8 bytes
	entrySize := 62 + pathLen
	if pathLen >= 0xFFF {
		entrySize++ // For null terminator
	}
	padding := (8 - (entrySize % 8)) % 8
	for i := 0; i < padding; i++ {
		buf.WriteByte(0)
	}

	return nil
}

func (idx *Index) writeTreeRecursive(node *dirNode) (string, error) {
	type treeEntry struct {
		mode  uint32
		name  string
		hash  string
		isDir bool
	}

	var entries []treeEntry

	// subdirectories
	for name, child := range node.children {
		childHash, err := idx.writeTreeRecursive(child)
		if err != nil {
			return "", err
		}
		entries = append(entries, treeEntry{
			mode:  0o040000, // Directory mode
			name:  name,
			hash:  childHash,
			isDir: true,
		})
	}

	for name, file := range node.files {
		entries = append(entries, treeEntry{
			mode:  file.Mode,
			name:  name,
			hash:  file.Hash,
			isDir: false,
		})
	}

	// sort entries (directories and files together, lexicographically)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// build tree object data
	var buf bytes.Buffer
	for _, entry := range entries {
		buf.WriteString(fmt.Sprintf("%06o", entry.mode))
		buf.WriteByte(' ')
		buf.WriteString(entry.name)
		buf.WriteByte(0)

		hashBytes, err := hex.DecodeString(entry.hash)
		if err != nil {
			return "", errors.NewIndexError(entry.name, fmt.Errorf("invalid hash %s: %w", entry.hash, err))
		}
		buf.Write(hashBytes)
	}

	if buf.Len() == 0 {
		return "", errors.NewIndexError("", fmt.Errorf("empty tree"))
	}

	treeHash := hash.ComputeObjectHash("tree", buf.Bytes())
	return treeHash, nil
}
