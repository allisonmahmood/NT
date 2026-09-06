package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/allisonmahmood/nt/internal/git"
)

// HomeAction distinguishes explicit visits from implicit returns and maintenance.
type HomeAction int

const (
	HomeMaintenance HomeAction = iota
	HomeReturn
	HomeVisit
)

type homeState struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	Visit  string `json:"visit,omitempty"`
}

// RefreshHome fast-forwards home to a successfully fetched, immutable target.
// Explicit visits may establish a baseline; maintenance only follows an existing
// one. A refusal is informational so callers can still navigate or create work.
func RefreshHome(dir, remote, branch, target string, action HomeAction) string {
	unlock, err := LockMutation(dir)
	if err != nil {
		return "home left unchanged: " + err.Error()
	}
	defer unlock()
	return refreshHome(dir, remote, branch, target, action)
}

func refreshHome(dir, remote, branch, target string, action HomeAction) string {
	left := func(reason string) string { return "home left unchanged: " + reason }
	if action == HomeMaintenance {
		enabled, ok := git.Query(dir, "config", "--local", "--type=bool", "--default", "true", "--get", "nt.autoRefreshHome")
		if !ok || enabled != "true" {
			return left("automatic refresh disabled or invalid nt.autoRefreshHome setting")
		}
	}
	if err := CheckIdle(dir); err != nil {
		return left(err.Error())
	}
	currentBranch, ok := git.Query(dir, "symbolic-ref", "--quiet", "HEAD")
	if !ok || currentBranch != "refs/heads/"+branch {
		return left("checkout is not on " + branch)
	}
	status, ok := git.Query(dir, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
	if !ok {
		return left("cannot inspect working tree")
	}
	if status != "" {
		return left("uncommitted changes; use nt <new-name> --take to take them into a worktree")
	}
	head, ok := git.Query(dir, "rev-parse", "--verify", "HEAD")
	if !ok {
		return left("cannot read HEAD")
	}
	common, ok := git.Query(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok {
		return left("cannot locate home state")
	}
	path := filepath.Join(common, "nt-home.json")
	state := homeState{}
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &state)
	}
	if err != nil && !os.IsNotExist(err) {
		return left("cannot read home state: " + err.Error())
	}
	if action == HomeMaintenance {
		if state.Commit == "" {
			return left("run nt home once to enable safe maintenance")
		}
		if state.Remote != remote || state.Branch != branch || state.Commit != head {
			return left("HEAD or tracking choice changed outside NT; run nt home to resume maintenance")
		}
	}
	if !git.OK(dir, "merge-base", "--is-ancestor", head, target) {
		return left("local commits, diverged history, or unreadable ancestry")
	}
	if head != target && !git.Run(dir, "-c", "core.hooksPath=/dev/null", "merge", "--ff-only", "--no-autostash", "--no-overwrite-ignore", "--no-edit", "--quiet", target) {
		return left("fast-forward refused; resolve Git's diagnostic before retrying nt home")
	}
	// Do not record a baseline if another Git process moved the branch meanwhile.
	actual, valid := git.Query(dir, "rev-parse", "HEAD")
	actualBranch, attached := git.Query(dir, "symbolic-ref", "--quiet", "HEAD")
	if !valid || !attached || actual != target || actualBranch != currentBranch {
		return left("HEAD changed during refresh; no maintenance baseline recorded")
	}
	message := "home is current"
	if head != target {
		if count, ok := git.Query(dir, "rev-list", "--count", head+".."+target); ok {
			message = "home updated by " + count + " commits"
		}
	}
	if action == HomeVisit {
		if state.Remote == remote && state.Branch == branch && state.Visit != "" && git.OK(dir, "merge-base", "--is-ancestor", state.Visit, target) {
			if count, ok := git.Query(dir, "rev-list", "--count", state.Visit+".."+target); ok && count != "0" {
				message += " · " + count + " new commits since your last nt home"
			}
		}
		state.Visit = target
	}
	state.Remote, state.Branch, state.Commit = remote, branch, target
	if err := writeHomeState(path, state); err != nil {
		return message + "; could not save maintenance baseline: " + err.Error()
	}
	return message
}

func writeHomeState(path string, state homeState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "nt-home-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("save home state: %w", err)
	}
	return nil
}
