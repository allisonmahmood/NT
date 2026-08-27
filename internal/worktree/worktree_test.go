package worktree

import (
	"reflect"
	"testing"
)

const samplePorcelain = `worktree /home/u/acme
HEAD abc
branch refs/heads/main

worktree /home/u/acme.worktrees/fix-login
HEAD def
branch refs/heads/fix-login

worktree /home/u/acme.worktrees/team/issue-123
HEAD aaa
branch refs/heads/team/issue-123

worktree /home/u/acme.worktrees/detached
HEAD bbb
detached
`

func sampleRepo() *Repo {
	wts := Parse(samplePorcelain)
	return &Repo{MainDir: wts[0].Path, Root: "/home/u/acme.worktrees", Worktrees: wts}
}

func TestParse(t *testing.T) {
	wts := Parse(samplePorcelain)
	if len(wts) != 4 {
		t.Fatalf("got %d worktrees, want 4", len(wts))
	}
	if wts[0].Branch != "main" || wts[0].Path != "/home/u/acme" {
		t.Errorf("main entry wrong: %+v", wts[0])
	}
	if wts[2].Branch != "team/issue-123" {
		t.Errorf("slash branch not preserved: %q", wts[2].Branch)
	}
	if !wts[3].Detached || wts[3].Branch != "" {
		t.Errorf("detached entry wrong: %+v", wts[3])
	}
}

func TestParseBare(t *testing.T) {
	wts := Parse("worktree /repo/bare\nbare\n")
	if len(wts) != 1 || !wts[0].Bare {
		t.Fatalf("bare not parsed: %+v", wts)
	}
}

func TestResolve(t *testing.T) {
	r := sampleRepo()
	tests := []struct {
		id        string
		want      string
		ambiguous bool
		none      bool
	}{
		{id: "fix-login", want: "/home/u/acme.worktrees/fix-login"},                      // branch name
		{id: "team/issue-123", want: "/home/u/acme.worktrees/team/issue-123"},            // slash branch
		{id: "/home/u/acme.worktrees/detached", want: "/home/u/acme.worktrees/detached"}, // exact path
		{id: "detached", want: "/home/u/acme.worktrees/detached"},                        // unique tail
		{id: "issue-123", want: "/home/u/acme.worktrees/team/issue-123"},                 // unique tail of nested
		{id: "nope", none: true}, // unknown
	}
	for _, tc := range tests {
		got, amb := r.Resolve(tc.id)
		switch {
		case tc.ambiguous:
			if len(amb) < 2 {
				t.Errorf("Resolve(%q): expected ambiguous, got %q amb=%v", tc.id, got, amb)
			}
		case tc.none:
			if got != "" || amb != nil {
				t.Errorf("Resolve(%q): expected none, got %q amb=%v", tc.id, got, amb)
			}
		default:
			if got != tc.want {
				t.Errorf("Resolve(%q) = %q, want %q", tc.id, got, tc.want)
			}
		}
	}
}

func TestResolveAmbiguousTail(t *testing.T) {
	wts := Parse(`worktree /m
HEAD a
branch refs/heads/main

worktree /m.worktrees/dup-a/leaf
HEAD b
detached

worktree /m.worktrees/dup-b/leaf
HEAD c
detached
`)
	r := &Repo{MainDir: wts[0].Path, Worktrees: wts}
	got, amb := r.Resolve("leaf")
	if got != "" || len(amb) != 2 {
		t.Fatalf("expected ambiguous 2, got %q amb=%v", got, amb)
	}
}

func TestTargetsExcludesMain(t *testing.T) {
	r := sampleRepo()
	got := r.Targets()
	want := []string{"fix-login", "team/issue-123", "/home/u/acme.worktrees/detached"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Targets() = %v, want %v", got, want)
	}
}

func TestDestNestsSlash(t *testing.T) {
	r := sampleRepo()
	if d := r.Dest("team/x"); d != "/home/u/acme.worktrees/team/x" {
		t.Errorf("Dest slash = %q", d)
	}
}
