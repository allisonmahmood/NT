package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func takeFixture(t *testing.T) *Repo {
	t.Helper()
	dir := gitFixture(t)
	// Git normalizes whitespace in stash messages; use the unique transfer ID.
	spaced := filepath.Join(t.TempDir(), "repo with  two spaces")
	if err := os.Rename(dir, spaced); err != nil {
		t.Fatal(err)
	}
	dir = spaced
	return &Repo{MainDir: dir, Root: filepath.Join(t.TempDir(), "trees"), Worktrees: []Worktree{{Path: dir, Branch: "main"}}}
}

func TestTakePreservesWorkAndHistory(t *testing.T) {
	r := takeFixture(t)
	dir := r.MainDir
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	// An unrelated stash must survive, and local commits stay on home.
	writeFile(t, filepath.Join(dir, "README"), "unrelated")
	runGit(t, dir, "stash", "push", "-m", "unrelated")
	unrelated := runGit(t, dir, "rev-parse", "refs/stash")
	runGit(t, dir, "commit", "--allow-empty", "-qm", "local commit")
	head := runGit(t, dir, "rev-parse", "HEAD")
	// A remote branch with a different full name must not block a new task.
	runGit(t, dir, "remote", "add", "team/upstream", t.TempDir())
	runGit(t, dir, "update-ref", "refs/remotes/team/upstream/prefix/task", head)
	writeFile(t, filepath.Join(dir, "README"), "staged\n")
	runGit(t, dir, "add", "README")
	writeFile(t, filepath.Join(dir, "README"), "staged\nworking\n")
	writeFile(t, filepath.Join(dir, "binary"), "\x00\x01\xff\n")
	runGit(t, dir, "add", "binary")
	writeFile(t, filepath.Join(dir, "notes with space\nand newline"), "untracked\n")
	if err := os.Symlink("README", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".git", "info", "exclude"), "environment\n")
	writeFile(t, filepath.Join(dir, "environment"), "ignored")
	staged := runGit(t, dir, "diff", "--cached", "--binary")
	working := runGit(t, dir, "diff", "--binary")
	snapshot, err := Take(r, dir, "task")
	if err != nil {
		t.Fatal(err)
	}
	dest := r.Dest("task")
	if runGit(t, dir, "rev-parse", "HEAD") != head || runGit(t, dest, "rev-parse", "HEAD") != head {
		t.Fatal("take moved history")
	}
	if runGit(t, dest, "diff", "--cached", "--binary") != staged || runGit(t, dest, "diff", "--binary") != working {
		t.Fatal("staged/unstaged changes did not round-trip")
	}
	if status := runGit(t, dir, "status", "--porcelain"); status != "" {
		t.Fatal("source not clean:", status)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "notes with space\nand newline")); err != nil || string(data) != "untracked\n" {
		t.Fatal("untracked content not transferred", err)
	}
	if target, err := os.Readlink(filepath.Join(dest, "link")); err != nil || target != "README" {
		t.Fatal("symlink not preserved", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "environment")); !os.IsNotExist(err) {
		t.Fatal("ignored file transferred")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "environment")); string(data) != "ignored" {
		t.Fatal("ignored source file changed")
	}
	stashes := runGit(t, dir, "stash", "list", "--format=%H")
	if !strings.Contains(stashes, unrelated) || !strings.Contains(stashes, snapshot) {
		t.Fatal("recovery or unrelated stash missing")
	}
}

func TestTakeRefusesBeforeTouchingSource(t *testing.T) {
	for _, scenario := range []string{"existing-branch", "existing-remote", "existing-path", "intent-to-add", "submodule", "nested-repo", "tracked-nested-repo", "bare-nested-repo", "nested-destination", "symlink-destination", "ignored-replacement", "merge"} {
		t.Run(scenario, func(t *testing.T) {
			r := takeFixture(t)
			dir := r.MainDir
			writeFile(t, filepath.Join(dir, "README"), "valuable work")
			switch scenario {
			case "existing-branch":
				runGit(t, dir, "branch", "task")
			case "existing-remote":
				runGit(t, dir, "remote", "add", "team/upstream", t.TempDir())
				runGit(t, dir, "update-ref", "refs/remotes/team/upstream/task", runGit(t, dir, "rev-parse", "HEAD"))
			case "existing-path":
				if err := os.MkdirAll(r.Dest("task"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "intent-to-add":
				writeFile(t, filepath.Join(dir, "ita"), "intent")
				runGit(t, dir, "add", "-N", "ita")
			case "submodule":
				head := runGit(t, dir, "rev-parse", "HEAD")
				runGit(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",module")
			case "nested-repo":
				nested := filepath.Join(dir, "nested")
				runGit(t, dir, "init", "-q", "-b", "main", nested)
				runGit(t, nested, "commit", "--allow-empty", "-qm", "nested")
			case "tracked-nested-repo":
				nested := filepath.Join(dir, "nested")
				if err := os.Mkdir(nested, 0o755); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(nested, "file"), "tracked by parent")
				runGit(t, dir, "add", "nested/file")
				runGit(t, dir, "init", "-q", "-b", "main", nested)
			case "bare-nested-repo":
				runGit(t, dir, "init", "-q", "--bare", "-b", "main", filepath.Join(dir, "bare"))
			case "nested-destination":
				r.Root = filepath.Join(dir, "trees")
			case "symlink-destination":
				if err := os.Symlink(dir, r.Root); err != nil {
					t.Fatal(err)
				}
			case "ignored-replacement":
				runGit(t, dir, "rm", "--cached", "README")
				writeFile(t, filepath.Join(dir, ".git", "info", "exclude"), "README\n")
			case "merge":
				writeFile(t, filepath.Join(dir, ".git", "MERGE_HEAD"), runGit(t, dir, "rev-parse", "HEAD"))
			}
			if _, err := Take(r, dir, "task"); err == nil {
				t.Fatal("unsafe transfer accepted")
			}
			if data, _ := os.ReadFile(filepath.Join(dir, "README")); string(data) != "valuable work" {
				t.Fatal("source changed on refusal")
			}
		})
	}
}

func TestTakeFailedApplyRetainsRecoverableSnapshot(t *testing.T) {
	r := takeFixture(t)
	writeFile(t, filepath.Join(r.MainDir, "README"), "recover me")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "git"), "#!/bin/sh\ncase \"$*\" in *'stash apply'*) exit 1;; esac\nexec \"$NT_TEST_REAL_GIT\" \"$@\"\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NT_TEST_REAL_GIT", realGit)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	if _, err := Take(r, r.MainDir, "task"); err == nil || !strings.Contains(err.Error(), "transfer incomplete") {
		t.Fatal("missing recovery error:", err)
	}
	if got := runGit(t, r.MainDir, "show", "refs/stash:README"); got != "recover me" {
		t.Fatal("failed transfer lost work:", got)
	}
}
