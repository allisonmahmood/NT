package worktree

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/allisonmahmood/nt/internal/config"
	"github.com/allisonmahmood/nt/internal/git"
)

// PruneDirs reaps abandoned `nt rm` trash (an interrupted background delete) and
// removes the empty parent dirs git's own prune leaves behind — without ever
// descending into a live worktree's working tree. Returns the count of empty
// dirs removed (including the root itself if it becomes empty). Mirrors the zsh
// find/rmdir sweep.
func PruneDirs(root string, live []string) int {
	if !dirExists(root) {
		return 0
	}
	liveSet := make(map[string]bool, len(live))
	for _, p := range live {
		liveSet[p] = true
	}

	skip := func(path string, d fs.DirEntry) bool {
		return liveSet[path] || strings.HasPrefix(d.Name(), config.TrashPrefix)
	}

	// 1) Reap any surviving trash dirs (not inside a live worktree).
	var trash []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if liveSet[path] {
			return fs.SkipDir
		}
		if strings.HasPrefix(d.Name(), config.TrashPrefix) {
			trash = append(trash, path)
			return fs.SkipDir
		}
		return nil
	})
	for _, t := range trash {
		_ = os.RemoveAll(t)
	}

	// 2) Gather candidate dirs (skip live worktrees and any surviving trash),
	//    then rmdir empties bottom-up until nothing more collapses.
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if skip(path, d) {
			return fs.SkipDir
		}
		dirs = append(dirs, path)
		return nil
	})
	// Deepest first so a parent is tried after its children are gone.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })

	removed := 0
	for {
		changed := false
		for _, d := range dirs {
			if !dirExists(d) {
				continue
			}
			if os.Remove(d) == nil { // rmdir: removes only if empty
				removed++
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if os.Remove(root) == nil { // the root itself, if now empty
		removed++
	}
	return removed
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// GoneBranches lists local branches whose upstream is gone (deleted on the
// remote) — the branches `nt prune` offers to delete. Needs an up-to-date
// `git fetch -p` to be accurate.
func GoneBranches() []string {
	out, ok := git.Query("", "for-each-ref", "--format=%(refname:short) %(upstream:track)", "refs/heads")
	if !ok {
		return nil
	}
	var gone []string
	for _, line := range git.Lines(out) {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 && parts[1] == "[gone]" {
			gone = append(gone, parts[0])
		}
	}
	return gone
}

// LivePaths returns the current worktree paths (re-queried, e.g. after a prune).
func LivePaths() []string {
	out, ok := git.Query("", "worktree", "list", "--porcelain")
	if !ok {
		return nil
	}
	var paths []string
	for _, w := range Parse(out) {
		paths = append(paths, w.Path)
	}
	return paths
}
