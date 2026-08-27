// Package shell implements the out-of-band cd-signal protocol and embeds the
// per-shell shim scripts.
package shell

import (
	"os"
	"syscall"

	"github.com/allisonmahmood/nt/internal/config"
)

// SignalCD asks the shell shim to cd to dir after the binary exits, by writing
// the absolute path to the file named in $NT_CD_FILE. Rich human output stays on
// stdout; only this side-channel moves the shell. Paths with spaces and quotes
// round-trip safely (no shell quoting, no eval). When NT_CD_FILE is unset (nt
// run without the shim) it's a no-op — the message is still printed.
//
// Security: the shim creates the handoff file with `mktemp` (a freshly-created,
// 0600, user-owned regular file), so an attacker on a shared temp dir can't
// pre-plant the path. We open with O_NOFOLLOW as defense-in-depth: if the final
// component is ever a symlink (e.g. a non-sticky TMPDIR), the open fails and we
// simply don't cd, rather than following it and truncating a victim file.
func SignalCD(dir string) {
	f := config.CDFile()
	if f == "" {
		return
	}
	// Best-effort: any failure just means no cd, never a crash.
	fh, err := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = fh.Close() }()
	_, _ = fh.WriteString(dir)
}
