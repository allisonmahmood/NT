#!/bin/zsh
# Parity runner: build the Go binary, put it on PATH, and drive it through the
# real zsh cd-shim (`nt init zsh`) against the adapted behavioral suite. This is
# the literal "works the same way the zsh version did" gate.
emulate -L zsh
set -e
REPO="${${(%):-%x}:A:h:h:h}"
BINDIR="$(mktemp -d)"
print "parity: building nt -> $BINDIR/nt"
( cd "$REPO" && go build -o "$BINDIR/nt" . )
export PATH="$BINDIR:$PATH"
exec zsh "$REPO/test/parity/test_parity.zsh"
