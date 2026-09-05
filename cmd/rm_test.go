package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/allisonmahmood/nt/internal/worktree"
)

func TestNestedBlockersUsesRegisteredPathSpelling(t *testing.T) {
	root := t.TempDir()
	realParent := filepath.Join(root, "real-parent")
	if err := os.Mkdir(realParent, 0o755); err != nil {
		t.Fatal(err)
	}

	parent := filepath.Join(root, "parent")
	if err := os.Symlink(realParent, parent); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	// Resolving only the child changes it to real-parent/child and loses the
	// parent/child prefix supplied by the worktree registry.
	registered := []worktree.Worktree{{Path: parent}, {Path: child}}
	blockers := nestedBlockers(registered, parent)
	if len(blockers) != 1 || blockers[0] != child {
		t.Fatalf("blockers = %v, want [%s]", blockers, child)
	}
}
