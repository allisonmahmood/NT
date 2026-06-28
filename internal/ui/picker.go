package ui

import "github.com/charmbracelet/huh"

// PickOne shows a single-select picker. Returns ("", false) on cancel or in a
// non-interactive context (huh errors when there's no tty) — callers treat that
// as "nothing selected", a clean no-op.
func PickOne(title string, options []string) (string, bool) {
	if len(options) == 0 {
		return "", false
	}
	var choice string
	if err := huh.NewSelect[string]().
		Title(title).
		Options(toOptions(options)...).
		Value(&choice).
		Run(); err != nil {
		return "", false
	}
	return choice, choice != ""
}

// PickMany shows a multi-select picker (space=mark, enter=confirm). Returns
// (nil, false) on cancel / non-tty.
func PickMany(title string, options []string) ([]string, bool) {
	if len(options) == 0 {
		return nil, false
	}
	var chosen []string
	if err := huh.NewMultiSelect[string]().
		Title(title).
		Options(toOptions(options)...).
		Value(&chosen).
		Run(); err != nil {
		return nil, false
	}
	return chosen, true
}

func toOptions(vals []string) []huh.Option[string] {
	opts := make([]huh.Option[string], len(vals))
	for i, v := range vals {
		opts[i] = huh.NewOption(v, v)
	}
	return opts
}
