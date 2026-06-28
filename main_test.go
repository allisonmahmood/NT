package main

import (
	"context"
	"os"
	"testing"

	"github.com/charmbracelet/fang"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/allisonmahmood/nt/cmd"
)

// TestMain lets the test binary masquerade as `nt` inside testscript scenarios,
// so .txtar scripts drive the real cobra command tree end-to-end (the layer the
// pure-Go unit tests don't reach). Runs without a shell — the cd-signal is
// asserted by inspecting $NT_CD_FILE.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"nt": func() {
			if err := fang.Execute(context.Background(), cmd.Root()); err != nil {
				os.Exit(1)
			}
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(e *testscript.Env) error {
			e.Setenv("HOME", e.WorkDir)
			e.Setenv("GIT_CONFIG_GLOBAL", e.WorkDir+"/.gitconfig-none")
			e.Setenv("GIT_CONFIG_SYSTEM", e.WorkDir+"/.gitconfig-none")
			e.Setenv("GIT_AUTHOR_NAME", "t")
			e.Setenv("GIT_AUTHOR_EMAIL", "t@t")
			e.Setenv("GIT_COMMITTER_NAME", "t")
			e.Setenv("GIT_COMMITTER_EMAIL", "t@t")
			e.Setenv("NT_NO_FETCH", "1")
			return nil
		},
	})
}
