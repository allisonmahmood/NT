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
	remotes, ok := git.Query(source, "remote")
	if !ok {
		return "", fmt.Errorf("cannot inspect configured remotes")
	}
	for _, ref := range git.Lines(refs) {
		if ref == "refs/heads/"+branch {
			return "", fmt.Errorf("--take requires a new branch; %q already exists", branch)
		}
		for _, remote := range git.Lines(remotes) {
			if ref == "refs/remotes/"+remote+"/"+branch {
				return "", fmt.Errorf("--take requires a new branch; %q already exists on %s", branch, remote)
			}
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
		if strings.Contains(subject, ": "+name+" ") {
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
	tracked, ok := git.Query(source, "ls-files", "-z")
	committed, valid := git.Query(source, "ls-tree", "-r", "--name-only", "-z", "HEAD")
	if !ok || !valid {
		return fmt.Errorf("cannot inspect tracked paths")
	}
	if err := checkNestedRepos(source, files+"\x00"+tracked+"\x00"+committed); err != nil {
		return err
	}
	// Stash restores HEAD in the source. A staged deletion may have an ignored
	// replacement at that path; leave that local environment file where it is.
	deleted, ok := git.Query(source, "diff", "--cached", "--name-only", "--no-renames", "--diff-filter=D", "-z")
	if !ok {
		return fmt.Errorf("cannot inspect staged deletions")
	}
	if deleted != "" {
		ignored, ok := git.Query(source, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
		if !ok {
			return fmt.Errorf("cannot inspect ignored replacements")
		}
		for _, path := range strings.Split(deleted, "\x00") {
			for _, replacement := range strings.Split(ignored, "\x00") {
				replacement = strings.TrimSuffix(replacement, "/")
				if path != "" && replacement != "" && (within(path, replacement) || within(replacement, path)) {
					return fmt.Errorf("ignored replacement at %q would be overwritten in the source; handle it manually", replacement)
				}
			}
		}
	}
	return nil
}

// A nested repo can be invisible to git status when its files are tracked by
// the parent. Inspect each containing directory once, including staged deletions.
func checkNestedRepos(source, paths string) error {
	seen := map[string]bool{".": true}
	for _, path := range strings.Split(paths, "\x00") {
		for dir := filepath.Dir(path); !seen[dir]; dir = filepath.Dir(dir) {
			seen[dir] = true
			candidate := filepath.Join(source, dir)
			if _, err := os.Lstat(filepath.Join(candidate, ".git")); !os.IsNotExist(err) {
				return fmt.Errorf("nested repository or unreadable metadata in %q needs manual handling", dir)
			}
			// Bare repositories have HEAD/objects instead of a .git entry.
			if _, err := os.Lstat(filepath.Join(candidate, "HEAD")); err == nil {
				if _, err := os.Lstat(filepath.Join(candidate, "objects")); !os.IsNotExist(err) {
					return fmt.Errorf("possible nested bare repository in %q needs manual handling", dir)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("cannot inspect directory %q", dir)
			}
		}
	}
	return nil
}
