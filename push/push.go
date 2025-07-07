package push

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/remote"
	"github.com/unkn0wn-root/git-go/repository"
)

type RefUpdateStatus int

const (
	RefUpdateOK RefUpdateStatus = iota
	RefUpdateRejected
	RefUpdateError
	RefUpdateUpToDate
	RefUpdateFastForward
	RefUpdateForced
)

type PushOptions struct {
	Remote         string
	Branch         string
	Force          bool
	SetUpstream    bool
	PushAll        bool
	PushTags       bool
	DryRun         bool
	Timeout        time.Duration
	ProgressWriter *os.File
}

type PushResult struct {
	Remote        string
	Branch        string
	OldCommit     string
	NewCommit     string
	UpdatedRefs   map[string]RefUpdateResult
	RejectedRefs  map[string]string
	FastForward   bool
	Forced        bool
	NewBranch     bool
	UpstreamSet   bool
	PushedObjects int
	PushedSize    int64
}

type RefUpdateResult struct {
	RefName string
	OldHash string
	NewHash string
	Status  RefUpdateStatus
	Message string
}

func (s RefUpdateStatus) String() string {
	switch s {
	case RefUpdateOK:
		return "ok"
	case RefUpdateRejected:
		return "rejected"
	case RefUpdateError:
		return "error"
	case RefUpdateUpToDate:
		return "up-to-date"
	case RefUpdateFastForward:
		return "fast-forward"
	case RefUpdateForced:
		return "forced"
	default:
		return "unknown"
	}
}

type Pusher struct {
	repo      *repository.Repository
	transport remote.Transport
	auth      *remote.AuthConfig
}

func NewPusher(repo *repository.Repository) *Pusher {
	auth, _ := remote.LoadAuthConfig()
	return &Pusher{
		repo: repo,
		auth: auth,
	}
}

func (p *Pusher) Push(ctx context.Context, options PushOptions) (*PushResult, error) {
	if options.Remote == "" {
		options.Remote = "origin"
	}

	if options.Timeout == 0 {
		options.Timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	rc := remote.NewRemoteConfig(p.repo.GitDir)
	if err := rc.Load(); err != nil {
		return nil, fmt.Errorf("failed to load remote config: %w", err)
	}

	remoteConfig, err := rc.GetRemote(options.Remote)
	if err != nil {
		return nil, fmt.Errorf("remote '%s' not found: %w", options.Remote, err)
	}

	transport, err := remote.CreateTransport(remoteConfig.PushURL, p.auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	defer transport.Close()

	p.transport = transport

	if err := transport.Connect(ctx, remoteConfig.PushURL); err != nil {
		return nil, fmt.Errorf("failed to connect to remote: %w", err)
	}

	currentBranch, err := p.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	if options.Branch == "" {
		options.Branch = currentBranch
	}

	localCommit, err := p.repo.GetHead()
	if err != nil {
		return nil, fmt.Errorf("failed to get local HEAD: %w", err)
	}

	if localCommit == "" {
		return nil, fmt.Errorf("no commits to push")
	}

	result := &PushResult{
		Remote:       options.Remote,
		Branch:       options.Branch,
		NewCommit:    localCommit,
		UpdatedRefs:  make(map[string]RefUpdateResult),
		RejectedRefs: make(map[string]string),
	}

	remoteRefs, err := transport.ListRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	remoteBranchRef := fmt.Sprintf("refs/heads/%s", options.Branch)
	remoteCommit, exists := remoteRefs[remoteBranchRef]

	if !exists {
		result.NewBranch = true
		result.OldCommit = ""
	} else {
		result.OldCommit = remoteCommit

		if remoteCommit == localCommit {
			result.UpdatedRefs[remoteBranchRef] = RefUpdateResult{
				RefName: remoteBranchRef,
				OldHash: remoteCommit,
				NewHash: localCommit,
				Status:  RefUpdateUpToDate,
				Message: "Everything up-to-date",
			}
			return result, nil
		}
	}

	if !options.Force && exists {
		canFastForward, err := p.canFastForward(remoteCommit, localCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to check fast-forward: %w", err)
		}

		if !canFastForward {
			result.RejectedRefs[remoteBranchRef] = "non-fast-forward"
			return result, fmt.Errorf("updates were rejected because the remote contains work that you do not have locally")
		}

		result.FastForward = true
	}

	if options.Force && exists {
		result.Forced = true
	}

	if options.DryRun {
		return result, nil
	}

	objectsToSend, err := p.getObjectsToSend(localCommit, remoteCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get objects to send: %w", err)
	}

	refUpdates := map[string]remote.RefUpdate{
		remoteBranchRef: {
			RefName: remoteBranchRef,
			OldHash: result.OldCommit,
			NewHash: result.NewCommit,
		},
	}

	if len(objectsToSend) > 0 {
		result.PushedObjects = len(objectsToSend)

		// Create pack data and send with refs
		packData, err := p.createPackFile(objectsToSend)
		if err != nil {
			return nil, fmt.Errorf("failed to create pack file: %w", err)
		}

		result.PushedSize = int64(len(packData))
		fmt.Printf("Pushing %d objects (%d bytes)\n", len(objectsToSend), len(packData))

		if err := transport.SendPack(ctx, refUpdates, packData); err != nil {
			return nil, fmt.Errorf("failed to send pack with data: %w", err)
		}
	} else {
		// No objects to send, just update refs
		if err := transport.SendPack(ctx, refUpdates, nil); err != nil {
			return nil, fmt.Errorf("failed to send pack: %w", err)
		}
	}

	status := RefUpdateOK
	if result.FastForward {
		status = RefUpdateFastForward
	} else if result.Forced {
		status = RefUpdateForced
	}

	result.UpdatedRefs[remoteBranchRef] = RefUpdateResult{
		RefName: remoteBranchRef,
		OldHash: result.OldCommit,
		NewHash: result.NewCommit,
		Status:  status,
		Message: p.getUpdateMessage(result),
	}

	if options.SetUpstream {
		if err := p.setUpstream(options.Branch, options.Remote); err != nil {
			return nil, fmt.Errorf("failed to set upstream: %w", err)
		}
		result.UpstreamSet = true
	}

	return result, nil
}

func (p *Pusher) canFastForward(remoteCommit, localCommit string) (bool, error) {
	if remoteCommit == "" {
		return true, nil
	}

	mergeBase, err := p.findMergeBase(remoteCommit, localCommit)
	if err != nil {
		return false, fmt.Errorf("failed to find merge base: %w", err)
	}

	return mergeBase == remoteCommit, nil
}

func (p *Pusher) findMergeBase(commit1, commit2 string) (string, error) {
	if commit1 == commit2 {
		return commit1, nil
	}

	ancestors1, err := p.getAncestors(commit1)
	if err != nil {
		return "", fmt.Errorf("failed to get ancestors of %s: %w", commit1, err)
	}

	ancestors2, err := p.getAncestors(commit2)
	if err != nil {
		return "", fmt.Errorf("failed to get ancestors of %s: %w", commit2, err)
	}

	for _, ancestor := range ancestors1 {
		for _, other := range ancestors2 {
			if ancestor == other {
				return ancestor, nil
			}
		}
	}

	return "", fmt.Errorf("no common ancestor found")
}

func (p *Pusher) getAncestors(commitHash string) ([]string, error) {
	var ancestors []string
	visited := make(map[string]bool)
	queue := []string{commitHash}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true
		ancestors = append(ancestors, current)

		obj, err := p.repo.LoadObject(current)
		if err != nil {
			continue
		}

		if commit, ok := obj.(*objects.Commit); ok {
			for _, parent := range commit.Parents() {
				if !visited[parent] {
					queue = append(queue, parent)
				}
			}
		}
	}

	return ancestors, nil
}

func (p *Pusher) getObjectsToSend(localCommit, remoteCommit string) ([]string, error) {
	var objectsToSend []string
	visited := make(map[string]bool)

	if remoteCommit != "" {
		remoteObjects, err := p.getAncestors(remoteCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to get remote ancestors: %w", err)
		}

		for _, obj := range remoteObjects {
			visited[obj] = true
		}
	}

	queue := []string{localCommit}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true
		objectsToSend = append(objectsToSend, current)

		obj, err := p.repo.LoadObject(current)
		if err != nil {
			continue
		}

		switch o := obj.(type) {
		case *objects.Commit:
			objectsToSend = append(objectsToSend, o.Tree())

			treeObj, err := p.repo.LoadObject(o.Tree())
			if err == nil {
				if tree, ok := treeObj.(*objects.Tree); ok {
					p.collectTreeObjects(tree, &objectsToSend, visited)
				}
			}

			for _, parent := range o.Parents() {
				if !visited[parent] {
					queue = append(queue, parent)
				}
			}
		}
	}

	return objectsToSend, nil
}

func (p *Pusher) collectTreeObjects(tree *objects.Tree, objectsToSend *[]string, visited map[string]bool) {
	for _, entry := range tree.Entries() {
		if visited[entry.Hash] {
			continue
		}
		visited[entry.Hash] = true
		*objectsToSend = append(*objectsToSend, entry.Hash)

		if entry.Mode == objects.FileModeTree {
			subTreeObj, err := p.repo.LoadObject(entry.Hash)
			if err == nil {
				if subTree, ok := subTreeObj.(*objects.Tree); ok {
					p.collectTreeObjects(subTree, objectsToSend, visited)
				}
			}
		}
	}
}

func (p *Pusher) createPackFile(objectHashes []string) ([]byte, error) {
	var packBuffer bytes.Buffer
	// write pack header: "PACK" + version + object count
	packBuffer.WriteString("PACK")
	binary.Write(&packBuffer, binary.BigEndian, uint32(2)) // Version 2
	binary.Write(&packBuffer, binary.BigEndian, uint32(len(objectHashes)))

	for _, hash := range objectHashes {
		objData, err := p.createPackObject(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to create pack object %s: %w", hash, err)
		}
		packBuffer.Write(objData)
	}

	// calculate and append SHA-1 checksum of pack data
	h := sha1.New()
	h.Write(packBuffer.Bytes())
	checksum := h.Sum(nil)
	packBuffer.Write(checksum)

	return packBuffer.Bytes(), nil
}

func (p *Pusher) createPackObject(hash string) ([]byte, error) {
	obj, err := p.repo.LoadObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to load object %s: %w", hash, err)
	}

	var objType int
	var objData []byte

	// determine object type and get raw data
	switch o := obj.(type) {
	case *objects.Blob:
		objType = 3 // OBJ_BLOB
		objData = o.Content()
	case *objects.Tree:
		objType = 2 // OBJ_TREE
		objData = o.Data()
	case *objects.Commit:
		objType = 1 // OBJ_COMMIT
		objData = o.Data()
	default:
		return nil, fmt.Errorf("unsupported object type for %s", hash)
	}

	header := p.createObjectHeader(objType, int64(len(objData)))

	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(objData); err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to compress object data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize compression: %w", err)
	}

	var result bytes.Buffer
	result.Write(header)
	result.Write(compressed.Bytes())
	return result.Bytes(), nil
}

func (p *Pusher) createObjectHeader(objType int, size int64) []byte {
	var header []byte

	// first byte: MSB=0, type (3 bits), size (4 bits)
	firstByte := byte((objType << 4) | (int(size) & 0xF))
	size >>= 4

	if size > 0 {
		firstByte |= 0x80 // Set continuation bit
	}

	header = append(header, firstByte)

	// additional bytes for larger sizes
	for size > 0 {
		nextByte := byte(size & 0x7F)
		size >>= 7
		if size > 0 {
			nextByte |= 0x80 // Set continuation bit
		}

		header = append(header, nextByte)
	}

	return header
}

func (p *Pusher) setUpstream(branch, remote string) error {
	configPath := filepath.Join(p.repo.GitDir, "config")

	config := fmt.Sprintf(`[branch "%s"]
	remote = %s
	merge = refs/heads/%s
`, branch, remote, branch)

	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(config); err != nil {
		return fmt.Errorf("failed to write upstream config: %w", err)
	}

	return nil
}

func (p *Pusher) getUpdateMessage(result *PushResult) string {
	if result.NewBranch {
		return fmt.Sprintf("new branch '%s'", result.Branch)
	}

	if result.Forced {
		return fmt.Sprintf("forced update %s..%s", result.OldCommit[:7], result.NewCommit[:7])
	}

	if result.FastForward {
		return fmt.Sprintf("fast-forward %s..%s", result.OldCommit[:7], result.NewCommit[:7])
	}

	return fmt.Sprintf("updated %s..%s", result.OldCommit[:7], result.NewCommit[:7])
}

func (p *Pusher) PushAll(ctx context.Context, options PushOptions) (*PushResult, error) {
	branches, err := p.getAllBranches()
	if err != nil {
		return nil, fmt.Errorf("failed to get all branches: %w", err)
	}

	result := &PushResult{
		Remote:       options.Remote,
		UpdatedRefs:  make(map[string]RefUpdateResult),
		RejectedRefs: make(map[string]string),
	}

	for _, branch := range branches {
		branchOptions := options
		branchOptions.Branch = branch
		branchOptions.PushAll = false

		branchResult, err := p.Push(ctx, branchOptions)
		if err != nil {
			result.RejectedRefs[fmt.Sprintf("refs/heads/%s", branch)] = err.Error()
			continue
		}

		for ref, update := range branchResult.UpdatedRefs {
			result.UpdatedRefs[ref] = update
		}

		for ref, reason := range branchResult.RejectedRefs {
			result.RejectedRefs[ref] = reason
		}
	}

	return result, nil
}

func (p *Pusher) getAllBranches() ([]string, error) {
	refsDir := filepath.Join(p.repo.GitDir, "refs", "heads")

	entries, err := os.ReadDir(refsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read refs directory: %w", err)
	}

	var branches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			branches = append(branches, entry.Name())
		}
	}

	return branches, nil
}

func (p *Pusher) PushTags(ctx context.Context, options PushOptions) (*PushResult, error) {
	tags, err := p.getAllTags()
	if err != nil {
		return nil, fmt.Errorf("failed to get all tags: %w", err)
	}

	result := &PushResult{
		Remote:       options.Remote,
		UpdatedRefs:  make(map[string]RefUpdateResult),
		RejectedRefs: make(map[string]string),
	}

	for _, tag := range tags {
		tagRef := fmt.Sprintf("refs/tags/%s", tag)

		tagHash, err := p.getTagHash(tag)
		if err != nil {
			result.RejectedRefs[tagRef] = err.Error()
			continue
		}

		result.UpdatedRefs[tagRef] = RefUpdateResult{
			RefName: tagRef,
			OldHash: "",
			NewHash: tagHash,
			Status:  RefUpdateOK,
			Message: fmt.Sprintf("new tag '%s'", tag),
		}
	}

	return result, nil
}

func (p *Pusher) getAllTags() ([]string, error) {
	tagsDir := filepath.Join(p.repo.GitDir, "refs", "tags")
	entries, err := os.ReadDir(tagsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read tags directory: %w", err)
	}

	var tags []string
	for _, entry := range entries {
		if !entry.IsDir() {
			tags = append(tags, entry.Name())
		}
	}

	return tags, nil
}

func (p *Pusher) getTagHash(tag string) (string, error) {
	tagPath := filepath.Join(p.repo.GitDir, "refs", "tags", tag)
	content, err := os.ReadFile(tagPath)
	if err != nil {
		return "", fmt.Errorf("failed to read tag %s: %w", tag, err)
	}

	return strings.TrimSpace(string(content)), nil
}

func DefaultPushOptions() PushOptions {
	return PushOptions{
		Remote:      "origin",
		Force:       false,
		SetUpstream: false,
		PushAll:     false,
		PushTags:    false,
		DryRun:      false,
		Timeout:     5 * time.Minute,
	}
}
