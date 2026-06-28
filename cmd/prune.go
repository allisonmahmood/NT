package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/ui"
	"github.com/allisonmahmood/nt/internal/worktree"
)

func newPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "prune",
		Aliases:       []string{"clean"},
		Short:         "Prune stale worktrees + empty dirs; offer to delete gone branches",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()

			// 1) Drop stale worktree admin entries (dirs that vanished from disk).
			git.RunQuiet(r.MainDir, "worktree", "prune")
			// 2) Reap abandoned trash + remove empty parent dirs (skipping live trees).
			removed := worktree.PruneDirs(r.Root, worktree.LivePaths())
			info("pruned stale worktree entries; removed %d empty dir(s)", removed)

			// 3) Offer to delete local branches whose upstream is gone.
			gone := worktree.GoneBranches()
			if len(gone) == 0 {
				info("no gone-upstream branches (run 'git fetch -p' first if you expected some)")
				return nil
			}
			// No terminal to drive the picker (script, CI): list, never delete.
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				info("local branches with a gone upstream (run 'nt prune' interactively to delete; 'git fetch -p' to refresh):")
				for _, b := range gone {
					fmt.Printf("  %s\n", b)
				}
				return nil
			}

			options := append([]string{allRow}, gone...)
			picks, ok := ui.PickMany("prune gone branches (space=mark, enter=delete)", options)
			if !ok || len(picks) == 0 {
				info("prune: kept all branches")
				return nil
			}
			var todelete []string
			selectedAll := false
			for _, p := range picks {
				if p == allRow {
					selectedAll = true
				}
			}
			if selectedAll {
				todelete = gone
			} else {
				for _, p := range picks {
					if p != allRow {
						todelete = append(todelete, p)
					}
				}
			}
			deleteBranches(r.MainDir, todelete)
			return nil
		},
	}
}

const allRow = "[ALL] delete every gone-upstream branch below"

// deleteBranches force-deletes the named local branches, reporting each with its
// old sha (recoverable from the reflog).
func deleteBranches(mainDir string, branches []string) {
	for _, b := range branches {
		if b == "" {
			continue
		}
		sha, _ := git.Query(mainDir, "rev-parse", "--short", b)
		if git.RunQuiet(mainDir, "branch", "-D", b) {
			info("deleted branch %s  (was %s — recover via git reflog)", b, sha)
		} else {
			warn("could not delete branch %s (checked out in a worktree?)", b)
		}
	}
}
