// Package cmd wires nt's cobra command tree. The binary is half the tool: the
// other half is a thin shell shim (see `nt init`) that performs the actual cd.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/ui"
)

// version is the binary version, set via SetVersion from main (ldflags).
var version = "dev"

// SetVersion records the build version shown by `nt --version`.
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "nt [branch] [base]",
		Version: version,
		Short:   "Git worktrees, minus the ceremony",
		Long: `nt — navigate tree. Spin up a git worktree (or jump to it if it exists),
cd in, and get out of your way. Worktrees live next to the main checkout in
<repo>.worktrees/<branch>.

  nt <branch> [base]    create/switch to a worktree and cd in
  nt cd   [branch]      cd to an existing worktree (picker if branch omitted)
  nt rm   [-f] [target] remove worktree(s) (multi-picker if target omitted)
  nt done [-f] [target] remove a worktree AND delete its local branch
  nt prune              prune stale worktrees + empty dirs; offer to delete gone branches
  nt home               cd back to the main checkout
  nt ls                 list this repo's worktrees, with dirty/ahead-behind

Add the shell integration to your rc file:  eval "$(nt init zsh)"`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare `nt` = hint line + ls; `nt <branch> [base]` = create/switch.
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()
			if len(args) == 0 {
				fmt.Println("nt <branch> | nt cd | nt rm | nt done | nt prune | nt home | nt ls   (nt -h for help)")
				fmt.Println(ui.Render(r))
				return nil
			}
			runCreate(r, args)
			return nil
		},
		ValidArgsFunction: completeCreate,
	}

	root.AddCommand(
		newCdCmd(),
		newHomeCmd(),
		newLsCmd(),
		newRmCmd(),
		newDoneCmd(),
		newPruneCmd(),
		newInitCmd(),
		newReapCmd(),
	)
	return root
}

// Root returns the configured root command (used by main and by tests).
func Root() *cobra.Command { return newRootCmd() }
