package worktree

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/allisonmahmood/nt/internal/git"
)

// Take transfers ordinary uncommitted work at the source's actual HEAD. The
// named stash is retained as recovery, including on success: deleting a numbered
// stash could delete somebody else's entry if another Git process pushes one.
func Take(r *Repo, source, branch string) (string, error) {
	unlock, err := LockMutation(source)
	if err != nil {
		return "", err
	}
	defer unlock()
	if err := CheckIdle(source); err != nil {
		return "", err
	}
	if !git.OK(source, "check-ref-format", "--branch", branch) || strings.HasPrefix(branch, "-") {
		return "", fmt.Errorf("invalid new branch name %q", branch)
	}
	refs, ok := git.Query(source, "for-each-ref", "--format=%(refname)", "refs/heads/"+branch, "refs/remotes")
	if !ok {
		return "", fmt.Errorf("cannot inspect existing branches")
	}
	for _, ref := range git.Lines(refs) {
		if ref == "refs/heads/"+branch || strings.HasSuffix(ref, "/"+branch) {
			return "", fmt.Errorf("--take requires a new branch; %q already exists", branch)
		}
	}
	dest := r.Dest(branch)
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		return "", fmt.Errorf("destination exists or cannot be inspected: %s", dest)
	}
	// A destination inside the source could itself be captured by stash -u.
	if within(dest, source) || within(source, dest) {
		return "", fmt.Errorf("--take requires separate, non-nested source and destination directories")
	}
	for _, wt := range r.Worktrees {
		if wt.Path != source && within(wt.Path, source) {
			return "", fmt.Errorf("nested worktree %s needs manual handling", wt.Path)
		}
	}
	if err := checkTakeFiles(source); err != nil {
		return "", err
	}
	status, ok := git.Query(source, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
	if !ok || status == "" {
		return "", fmt.Errorf("--take needs readable, uncommitted changes in the current worktree")
	}
	head, ok := git.Query(source, "rev-parse", "--verify", "HEAD")
	if !ok {
		return "", fmt.Errorf("--take needs an existing source commit")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(dest))
	if err != nil {
		return "", fmt.Errorf("cannot resolve destination: %w", err)
	}
	physicalDest := filepath.Join(parent, filepath.Base(dest))
	if within(physicalDest, source) || within(source, physicalDest) {
		return "", fmt.Errorf("--take destination resolves inside the source checkout")
	}
	if !git.Run(source, "-c", "core.hooksPath=/dev/null", "worktree", "add", "--no-track", "-b", branch, dest, head) {
		return "", fmt.Errorf("could not create destination; source work left in place")
	}
	// Report the recovery name before the first operation that removes source
	// files. Git stores the snapshot in refs/stash before cleaning the checkout.
	name := "nt-take-" + rand.Text()
	fmt.Printf("nt: taking work into %s; recovery stash: %s (git stash list)\n", branch, name)
	message := name + " from " + source + " into " + dest
	if !git.Run(source, "-c", "core.hooksPath=/dev/null", "stash", "push", "--include-untracked", "--message", message) {
		return "", fmt.Errorf("transfer stopped; source and destination retained; inspect git stash list for %s before retrying", name)
	}
	// Locate by our unique message, not stash@{0}: other worktrees share the
	// stash stack and may have pushed their own entry in the meantime.
	list, ok := git.Query(source, "stash", "list", "--format=%H%x09%gs")
	if !ok {
		return "", fmt.Errorf("cannot locate recovery stash %s; inspect git stash list; destination: %s", name, dest)
	}
	snapshot := ""
	for _, line := range git.Lines(list) {
		oid, subject, _ := strings.Cut(line, "\t")
		if strings.Contains(subject, ": "+message) {
			snapshot = oid
			break
		}
	}
	if snapshot == "" {
		return "", fmt.Errorf("source changed during transfer; inspect source, destination %s, and git stash list for %s", dest, name)
	}
	base, ok := git.Query(source, "rev-parse", snapshot+"^1")
	if !ok || base != head {
		return "", fmt.Errorf("source HEAD changed during capture; work saved as %s; recover with git stash apply --index %s in its original checkout", name, snapshot)
	}
	if !git.Run(dest, "-c", "core.hooksPath=/dev/null", "stash", "apply", "--index", snapshot) {
		return "", fmt.Errorf("transfer incomplete in %s; captured work retained in stash %s (%s); resolve there or apply with git stash apply --index %s in a clean checkout at %s", dest, name, snapshot, snapshot, head)
	}
	return snapshot, nil
}

func within(path, parent string) bool {
	return path == parent || strings.HasPrefix(path, parent+string(os.PathSeparator))
}

func checkTakeFiles(source string) error {
	// Intent-to-add is not restored faithfully by stash --index. Refuse it
	// rather than silently turning it into an ordinary untracked/staged file.
	visible, ok := git.Query(source, "diff", "--cached", "--raw", "--ita-visible-in-index")
	invisible, valid := git.Query(source, "diff", "--cached", "--raw", "--ita-invisible-in-index")
	if !ok || !valid || visible != invisible {
		return fmt.Errorf("intent-to-add or unreadable index needs manual handling")
	}
	files, ok := git.Query(source, "ls-files", "--others", "--exclude-standard", "-z")
	if !ok {
		return fmt.Errorf("cannot inspect untracked files")
	}
	for _, name := range strings.Split(files, "\x00") {
		if name == "" {
			continue
		}
		entry, err := os.Lstat(filepath.Join(source, name))
		if err != nil || (!entry.Mode().IsRegular() && entry.Mode()&os.ModeSymlink == 0) {
			return fmt.Errorf("untracked directory, nested repository, or special file %q needs manual handling", name)
		}
	}
	return nil
}
