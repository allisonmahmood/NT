#!/bin/zsh
# Tests for the tab-completion: that `compdef` binds _nt to nt, and that the
# candidate-generating helpers return exactly the branches we expect.
emulate -L zsh
REPO="${${(%):-%x}:A:h:h}"
TMP="$(mktemp -d)"; TMP="${TMP:A}"   # :A resolves the macOS /var -> /private/var symlink
trap 'rm -rf "$TMP"' EXIT
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t

# fixture: origin with main + a feature branch; a checkout with two worktrees.
cd "$TMP"
git init -q --bare -b main remote.git
git clone -q remote.git seed
cd seed
print init > README.md; git add -A; git commit -qm init; git push -q -u origin main
git checkout -q -b feature-x; print x >> README.md; git commit -qam x; git push -q -u origin feature-x
git checkout -q main
cd "$TMP"; git clone -q remote.git example-repo
cd "$TMP/example-repo"; git remote set-head origin -a >/dev/null 2>&1
source "$REPO/nt.plugin.zsh"
NT_NO_FETCH=1 nt fix-login >/dev/null      # adds a worktree on a new branch
cd "$TMP/example-repo"
NT_NO_FETCH=1 nt feature-x >/dev/null       # worktree tracking origin/feature-x
cd "$TMP/example-repo"
git worktree add -q --detach "$TMP/example-repo.worktrees/_detached" >/dev/null 2>&1  # branch-less worktree
detpath="$TMP/example-repo.worktrees/_detached"

pass=0; fail=0
check() { if [[ "$2" == "$3" ]]; then print "  PASS: $1"; ((pass++))
          else print "  FAIL: $1\n        got:      [$2]\n        expected: [$3]"; ((fail++)); fi }
contains() { if [[ "$2" == *"$3"* ]]; then print "  PASS: $1"; ((pass++))
             else print "  FAIL: $1 (missing '$3')"; ((fail++)); fi }

print "\n=== compdef binds _nt to nt ==="
reg="$(zsh -f -c '
  autoload -Uz compinit; compinit -u >/dev/null 2>&1
  source "'"$REPO"'/nt.plugin.zsh"
  print -r -- "${_comps[nt]:-NONE}"
')"
check "_comps[nt]" "$reg" "_nt"

print "\n=== _nt_wt_branches (cd candidates: all worktree branches) ==="
got="$(_nt_wt_branches | sort | tr '\n' ' ')"
check "all worktree branches" "$got" "feature-x fix-login main "

print "\n=== _nt_all_branches (create / base candidates) ==="
got="$(_nt_all_branches | tr '\n' ' ')"
check "local+remote, deduped, no HEAD" "$got" "feature-x fix-login main "

print "\n=== _nt_wt_targets (rm candidates: branch names + detached paths, no main) ==="
targets="$(_nt_wt_targets)"
contains "branch-backed feature-x by name" "$targets" "feature-x"
contains "branch-backed fix-login by name" "$targets" "fix-login"
contains "detached worktree by path"       "$targets" "$detpath"
check    "main checkout excluded"          "$(_nt_wt_targets | awk '$0=="main"')" ""

print "\n=== _nt routes the new subcommands (catches a mistyped case arm) ==="
# `zsh -n` proves the file parses but not that the routing arms exist/are spelled
# right; grep the source for the subcmd entries and case labels as a cheap proxy.
nt_src="$(<"$REPO/completions/_nt")"
contains "subcmd entry: done"      "$nt_src" "done:"
contains "subcmd entry: prune"     "$nt_src" "prune:"
contains "case arm: done|finish"   "$nt_src" "done|finish"
contains "case arm: prune|clean"   "$nt_src" "prune|clean"

print "\n=== completion files parse cleanly ==="
zsh -n "$REPO/nt.plugin.zsh"   && { print "  PASS: nt.plugin.zsh"; ((pass++)); } || { print "  FAIL: nt.plugin.zsh"; ((fail++)); }
zsh -n "$REPO/completions/_nt" && { print "  PASS: _nt"; ((pass++)); }           || { print "  FAIL: _nt"; ((fail++)); }

print "\n=== RESULT: $pass passed, $fail failed ==="
[[ $fail -eq 0 ]]
