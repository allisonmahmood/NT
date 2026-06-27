// Package worktree holds nt's domain model: parsing `git worktree list`, locating
// the worktrees root, and resolving user-supplied identifiers to worktree paths.
package worktree

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/allisonmahmood/nt/internal/config"
	"github.com/allisonmahmood/nt/internal/git"
)

// ErrNotRepo is returned by Load when not inside a git work tree.
var ErrNotRepo = errors.New("not inside a git repository")

// Worktree is one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string // absolute path
	Branch   string // short branch name; empty when detached/bare
	Detached bool
	Bare     bool
}

// Repo is the resolved context every nt subcommand operates within.
type Repo struct {
	MainDir   string     // the main checkout (first porcelain entry)
	Root      string     // where worktrees live: <repo>.worktrees (or $NT_ROOT)
	Worktrees []Worktree // in porcelain order (main checkout first)
}

// Parse turns `git worktree list --porcelain` output into Worktrees, preserving
// order. A blank line separates entries; a detached entry has `detached` instead
// of a `branch` line; a bare main repo has `bare`.
func Parse(porcelain string) []Worktree {
	var wts []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			wts = append(wts, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(porcelain, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case cur == nil:
			// ignore stray lines before the first entry
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "detached":
			cur.Detached = true
		case line == "bare":
			cur.Bare = true
		}
	}
	flush()
	return wts
}

// Load resolves the current repo, or returns ErrNotRepo when not inside one.
func Load() (*Repo, error) {
	if _, ok := git.Query("", "rev-parse", "--is-inside-work-tree"); !ok {
		return nil, ErrNotRepo
	}
	out, ok := git.Query("", "worktree", "list", "--porcelain")
	if !ok {
		return nil, ErrNotRepo
	}
	wts := Parse(out)
	if len(wts) == 0 {
		return nil, ErrNotRepo
	}
	main := wts[0].Path
	root := config.RootOverride()
	if root == "" {
		root = filepath.Join(filepath.Dir(main), filepath.Base(main)+".worktrees")
	}
	return &Repo{MainDir: main, Root: root, Worktrees: wts}, nil
}

// Dest is the path a worktree for branch would live at (slash branches nest).
func (r *Repo) Dest(branch string) string {
	return filepath.Join(r.Root, filepath.FromSlash(branch))
}

// ByBranch returns the worktree path for an exact branch short-name, or "".
func (r *Repo) ByBranch(branch string) string {
	for _, w := range r.Worktrees {
		if w.Branch != "" && w.Branch == branch {
			return w.Path
		}
	}
	return ""
}

// Resolve maps an identifier — branch short-name, absolute worktree path, or a
// unique trailing path segment — to a worktree path. It mirrors the zsh
// _nt_resolve_wt: branch match wins; then exact path; then a unique trailing
// segment. If a trailing segment matches more than one worktree it is ambiguous:
// path is "" and the candidate list is returned.
func (r *Repo) Resolve(id string) (path string, ambiguous []string) {
	if p := r.ByBranch(id); p != "" {
		return p, nil
	}
	for _, w := range r.Worktrees {
		if w.Path == id {
			return w.Path, nil
		}
	}
	var tail []string
	suffix := "/" + id
	for _, w := range r.Worktrees {
		if strings.HasSuffix(w.Path, suffix) {
			tail = append(tail, w.Path)
		}
	}
	switch len(tail) {
	case 0:
		return "", nil
	case 1:
		return tail[0], nil
	default:
		return "", tail
	}
}

// BranchAt returns the branch backing the worktree at path, or "" if detached.
func (r *Repo) BranchAt(path string) string {
	for _, w := range r.Worktrees {
		if w.Path == path {
			return w.Branch
		}
	}
	return ""
}

// Targets lists removable worktree identifiers (skips the main checkout):
// branch-backed -> branch name; detached -> full path. Used by `rm`/`done`
// pickers and completion.
func (r *Repo) Targets() []string {
	var out []string
	for i, w := range r.Worktrees {
		if i == 0 {
			continue // main checkout
		}
		if w.Branch != "" {
			out = append(out, w.Branch)
		} else {
			out = append(out, w.Path)
		}
	}
	return out
}

// BranchNames lists the branch short-names that currently have a worktree
// (including the main checkout). Used by `cd` completion.
func (r *Repo) BranchNames() []string {
	var out []string
	for _, w := range r.Worktrees {
		if w.Branch != "" {
			out = append(out, w.Branch)
		}
	}
	return out
}
