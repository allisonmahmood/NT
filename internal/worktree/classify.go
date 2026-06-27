package worktree

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/allisonmahmood/nt/internal/git"
)

// ClassifySimple decides, in parallel, which target worktrees are *provably*
// simple — and therefore safe for the instant rename-aside fast delete. Anything
// not provably simple is left out of the map so the caller hands it to
// `git worktree remove` itself (fail-safe: a misclassification just takes git's
// slower, correct path). force is "" or "--force".
//
// "Simple" = readable by git, clean (`--ignore-submodules=none`, exactly git's
// own check; skipped under --force), unlocked, and submodule-free (no mode-160000
// gitlink in the index). Mirrors the zsh fan-out, including failing CLOSED on any
// git error.
func ClassifySimple(force string, targets []string) map[string]bool {
	results := make([]bool, len(targets))
	var g errgroup.Group
	g.SetLimit(classifyParallelism())
	for i, wt := range targets {
		i, wt := i, wt
		g.Go(func() error {
			results[i] = isSimple(force, wt)
			return nil
		})
	}
	_ = g.Wait()
	simple := make(map[string]bool, len(targets))
	for i, wt := range targets {
		if results[i] {
			simple[wt] = true
		}
	}
	return simple
}

func isSimple(force, wt string) bool {
	admin, ok := git.Query(wt, "rev-parse", "--absolute-git-dir")
	if !ok {
		return false // unreadable -> let git decide
	}
	if fileExists(filepath.Join(admin, "locked")) {
		return false // locked
	}
	if ls, ok := git.Query(wt, "ls-files", "-s"); ok {
		for _, line := range strings.Split(ls, "\n") {
			if strings.HasPrefix(line, "160000 ") {
				return false // submodule / gitlink
			}
		}
	}
	if force == "" {
		st, ok := git.Query(wt, "status", "--porcelain", "--ignore-submodules=none")
		if !ok {
			return false // git error -> fail closed
		}
		if st != "" {
			return false // dirty
		}
	}
	return true
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func classifyParallelism() int {
	n := runtime.NumCPU() * 2
	if n > 16 {
		n = 16
	}
	if n < 1 {
		n = 1
	}
	return n
}
