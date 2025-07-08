package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/internal/commands/blame"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

var blameCmd = &cobra.Command{
	Use:   "blame <file>",
	Short: "Show what revision and author last modified each line of a file",
	Long:  "Annotate each line in the given file with information about the last commit that modified the line",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		filePath := args[0]

		fullPath := filepath.Join(workDir, filePath)
		if _, err := os.Stat(fullPath); err != nil {
			return fmt.Errorf("file does not exist: %s", filePath)
		}

		result, err := blame.BlameFile(repo, filePath, "")
		if err != nil {
			return fmt.Errorf("failed to blame file: %w", err)
		}

		fmt.Print(result.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(blameCmd)
}
