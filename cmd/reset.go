package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/unkn0wn-root/git-go/internal/core/discovery"
	"github.com/unkn0wn-root/git-go/pkg/display"
	"github.com/unkn0wn-root/git-go/internal/core/repository"
	"github.com/unkn0wn-root/git-go/internal/commands/reset"
)

var (
	resetSoft  bool
	resetMixed bool
	resetHard  bool
)

var resetCmd = &cobra.Command{
	Use:   "reset [<mode>] [<commit>] [--] [<pathspec>...]",
	Short: "Reset current HEAD to the specified state",
	Long: `Reset current HEAD to the specified state.

Reset modes:
  --soft   Reset only HEAD
  --mixed  Reset HEAD and index (default)
  --hard   Reset HEAD, index, and working tree

If no mode is specified, defaults to --mixed.
If no commit is specified, defaults to HEAD.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := discovery.FindRepositoryFromCwd()
		if err != nil {
			return fmt.Errorf("not a git repository (or any of the parent directories)")
		}

		repo := repository.New(workDir)

		// Determine reset mode
		mode := reset.ResetModeMixed // default
		modeCount := 0
		if resetSoft {
			mode = reset.ResetModeSoft
			modeCount++
		}
		if resetMixed {
			mode = reset.ResetModeMixed
			modeCount++
		}
		if resetHard {
			mode = reset.ResetModeHard
			modeCount++
		}

		if modeCount > 1 {
			return fmt.Errorf("cannot specify multiple reset modes")
		}

		target := ""
		var paths []string

		if len(args) > 0 {
			// First argument could be a commit reference
			if !isPath(args[0]) {
				target = args[0]
				paths = args[1:]
			} else {
				paths = args
			}
		}

		if len(paths) > 0 && mode != reset.ResetModeMixed {
			return fmt.Errorf("cannot specify paths with --soft or --hard")
		}

		if err := reset.Reset(repo, target, mode, paths); err != nil {
			return fmt.Errorf("reset failed: %w", err)
		}

		if len(paths) == 0 {
			fmt.Printf("%s HEAD is now at %s\n", display.Success("✓"), display.Hash(target))
		}

		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVar(&resetSoft, "soft", false, "reset only HEAD")
	resetCmd.Flags().BoolVar(&resetMixed, "mixed", false, "reset HEAD and index (default)")
	resetCmd.Flags().BoolVar(&resetHard, "hard", false, "reset HEAD, index, and working tree")

	rootCmd.AddCommand(resetCmd)
}

// isPath checks if a string looks like a file path rather than a commit reference
func isPath(s string) bool {
	// simple heuristic: if it contains a slash or starts with a dot, it's probably a path
	// I know, in full impl. this would be more sophisticated but just for learning purposes
	// it is what it is
	return len(s) > 0 && (s[0] == '.' || s[0] == '/' || containsSlash(s))
}

func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}
