package ui

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/worktree"
)

// Render builds the `nt ls` table: one row per worktree with a dirty marker,
// ahead/behind vs upstream, the path, and a (main) tag, followed by the legend.
// Main checkout is listed first (porcelain order). Speed: ahead/behind comes
// from ONE for-each-ref; the per-worktree dirty checks are fanned out.
func Render(r *worktree.Repo) string {
	up, dn := Arrows()
	wts := r.Worktrees
	if len(wts) == 0 {
		return ""
	}

	ups, trk := trackingInfo(wts)
	dirty := dirtyFlags(wts)

	type row struct{ marker, disp, ab, path, tag string }
	rows := make([]row, 0, len(wts))
	maxb, maxa := 1, 1
	for _, w := range wts {
		disp := w.Branch
		if disp == "" {
			disp = "(detached)"
		}
		marker, ab, tag := " ", "-", ""
		if !dirExists(w.Path) {
			marker, ab = "?", "?" // path missing on disk (run `nt prune`)
		} else {
			if dirty[w.Path] {
				marker = "*"
			}
			if w.Branch != "" {
				tk := trk[w.Branch]
				switch {
				case strings.Contains(tk, "gone"):
					ab = "gone"
				case ups[w.Branch]:
					ab = aheadBehind(tk, up, dn)
				}
			}
		}
		if w.Path == r.MainDir {
			tag = "  (main)"
		}
		rows = append(rows, row{marker, disp, ab, w.Path, tag})
		if n := utf8.RuneCountInString(disp); n > maxb {
			maxb = n
		}
		if n := utf8.RuneCountInString(ab); n > maxa {
			maxa = n
		}
	}

	var b strings.Builder
	for _, rw := range rows {
		fmt.Fprintf(&b, "%s %s  %s  %s%s\n",
			rw.marker, padRune(rw.disp, maxb), padRune(rw.ab, maxa), rw.path, rw.tag)
	}
	b.WriteString("* = uncommitted changes")
	return b.String()
}

// aheadBehind turns git's %(upstream:track) ("[ahead 2, behind 1]" / "" / ...)
// into the compact column: "=", "↑n", "↓n", or "↑n↓m".
func aheadBehind(tk, up, dn string) string {
	if tk == "" {
		return "="
	}
	ahd := parseCount(tk, "ahead ")
	beh := parseCount(tk, "behind ")
	switch {
	case ahd > 0 && beh > 0:
		return up + strconv.Itoa(ahd) + dn + strconv.Itoa(beh)
	case ahd > 0:
		return up + strconv.Itoa(ahd)
	case beh > 0:
		return dn + strconv.Itoa(beh)
	default:
		return "="
	}
}

func parseCount(s, key string) int {
	i := strings.Index(s, key)
	if i < 0 {
		return 0
	}
	rest := s[i+len(key):]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	n, _ := strconv.Atoi(rest[:j])
	return n
}

// trackingInfo gets, in one for-each-ref, whether each worktree branch has an
// upstream and its track string. Scoped to the worktree branches only.
func trackingInfo(wts []worktree.Worktree) (ups map[string]bool, trk map[string]string) {
	ups = map[string]bool{}
	trk = map[string]string{}
	var refs []string
	for _, w := range wts {
		if w.Branch != "" {
			refs = append(refs, "refs/heads/"+w.Branch)
		}
	}
	if len(refs) == 0 {
		return
	}
	args := append([]string{"for-each-ref",
		"--format=%(refname:short)\t%(upstream)\t%(upstream:track)"}, refs...)
	out, ok := git.Query("", args...)
	if !ok {
		return
	}
	for _, line := range git.Lines(out) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		name := parts[0]
		if len(parts) > 1 && parts[1] != "" {
			ups[name] = true
		}
		if len(parts) > 2 {
			trk[name] = parts[2]
		}
	}
	return
}

// dirtyFlags fans the per-worktree `git status --porcelain` out in parallel
// (bounded) — that's the slow part of `ls`.
func dirtyFlags(wts []worktree.Worktree) map[string]bool {
	out := make(map[string]bool, len(wts))
	results := make([]bool, len(wts))
	var g errgroup.Group
	g.SetLimit(parallelism())
	for i, w := range wts {
		i, path := i, w.Path
		g.Go(func() error {
			s, ok := git.Query(path, "status", "--porcelain")
			results[i] = ok && s != ""
			return nil
		})
	}
	_ = g.Wait()
	for i, w := range wts {
		out[w.Path] = results[i]
	}
	return out
}

func padRune(s string, width int) string {
	if pad := width - utf8.RuneCountInString(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func parallelism() int {
	n := runtime.NumCPU() * 2
	if n > 16 {
		n = 16
	}
	if n < 1 {
		n = 1
	}
	return n
}
