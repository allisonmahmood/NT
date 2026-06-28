package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/ui"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "ls",
		Aliases:       []string{"list"},
		Short:         "List this repo's worktrees, with dirty/ahead-behind",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := loadRepo()
			fmt.Println(ui.Render(r))
			return nil
		},
	}
}
