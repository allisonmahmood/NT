package cmd

import (
	"github.com/allisonmahmood/nt/internal/config"
	"github.com/allisonmahmood/nt/internal/git"
	"github.com/allisonmahmood/nt/internal/worktree"
)

// fetchedRemote pins the default-branch tip so creation and home maintenance
// agree even when another process fetches again during this invocation.
type fetchedRemote struct {
	name    string
	branch  string
	commit  string
	fetched bool
}

func fetchRemote(r *worktree.Repo) fetchedRemote {
	remote := fetchedRemote{name: config.Remote(), branch: "main"}
	if !git.OK(r.MainDir, "remote", "get-url", remote.name) {
		remote.name = ""
		return remote
	}
	if !config.NoFetch() {
		// FETCH_HEAD is shared by NT commands fetching from home. Hold the same
		// repository lock until its immutable tip and advertised default are read.
		unlock, err := worktree.LockMutation(r.MainDir)
		if err != nil {
			warn("fetch skipped: %s; using cached refs for creation", err)
		} else {
			defer unlock()
			info("fetching %s ...", remote.name)
			remote.fetched = git.Run(r.MainDir, "fetch", "--quiet", remote.name)
			if !remote.fetched {
				warn("warning: fetch failed, using cached refs for creation; home left unchanged")
			}
		}
	}
	remote.branch = git.DefaultBranch(r.MainDir, remote.name)
	if remote.fetched {
		if branch, ok := git.RemoteDefaultBranch(r.MainDir, remote.name); ok {
			remote.branch = branch
		} else {
			remote.fetched = false
			warn("could not verify the remote default branch; home left unchanged")
		}
	}
	remote.commit, _ = git.Query(r.MainDir, "rev-parse", "--verify", "refs/remotes/"+remote.name+"/"+remote.branch+"^{commit}")
	if remote.fetched {
		if tip, ok := git.FetchedBranch(r.MainDir, remote.branch); ok {
			remote.commit = tip
		} else {
			remote.fetched = false
		}
	}
	return remote
}

func refreshHome(r *worktree.Repo, remote fetchedRemote, action worktree.HomeAction) {
	if !remote.fetched || remote.commit == "" {
		if action != worktree.HomeMaintenance {
			info("home left unchanged: no freshly fetched default branch (offline, fetch disabled/failed, or no remote)")
		}
		return
	}
	info("%s", worktree.RefreshHome(r.MainDir, remote.name, remote.branch, remote.commit, action))
}
