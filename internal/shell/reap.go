//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"syscall"
)

// SpawnReap reclaims disk off the interactive path: it starts a detached child
// (`nt __reap <paths…>`, new session, no Wait) to rm the rename-aside trash dirs,
// so `nt rm` returns instantly however big the trees are. Crash-safe: `nt prune`
// reaps any trash a killed reaper leaves behind, so correctness never depends on
// this child finishing. Falls back to a synchronous remove if self-exec fails.
func SpawnReap(paths []string) {
	if len(paths) == 0 {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		for _, p := range paths {
			_ = os.RemoveAll(p)
		}
		return
	}
	cmd := exec.Command(exe, append([]string{"__reap"}, paths...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		for _, p := range paths {
			_ = os.RemoveAll(p)
		}
		return
	}
	_ = cmd.Process.Release()
}
