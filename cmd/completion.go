package cmd

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/worktree"
)

// completeCreate offers every local+remote branch short-name (for both the
// branch arg and the base arg) — the same candidates the zsh _nt produced.
func completeCreate(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return allBranches(), cobra.ShellCompDirectiveNoFileComp
}

// completeCd offers branches that currently have a worktree.
func completeCd(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	r, err := worktree.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return r.BranchNames(), cobra.ShellCompDirectiveNoFileComp
}

// completeTargets offers every removable worktree (branch name, or full path for
// detached ones) plus -f. Used by rm/done.
func completeTargets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	r, err := worktree.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return append(r.Targets(), "-f"), cobra.ShellCompDirectiveNoFileComp
}

// allBranches lists local + remote branch short-names (remote prefix stripped,
// */HEAD dropped, de-duped, sorted) — mirrors zsh _nt_all_branches.
func allBranches() []string {
	out, ok := git.Query("", "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var res []string
	for _, ref := range git.Lines(out) {
		if strings.HasSuffix(ref, "/HEAD") {
			continue
		}
		name := ref
		switch {
		case strings.HasPrefix(name, "refs/heads/"):
			name = name[len("refs/heads/"):]
		case strings.HasPrefix(name, "refs/remotes/"):
			name = name[len("refs/remotes/"):]
			if i := strings.IndexByte(name, '/'); i >= 0 {
				name = name[i+1:]
			}
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		res = append(res, name)
	}
	sort.Strings(res)
	return res
}
