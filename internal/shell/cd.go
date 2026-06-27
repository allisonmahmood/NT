// Package shell implements the out-of-band cd-signal protocol and embeds the
// per-shell shim scripts.
package shell

import (
	"os"

	"github.com/allisonmahmood/nt/internal/config"
)

// SignalCD asks the shell shim to cd to dir after the binary exits, by writing
// the absolute path to the file named in $NT_CD_FILE. Rich human output stays on
// stdout; only this side-channel moves the shell. Paths with spaces/quotes/
// newlines round-trip as raw bytes (no shell quoting, no eval). When NT_CD_FILE
// is unset (nt run without the shim) it's a no-op — the message is still printed.
func SignalCD(dir string) {
	f := config.CDFile()
	if f == "" {
		return
	}
	// Best-effort: a failed write just means no cd, never a crash.
	_ = os.WriteFile(f, []byte(dir), 0o600)
}
