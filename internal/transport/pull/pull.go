package pull

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/internal/core/index"
	"github.com/unkn0wn-root/git-go/internal/core/objects"
	"github.com/unkn0wn-root/git-go/internal/core/pack"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
	"github.com/unkn0wn-root/git-go/internal/transport/remote"
	"github.com/unkn0wn-root/git-go/pkg/errors"
)

const (
	defaultRemote   = "origin"
	defaultTimeout  = 5 * time.Minute
	defaultDirMode  = 0755
	defaultFileMode = 0644
	executableMode  = 0755
)

type PullStrategy int

const (
	PullMerge PullStrategy = iota
	PullRebase
	PullFastForward
)

type PullOptions struct {
	Remote         string
	Branch         string
	Strategy       PullStrategy
	AllowUnrelated bool
	Force          bool
	Prune          bool
	Depth          int
	Timeout        time.Duration
}

type PullResult struct {
	Strategy      PullStrategy
	OldCommit     string
	NewCommit     string
	UpdatedRefs   map[string]string
	ConflictFiles []string
	FastForward   bool
	CommitsAhead  int
	CommitsBehind int
	MergeCommit   string
	UpdatedFiles  []string
	DeletedFiles  []string
	AddedFiles    []string
}

type Puller struct {
	repo      *repository.Repository
	transport remote.Transport
	auth      *remote.AuthConfig
	index     *index.Index
}

func NewPuller(repo *repository.Repository) *Puller {
	auth, _ := remote.LoadAuthConfig()
	return &Puller{
		repo:  repo,
		auth:  auth,
		index: index.New(repo.GitDir),
	}
}

func (p *Puller) Pull(ctx context.Context, options PullOptions) (*PullResult, error) {
	if options.Remote == "" {
		options.Remote = defaultRemote
	}

	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
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

	transport, err := remote.CreateTransport(remoteConfig.FetchURL, p.auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	defer transport.Close()

	p.transport = transport

	if err := transport.Connect(ctx, remoteConfig.FetchURL); err != nil {
		return nil, fmt.Errorf("failed to connect to remote: %w", err)
	}

	currentBranch, err := p.repo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	if options.Branch == "" {
		options.Branch = currentBranch
	}

	remoteRefs, err := transport.ListRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	remoteBranchRef := fmt.Sprintf("refs/heads/%s", options.Branch)
	remoteCommit, exists := remoteRefs[remoteBranchRef]
	if !exists {
		return nil, fmt.Errorf("remote branch '%s' not found", options.Branch)
	}

	localCommit, err := p.repo.GetHead()
	if err != nil && !stderrors.Is(err, errors.ErrReferenceNotFound) {
		return nil, fmt.Errorf("failed to get local HEAD: %w", err)
	}

	if localCommit == remoteCommit {
		return &PullResult{
			Strategy:      options.Strategy,
			OldCommit:     localCommit,
			NewCommit:     remoteCommit,
			UpdatedRefs:   make(map[string]string),
			FastForward:   true,
			CommitsAhead:  0,
			CommitsBehind: 0,
		}, nil
	}

	result := &PullResult{
		Strategy:    options.Strategy,
		OldCommit:   localCommit,
		NewCommit:   remoteCommit,
		UpdatedRefs: make(map[string]string),
	}

	if err := p.fetchCommits(ctx, []string{remoteCommit}, []string{localCommit}); err != nil {
		return nil, fmt.Errorf("failed to fetch commits: %w", err)
	}

	if err := p.updateRemoteRefs(remoteRefs, options.Remote); err != nil {
		return nil, fmt.Errorf("failed to update remote refs: %w", err)
	}

	if localCommit == "" {
		result.FastForward = true
		branchRef := fmt.Sprintf("refs/heads/%s", currentBranch)
		if err := p.repo.UpdateRef(branchRef, remoteCommit); err != nil {
			return nil, fmt.Errorf("failed to update branch ref: %w", err)
		}
		result.UpdatedRefs[branchRef] = remoteCommit
		return result, nil
	}

	mergeBase, err := p.findMergeBase(localCommit, remoteCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to find merge base: %w", err)
	}

	if mergeBase == localCommit {
		result.FastForward = true
		if err := p.fastForward(currentBranch, remoteCommit, result); err != nil {
			return nil, fmt.Errorf("fast-forward failed: %w", err)
		}
		return result, nil
	}

	if mergeBase == remoteCommit {
		result.FastForward = false
		result.CommitsAhead = 1
		return result, nil
	}

	switch options.Strategy {
	case PullMerge:
		if err := p.performMerge(currentBranch, remoteCommit, result); err != nil {
			return nil, fmt.Errorf("merge failed: %w", err)
		}
	case PullRebase:
		if err := p.performRebase(currentBranch, remoteCommit, result); err != nil {
			return nil, fmt.Errorf("rebase failed: %w", err)
		}
	case PullFastForward:
		if !result.FastForward {
			return nil, fmt.Errorf("cannot fast-forward: branches have diverged")
		}
	}

	return result, nil
}

func (p *Puller) fetchCommits(ctx context.Context, wants, haves []string) error {
	packReader, err := p.transport.FetchPack(ctx, wants, haves)
	if err != nil {
		return fmt.Errorf("failed to fetch pack: %w", err)
	}
	defer packReader.Close()

	return p.processPack(packReader)
}

func (p *Puller) processPack(reader remote.PackReader) error {
	processor := pack.NewPackProcessor(p.repo)
	return processor.ProcessPack(reader)
}

func (p *Puller) updateRemoteRefs(remoteRefs map[string]string, remoteName string) error {
	remoteRefsDir := filepath.Join(p.repo.GitDir, "refs", "remotes", remoteName)
	if err := os.MkdirAll(remoteRefsDir, defaultDirMode); err != nil {
		return fmt.Errorf("failed to create remote refs directory: %w", err)
	}

	for refName, hash := range remoteRefs {
		if strings.HasPrefix(refName, "refs/heads/") {
			branchName := strings.TrimPrefix(refName, "refs/heads/")
			remoteRefPath := filepath.Join(remoteRefsDir, branchName)

			if err := os.WriteFile(remoteRefPath, []byte(hash+"\n"), defaultFileMode); err != nil {
				return fmt.Errorf("failed to update remote ref %s: %w", refName, err)
			}
		}
	}

	return nil
}

func (p *Puller) findMergeBase(commit1, commit2 string) (string, error) {
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

func (p *Puller) getAncestors(commitHash string) ([]string, error) {
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

func (p *Puller) fastForward(branch, targetCommit string, result *PullResult) error {
	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	if err := p.repo.UpdateRef(branchRef, targetCommit); err != nil {
		return fmt.Errorf("failed to update branch ref: %w", err)
	}

	result.UpdatedRefs[branchRef] = targetCommit

	if err := p.updateWorkingDirectory(targetCommit, result); err != nil {
		return fmt.Errorf("failed to update working directory: %w", err)
	}

	return nil
}

func (p *Puller) performMerge(branch, remoteCommit string, result *PullResult) error {
	localCommit := result.OldCommit
	mergeMessage := fmt.Sprintf("Merge remote-tracking branch 'origin/%s' into %s", branch, branch)

	now := time.Now()
	author := &objects.Signature{
		Name:  "Git Pull",
		Email: "git@local.repo",
		When:  now,
	}

	treeHash, err := p.createMergeTree(localCommit, remoteCommit)
	if err != nil {
		return fmt.Errorf("failed to create merge tree: %w", err)
	}

	mergeCommit := objects.NewCommit(
		treeHash,
		[]string{localCommit, remoteCommit},
		author,
		author,
		mergeMessage,
	)

	mergeCommitHash, err := p.repo.StoreObject(mergeCommit)
	if err != nil {
		return fmt.Errorf("failed to store merge commit: %w", err)
	}

	branchRef := fmt.Sprintf("refs/heads/%s", branch)
	if err := p.repo.UpdateRef(branchRef, mergeCommitHash); err != nil {
		return fmt.Errorf("failed to update branch ref: %w", err)
	}

	result.MergeCommit = mergeCommitHash
	result.UpdatedRefs[branchRef] = mergeCommitHash

	if err := p.updateWorkingDirectory(mergeCommitHash, result); err != nil {
		return fmt.Errorf("failed to update working directory: %w", err)
	}

	return nil
}

func (p *Puller) ensureIndexLoaded() error {
	if p.index == nil {
		p.index = index.New(p.repo.GitDir)
	}

	if err := p.index.Load(); err != nil {
		return fmt.Errorf("failed to load git index: %w", err)
	}
	return nil
}

func (p *Puller) performRebase(branch, remoteCommit string, result *PullResult) error {
	return fmt.Errorf("rebase strategy not implemented yet")
}

func (p *Puller) createMergeTree(localCommit, remoteCommit string) (string, error) {
	localObj, err := p.repo.LoadObject(localCommit)
	if err != nil {
		return "", fmt.Errorf("failed to load local commit: %w", err)
	}

	localCommitObj, ok := localObj.(*objects.Commit)
	if !ok {
		return "", fmt.Errorf("local commit is not a commit object")
	}

	return localCommitObj.Tree(), nil
}

func (p *Puller) updateWorkingDirectory(commitHash string, result *PullResult) error {
	// ensure index is loaded before using it
	if err := p.ensureIndexLoaded(); err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	obj, err := p.repo.LoadObject(commitHash)
	if err != nil {
		return fmt.Errorf("failed to load commit: %w", err)
	}

	commit, ok := obj.(*objects.Commit)
	if !ok {
		return fmt.Errorf("object is not a commit")
	}

	treeObj, err := p.repo.LoadObject(commit.Tree())
	if err != nil {
		return fmt.Errorf("failed to load tree: %w", err)
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return fmt.Errorf("object is not a tree")
	}

	p.index.Clear()

	updatedFiles, err := p.repo.CheckoutTreeWithIndex(tree, p.index, "")
	if err != nil {
		return err
	}
	result.UpdatedFiles = append(result.UpdatedFiles, updatedFiles...)
	if err := p.index.Save(); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

func (p *Puller) checkoutTree(tree *objects.Tree, prefix string, result *PullResult) error {
	for _, entry := range tree.Entries() {
		fullPath := filepath.Join(p.repo.WorkDir, prefix, entry.Name)

		switch entry.Mode {
		case objects.FileModeTree:
			if err := os.MkdirAll(fullPath, defaultDirMode); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}

			subTreeObj, err := p.repo.LoadObject(entry.Hash)
			if err != nil {
				// if subtree object not found, it may not have been included in this pack
				// This can happen when the subtree hasn't changed and only files were updated
				// skip this subtree since the working directory files are already correct
				continue
			}

			subTree, ok := subTreeObj.(*objects.Tree)
			if !ok {
				return fmt.Errorf("subtree object is not a tree")
			}

			if err := p.checkoutTree(subTree, filepath.Join(prefix, entry.Name), result); err != nil {
				return err
			}

		case objects.FileModeBlob, objects.FileModeExecutable:
			blobObj, err := p.repo.LoadObject(entry.Hash)
			if err != nil {
				// skip files whose blobs can't be loaded (they may not be in the pack)
				continue
			}

			blob, ok := blobObj.(*objects.Blob)
			if !ok {
				return fmt.Errorf("blob object is not a blob")
			}

			if err := os.MkdirAll(filepath.Dir(fullPath), defaultDirMode); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", fullPath, err)
			}

			mode := os.FileMode(defaultFileMode)
			if entry.Mode == objects.FileModeExecutable {
				mode = os.FileMode(executableMode)
			}

			if err := os.WriteFile(fullPath, blob.Content(), mode); err != nil {
				return fmt.Errorf("failed to write file %s: %w", fullPath, err)
			}

			result.UpdatedFiles = append(result.UpdatedFiles, filepath.Join(prefix, entry.Name))
		}
	}

	return nil
}

func DefaultPullOptions() PullOptions {
	return PullOptions{
		Remote:         defaultRemote,
		Strategy:       PullMerge,
		AllowUnrelated: false,
		Force:          false,
		Prune:          false,
		Depth:          0,
		Timeout:        defaultTimeout,
	}
}
