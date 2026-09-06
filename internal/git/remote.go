package git

import "strings"

// DefaultBranch resolves the selected remote's default, preserving slashed names.
func DefaultBranch(dir, remote string) string {
	if ref, ok := Query(dir, "symbolic-ref", "--quiet", "refs/remotes/"+remote+"/HEAD"); ok {
		return strings.TrimPrefix(ref, "refs/remotes/"+remote+"/")
	}
	for _, branch := range []string{"main", "master"} {
		if OK(dir, "show-ref", "--quiet", "--verify", "refs/remotes/"+remote+"/"+branch) {
			return branch
		}
	}
	return "main"
}
