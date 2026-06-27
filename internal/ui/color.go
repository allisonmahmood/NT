// Package ui renders nt's human-facing output: the ls table, glyphs, colors,
// and the interactive pickers.
package ui

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// Color is applied only when stdout is a real terminal and NO_COLOR is unset, so
// piped/redirected output (and the parity/test harnesses) stay byte-for-byte
// plain. Widths are always computed on the uncolored text.
var colorize = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}()

const (
	cReset  = "\x1b[0m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
)

func paint(s, code string) string {
	if !colorize || s == "" {
		return s
	}
	return code + s + cReset
}

func styleMarker(m string) string {
	switch m {
	case "*":
		return paint(m, cYellow)
	case "?":
		return paint(m, cRed)
	default:
		return m
	}
}

func styleTag(tag string) string { return paint(tag, cDim) }

// styleAB colors the ahead/behind token: green ahead, red behind/gone/missing,
// yellow when diverged both ways, dim for in-sync / no-upstream.
func styleAB(ab string) string {
	switch ab {
	case "=", "-":
		return paint(ab, cDim)
	case "gone", "?":
		return paint(ab, cRed)
	}
	hasUp := strings.ContainsAny(ab, "↑^")
	hasDn := strings.ContainsAny(ab, "↓v")
	switch {
	case hasUp && hasDn:
		return paint(ab, cYellow)
	case hasUp:
		return paint(ab, cGreen)
	case hasDn:
		return paint(ab, cRed)
	default:
		return ab
	}
}
