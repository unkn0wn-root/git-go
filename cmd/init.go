package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/display"
	"github.com/unkn0wn-root/git-go/repository"
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new Git repository",
	Long:  "Initialize a new Git repository in the current directory or specified directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir := "."
		if len(args) > 0 {
			workDir = args[0]
		}

		absPath, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		repo := repository.New(absPath)
		if err := repo.Init(); err != nil {
			return err
		}

		if workDir == "." {
			fmt.Println(display.FormatInitResult(".", false))
		} else {
			fmt.Println(display.FormatInitResult(workDir, false))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
