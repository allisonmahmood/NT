// Package config centralizes nt's environment knobs and constants so the
// producer and consumer of each value can never drift apart.
package config

import "os"

// TrashPrefix names the rename-aside dirs `nt rm` leaves for its background
// delete. Single source of truth: the producer (worktree.Remove) and the reaper
// (`nt prune`) both build off this.
const TrashPrefix = ".nt-trash-"

// Remote is the git remote nt fetches from / tracks against. Defaults to
// "origin"; override with NT_REMOTE.
func Remote() string {
	if r := os.Getenv("NT_REMOTE"); r != "" {
		return r
	}
	return "origin"
}

// NoFetch reports whether the network fetch should be skipped (offline/speed).
func NoFetch() bool {
	return os.Getenv("NT_NO_FETCH") != ""
}

// RootOverride returns an explicit worktrees-root location, or "" to use the
// default (<repo>.worktrees next to the main checkout).
func RootOverride() string {
	return os.Getenv("NT_ROOT")
}

// CDFile is the path nt writes its target directory to so the shell shim can cd
// there after the binary exits. Empty when nt is run without the shell shim.
func CDFile() string {
	return os.Getenv("NT_CD_FILE")
}
