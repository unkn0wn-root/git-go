package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/display"
	"github.com/unkn0wn-root/git-go/push"
	"github.com/unkn0wn-root/git-go/repository"
)

var (
	pushRemote      string
	pushBranch      string
	pushForce       bool
	pushSetUpstream bool
	pushAll         bool
	pushTags        bool
	pushDryRun      bool
	pushTimeout     time.Duration
)

var pushCmd = &cobra.Command{
	Use:   "push [<remote>] [<branch>]",
	Short: "Update remote refs along with associated objects",
	Long: `Updates remote refs using local refs, while sending objects necessary to complete the given refs.
When no remote is configured, the command defaults to 'origin'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		options := push.DefaultPushOptions()

		if len(args) > 0 {
			options.Remote = args[0]
		}
		if len(args) > 1 {
			options.Branch = args[1]
		}

		if pushRemote != "" {
			options.Remote = pushRemote
		}
		if pushBranch != "" {
			options.Branch = pushBranch
		}

		options.Force = pushForce
		options.SetUpstream = pushSetUpstream
		options.PushAll = pushAll
		options.PushTags = pushTags
		options.DryRun = pushDryRun
		options.Timeout = pushTimeout

		pusher := push.NewPusher(repo)
		ctx := context.Background()

		if pushDryRun {
			fmt.Println(display.Info("This is a dry run. No changes will be made to the remote repository."))
		}

		fmt.Printf("%s\n", display.Info(fmt.Sprintf("Pushing to %s...", options.Remote)))

		var result *push.PushResult

		if pushAll {
			result, err = pusher.PushAll(ctx, options)
		} else if pushTags {
			result, err = pusher.PushTags(ctx, options)
		} else {
			result, err = pusher.Push(ctx, options)
		}

		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		printPushResult(result)
		return nil
	},
}

func printPushResult(result *push.PushResult) {
	if pushDryRun {
		fmt.Println(display.Success("Dry run completed successfully."))
		return
	}

	if len(result.UpdatedRefs) == 0 && len(result.RejectedRefs) == 0 {
		fmt.Println(display.Info("Everything up-to-date"))
		return
	}

	// Convert push result to display format
	updates := make(map[string]display.RefUpdate)
	for refName, update := range result.UpdatedRefs {
		updates[refName] = display.RefUpdate{
			Status:  display.RefUpdateStatus(update.Status),
			OldHash: update.OldHash,
			NewHash: update.NewHash,
		}
	}

	for refName, reason := range result.RejectedRefs {
		updates[refName] = display.RefUpdate{
			Status: display.RefUpdateRejected,
			Reason: reason,
		}
	}

	fmt.Print(display.FormatPushResult(result.Remote, updates, result.NewBranch, result.UpstreamSet, result.Branch))

	if result.PushedObjects > 0 {
		fmt.Printf("Pushed %d object(s)", result.PushedObjects)
		if result.PushedSize > 0 {
			fmt.Printf(" (%s)", display.FormatBytes(result.PushedSize))
		}
		fmt.Println()
	}

	if len(result.RejectedRefs) > 0 {
		fmt.Println()
		hints := []string{
			"Updates were rejected because the remote contains work that you do",
			"not have locally. This is usually caused by another repository pushing",
			"to the same ref. You may want to first integrate the remote changes",
			"(e.g., 'git pull ...') before pushing again.",
			"See the 'Note about fast-forwards' in 'git push --help' for details.",
		}
		fmt.Print(display.FormatHintMessage(hints))
	}
}

func extractBranchName(refName string) string {
	if refName == "" {
		return ""
	}

	if strings.HasPrefix(refName, "refs/heads/") {
		return refName[11:]
	}

	if strings.HasPrefix(refName, "refs/tags/") {
		return refName[10:]
	}

	return refName
}


func init() {
	pushCmd.Flags().StringVarP(&pushRemote, "remote", "r", "", "remote repository")
	pushCmd.Flags().StringVarP(&pushBranch, "branch", "b", "", "branch to push")
	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false, "force push even if it results in non-fast-forward")
	pushCmd.Flags().BoolVarP(&pushSetUpstream, "set-upstream", "u", false, "set upstream for the current branch")
	pushCmd.Flags().BoolVar(&pushAll, "all", false, "push all branches")
	pushCmd.Flags().BoolVar(&pushTags, "tags", false, "push all tags")
	pushCmd.Flags().BoolVar(&pushDryRun, "dry-run", false, "show what would be pushed without actually pushing")
	pushCmd.Flags().DurationVar(&pushTimeout, "timeout", 5*time.Minute, "timeout for push operation")

	rootCmd.AddCommand(pushCmd)
}
