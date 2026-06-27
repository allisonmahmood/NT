// Package shellinit embeds the per-shell `nt` shim functions that perform the
// actual cd (a child process can't change its parent shell's directory, so there
// is always a thin shell function — the zoxide model).
package shellinit

import _ "embed"

//go:embed nt.zsh
var Zsh string

//go:embed nt.bash
var Bash string

//go:embed nt.fish
var Fish string
