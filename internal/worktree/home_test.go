package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func homeFixture(t *testing.T) (dir, upstream, base, tip string) {
	t.Helper()
	dir = gitFixture(t)
	base = runGit(t, dir, "rev-parse", "HEAD")
	upstream = filepath.Join(t.TempDir(), "upstream")
	addWorktree(t, dir, upstream, "upstream")
	writeFile(t, filepath.Join(upstream, "new"), "upstream\n")
	runGit(t, upstream, "add", "new")
	runGit(t, upstream, "commit", "-qm", "upstream")
	tip = runGit(t, upstream, "rev-parse", "HEAD")
	return
}

func TestHomeMaintenanceAndVisits(t *testing.T) {
	dir, _, base, tip := homeFixture(t)
	if msg := RefreshHome(dir, "origin", "main", tip, HomeMaintenance); !strings.Contains(msg, "run nt home once") {
		t.Fatal(msg)
	}
	if got := runGit(t, dir, "rev-parse", "HEAD"); got != base {
		t.Fatal("maintenance adopted an unknown checkout")
	}
	RefreshHome(dir, "origin", "main", base, HomeVisit)
	if msg := RefreshHome(dir, "origin", "main", tip, HomeMaintenance); !strings.Contains(msg, "updated by 1") {
		t.Fatal(msg)
	}
	if msg := RefreshHome(dir, "origin", "main", tip, HomeVisit); !strings.Contains(msg, "1 new commits since your last nt home") {
		t.Fatal(msg)
	}
	if msg := RefreshHome(dir, "origin", "main", tip, HomeVisit); strings.Contains(msg, "since your last") {
		t.Fatal("repeated visit recounted commits:", msg)
	}
	runGit(t, dir, "reset", "--hard", base)
	if msg := RefreshHome(dir, "origin", "main", tip, HomeMaintenance); !strings.Contains(msg, "changed outside NT") {
		t.Fatal(msg)
	}
	if got := runGit(t, dir, "rev-parse", "HEAD"); got != base {
		t.Fatal("maintenance undid a deliberate reset")
	}
	RefreshHome(dir, "origin", "main", tip, HomeVisit)
	if got := runGit(t, dir, "rev-parse", "HEAD"); got != tip {
		t.Fatal("explicit home did not resume")
	}
}

func TestHomeRefusals(t *testing.T) {
	for _, scenario := range []string{"unstaged", "staged", "untracked", "diverged", "detached", "wrong-branch", "merge", "rebase", "bisect", "index-lock", "HEAD.lock", "ORIG_HEAD.lock", "refs/heads/main.lock", "packed-refs.lock", "skip-worktree", "assume-unchanged", "corrupt-state", "ignored-collision", "opt-out", "invalid-config"} {
		t.Run(scenario, func(t *testing.T) {
			dir, _, base, tip := homeFixture(t)
			RefreshHome(dir, "origin", "main", base, HomeVisit)
			switch scenario {
			case "unstaged", "staged", "diverged":
				writeFile(t, filepath.Join(dir, "README"), "local\n")
				if scenario != "unstaged" {
					runGit(t, dir, "add", "README")
				}
				if scenario == "diverged" {
					runGit(t, dir, "commit", "-qm", "local")
				}
			case "untracked":
				writeFile(t, filepath.Join(dir, "scratch"), "local")
			case "detached":
				runGit(t, dir, "checkout", "--detach")
			case "wrong-branch":
				runGit(t, dir, "checkout", "-b", "experiment")
			case "merge":
				writeFile(t, filepath.Join(dir, ".git", "MERGE_HEAD"), tip)
			case "rebase":
				if err := os.Mkdir(filepath.Join(dir, ".git", "rebase-merge"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "bisect":
				writeFile(t, filepath.Join(dir, ".git", "BISECT_LOG"), "bisect")
			case "index-lock":
				writeFile(t, filepath.Join(dir, ".git", "index.lock"), "")
			case "HEAD.lock", "ORIG_HEAD.lock", "refs/heads/main.lock", "packed-refs.lock":
				writeFile(t, filepath.Join(dir, ".git", scenario), "")
			case "skip-worktree", "assume-unchanged":
				runGit(t, dir, "update-index", "--"+scenario, "README")
				writeFile(t, filepath.Join(dir, "README"), "hidden changes")
			case "corrupt-state":
				writeFile(t, filepath.Join(dir, ".git", "nt-home.json"), "broken")
			case "ignored-collision":
				writeFile(t, filepath.Join(dir, ".git", "info", "exclude"), "new\n")
				writeFile(t, filepath.Join(dir, "new"), "do not overwrite")
				// The incoming tracked file is ignored only in home's checkout.
			case "opt-out":
				runGit(t, dir, "config", "nt.autoRefreshHome", "false")
			case "invalid-config":
				runGit(t, dir, "config", "nt.autoRefreshHome", "invalid")
			}
			before := runGit(t, dir, "rev-parse", "HEAD")
			msg := RefreshHome(dir, "origin", "main", tip, HomeMaintenance)
			if !strings.Contains(msg, "home left unchanged") && (scenario != "ignored-collision" || !strings.Contains(msg, "home refresh failed")) {
				t.Fatal(msg)
			}
			if got := runGit(t, dir, "rev-parse", "HEAD"); got != before {
				t.Fatal("refused refresh changed HEAD")
			}
			if scenario == "ignored-collision" {
				data, _ := os.ReadFile(filepath.Join(dir, "new"))
				if string(data) != "do not overwrite" {
					t.Fatal("ignored file was overwritten")
				}
			}
			if scenario == "opt-out" {
				if msg := RefreshHome(dir, "origin", "main", tip, HomeVisit); !strings.Contains(msg, "updated by 1") {
					t.Fatal("opt-out disabled explicit home:", msg)
				}
			}
		})
	}
}

func TestHomePartialMergeFailureIsReportedHonestly(t *testing.T) {
	dir, _, base, tip := homeFixture(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "git"), "#!/bin/sh\ncase \"$*\" in *'merge --ff-only'*) touch .git/refs/heads/main.lock;; esac\nexec \"$NT_TEST_REAL_GIT\" \"$@\"\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NT_TEST_REAL_GIT", realGit)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	message := RefreshHome(dir, "origin", "main", tip, HomeVisit)
	if strings.Contains(message, "left unchanged") || !strings.Contains(message, "inspect git status") {
		t.Fatal("partial mutation was misreported:", message)
	}
	if runGit(t, dir, "rev-parse", "HEAD") != base || runGit(t, dir, "status", "--porcelain") == "" {
		t.Fatal("fixture did not reproduce the partial update")
	}
}

func TestMutationLockSharedAcrossWorktrees(t *testing.T) {
	dir, upstream, _, tip := homeFixture(t)
	unlock, err := LockMutation(upstream)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if msg := RefreshHome(dir, "origin", "main", tip, HomeVisit); !strings.Contains(msg, "another NT") {
		t.Fatal(msg)
	}
}
