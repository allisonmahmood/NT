#!/usr/bin/env bash
# Exercise the actual cd hooks and completion registration in each supported shell.
# Pass a packaged binary to test exactly what users will install.
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
fixture_root="$(mktemp -d)"
trap 'rm -rf "$fixture_root"' EXIT
mkdir "$fixture_root/bin"
if [[ $# -eq 1 ]]; then
  install -m 0755 "$1" "$fixture_root/bin/nt"
else
  (cd "$repo_root" && go build -o "$fixture_root/bin/nt" .)
fi
export PATH="$fixture_root/bin:$PATH"
unset NT_ROOT NT_REMOTE GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_INDEX_FILE
unset GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_CONFIG_COUNT
export NT_NO_FETCH=1 GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid

cat > "$fixture_root/behavior" <<'BEHAVIOR'
nt feature main || exit 1
test "$PWD" = "$NT_TEST_REPO.worktrees/feature" || exit 1
nt home || exit 1
test "$PWD" = "$NT_TEST_REPO" || exit 1
nt __complete cd '' | command grep -q feature || exit 1
nt cd feature || exit 1
test "$PWD" = "$NT_TEST_REPO.worktrees/feature" || exit 1
printf dirty >> README
! nt rm feature || exit 1
test -f "$NT_TEST_REPO.worktrees/feature/README" || exit 1
test "$PWD" = "$NT_TEST_REPO.worktrees/feature" || exit 1
git checkout -- README || exit 1
nt done feature || exit 1
test "$PWD" = "$NT_TEST_REPO" || exit 1
test ! -d "$NT_TEST_REPO.worktrees/feature" || exit 1
! git show-ref --verify --quiet refs/heads/feature || exit 1
BEHAVIOR
export NT_TEST_BEHAVIOR="$fixture_root/behavior"
for shell in bash zsh fish; do
  command -v "$shell" >/dev/null
  export NT_TEST_REPO="$fixture_root/$shell repo space'quote"
  git init -q -b main "$NT_TEST_REPO"
  printf initial > "$NT_TEST_REPO/README"
  git -C "$NT_TEST_REPO" add README
  git -C "$NT_TEST_REPO" commit -qm initial
  (
    cd "$NT_TEST_REPO"
    case "$shell" in
      bash) bash --noprofile --norc -c 'eval "$(nt init bash)"; complete -p nt || exit 1; source "$NT_TEST_BEHAVIOR"' ;;
      zsh) zsh -f -c 'autoload -Uz compinit; compinit -D -u; eval "$(nt init zsh)"; test "${_comps[nt]}" = _nt || exit 1; source "$NT_TEST_BEHAVIOR"' ;;
      fish) fish --no-config -c 'nt init fish | source; complete | string match -q "*__nt_prepare_completions*"; or exit 1; source "$NT_TEST_BEHAVIOR"' ;;
    esac
  )
  echo "$shell: shell integration passed"
done
