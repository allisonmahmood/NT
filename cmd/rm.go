package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/config"
	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/shell"
	"github.com/allisonmahmood/nt/internal/ui"
	"github.com/allisonmahmood/nt/internal/worktree"
)

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "rm [-f] [target...]",
		Aliases:            []string{"remove"},
		Short:              "Remove worktree(s) (multi-picker if target omitted)",
		DisableFlagParsing: true, // -f may sit anywhere among targets
		SilenceUsage:       true,
		SilenceErrors:      true,
		ValidArgsFunction:  completeTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			if hasHelp(args) {
				return cmd.Help()
			}
			r := loadRepo()
			force, rest := splitForce(args)

			var targets []string
			if len(rest) > 0 {
				// Resolve every explicit target up front; refuse the main checkout
				// here too — a typo or stray `main` bails before anything is removed.
				for _, arg := range rest {
					path, ambiguous := r.Resolve(arg)
					if len(ambiguous) > 0 {
						warn("'%s' matches multiple worktrees:", arg)
						for _, m := range ambiguous {
							fmt.Fprintf(os.Stderr, "  %s\n", m)
						}
						os.Exit(1)
					}
					if path == "" {
						fail("no worktree for branch or path '%s'", arg)
					}
					if path == r.MainDir {
						fail("refusing to remove the main checkout")
					}
					targets = append(targets, path)
				}
			} else {
				picks, ok := ui.PickMany("remove worktree (space=mark, enter=remove)", r.Targets())
				if !ok || len(picks) == 0 {
					return nil
				}
				for _, choice := range picks {
					if path, _ := r.Resolve(choice); path != "" {
						targets = append(targets, path)
					}
				}
			}

			targets = dedupe(targets)
			if len(targets) == 0 {
				return nil
			}

			// Note whether we're standing in a tree we're about to remove (the bg
			// delete can't cd for us); keep the main-checkout guard as a backstop.
			pwd := currentDir()
			var doomed []string
			stepOut := false
			for _, t := range targets {
				if t == r.MainDir {
					warn("refusing to remove the main checkout")
					continue
				}
				if insideDir(pwd, t) {
					stepOut = true
				}
				doomed = append(doomed, t)
			}
			if len(doomed) == 0 {
				os.Exit(1)
			}

			rc := removeWorktrees(force, r, doomed)
			// Relocate only once the tree we were standing in is actually gone — a
			// refused removal must leave the shell where it is, not yank it to main.
			if stepOut && !pathExists(pwd) {
				shell.SignalCD(r.MainDir)
			}
			if rc != 0 {
				os.Exit(rc)
			}
			return nil
		},
	}
}

// removeWorktrees removes children before parents, checking for surviving nested
// worktrees before every removal. Simple trees are renamed aside and reclaimed
// in the background; other trees are handed to git. Returns 1 on any refusal.
func removeWorktrees(force string, r *worktree.Repo, targets []string) int {
	// A descendant always has a longer registered path than its ancestor.
	slices.SortStableFunc(targets, func(a, b string) int { return len(b) - len(a) })
	simple := worktree.ClassifySimple(force, targets)
	rc := 0
	var doomed []string
	pid := os.Getpid()
	seq := 0
	for _, t := range targets {
		if blockers := nestedBlockers(r.Worktrees, t); len(blockers) > 0 {
			for _, child := range blockers {
				warn("refusing to remove %s: contains nested worktree %s", t, child)
			}
			rc = 1
			continue
		}

		if !simple[t] {
			gitArgs := []string{"worktree", "remove"}
			if force != "" {
				gitArgs = append(gitArgs, force)
			}
			gitArgs = append(gitArgs, t)
			if git.Run(r.MainDir, gitArgs...) {
				info("removed %s", t)
			} else {
				rc = 1
			}
			continue
		}

		seq++
		trash := filepath.Join(filepath.Dir(t), fmt.Sprintf("%s%d-%d", config.TrashPrefix, pid, seq))
		if os.Rename(t, trash) == nil {
			doomed = append(doomed, trash)
			info("removed %s", t)
		} else {
			warn("could not remove %s", t)
			rc = 1
		}
	}

	if len(doomed) > 0 {
		git.RunQuiet(r.MainDir, "worktree", "prune")
		shell.SpawnReap(doomed)
	}
	return rc
}

// nestedBlockers includes every registered descendant still present on disk,
// including selected children whose removal failed. Unknown state fails closed.
func nestedBlockers(registered []worktree.Worktree, target string) []string {
	var blockers []string
	prefix := target + string(os.PathSeparator)
	for _, candidate := range registered {
		if !strings.HasPrefix(candidate.Path, prefix) {
			continue
		}
		if _, err := os.Lstat(candidate.Path); os.IsNotExist(err) {
			continue
		}
		blockers = append(blockers, candidate.Path)
	}
	return blockers
}

// splitForce pulls -f/--force out from anywhere among the args.
func splitForce(args []string) (force string, rest []string) {
	for _, a := range args {
		switch a {
		case "-f", "--force":
			force = "--force"
		default:
			rest = append(rest, a)
		}
	}
	return
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func hasHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
