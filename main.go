// Command nt — navigate tree. Git worktrees, minus the ceremony.
//
// nt is half a binary, half a shell shim: the binary does the git work and
// writes its target dir to $NT_CD_FILE; the shim (see `nt init`) reads it and
// performs the cd that a child process never could.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"github.com/allisonmahmood/nt/cmd"
)

func main() {
	if err := fang.Execute(context.Background(), cmd.Root()); err != nil {
		os.Exit(1)
	}
}
