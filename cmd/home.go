package cmd

import (
	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/shell"
)

func newHomeCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "home",
		Short:         "cd back to the main checkout",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()
			shell.SignalCD(r.MainDir)
			info("→ %s  (main checkout)", r.MainDir)
			return nil
		},
	}
}
