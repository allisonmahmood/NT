package cmd

import (
	"fmt"
	"os"

	"github.com/allisonmahmood/nt/internal/worktree"
)

// loadRepo resolves the repo or prints the canonical not-a-repo error and exits
// 1 — every worktree-touching subcommand calls this first.
func loadRepo() *worktree.Repo {
	r, err := worktree.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nt: not inside a git repository")
		os.Exit(1)
	}
	return r
}

// info prints an "nt: …" status line to stdout.
func info(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "nt: "+format+"\n", a...)
}

// warn prints an "nt: …" line to stderr (non-fatal).
func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "nt: "+format+"\n", a...)
}

// fail prints an "nt: …" error to stderr and exits 1.
func fail(format string, a ...any) {
	warn(format, a...)
	os.Exit(1)
}

// currentDir returns the shell's working directory for the step-out check
// (whether we're standing inside a worktree about to be removed). It prefers
// os.Getwd but falls back to $PWD — the shell's logical cwd, what the zsh
// original used — so a Getwd failure never silently strands the shell.
func currentDir() string {
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return wd
	}
	return os.Getenv("PWD")
}
