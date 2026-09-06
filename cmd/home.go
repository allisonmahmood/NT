package cmd

import (
	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/shell"
	"github.com/allisonmahmood/nt/internal/worktree"
)

func newHomeCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "home",
		Short:         "Refresh the main checkout safely and cd home",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()
			refreshHome(r, fetchRemote(r), worktree.HomeVisit)
			shell.SignalCD(r.MainDir)
			info("→ %s  (main checkout)", r.MainDir)
			return nil
		},
	}
}
