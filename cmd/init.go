package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/shellinit"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [zsh|bash|fish]",
		Short: "Print shell integration (cd shim + completions) to eval/source",
		Long: `Print the shell integration for the given shell. Add to your rc file:

  zsh:   eval "$(nt init zsh)"
  bash:  eval "$(nt init bash)"
  fish:  nt init fish | source

This defines the thin ` + "`nt`" + ` shell function that performs the actual cd (a child
process can't change its parent shell's directory) and registers tab completion.`,
		Args:          cobra.ExactArgs(1),
		ValidArgs:     []string{"zsh", "bash", "fish"},
		SilenceUsage:  true,
		SilenceErrors: true,
		// init must work OUTSIDE a repo (it runs in your rc file), so it never
		// loads the repo.
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			root := cmd.Root()
			switch args[0] {
			case "zsh":
				fmt.Fprint(out, shellinit.Zsh)
				// GenZshCompletion emits `compdef _nt nt`, which registers the
				// completion when this is sourced in an interactive shell (after
				// compinit). Needs compinit to have run — standard in any zsh that
				// has tab-completion, and what every plugin manager guarantees.
				if err := root.GenZshCompletion(out); err != nil {
					return err
				}
			case "bash":
				fmt.Fprint(out, shellinit.Bash)
				if err := root.GenBashCompletionV2(out, true); err != nil {
					return err
				}
			case "fish":
				fmt.Fprint(out, shellinit.Fish)
				if err := root.GenFishCompletion(out, true); err != nil {
					return err
				}
			default:
				fail("unknown shell '%s' (want zsh|bash|fish)", args[0])
			}
			return nil
		},
	}
}
