package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/unkn0wn-root/git-go/pull"
	"github.com/unkn0wn-root/git-go/repository"
	"github.com/spf13/cobra"
)

var (
	pullRemote         string
	pullBranch         string
	pullRebase         bool
	pullFastForward    bool
	pullAllowUnrelated bool
	pullForce          bool
	pullPrune          bool
	pullDepth          int
	pullTimeout        time.Duration
)

var pullCmd = &cobra.Command{
	Use:   "pull [<remote>] [<branch>]",
	Short: "Fetch from and integrate with another repository or branch",
	Long: `Incorporates changes from a remote repository into the current branch.
In its default mode, git pull is shorthand for git fetch followed by git merge FETCH_HEAD.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		options := pull.DefaultPullOptions()
		
		if len(args) > 0 {
			options.Remote = args[0]
		}
		if len(args) > 1 {
			options.Branch = args[1]
		}

		if pullRemote != "" {
			options.Remote = pullRemote
		}
		if pullBranch != "" {
			options.Branch = pullBranch
		}

		if pullRebase {
			options.Strategy = pull.PullRebase
		} else if pullFastForward {
			options.Strategy = pull.PullFastForward
		} else {
			options.Strategy = pull.PullMerge
		}

		options.AllowUnrelated = pullAllowUnrelated
		options.Force = pullForce
		options.Prune = pullPrune
		options.Depth = pullDepth
		options.Timeout = pullTimeout

		puller := pull.NewPuller(repo)
		ctx := context.Background()

		fmt.Printf("Pulling from %s...\n", options.Remote)
		
		result, err := puller.Pull(ctx, options)
		if err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}

		printPullResult(result)
		return nil
	},
}

func printPullResult(result *pull.PullResult) {
	if result.OldCommit == result.NewCommit {
		fmt.Println("Already up to date.")
		return
	}

	if result.FastForward {
		fmt.Printf("Fast-forward\n")
		if result.OldCommit != "" {
			fmt.Printf(" %s..%s\n", result.OldCommit[:7], result.NewCommit[:7])
		} else {
			fmt.Printf(" * [new branch] -> %s\n", result.NewCommit[:7])
		}
	} else if result.MergeCommit != "" {
		fmt.Printf("Merge made by the 'recursive' strategy.\n")
		fmt.Printf(" %s\n", result.MergeCommit[:7])
	}

	if len(result.UpdatedFiles) > 0 {
		fmt.Printf(" %d file(s) changed", len(result.UpdatedFiles))
		if len(result.AddedFiles) > 0 {
			fmt.Printf(", %d insertion(s)", len(result.AddedFiles))
		}
		if len(result.DeletedFiles) > 0 {
			fmt.Printf(", %d deletion(s)", len(result.DeletedFiles))
		}
		fmt.Println()
	}

	if len(result.ConflictFiles) > 0 {
		fmt.Printf("CONFLICT: Merge conflicts in %d file(s):\n", len(result.ConflictFiles))
		for _, file := range result.ConflictFiles {
			fmt.Printf("  %s\n", file)
		}
		fmt.Println("Automatic merge failed; fix conflicts and then commit the result.")
	}

	if result.CommitsBehind > 0 {
		fmt.Printf("Your branch is behind '%s' by %d commit(s).\n", 
			result.NewCommit[:7], result.CommitsBehind)
	}

	if result.CommitsAhead > 0 {
		fmt.Printf("Your branch is ahead of '%s' by %d commit(s).\n", 
			result.OldCommit[:7], result.CommitsAhead)
	}
}

func init() {
	pullCmd.Flags().StringVarP(&pullRemote, "remote", "r", "", "remote repository")
	pullCmd.Flags().StringVarP(&pullBranch, "branch", "b", "", "branch to pull")
	pullCmd.Flags().BoolVar(&pullRebase, "rebase", false, "rebase current branch on top of upstream branch")
	pullCmd.Flags().BoolVar(&pullFastForward, "ff-only", false, "only allow fast-forward merges")
	pullCmd.Flags().BoolVar(&pullAllowUnrelated, "allow-unrelated-histories", false, "allow merging unrelated histories")
	pullCmd.Flags().BoolVar(&pullForce, "force", false, "force pull even if it results in non-fast-forward")
	pullCmd.Flags().BoolVar(&pullPrune, "prune", false, "remove remote tracking branches that no longer exist")
	pullCmd.Flags().IntVar(&pullDepth, "depth", 0, "limit fetching to the specified number of commits")
	pullCmd.Flags().DurationVar(&pullTimeout, "timeout", 5*time.Minute, "timeout for pull operation")

	rootCmd.AddCommand(pullCmd)
}