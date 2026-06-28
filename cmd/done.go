package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/shell"
	"github.com/allisonmahmood/nt/internal/ui"
)

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "done [-f] [target]",
		Aliases:            []string{"finish"},
		Short:              "Remove a worktree AND delete its local branch",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		ValidArgsFunction:  completeTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hasHelp(args) {
				return cmd.Help()
			}
			r := loadRepo()

			// Accept -f/--force anywhere among the args, exactly like `nt rm` (the
			// zsh original only honored a leading -f for done — an inconsistency, so
			// `nt done feature -f` silently dropped the force).
			force, rest := splitForce(args)
			dforce := force != ""
			args = rest

			var target string
			if len(args) > 0 {
				path, ambiguous := r.Resolve(args[0])
				if len(ambiguous) > 0 {
					warn("'%s' matches multiple worktrees:", args[0])
					for _, m := range ambiguous {
						fmt.Fprintf(os.Stderr, "  %s\n", m)
					}
					os.Exit(1)
				}
				if path == "" {
					fail("no worktree for branch or path '%s'", args[0])
				}
				target = path
			} else {
				choice, ok := ui.PickOne("done (remove + delete branch)", r.Targets())
				if !ok || choice == "" {
					return nil
				}
				if path, _ := r.Resolve(choice); path != "" {
					target = path
				}
			}
			if target == "" {
				return nil
			}
			if target == r.MainDir {
				fail("refusing to remove the main checkout")
			}

			doneb := r.BranchAt(target)

			// Remember whether we're standing inside the tree; only actually step
			// out once git confirms the removal — a refused removal (e.g. a dirty
			// worktree without -f) must leave the shell where it is.
			inTree := insideDir(currentDir(), target)

			rmArgs := []string{"worktree", "remove"}
			if dforce {
				rmArgs = append(rmArgs, "--force")
			}
			rmArgs = append(rmArgs, target)
			if !git.Run(r.MainDir, rmArgs...) {
				os.Exit(1)
			}
			if inTree {
				shell.SignalCD(r.MainDir)
			}
			info("removed %s", target)

			if doneb == "" {
				info("(detached worktree — no branch to delete)")
				return nil
			}

			delFlag := "-d"
			if dforce {
				delFlag = "-D"
			}
			sha, _ := git.Query(r.MainDir, "rev-parse", "--short", doneb)
			if git.RunQuiet(r.MainDir, "branch", delFlag, doneb) {
				info("deleted branch %s  (was %s — recover via git reflog)", doneb, sha)
			} else {
				// The worktree is already gone, so `nt done -f <branch>` can no
				// longer resolve it; point only at the recovery that still works.
				fail("branch '%s' not fully merged — kept. Force-delete with 'git branch -D %s'.", doneb, doneb)
			}
			return nil
		},
	}
}
