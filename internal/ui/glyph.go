package ui

import (
	"os"
	"strings"
)

// Arrows returns the up/down ahead-behind glyphs: real arrows under a UTF-8
// locale, ASCII fallback (^/v) elsewhere so columns still align on C/POSIX.
// Mirrors the zsh check against LC_ALL/LC_CTYPE/LANG for a "*utf*8*" pattern.
func Arrows() (up, down string) {
	lc := firstNonEmpty(os.Getenv("LC_ALL"), os.Getenv("LC_CTYPE"), os.Getenv("LANG"))
	lc = strings.ToLower(lc)
	if i := strings.Index(lc, "utf"); i >= 0 && strings.Contains(lc[i+3:], "8") {
		return "↑", "↓"
	}
	return "^", "v"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
