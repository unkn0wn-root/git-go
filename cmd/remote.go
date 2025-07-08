package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/display"
	"github.com/unkn0wn-root/git-go/remote"
	"github.com/unkn0wn-root/git-go/repository"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote repositories",
	Long:  "Manage the set of repositories (\"remotes\") whose branches you track.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Add a remote repository",
	Long:  "Add a remote named <name> for the repository at <url>.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		name := args[0]
		url := args[1]

		rc := remote.NewRemoteConfig(repo.GitDir)
		if err := rc.Load(); err != nil {
			return fmt.Errorf("failed to load remote config: %w", err)
		}

		if err := rc.AddRemote(name, url); err != nil {
			return fmt.Errorf("failed to add remote: %w", err)
		}

		fmt.Printf("%s Added remote %s with URL %s\n", 
			display.Success("✓"), 
			display.Emphasis(name), 
			display.Path(url))
		return nil
	},
}

var remoteRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a remote repository",
	Long:  "Remove the remote named <name>. All remote-tracking branches and configuration settings for the remote are removed.",
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

		name := args[0]

		rc := remote.NewRemoteConfig(repo.GitDir)
		if err := rc.Load(); err != nil {
			return fmt.Errorf("failed to load remote config: %w", err)
		}

		if err := rc.RemoveRemote(name); err != nil {
			return fmt.Errorf("failed to remove remote: %w", err)
		}

		fmt.Printf("%s Removed remote %s\n", 
			display.Success("✓"), 
			display.Emphasis(name))
		return nil
	},
}

var remoteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List remote repositories",
	Long:  "Show the remote repositories configured for this repository.",
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		repo := repository.New(workDir)
		if !repo.Exists() {
			return fmt.Errorf("not a git repository")
		}

		rc := remote.NewRemoteConfig(repo.GitDir)
		if err := rc.Load(); err != nil {
			return fmt.Errorf("failed to load remote config: %w", err)
		}

		remotes := rc.ListRemotes()
		if len(remotes) == 0 {
			fmt.Println(display.Hint("No remotes configured"))
			return nil
		}

		for _, r := range remotes {
			fmt.Printf("%s\t%s %s\n", 
				display.Emphasis(r.Name), 
				display.Path(r.FetchURL), 
				display.Secondary("(fetch)"))
			if r.PushURL != r.FetchURL {
				fmt.Printf("%s\t%s %s\n", 
					display.Emphasis(r.Name), 
					display.Path(r.PushURL), 
					display.Secondary("(push)"))
			} else {
				fmt.Printf("%s\t%s %s\n", 
					display.Emphasis(r.Name), 
					display.Path(r.PushURL), 
					display.Secondary("(push)"))
			}
		}

		return nil
	},
}

var remoteShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show information about a remote repository",
	Long:  "Show information about the remote named <name>.",
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

		name := args[0]

		rc := remote.NewRemoteConfig(repo.GitDir)
		if err := rc.Load(); err != nil {
			return fmt.Errorf("failed to load remote config: %w", err)
		}

		r, err := rc.GetRemote(name)
		if err != nil {
			return fmt.Errorf("failed to get remote: %w", err)
		}

		fmt.Printf("%s remote %s\n", display.Success("*"), display.Emphasis(r.Name))
		fmt.Printf("  %s %s\n", display.Info("Fetch URL:"), display.Path(r.FetchURL))
		fmt.Printf("  %s %s\n", display.Info("Push  URL:"), display.Path(r.PushURL))

		return nil
	},
}

func init() {
	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteRemoveCmd)
	remoteCmd.AddCommand(remoteListCmd)
	remoteCmd.AddCommand(remoteShowCmd)

	rootCmd.AddCommand(remoteCmd)
}
