#!/bin/zsh
# Functional tests for nt(). Builds a throwaway origin + checkout, exercises
# every code path, asserts on $PWD / branch / tracking / removal.
emulate -L zsh
REPO="${${(%):-%x}:A:h:h}"
TMP="$(mktemp -d)"; TMP="${TMP:A}"   # :A resolves the macOS /var -> /private/var symlink
trap 'rm -rf "$TMP"' EXIT
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t

cd "$TMP"
git init -q --bare -b main remote.git
git clone -q remote.git seed
cd seed
print init > README.md; git add -A; git commit -qm init; git push -q -u origin main
git checkout -q -b team/issue-123-some-feature
print feat >> README.md; git commit -qam feat
git push -q -u origin team/issue-123-some-feature
git checkout -q main
cd "$TMP"
git clone -q remote.git example-repo
cd "$TMP/example-repo"; git remote set-head origin -a >/dev/null 2>&1

source "$REPO/nt.plugin.zsh"

pass=0; fail=0
check() { if [[ "$2" == "$3" ]]; then print "  PASS: $1"; ((pass++))
          else print "  FAIL: $1\n        got:      $2\n        expected: $3"; ((fail++)); fi }
contains() { if [[ "$2" == *"$3"* ]]; then print "  PASS: $1"; ((pass++))
             else print "  FAIL: $1 (missing '$3')"; ((fail++)); fi }
absent() { [[ -e "$1" ]] && { print "  FAIL: $2 (still on disk)"; ((fail++)); } || { print "  PASS: $2"; ((pass++)); } }

print "\n=== create: brand-new branch off origin/main ==="
cd "$TMP/example-repo"; nt fix-login
check "pwd" "$PWD" "$TMP/example-repo.worktrees/fix-login"
check "branch" "$(git branch --show-current)" "fix-login"
check "no upstream" "$(git rev-parse --abbrev-ref '@{u}' 2>/dev/null)" ""

print "\n=== create: existing remote branch with slash ==="
cd "$TMP/example-repo"; nt team/issue-123-some-feature
check "pwd" "$PWD" "$TMP/example-repo.worktrees/team/issue-123-some-feature"
check "tracks origin" "$(git rev-parse --abbrev-ref '@{u}' 2>/dev/null)" "origin/team/issue-123-some-feature"

print "\n=== create: re-run -> cd, no duplicate ==="
cd "$TMP/example-repo"; before="$(git worktree list | wc -l)"; nt fix-login; after="$(git worktree list | wc -l)"
check "pwd reused" "$PWD" "$TMP/example-repo.worktrees/fix-login"
check "no new tree" "$before" "$after"

print "\n=== create: explicit base + run from inside a worktree ==="
cd "$TMP/example-repo.worktrees/fix-login"; nt sibling-test
check "anchored to main, sibling path" "$PWD" "$TMP/example-repo.worktrees/sibling-test"

print "\n=== nt ls / nt -h / bare nt ==="
contains "ls shows fix-login" "$(cd "$TMP/example-repo"; nt ls)" "fix-login"
contains "-h shows usage"     "$(cd "$TMP/example-repo"; nt -h)" "nt cd"
( cd "$TMP/example-repo"; nt >/dev/null ); check "bare nt returns 0" "$?" "0"

print "\n=== nt cd <branch> ==="
cd "$TMP/example-repo"; nt cd fix-login
check "cd by name" "$PWD" "$TMP/example-repo.worktrees/fix-login"
cd "$TMP/example-repo"; nt cd does-not-exist; check "cd unknown -> nonzero" "$?" "1"

print "\n=== nt rm <branch> ==="
cd "$TMP/example-repo"; nt rm sibling-test >/dev/null
contains "rm removed it from list" "$(git worktree list)" "fix-login"  # sanity: list still works
[[ "$(git worktree list)" == *"sibling-test"* ]] && { print "  FAIL: sibling-test still listed"; ((fail++)); } || { print "  PASS: sibling-test gone from list"; ((pass++)); }
absent "$TMP/example-repo.worktrees/sibling-test" "dir gone from disk"

print "\n=== nt rm refuses the main checkout ==="
cd "$TMP/example-repo"; nt rm main; check "rm main -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo/.git" ]] && { print "  PASS: main checkout intact"; ((pass++)); } || { print "  FAIL: main gone"; ((fail++)); }

print "\n=== nt rm the worktree you're standing in ==="
cd "$TMP/example-repo"; nt temp-del >/dev/null
check "now inside temp-del" "$PWD" "$TMP/example-repo.worktrees/temp-del"
nt rm temp-del >/dev/null
check "stepped out to main" "$PWD" "$TMP/example-repo"
absent "$TMP/example-repo.worktrees/temp-del" "temp-del removed"

print "\n=== nt rm <path>: remove a branch-less (detached) worktree ==="
cd "$TMP/example-repo"
git worktree add -q --detach "$TMP/example-repo.worktrees/det-by-path" >/dev/null 2>&1
nt rm "$TMP/example-repo.worktrees/det-by-path" >/dev/null
absent "$TMP/example-repo.worktrees/det-by-path" "detached worktree removed by full path"

print "\n=== nt rm <leaf>: resolve a detached worktree by unique path leaf ==="
cd "$TMP/example-repo"
git worktree add -q --detach "$TMP/example-repo.worktrees/det-by-leaf" >/dev/null 2>&1
nt rm det-by-leaf >/dev/null
absent "$TMP/example-repo.worktrees/det-by-leaf" "detached worktree removed by leaf"

print "\n=== nt rm: ambiguous leaf is refused, removes nothing ==="
cd "$TMP/example-repo"
mkdir -p "$TMP/example-repo.worktrees/dup-a" "$TMP/example-repo.worktrees/dup-b"
git worktree add -q --detach "$TMP/example-repo.worktrees/dup-a/leaf" >/dev/null 2>&1
git worktree add -q --detach "$TMP/example-repo.worktrees/dup-b/leaf" >/dev/null 2>&1
nt rm leaf 2>/dev/null; check "ambiguous leaf -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo.worktrees/dup-a/leaf" && -d "$TMP/example-repo.worktrees/dup-b/leaf" ]] \
  && { print "  PASS: both ambiguous worktrees intact"; ((pass++)); } \
  || { print "  FAIL: an ambiguous worktree was removed"; ((fail++)); }

print "\n=== not in a repo -> error ==="
cd "$TMP"; nt foo;    check "create -> nonzero" "$?" "1"
cd "$TMP"; nt cd foo; check "cd -> nonzero" "$?" "1"

print "\n=== RESULT: $pass passed, $fail failed ==="
[[ $fail -eq 0 ]]
