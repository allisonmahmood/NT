// Package shellinit embeds the per-shell `nt` shim functions that perform the
// actual cd (a child process can't change its parent shell's directory, so there
// is always a thin shell function — the zoxide model).
package shellinit

import _ "embed"

// Zsh is the zsh `nt` shim function.
//
//go:embed nt.zsh
var Zsh string

// Bash is the bash `nt` shim function.
//
//go:embed nt.bash
var Bash string

// Fish is the fish `nt` shim function.
//
//go:embed nt.fish
var Fish string
