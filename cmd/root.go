// Package cmd wires nt's cobra command tree. The binary is half the tool: the
// other half is a thin shell shim (see `nt init`) that performs the actual cd.
package cmd

import (
	"fmt"

	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/shell"
	"github.com/allisonmahmood/nt/internal/worktree"

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
	var take bool
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
  nt home               safely refresh the main checkout and cd home
  nt <branch> --take     take uncommitted work into a new worktree
  nt ls                 list this repo's worktrees, with dirty/ahead-behind

Add the shell integration to your rc file:  eval "$(nt init zsh)"`,
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare `nt` = hint line + ls; `nt <branch> [base]` = create/switch.
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()
			if take {
				if len(args) != 1 {
					return fmt.Errorf("--take requires one new branch name and no base")
				}
				source, ok := git.Query("", "rev-parse", "--show-toplevel")
				if !ok {
					return fmt.Errorf("cannot locate source worktree")
				}
				snapshot, err := worktree.Take(r, source, args[0])
				if err != nil {
					return err
				}
				info("moved uncommitted work; staged changes preserved; recovery retained in git stash list (%s)", snapshot)
				refreshHome(r, fetchRemote(r), worktree.HomeMaintenance)
				shell.SignalCD(r.Dest(args[0]))
				info("→ %s", r.Dest(args[0]))
				return nil
			}
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

	root.Flags().BoolVar(&take, "take", false, "Take uncommitted work at its current base into a new worktree (excludes ignored files and unsaved buffers)")

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
