package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "init")
	run("worktree", "add", "-q", "-b", "feature", filepath.Join(dir, "wt-feature"))
	return dir
}

func TestCompleteCdAndTargets(t *testing.T) {
	t.Chdir(fixtureRepo(t))

	cd, _ := completeCd(nil, nil, "")
	if !slices.Contains(cd, "main") || !slices.Contains(cd, "feature") {
		t.Errorf("completeCd = %v, want main+feature", cd)
	}

	targets, _ := completeTargets(nil, nil, "")
	if slices.Contains(targets, "main") {
		t.Errorf("completeTargets must exclude the main checkout: %v", targets)
	}
	if !slices.Contains(targets, "feature") || !slices.Contains(targets, "-f") {
		t.Errorf("completeTargets = %v, want feature + -f", targets)
	}

	all := allBranches()
	if !slices.Contains(all, "main") || !slices.Contains(all, "feature") {
		t.Errorf("allBranches = %v, want main+feature", all)
	}
}

func TestCompletionScriptsNonEmpty(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		var buf bytes.Buffer
		root := newRootCmd()
		var err error
		switch shell {
		case "zsh":
			err = root.GenZshCompletion(&buf)
		case "bash":
			err = root.GenBashCompletionV2(&buf, true)
		case "fish":
			err = root.GenFishCompletion(&buf, true)
		}
		if err != nil {
			t.Errorf("%s completion: %v", shell, err)
		}
		if buf.Len() == 0 {
			t.Errorf("%s completion is empty", shell)
		}
	}
}
