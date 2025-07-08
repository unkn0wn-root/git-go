package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/internal/commands/add"
	"github.com/unkn0wn-root/git-go/internal/core/discovery"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

var addCmd = &cobra.Command{
	Use:   "add <pathspec>...",
	Short: "Add file contents to the index",
	Long:  "Add file contents to the index (staging area)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := discovery.FindRepositoryFromCwd()
		if err != nil {
			return fmt.Errorf("not a git repository (or any of the parent directories)")
		}

		repo := repository.New(workDir)
		return add.AddFiles(repo, args)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
