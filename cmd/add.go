package cmd

import (
	"fmt"

	"github.com/unkn0wn-root/git-go/add"
	"github.com/unkn0wn-root/git-go/discovery"
	"github.com/unkn0wn-root/git-go/repository"
	"github.com/spf13/cobra"
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
