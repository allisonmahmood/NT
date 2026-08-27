package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// newReapCmd is the hidden background-delete worker spawned (detached) by
// `nt rm` to reclaim disk for renamed-aside trash dirs.
func newReapCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__reap [paths...]",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			for _, p := range args {
				_ = os.RemoveAll(p)
			}
		},
	}
}
