package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/clone"
)

var (
	cloneDirectory    string
	cloneBranch       string
	cloneDepth        int
	cloneBare         bool
	cloneMirror       bool
	cloneShallow      bool
	cloneSingleBranch bool
	cloneProgress     bool
	cloneTimeout      time.Duration
)

var cloneCmd = &cobra.Command{
	Use:   "clone <repository> [<directory>]",
	Short: "Clone a repository into a new directory",
	Long: `Clones a repository into a newly created directory, creates remote-tracking branches
for each branch in the cloned repository, and creates and checks out an initial branch
that is forked from the cloned repository's currently active branch.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repository := args[0]

		directory := cloneDirectory
		if len(args) > 1 {
			directory = args[1]
		}

		options := clone.DefaultCloneOptions()
		options.URL = repository
		options.Directory = directory
		options.Branch = cloneBranch
		options.Depth = cloneDepth
		options.Bare = cloneBare
		options.Mirror = cloneMirror
		options.Shallow = cloneShallow
		options.SingleBranch = cloneSingleBranch
		options.Progress = cloneProgress
		options.Timeout = cloneTimeout

		if options.Progress {
			options.ProgressWriter = os.Stdout
		}

		cloner := clone.NewCloner()
		ctx := context.Background()

		if options.Progress {
			fmt.Printf("Cloning into '%s'...\n", options.Directory)
		}

		result, err := cloner.Clone(ctx, options)
		if err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}

		printCloneResult(result, options)
		return nil
	},
}

func printCloneResult(result *clone.CloneResult, options clone.CloneOptions) {
	if !options.Progress {
		return
	}

	if result.CheckedOut {
		fmt.Printf("Switched to branch '%s'\n", result.DefaultBranch)
	}

	if len(result.FetchedRefs) > 0 {
		branchCount := 0
		for ref := range result.FetchedRefs {
			if len(ref) > 11 && ref[:11] == "refs/heads/" {
				branchCount++
			}
		}
		if branchCount > 1 {
			fmt.Printf("Branch '%s' set up to track remote branch '%s' from '%s'.\n",
				result.DefaultBranch, result.DefaultBranch, result.RemoteName)
		}
	}

	if result.ObjectCount > 0 {
		fmt.Printf("Received %d objects\n", result.ObjectCount)
	}
}

func init() {
	cloneCmd.Flags().StringVarP(&cloneDirectory, "directory", "d", "", "directory to clone into")
	cloneCmd.Flags().StringVarP(&cloneBranch, "branch", "b", "", "branch to checkout")
	cloneCmd.Flags().IntVar(&cloneDepth, "depth", 0, "create a shallow clone with history truncated to the specified number of commits")
	cloneCmd.Flags().BoolVar(&cloneBare, "bare", false, "create a bare repository")
	cloneCmd.Flags().BoolVar(&cloneMirror, "mirror", false, "create a mirror repository")
	cloneCmd.Flags().BoolVar(&cloneShallow, "shallow-since", false, "create a shallow clone since a given time")
	cloneCmd.Flags().BoolVar(&cloneSingleBranch, "single-branch", false, "clone only one branch")
	cloneCmd.Flags().BoolVar(&cloneProgress, "progress", true, "show progress")
	cloneCmd.Flags().DurationVar(&cloneTimeout, "timeout", 10*time.Minute, "timeout for clone operation")

	rootCmd.AddCommand(cloneCmd)
}
