package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/pkg/display"
	"github.com/unkn0wn-root/git-go/internal/transport/pull"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
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

		fmt.Printf("%s Pulling from %s...\n", display.Info("⬇"), display.Emphasis(options.Remote))

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
		fmt.Println(display.Success("Already up to date."))
		return
	}

	if result.FastForward {
		fmt.Printf("%s\n", display.Success("Fast-forward"))
		if result.OldCommit != "" {
			fmt.Printf(" %s..%s\n", display.Hash(result.OldCommit[:7]), display.Hash(result.NewCommit[:7]))
		} else {
			fmt.Printf(" %s [new branch] -> %s\n", display.Success("*"), display.Hash(result.NewCommit[:7]))
		}
	} else if result.MergeCommit != "" {
		fmt.Printf("%s\n", display.Success("Merge made by the 'recursive' strategy."))
		fmt.Printf(" %s\n", display.Hash(result.MergeCommit[:7]))
	}

	if len(result.UpdatedFiles) > 0 {
		fmt.Printf(" %s file(s) changed", display.Emphasis(fmt.Sprintf("%d", len(result.UpdatedFiles))))
		if len(result.AddedFiles) > 0 {
			fmt.Printf(", %s insertion(s)", display.Success(fmt.Sprintf("%d", len(result.AddedFiles))))
		}
		if len(result.DeletedFiles) > 0 {
			fmt.Printf(", %s deletion(s)", display.Error(fmt.Sprintf("%d", len(result.DeletedFiles))))
		}
		fmt.Println()
	}

	if len(result.ConflictFiles) > 0 {
		fmt.Printf("%s Merge conflicts in %d file(s):\n", display.Error("CONFLICT:"), len(result.ConflictFiles))
		for _, file := range result.ConflictFiles {
			fmt.Printf("  %s\n", display.Path(file))
		}
		fmt.Println(display.Warning("Automatic merge failed; fix conflicts and then commit the result."))
	}

	if result.CommitsBehind > 0 {
		fmt.Printf("%s Your branch is behind %s by %s commit(s).\n",
			display.Info("ℹ"), display.Hash(result.NewCommit[:7]), display.Emphasis(fmt.Sprintf("%d", result.CommitsBehind)))
	}

	if result.CommitsAhead > 0 {
		fmt.Printf("%s Your branch is ahead of %s by %s commit(s).\n",
			display.Info("ℹ"), display.Hash(result.OldCommit[:7]), display.Emphasis(fmt.Sprintf("%d", result.CommitsAhead)))
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
