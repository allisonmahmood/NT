// Package git is a thin, parity-focused wrapper around the `git` CLI. nt shells
// out (rather than using a Go git library) precisely because the behavior the
// tests pin is git's own exact worktree semantics (locked/dirty/submodule
// refusals); shelling out makes that parity automatic.
package git

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// Query runs git in dir (empty = current dir) and returns trimmed stdout and
// whether it exited 0. stderr is discarded — use it for read-only lookups where
// a failure just means "no/unknown".
func Query(dir string, args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimRight(out.String(), "\n"), true
}

// OK runs git and reports only success (for `--quiet --verify` style checks).
func OK(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// Run executes git with its stdout/stderr connected to the user's terminal, so
// git's own progress and (crucially) its precise refusal messages are shown —
// matching what the zsh plugin did for `worktree add`/`worktree remove`/`fetch`.
// Returns whether it exited 0.
func Run(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run() == nil
}

// RunQuiet executes git, discarding output, returning whether it exited 0.
func RunQuiet(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Run() == nil
}

// Lines splits trimmed git output into non-empty lines.
func Lines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
