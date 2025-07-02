package cmd

import (
	"fmt"
	"os"

	"github.com/unkn0wn-root/git-go/log"
	"github.com/unkn0wn-root/git-go/repository"
	"github.com/spf13/cobra"
)

var (
	maxCount int
	oneline  bool
	graph    bool
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show commit logs",
	Long:  "Show the commit history starting from the current HEAD",
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		options := log.LogOptions{
			MaxCount: maxCount,
			Oneline:  oneline,
			Graph:    graph,
		}

		return log.ShowLog(repo, options)
	},
}

func init() {
	logCmd.Flags().IntVarP(&maxCount, "max-count", "n", 0, "limit the number of commits to output")
	logCmd.Flags().BoolVar(&oneline, "oneline", false, "shorthand for --pretty=oneline --abbrev-commit")
	logCmd.Flags().BoolVar(&graph, "graph", false, "draw a text-based graphical representation")

	rootCmd.AddCommand(logCmd)
}
