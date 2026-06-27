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

print "\n=== nt home -> back to the main checkout ==="
cd "$TMP/example-repo.worktrees/fix-login"; nt home
check "home from a worktree -> main checkout" "$PWD" "$TMP/example-repo"
nt home; check "home while already home -> stays put, returns 0" "$PWD" "$TMP/example-repo"
contains "-h lists home" "$(cd "$TMP/example-repo"; nt -h)" "nt home"

print "\n=== nt rm <branch> ==="
cd "$TMP/example-repo"; nt rm sibling-test >/dev/null
contains "rm removed it from list" "$(git worktree list)" "fix-login"  # sanity: list still works
[[ "$(git worktree list)" == *"sibling-test"* ]] && { print "  FAIL: sibling-test still listed"; ((fail++)); } || { print "  PASS: sibling-test gone from list"; ((pass++)); }
absent "$TMP/example-repo.worktrees/sibling-test" "dir gone from disk"

print "\n=== nt rm refuses the main checkout ==="
cd "$TMP/example-repo"; nt rm main; check "rm main -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo/.git" ]] && { print "  PASS: main checkout intact"; ((pass++)); } || { print "  FAIL: main gone"; ((fail++)); }

print "\n=== nt rm main <other>: main among targets aborts the whole batch ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt batch-keep >/dev/null
cd "$TMP/example-repo"; nt rm main batch-keep 2>/dev/null; check "main among targets -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo.worktrees/batch-keep" ]] \
  && { print "  PASS: nothing removed when main is in the batch"; ((pass++)); } \
  || { print "  FAIL: removed a worktree even though main was in the batch"; ((fail++)); }
[[ -d "$TMP/example-repo/.git" ]] && { print "  PASS: main checkout intact (mixed batch)"; ((pass++)); } || { print "  FAIL: main gone"; ((fail++)); }
cd "$TMP/example-repo"; nt rm batch-keep >/dev/null   # cleanup

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

print "\n=== nt rm <a> <b>: remove several worktrees in one call ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt multi-a >/dev/null
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt multi-b >/dev/null
cd "$TMP/example-repo"; nt rm multi-a multi-b >/dev/null; check "rm of several -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/multi-a" "first of several removed"
absent "$TMP/example-repo.worktrees/multi-b" "second of several removed"

print "\n=== nt rm <good> <bad>: a single bad target removes nothing ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt multi-keep >/dev/null
cd "$TMP/example-repo"; nt rm multi-keep no-such-worktree 2>/dev/null; check "bad target among them -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo.worktrees/multi-keep" ]] \
  && { print "  PASS: good target untouched when another is bad"; ((pass++)); } \
  || { print "  FAIL: removed a worktree despite a bad target in the list"; ((fail++)); }
cd "$TMP/example-repo"; nt rm multi-keep >/dev/null   # cleanup

print "\n=== nt rm <dup> <dup>: repeated / aliased targets de-dup, removed once ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt dup-target >/dev/null
cd "$TMP/example-repo"; nt rm dup-target dup-target >/dev/null; check "repeated target -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/dup-target" "repeated target removed once"
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt dup-alias >/dev/null
cd "$TMP/example-repo"; nt rm dup-alias "$TMP/example-repo.worktrees/dup-alias" >/dev/null; check "branch + its own path -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/dup-alias" "two spellings of one tree removed once"

print "\n=== nt rm <a> <b> -f: force flag is accepted in any position ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt fpos-a >/dev/null
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt fpos-b >/dev/null
print scratch >> "$TMP/example-repo.worktrees/fpos-a/dirty.txt"   # untracked -> needs -f to remove
cd "$TMP/example-repo"; nt rm fpos-a fpos-b -f >/dev/null; check "trailing -f -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/fpos-a" "dirty target removed via trailing -f"
absent "$TMP/example-repo.worktrees/fpos-b" "clean target removed via trailing -f"

print "\n=== nt ls: dirty marker + legend + in-sync ==="
# LC_ALL=C.UTF-8 pins the up/down glyphs to arrows so the assertions are locale-independent.
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt ls-dirty >/dev/null
print scratch >> dirty.txt                       # make ls-dirty worktree dirty
cd "$TMP/example-repo"; lsout="$(LC_ALL=C.UTF-8 nt ls)"
contains "ls shows the branch"        "$lsout" "ls-dirty"
contains "ls marks dirty with *"      "$lsout" "* ls-dirty"
contains "ls tags the main checkout"  "$lsout" "(main)"
contains "ls prints the legend"       "$lsout" "uncommitted changes"
contains "ls renders in-sync ="       "$lsout" "= "
contains "ls shows no-upstream as -"  "$lsout" "ls-dirty"   # no-track branch -> '-' column

print "\n=== nt ls: ahead/behind arrows ==="
# team/issue-123-some-feature tracks origin and is in sync; a local commit -> ahead 1.
cd "$TMP/example-repo.worktrees/team/issue-123-some-feature"; print ahead >> ahead.txt; git add -A; git commit -qm ahead
cd "$TMP/example-repo"; contains "ls shows ahead arrow ↑1" "$(LC_ALL=C.UTF-8 nt ls)" "↑1"
# move it one commit behind its upstream -> behind 1.
git -C "$TMP/example-repo.worktrees/team/issue-123-some-feature" reset --hard '@{u}~1' >/dev/null 2>&1
cd "$TMP/example-repo"; contains "ls shows behind arrow ↓1" "$(LC_ALL=C.UTF-8 nt ls)" "↓1"
cd "$TMP/example-repo"; nt rm -f ls-dirty >/dev/null

print "\n=== nt done: merged branch -> worktree AND branch deleted ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt done-merged >/dev/null   # new branch, no extra commits -> merged
cd "$TMP/example-repo"; nt done done-merged >/dev/null; check "done merged -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/done-merged" "done removed the worktree"
git show-ref --verify --quiet refs/heads/done-merged \
  && { print "  FAIL: merged branch should be deleted"; ((fail++)); } \
  || { print "  PASS: merged branch deleted"; ((pass++)); }

print "\n=== nt done: unmerged branch -> worktree gone, branch KEPT (nonzero) ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt done-unmerged >/dev/null
print w >> w.txt; git add -A; git commit -qm w                    # unmerged commit
cd "$TMP/example-repo"; nt done done-unmerged 2>/dev/null; check "done unmerged -> nonzero" "$?" "1"
absent "$TMP/example-repo.worktrees/done-unmerged" "done removed the worktree even when keeping branch"
git show-ref --verify --quiet refs/heads/done-unmerged \
  && { print "  PASS: unmerged branch kept"; ((pass++)); } \
  || { print "  FAIL: unmerged branch was deleted"; ((fail++)); }

print "\n=== nt done -f: force-delete the unmerged branch ==="
cd "$TMP/example-repo"
git worktree add -q "$TMP/example-repo.worktrees/done-force" done-unmerged >/dev/null 2>&1
cd "$TMP/example-repo"; nt done -f done-force >/dev/null; check "done -f -> 0" "$?" "0"
git show-ref --verify --quiet refs/heads/done-unmerged \
  && { print "  FAIL: -f should force-delete the branch"; ((fail++)); } \
  || { print "  PASS: -f force-deleted the branch"; ((pass++)); }

print "\n=== nt done refuses the main checkout ==="
cd "$TMP/example-repo"; nt done main 2>/dev/null; check "done main -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo/.git" ]] && { print "  PASS: main checkout intact"; ((pass++)); } || { print "  FAIL: main gone"; ((fail++)); }

print "\n=== nt done: detached worktree -> removed, no branch to delete (0) ==="
cd "$TMP/example-repo"; git worktree add -q --detach "$TMP/example-repo.worktrees/done-det" >/dev/null 2>&1
cd "$TMP/example-repo"; doneout="$(nt done done-det 2>&1)"; check "done detached -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/done-det" "done removed the detached worktree"
contains "done detached notes no branch" "$doneout" "no branch to delete"

print "\n=== nt done: dirty worktree needs -f to force removal ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt done-dirty >/dev/null
print x >> dirtyfile.txt                          # untracked -> dirty worktree
cd "$TMP/example-repo"; nt done done-dirty 2>/dev/null; check "done dirty (no -f) -> nonzero" "$?" "1"
[[ -d "$TMP/example-repo.worktrees/done-dirty" ]] && { print "  PASS: dirty worktree intact without -f"; ((pass++)); } || { print "  FAIL: dirty worktree removed without -f"; ((fail++)); }
cd "$TMP/example-repo"; nt done -f done-dirty >/dev/null; check "done -f dirty -> 0" "$?" "0"
absent "$TMP/example-repo.worktrees/done-dirty" "done -f force-removed the dirty worktree"

print "\n=== _nt_gone_branches: detects a gone upstream ==="
cd "$TMP/example-repo"
git push -q origin "HEAD:refs/heads/throwaway"          # create remote branch
git fetch -q origin
git branch --quiet throwaway origin/throwaway 2>/dev/null
git branch --quiet --set-upstream-to=origin/throwaway throwaway >/dev/null 2>&1
git push -q origin --delete throwaway                   # remote vanishes
git fetch -pq origin                                    # prune -> upstream marked gone
contains "throwaway listed as gone" "$(_nt_gone_branches)" "throwaway"

print "\n=== _nt_delete_branches: deletes and reports (the prune delete loop) ==="
delout="$(_nt_delete_branches throwaway 2>&1)"
contains "reports the deletion" "$delout" "deleted branch throwaway"
contains "reports the old sha"  "$delout" "recover via git reflog"
git show-ref --verify --quiet refs/heads/throwaway \
  && { print "  FAIL: throwaway not deleted"; ((fail++)); } \
  || { print "  PASS: _nt_delete_branches removed throwaway"; ((pass++)); }

print "\n=== nt prune: worktree prune + empty-dir cleanup ==="
# Use a fresh nested namespace so no other worktree shares the parent dir.
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt nest-ns/leaf >/dev/null
cd "$TMP/example-repo"; nt rm nest-ns/leaf >/dev/null   # leaves an empty nest-ns/ dir
[[ -d "$TMP/example-repo.worktrees/nest-ns" ]] && { print "  PASS: empty nest-ns/ dir present pre-prune"; ((pass++)); } || { print "  FAIL: setup"; ((fail++)); }
nt prune </dev/null >/dev/null                          # </dev/null: non-tty -> no interactive branch deletion
absent "$TMP/example-repo.worktrees/nest-ns" "prune removed the empty nest-ns/ dir"

print "\n=== nt prune: stale (deleted-on-disk) worktree entry ==="
cd "$TMP/example-repo"; NT_NO_FETCH=1 nt stale-wt >/dev/null
cd "$TMP/example-repo"; rm -rf "$TMP/example-repo.worktrees/stale-wt"   # vanish without git's knowledge
nt prune </dev/null >/dev/null
[[ "$(git worktree list)" == *"stale-wt"* ]] && { print "  FAIL: stale entry still listed"; ((fail++)); } || { print "  PASS: stale entry pruned"; ((pass++)); }

print "\n=== nt prune (non-tty): lists gone branches but never deletes them ==="
cd "$TMP/example-repo"
git push -q origin "HEAD:refs/heads/gonelist"
git fetch -q origin
git branch --quiet gonelist origin/gonelist 2>/dev/null
git branch --quiet --set-upstream-to=origin/gonelist gonelist >/dev/null 2>&1
git push -q origin --delete gonelist
git fetch -pq origin                                   # upstream now [gone]
pruneout="$(cd "$TMP/example-repo"; nt prune </dev/null 2>&1)"   # </dev/null -> non-tty guard
contains "prune lists the gone branch" "$pruneout" "gonelist"
git show-ref --verify --quiet refs/heads/gonelist \
  && { print "  PASS: non-tty prune did NOT delete the branch"; ((pass++)); } \
  || { print "  FAIL: non-tty prune deleted a branch (safety guard broken!)"; ((fail++)); }
git branch -D gonelist >/dev/null 2>&1

print "\n=== _nt_need_fzf: clear message when fzf is missing ==="
fzout="$(PATH=/var/empty _nt_need_fzf 2>&1)"; fzrc=$?
check    "fzf-missing -> nonzero" "$fzrc" "1"
contains "fzf-missing message"    "$fzout" "fzf required"

print "\n=== not in a repo -> error ==="
cd "$TMP"; nt foo;    check "create -> nonzero" "$?" "1"
cd "$TMP"; nt cd foo; check "cd -> nonzero" "$?" "1"

print "\n=== RESULT: $pass passed, $fail failed ==="
[[ $fail -eq 0 ]]
