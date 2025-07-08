package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/display"
)

var rootCmd = &cobra.Command{
	Use:   "git-go",
	Short: "A Git implementation in Go",
	Long: `git-go is a complete Git implementation in Go.
It supports the core Git functionality including init, add, commit, diff, blame and reset.`,
	Version: "1.0.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", display.Error("Error:"), err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
