package cmd

import (
	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/shell"
	"github.com/allisonmahmood/nt/internal/ui"
)

func newCdCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "cd [branch]",
		Short:             "cd to an existing worktree (picker if branch omitted)",
		Args:              cobra.MaximumNArgs(1),
		SilenceUsage:      true,
		SilenceErrors:     true,
		ValidArgsFunction: completeCd,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()
			var target string
			if len(args) == 1 {
				target = r.ByBranch(args[0])
				if target == "" {
					fail("no worktree for branch '%s'", args[0])
				}
			} else {
				paths := make([]string, 0, len(r.Worktrees))
				for _, w := range r.Worktrees {
					paths = append(paths, w.Path)
				}
				p, ok := ui.PickOne("cd worktree", paths)
				if !ok || p == "" {
					return nil
				}
				target = p
			}
			shell.SignalCD(target)
			info("→ %s", target)
			return nil
		},
	}
}
