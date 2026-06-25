# nt — git worktree quick-switch.
#
# Install: add this line to your ~/.zshrc, then start a new shell (or `source` it):
#     source ~/Developer/nt/nt.plugin.zsh
#
# Provides the `nt` command plus zsh tab-completion (see completions/_nt).

# Resolve this file's directory (works when sourced).
NT_DIR="${${(%):-%x}:A:h}"

# Make completion discoverable, and register it even if compinit already ran
# (e.g. oh-my-zsh runs compinit before this file is sourced).
fpath+=("$NT_DIR/completions")
if (( $+functions[compdef] )); then
  autoload -Uz _nt
  compdef _nt nt
fi

# --- helpers (also used by the completion) -----------------------------------

# Branches that currently have a worktree, one per line (incl. the main checkout).
# Used by `nt cd` completion.
_nt_wt_branches() {
  command git worktree list --porcelain 2>/dev/null | awk '
    /^branch /{ b=$2; sub("refs/heads/","",b); print b }'
}

# Removable worktree identifiers, one per line (skips the main checkout).
# Branch-backed worktrees -> branch short-name; branch-less (detached) -> full path.
# Used by `nt rm` completion and its fzf picker.
_nt_wt_targets() {
  command git worktree list --porcelain 2>/dev/null | awk '
    function flush() { if (n > 1 && path != "") print (br != "" ? br : path) }
    /^worktree /{ flush(); n++; path=substr($0,10); br="" }
    /^branch /  { br=$2; sub("refs/heads/","",br) }
    END         { flush() }'
}

# Resolve a worktree identifier to its absolute path.
#   $1 = identifier: a branch short-name, an absolute worktree path, or a unique
#        path tail (last component, or any trailing "/segment").
#   $2 = output of `git worktree list --porcelain`.
# Prints the path and returns 0 on a unique match; returns 1 if nothing matches;
# returns 2 (and lists the candidates on stderr) if a tail matches more than one.
_nt_resolve_wt() {
  emulate -L zsh
  local id="$1" wtlist="$2" target
  target="$(awk -v b="refs/heads/$id" '/^worktree /{p=substr($0,10)} $0=="branch "b{print p; exit}' <<<"$wtlist")"
  if [[ -z "$target" ]]; then
    local -a wpaths matches
    wpaths=(${(f)"$(awk '/^worktree /{print substr($0,10)}' <<<"$wtlist")"})
    matches=(${(M)wpaths:#$id})                          # exact path
    (( $#matches == 0 )) && matches=(${(M)wpaths:#*/$id}) # unique trailing segment(s)
    if (( $#matches > 1 )); then
      print -u2 "nt: '$id' matches multiple worktrees:"
      printf '  %s\n' "${matches[@]}" >&2
      return 2
    fi
    (( $#matches == 1 )) && target="${matches[1]}"
  fi
  [[ -n "$target" ]] || return 1
  print -r -- "$target"
}

# Local + remote branch short-names (remote prefix stripped, */HEAD dropped, de-duped).
_nt_all_branches() {
  command git for-each-ref --format='%(refname)' refs/heads refs/remotes 2>/dev/null | awk '
    /\/HEAD$/ { next }
    { sub("^refs/heads/", ""); sub("^refs/remotes/[^/]+/", ""); print }' | sort -u
}

# --- nt() --------------------------------------------------------------------
# nt <branch> [base]   create/switch to a worktree and cd into it
# nt cd  [branch]      cd to an existing worktree (fzf picker if branch omitted)
# nt rm  [-f] [branch] remove a worktree (fzf picker if branch omitted)
# nt ls                list this repo's worktrees   (nt -h for help)
#
# Worktrees live next to the main checkout in <repo>.worktrees/<branch>.
#   nt fix-login        new branch 'fix-login' from latest origin/main
#   nt team/issue-123-x   worktree tracking origin/team/issue-123-x (latest)
#   nt spike main~3     new branch 'spike' forked from base 'main~3'
# Re-running create for a branch that already has a worktree just cd's you there.
# Set NT_NO_FETCH=1 to skip the network fetch (offline / speed).
nt() {
  emulate -L zsh

  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    print -u2 "nt: not inside a git repository"
    return 1
  fi

  # Anchor to the MAIN checkout (first 'worktree' line) so trees land in one place.
  local wtlist main_dir repo_name root
  wtlist="$(git worktree list --porcelain)"
  main_dir="$(awk '/^worktree /{print substr($0,10); exit}' <<<"$wtlist")"
  repo_name="${main_dir:t}"
  root="${main_dir:h}/${repo_name}.worktrees"

  case "${1:-}" in
    -h|--help|help)
      print "usage:"
      print "  nt <branch> [base]    create/switch to a worktree (new branch from latest origin/main)"
      print "  nt cd  [branch]       cd to a worktree (fzf picker if branch omitted)"
      print "  nt rm  [-f] [branch]  remove a worktree (fzf picker if branch omitted)"
      print "  nt ls                 list this repo's worktrees"
      return 0
      ;;
    "")
      print "nt <branch> | nt cd [branch] | nt rm [branch] | nt ls   (nt -h for help)"
      git worktree list
      return 0
      ;;
    ls|list)
      git worktree list
      return 0
      ;;
    cd)
      shift
      local target
      if [[ -n "${1:-}" ]]; then
        target="$(awk -v b="refs/heads/$1" '/^worktree /{p=substr($0,10)} $0=="branch "b{print p; exit}' <<<"$wtlist")"
        [[ -z "$target" ]] && { print -u2 "nt: no worktree for branch '$1'"; return 1; }
      else
        target="$(git worktree list | fzf --height=40% --reverse --prompt='cd worktree> ')" || return
        target="${target%% *}"
      fi
      [[ -n "$target" ]] && cd "$target" && print "nt: → $target"
      return
      ;;
    rm|remove)
      shift
      local force=""
      if [[ "${1:-}" == "-f" || "${1:-}" == "--force" ]]; then force="--force"; shift; fi
      local target rcr
      if [[ -n "${1:-}" ]]; then
        target="$(_nt_resolve_wt "$1" "$wtlist")"; rcr=$?
        (( rcr == 2 )) && return 1            # ambiguous; candidates already listed
        if (( rcr != 0 )) || [[ -z "$target" ]]; then
          print -u2 "nt: no worktree for branch or path '$1'"; return 1
        fi
      else
        local choice
        choice="$(_nt_wt_targets | fzf --height=40% --reverse --prompt='remove worktree> ')" || return
        [[ -z "$choice" ]] && return
        target="$(_nt_resolve_wt "$choice" "$wtlist")" || return
      fi
      [[ -z "$target" ]] && return
      if [[ "$target" == "$main_dir" ]]; then
        print -u2 "nt: refusing to remove the main checkout"
        return 1
      fi
      # If we're standing inside the tree we're removing, step out to main first.
      case "$PWD/" in "$target"/*) cd "$main_dir";; esac
      git worktree remove $force "$target" && print "nt: removed $target"
      return
      ;;
  esac

  # --- default: create or switch to a worktree for branch $1 (base = $2) -------
  local branch="$1" base="${2:-}"

  # Already have a worktree for this branch? Just jump to it.
  local existing
  existing="$(awk -v b="refs/heads/$branch" '/^worktree /{p=substr($0,10)} $0=="branch "b{print p; exit}' <<<"$wtlist")"
  if [[ -n "$existing" ]]; then
    print "nt: worktree for '$branch' already exists"
    cd "$existing" && print "nt: → $existing"
    return
  fi

  # Resolve remote + default branch, fetch latest refs.
  local remote="origin"
  git remote get-url "$remote" >/dev/null 2>&1 || remote=""
  if [[ -n "$remote" && -z "${NT_NO_FETCH:-}" ]]; then
    print "nt: fetching $remote ..."
    git fetch --quiet "$remote" || print -u2 "nt: warning: fetch failed, using cached refs"
  fi

  local default_branch=""
  if [[ -n "$remote" ]]; then
    default_branch="$(git symbolic-ref --quiet "refs/remotes/$remote/HEAD" 2>/dev/null)"
    default_branch="${default_branch##*/}"
  fi
  if [[ -z "$default_branch" ]]; then
    if   [[ -n "$remote" ]] && git show-ref --quiet --verify "refs/remotes/$remote/main";   then default_branch="main"
    elif [[ -n "$remote" ]] && git show-ref --quiet --verify "refs/remotes/$remote/master"; then default_branch="master"
    else default_branch="main"
    fi
  fi

  # Build target path (branch may contain '/', e.g. team/issue-123-...).
  local dest="$root/$branch"
  if [[ -e "$dest" ]]; then
    print -u2 "nt: target path already exists: $dest"
    return 1
  fi
  mkdir -p "${dest:h}" || return 1

  # Create the worktree, choosing how based on where the branch lives.
  local rc
  if git show-ref --quiet --verify "refs/heads/$branch"; then
    # Local branch exists. Fast-forward to origin if it's a clean FF (gets latest
    # without ever clobbering local commits).
    if [[ -n "$remote" ]] && git show-ref --quiet --verify "refs/remotes/$remote/$branch"; then
      if git merge-base --is-ancestor "refs/heads/$branch" "refs/remotes/$remote/$branch" 2>/dev/null; then
        git branch -f "$branch" "$remote/$branch" 2>/dev/null \
          || print -u2 "nt: note: couldn't fast-forward local '$branch'"
      else
        print -u2 "nt: note: local '$branch' diverged from $remote/$branch; using local copy"
      fi
    fi
    print "nt: + worktree on existing branch '$branch'"
    git worktree add "$dest" "$branch"; rc=$?
  elif [[ -n "$remote" ]] && git show-ref --quiet --verify "refs/remotes/$remote/$branch"; then
    print "nt: + worktree tracking $remote/$branch (latest)"
    git worktree add --track -b "$branch" "$dest" "$remote/$branch"; rc=$?
  else
    local want="${base:-$default_branch}" start
    if   [[ -n "$remote" ]] && git show-ref --quiet --verify "refs/remotes/$remote/$want"; then start="$remote/$want"
    elif git show-ref --quiet --verify "refs/heads/$want";                                  then start="$want"
    elif git rev-parse --quiet --verify "$want^{commit}" >/dev/null 2>&1;                    then start="$want"
    else
      print -u2 "nt: base '$want' not found on $remote or locally"
      rmdir "${dest:h}" 2>/dev/null
      return 1
    fi
    print "nt: + new branch '$branch' from $start"
    git worktree add --no-track -b "$branch" "$dest" "$start"; rc=$?
  fi

  if (( rc != 0 )); then
    print -u2 "nt: git worktree add failed"
    rmdir "$dest" 2>/dev/null
    return 1
  fi

  cd "$dest" && print "nt: → $dest  (branch: $branch)   run 'cc' to start Claude"
}
