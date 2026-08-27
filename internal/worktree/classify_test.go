package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitFixture builds a throwaway repo with a couple of commits and returns its
// path. Each test gets its own.
func gitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	run := func(wd string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = wd
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(dir, "add", "-A")
	run(dir, "commit", "-qm", "init")
	return dir
}

func addWorktree(t *testing.T, repo, path, branch string) {
	t.Helper()
	c := exec.Command("git", "worktree", "add", "-q", "-b", branch, path)
	c.Dir = repo
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
}

func TestClassifySimple_Clean(t *testing.T) {
	repo := gitFixture(t)
	wt := filepath.Join(repo, "wt-clean")
	addWorktree(t, repo, wt, "clean")
	if !ClassifySimple("", []string{wt})[wt] {
		t.Errorf("clean worktree should be simple (fast-deletable)")
	}
}

func TestClassifySimple_Dirty(t *testing.T) {
	repo := gitFixture(t)
	wt := filepath.Join(repo, "wt-dirty")
	addWorktree(t, repo, wt, "dirty")
	if err := os.WriteFile(filepath.Join(wt, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ClassifySimple("", []string{wt})[wt] {
		t.Errorf("dirty worktree must NOT be simple without --force")
	}
	if !ClassifySimple("--force", []string{wt})[wt] {
		t.Errorf("dirty worktree should be simple under --force")
	}
}

func TestClassifySimple_Locked(t *testing.T) {
	repo := gitFixture(t)
	wt := filepath.Join(repo, "wt-locked")
	addWorktree(t, repo, wt, "locked")
	c := exec.Command("git", "worktree", "lock", wt)
	c.Dir = repo
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("lock: %v\n%s", err, out)
	}
	if ClassifySimple("", []string{wt})[wt] {
		t.Errorf("locked worktree must NOT be simple")
	}
	if ClassifySimple("--force", []string{wt})[wt] {
		t.Errorf("locked worktree must NOT be simple even with --force (mirror git)")
	}
}

func TestClassifySimple_BrokenFailsClosed(t *testing.T) {
	repo := gitFixture(t)
	wt := filepath.Join(repo, "wt-broken")
	addWorktree(t, repo, wt, "broken")
	// Break the gitdir link so git errors on this worktree.
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /nonexistent/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ClassifySimple("", []string{wt})[wt] {
		t.Errorf("git-error worktree must fail CLOSED (not simple)")
	}
	if ClassifySimple("--force", []string{wt})[wt] {
		t.Errorf("git-error worktree must fail CLOSED even with --force")
	}
}

func TestGoneBranchesEmpty(t *testing.T) {
	repo := gitFixture(t)
	t.Chdir(repo)
	if g := GoneBranches(); len(g) != 0 {
		t.Errorf("fresh repo has no gone branches, got %v", g)
	}
}
