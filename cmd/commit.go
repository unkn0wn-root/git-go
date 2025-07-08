package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/internal/commands/commit"
	"github.com/unkn0wn-root/git-go/pkg/display"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
)

var (
	commitMessage string
	authorName    string
	authorEmail   string
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository",
	Long:  "Create a new commit with the changes in the index",
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)

		opts := commit.CommitOptions{
			Message:     commitMessage,
			AuthorName:  authorName,
			AuthorEmail: authorEmail,
		}

		commitHash, err := commit.CreateCommit(repo, opts)
		if err != nil {
			return err
		}

		branch, err := repo.GetCurrentBranch()
		if err != nil {
			branch = "main"
		}

		parents := []string{}
		parentHash, _ := repo.GetHead()
		if parentHash != "" {
			parents = append(parents, parentHash)
		}

		if len(parents) == 0 {
			fmt.Printf("[%s %s %s] %s\n", 
				display.Branch(branch), 
				display.Secondary("(root-commit)"), 
				display.Hash(commitHash), 
				commitMessage)
		} else {
			fmt.Printf("[%s %s] %s\n", 
				display.Branch(branch), 
				display.Hash(commitHash), 
				commitMessage)
		}

		return nil
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitMessage, "message", "m", "", "commit message")
	commitCmd.Flags().StringVar(&authorName, "author-name", "", "author name")
	commitCmd.Flags().StringVar(&authorEmail, "author-email", "", "author email")
	commitCmd.MarkFlagRequired("message")

	rootCmd.AddCommand(commitCmd)
}
