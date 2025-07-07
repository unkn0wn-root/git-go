package clone

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unkn0wn-root/git-go/index"
	"github.com/unkn0wn-root/git-go/objects"
	"github.com/unkn0wn-root/git-go/pack"
	"github.com/unkn0wn-root/git-go/remote"
	"github.com/unkn0wn-root/git-go/repository"
)

type CloneOptions struct {
	URL            string
	Directory      string
	Branch         string
	Depth          int
	Bare           bool
	Mirror         bool
	Shallow        bool
	SingleBranch   bool
	Progress       bool
	Timeout        time.Duration
	ProgressWriter *os.File
}

type CloneResult struct {
	Repository    *repository.Repository
	RemoteName    string
	DefaultBranch string
	ClonedCommit  string
	FetchedRefs   map[string]string
	CheckedOut    bool
	ObjectCount   int
}

type Cloner struct {
	auth *remote.AuthConfig
}

func NewCloner() *Cloner {
	auth, _ := remote.LoadAuthConfig()
	return &Cloner{auth: auth}
}

func (c *Cloner) Clone(ctx context.Context, options CloneOptions) (*CloneResult, error) {
	if options.URL == "" {
		return nil, fmt.Errorf("repository URL is required")
	}

	if options.Timeout == 0 {
		options.Timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	targetDir := options.Directory
	if targetDir == "" {
		targetDir = c.inferDirectoryName(options.URL)
	}

	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	if _, err := os.Stat(absPath); err == nil {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read target directory: %w", err)
		}
		if len(entries) > 0 {
			return nil, fmt.Errorf("destination path '%s' already exists and is not an empty directory", absPath)
		}
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	repo := repository.New(absPath)
	if err := repo.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	result := &CloneResult{
		Repository:  repo,
		RemoteName:  "origin",
		FetchedRefs: make(map[string]string),
	}

	if err := c.setupRemote(repo, result.RemoteName, options.URL); err != nil {
		return nil, fmt.Errorf("failed to setup remote: %w", err)
	}

	transport, err := remote.CreateTransport(options.URL, c.auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	defer transport.Close()

	if err := transport.Connect(ctx, options.URL); err != nil {
		return nil, fmt.Errorf("failed to connect to remote: %w", err)
	}

	remoteRefs, err := transport.ListRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	if len(remoteRefs) == 0 {
		return nil, fmt.Errorf("remote repository has no refs")
	}

	defaultBranch := c.determineDefaultBranch(remoteRefs, options.Branch)
	if defaultBranch == "" {
		return nil, fmt.Errorf("could not determine default branch")
	}
	result.DefaultBranch = defaultBranch

	defaultBranchRef := fmt.Sprintf("refs/heads/%s", defaultBranch)
	commitHash, exists := remoteRefs[defaultBranchRef]
	if !exists {
		return nil, fmt.Errorf("default branch '%s' not found on remote", defaultBranch)
	}
	result.ClonedCommit = commitHash

	var wants []string
	if options.SingleBranch {
		wants = []string{commitHash}
	} else {
		for _, hash := range remoteRefs {
			wants = append(wants, hash)
		}
	}

	if options.Progress && options.ProgressWriter != nil {
		fmt.Fprintf(options.ProgressWriter, "Fetching objects...\n")
	}

	packReader, err := transport.FetchPack(ctx, wants, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pack: %w", err)
	}
	defer packReader.Close()

	if err := c.processPack(repo, packReader); err != nil {
		return nil, fmt.Errorf("failed to process pack: %w", err)
	}

	result.ObjectCount = c.countObjects(repo)

	if err := c.updateRemoteRefs(repo, remoteRefs, result.RemoteName, options.SingleBranch, defaultBranch); err != nil {
		return nil, fmt.Errorf("failed to update remote refs: %w", err)
	}

	result.FetchedRefs = remoteRefs

	if !options.Bare {
		if err := c.createLocalBranch(repo, defaultBranch, commitHash); err != nil {
			return nil, fmt.Errorf("failed to create local branch: %w", err)
		}

		if err := c.checkoutBranch(repo, commitHash, result); err != nil {
			return nil, fmt.Errorf("failed to checkout branch: %w", err)
		}

		result.CheckedOut = true
	}

	if options.Progress && options.ProgressWriter != nil {
		fmt.Fprintf(options.ProgressWriter, "Cloning complete.\n")
	}

	return result, nil
}

func (c *Cloner) inferDirectoryName(url string) string {
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "repository"
	}

	name := parts[len(parts)-1]
	if strings.HasSuffix(name, ".git") {
		name = strings.TrimSuffix(name, ".git")
	}

	if strings.Contains(name, ":") {
		parts := strings.Split(name, ":")
		if len(parts) > 1 {
			name = parts[len(parts)-1]
		}
	}

	if name == "" {
		return "repository"
	}

	return name
}

func (c *Cloner) setupRemote(repo *repository.Repository, remoteName, url string) error {
	rc := remote.NewRemoteConfig(repo.GitDir)
	if err := rc.Load(); err != nil {
		return fmt.Errorf("failed to load remote config: %w", err)
	}

	return rc.AddRemote(remoteName, url)
}

func (c *Cloner) determineDefaultBranch(remoteRefs map[string]string, preferredBranch string) string {
	if preferredBranch != "" {
		branchRef := fmt.Sprintf("refs/heads/%s", preferredBranch)
		if _, exists := remoteRefs[branchRef]; exists {
			return preferredBranch
		}
		return ""
	}

	if headRef, exists := remoteRefs["HEAD"]; exists {
		for ref, hash := range remoteRefs {
			if hash == headRef && strings.HasPrefix(ref, "refs/heads/") {
				return strings.TrimPrefix(ref, "refs/heads/")
			}
		}
	}

	defaultNames := []string{"main", "master", "develop", "trunk"}
	for _, name := range defaultNames {
		branchRef := fmt.Sprintf("refs/heads/%s", name)
		if _, exists := remoteRefs[branchRef]; exists {
			return name
		}
	}

	for ref := range remoteRefs {
		if strings.HasPrefix(ref, "refs/heads/") {
			return strings.TrimPrefix(ref, "refs/heads/")
		}
	}

	return ""
}

func (c *Cloner) processPack(repo *repository.Repository, packReader remote.PackReader) error {
	processor := pack.NewPackProcessor(repo)
	if err := processor.ProcessPack(packReader); err != nil {
		return fmt.Errorf("failed to process pack with full object transfer: %w", err)
	}

	return nil
}

func (c *Cloner) updateRemoteRefs(repo *repository.Repository, remoteRefs map[string]string, remoteName string, singleBranch bool, defaultBranch string) error {
	remoteRefsDir := filepath.Join(repo.GitDir, "refs", "remotes", remoteName)
	if err := os.MkdirAll(remoteRefsDir, 0755); err != nil {
		return fmt.Errorf("failed to create remote refs directory: %w", err)
	}

	for refName, hash := range remoteRefs {
		if !strings.HasPrefix(refName, "refs/heads/") {
			continue
		}

		branchName := strings.TrimPrefix(refName, "refs/heads/")
		if singleBranch && branchName != defaultBranch {
			continue
		}

		remoteRefPath := filepath.Join(remoteRefsDir, branchName)
		if err := os.WriteFile(remoteRefPath, []byte(hash+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to update remote ref %s: %w", refName, err)
		}
	}

	return nil
}

func (c *Cloner) createLocalBranch(repo *repository.Repository, branchName, commitHash string) error {
	branchRef := fmt.Sprintf("refs/heads/%s", branchName)
	if err := repo.UpdateRef(branchRef, commitHash); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	headPath := filepath.Join(repo.GitDir, "HEAD")
	headContent := fmt.Sprintf("ref: %s\n", branchRef)
	if err := os.WriteFile(headPath, []byte(headContent), 0644); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}

	return nil
}

func (c *Cloner) checkoutBranch(repo *repository.Repository, commitHash string, result *CloneResult) error {
	obj, err := repo.LoadObject(commitHash)
	if err != nil {
		return fmt.Errorf("failed to load commit object %s: %w", commitHash, err)
	}

	commit, ok := obj.(*objects.Commit)
	if !ok {
		return fmt.Errorf("object is not a commit")
	}

	treeObj, err := repo.LoadObject(commit.Tree())
	if err != nil {
		return fmt.Errorf("failed to load tree: %w", err)
	}

	tree, ok := treeObj.(*objects.Tree)
	if !ok {
		return fmt.Errorf("tree object is not a tree")
	}

	idx := index.New(repo.GitDir)
	updatedFiles, err := repo.CheckoutTreeWithIndex(tree, idx, "")
	if err != nil {
		return err
	}

	for range updatedFiles {
		result.ObjectCount++
	}

	if err := idx.Save(); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

func (c *Cloner) countObjects(repo *repository.Repository) int {
	objectsDir := filepath.Join(repo.GitDir, "objects")
	count := 0

	filepath.Walk(objectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && len(filepath.Base(path)) == 38 {
			// git object files are 38 characters (2 for directory + 38 for filename)
			count++
		}
		return nil
	})

	return count
}

func DefaultCloneOptions() CloneOptions {
	return CloneOptions{
		Branch:       "",
		Depth:        0,
		Bare:         false,
		Mirror:       false,
		Shallow:      false,
		SingleBranch: false,
		Progress:     true,
		Timeout:      10 * time.Minute,
	}
}
