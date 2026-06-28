package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/allisonmahmood/nt/internal/config"
	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/shell"
	"github.com/allisonmahmood/nt/internal/worktree"
)

// runCreate is the default path: create or switch to a worktree for args[0]
// (optional base = args[1]) and cd into it. Ports the zsh nt() default branch.
func runCreate(r *worktree.Repo, args []string) {
	branch := args[0]
	base := ""
	if len(args) > 1 {
		base = args[1]
	}

	// Already have a worktree for this branch? Just jump to it.
	if existing := r.ByBranch(branch); existing != "" {
		info("worktree for '%s' already exists", branch)
		shell.SignalCD(existing)
		info("→ %s", existing)
		return
	}

	// Resolve remote + default branch, fetch latest refs.
	remote := config.Remote()
	if !git.OK(r.MainDir, "remote", "get-url", remote) {
		remote = ""
	}
	if remote != "" && !config.NoFetch() {
		info("fetching %s ...", remote)
		// Run (not RunQuiet) so git's own stderr (auth/network diagnostics) reaches
		// the terminal on failure, as the zsh original did; --quiet keeps success
		// silent.
		if !git.Run(r.MainDir, "fetch", "--quiet", remote) {
			warn("warning: fetch failed, using cached refs")
		}
	}

	defaultBranch := ""
	if remote != "" {
		if ref, ok := git.Query(r.MainDir, "symbolic-ref", "--quiet", "refs/remotes/"+remote+"/HEAD"); ok {
			// Strip the full "refs/remotes/<remote>/" prefix, not just up to the
			// last '/', so a slashed default branch (e.g. release/v2) survives.
			defaultBranch = strings.TrimPrefix(ref, "refs/remotes/"+remote+"/")
		}
	}
	if defaultBranch == "" {
		switch {
		case remote != "" && git.OK(r.MainDir, "show-ref", "--quiet", "--verify", "refs/remotes/"+remote+"/main"):
			defaultBranch = "main"
		case remote != "" && git.OK(r.MainDir, "show-ref", "--quiet", "--verify", "refs/remotes/"+remote+"/master"):
			defaultBranch = "master"
		default:
			defaultBranch = "main"
		}
	}

	// Build target path (branch may contain '/').
	dest := r.Dest(branch)
	if _, err := os.Stat(dest); err == nil { // matches zsh `[[ -e ]]` (follows symlinks)
		fail("target path already exists: %s", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fail("could not create worktrees dir %s: %v", filepath.Dir(dest), err)
	}

	hasLocal := git.OK(r.MainDir, "show-ref", "--quiet", "--verify", "refs/heads/"+branch)
	hasRemote := remote != "" && git.OK(r.MainDir, "show-ref", "--quiet", "--verify", "refs/remotes/"+remote+"/"+branch)

	var ok bool
	switch {
	case hasLocal:
		// Local branch exists. Fast-forward to origin only on a clean FF.
		if hasRemote {
			if git.OK(r.MainDir, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/remotes/"+remote+"/"+branch) {
				if !git.RunQuiet(r.MainDir, "branch", "-f", branch, remote+"/"+branch) {
					warn("note: couldn't fast-forward local '%s'", branch)
				}
			} else {
				warn("note: local '%s' diverged from %s/%s; using local copy", branch, remote, branch)
			}
		}
		info("+ worktree on existing branch '%s'", branch)
		ok = git.Run(r.MainDir, "worktree", "add", dest, branch)
	case hasRemote:
		info("+ worktree tracking %s/%s (latest)", remote, branch)
		ok = git.Run(r.MainDir, "worktree", "add", "--track", "-b", branch, dest, remote+"/"+branch)
	default:
		want := base
		if want == "" {
			want = defaultBranch
		}
		var start string
		switch {
		case remote != "" && git.OK(r.MainDir, "show-ref", "--quiet", "--verify", "refs/remotes/"+remote+"/"+want):
			start = remote + "/" + want
		case git.OK(r.MainDir, "show-ref", "--quiet", "--verify", "refs/heads/"+want):
			start = want
		case git.OK(r.MainDir, "rev-parse", "--quiet", "--verify", want+"^{commit}"):
			start = want
		default:
			_ = os.Remove(filepath.Dir(dest))
			fail("base '%s' not found on %s or locally", want, remote)
		}
		info("+ new branch '%s' from %s", branch, start)
		ok = git.Run(r.MainDir, "worktree", "add", "--no-track", "-b", branch, dest, start)
	}

	if !ok {
		_ = os.Remove(dest)
		fail("git worktree add failed")
	}

	shell.SignalCD(dest)
	info("→ %s  (branch: %s)", dest, branch)
}
