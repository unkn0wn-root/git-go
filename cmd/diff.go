package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/diff"
	"github.com/unkn0wn-root/git-go/repository"
)

var (
	cached bool
	staged bool
)

var diffCmd = &cobra.Command{
	Use:   "diff [<path>...]",
	Short: "Show changes between commits, commit and working tree, etc",
	Long:  "Show differences between the working directory and the index, or between commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		if cached || staged {
			return diff.ShowStagedDiff(repo, args)
		}

		return diff.ShowWorkingTreeDiff(repo, args)
	},
}

func init() {
	diffCmd.Flags().BoolVar(&cached, "cached", false, "show diff between index and HEAD")
	diffCmd.Flags().BoolVar(&staged, "staged", false, "show diff between index and HEAD (same as --cached)")

	rootCmd.AddCommand(diffCmd)
}
