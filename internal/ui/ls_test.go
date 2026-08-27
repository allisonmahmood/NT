package ui

import (
	"os"
	"testing"
)

func TestAheadBehind(t *testing.T) {
	up, dn := "↑", "↓"
	tests := []struct {
		track string
		want  string
	}{
		{"", "="},
		{"[ahead 2]", "↑2"},
		{"[behind 3]", "↓3"},
		{"[ahead 2, behind 3]", "↑2↓3"},
		{"[ahead 12, behind 7]", "↑12↓7"},
	}
	for _, tc := range tests {
		if got := aheadBehind(tc.track, up, dn); got != tc.want {
			t.Errorf("aheadBehind(%q) = %q, want %q", tc.track, got, tc.want)
		}
	}
}

func TestArrowsLocale(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "C.UTF-8")
	if up, dn := Arrows(); up != "↑" || dn != "↓" {
		t.Errorf("UTF-8 locale: got %q/%q, want arrows", up, dn)
	}
	t.Setenv("LANG", "C")
	if up, dn := Arrows(); up != "^" || dn != "v" {
		t.Errorf("C locale: got %q/%q, want ascii", up, dn)
	}
	// LC_ALL takes precedence over LANG.
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("LANG", "C")
	if up, _ := Arrows(); up != "↑" {
		t.Errorf("LC_ALL precedence failed: %q", up)
	}
	_ = os.Unsetenv("LC_ALL")
}

func TestParseCount(t *testing.T) {
	if n := parseCount("[ahead 42, behind 1]", "ahead "); n != 42 {
		t.Errorf("ahead count = %d", n)
	}
	if n := parseCount("[behind 7]", "behind "); n != 7 {
		t.Errorf("behind count = %d", n)
	}
	if n := parseCount("[gone]", "ahead "); n != 0 {
		t.Errorf("missing key = %d, want 0", n)
	}
}
