package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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

			// Step out of any tree we're about to remove (the bg delete can't cd
			// for us); keep the main-checkout guard as a backstop.
			pwd := currentDir()
			var doomed []string
			stepOut := false
			for _, t := range targets {
				if t == r.MainDir {
					warn("refusing to remove the main checkout")
					continue
				}
				if pwd == t || strings.HasPrefix(pwd, t+string(os.PathSeparator)) {
					stepOut = true
				}
				doomed = append(doomed, t)
			}
			if len(doomed) == 0 {
				os.Exit(1)
			}
			if stepOut {
				shell.SignalCD(r.MainDir)
			}

			if rc := removeWorktrees(force, r.MainDir, doomed); rc != 0 {
				os.Exit(rc)
			}
			return nil
		},
	}
}

// removeWorktrees removes a set of resolved worktree paths fast: provably-simple
// trees are renamed aside (instant) and reclaimed in the background; the rest are
// handed to `git worktree remove`. Returns 0, or 1 if any tree was refused/failed.
func removeWorktrees(force, mainDir string, targets []string) int {
	simple := worktree.ClassifySimple(force, targets)
	rc := 0

	// Pass 1: delegate the not-provably-simple ones to git (it prints its own
	// precise refusal reason).
	for _, t := range targets {
		if simple[t] {
			continue
		}
		gitArgs := []string{"worktree", "remove"}
		if force != "" {
			gitArgs = append(gitArgs, force)
		}
		gitArgs = append(gitArgs, t)
		if git.Run(mainDir, gitArgs...) {
			info("removed %s", t)
		} else {
			rc = 1
		}
	}

	// Pass 2: fast-path the simple ones — rename aside (instant), reclaim later.
	var doomed []string
	swept := false
	pid := os.Getpid()
	seq := 0
	for _, t := range targets {
		if !simple[t] {
			continue
		}
		if !pathExists(t) {
			// Already gone — e.g. a parent worktree dragged this nested one along.
			info("removed %s", t)
			swept = true
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

	if len(doomed) > 0 || swept {
		git.RunQuiet(mainDir, "worktree", "prune") // drop the renamed-aside (and dragged) entries
		if len(doomed) > 0 {
			shell.SpawnReap(doomed)
		}
	}
	return rc
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
