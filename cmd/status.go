package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/repository"
	"github.com/unkn0wn-root/git-go/status"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the working tree status",
	Long:  "Show the working tree status including staged, unstaged, and untracked files",
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		result, err := status.GetStatus(repo)
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		fmt.Print(result.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
