package git

import (
	"os"
	"strings"
)

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

// RemoteDefaultBranch discovers the advertised default without rewriting a
// user's cached remote HEAD. Fetch alone may leave that symbolic ref stale.
func RemoteDefaultBranch(dir, remote string) (string, bool) {
	out, ok := Query(dir, "ls-remote", "--symref", remote, "HEAD")
	if !ok {
		return "", false
	}
	for _, line := range strings.Split(out, "\n") {
		ref, name, _ := strings.Cut(line, "\t")
		if name == "HEAD" && strings.HasPrefix(ref, "ref: refs/heads/") {
			return strings.TrimPrefix(ref, "ref: refs/heads/"), true
		}
	}
	return "", false
}

// FetchedBranch reads the actual fetched tip. A successful fetch need not update
// its remote-tracking ref when the user has configured a narrower fetch refspec.
func FetchedBranch(dir, branch string) (string, bool) {
	path, ok := Query(dir, "rev-parse", "--path-format=absolute", "--git-path", "FETCH_HEAD")
	if !ok {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) == 3 && strings.HasPrefix(fields[2], "branch '"+branch+"' of ") {
			return Query(dir, "rev-parse", "--verify", "--end-of-options", fields[0]+"^{commit}")
		}
	}
	return "", false
}
