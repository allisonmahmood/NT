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

# Local branches whose upstream is gone (deleted on the remote). One per line.
# These are the branches `nt prune` offers to delete. Needs an up-to-date prune
# (`git fetch -p`) to be accurate — git only marks an upstream "gone" once it's
# noticed the remote ref vanished.
_nt_gone_branches() {
  command git for-each-ref --format='%(refname:short) %(upstream:track)' refs/heads 2>/dev/null \
    | awk '$2=="[gone]"{print $1}'
}

# Force-delete the named local branches, reporting each (with its old sha so it's
# recoverable from the reflog). Shared by every `nt prune` branch-cleanup path.
_nt_delete_branches() {
  emulate -L zsh
  local b sha
  for b in "$@"; do
    [[ -z "$b" ]] && continue
    sha="$(command git rev-parse --short "$b" 2>/dev/null)"
    if command git branch -D "$b" >/dev/null 2>&1; then
      print "nt: deleted branch $b  (was $sha — recover via git reflog)"
    else
      print -u2 "nt: could not delete branch $b (checked out in a worktree?)"
    fi
  done
}

# True (0) when fzf is available; otherwise prints a hint and returns 1. Gates
# every interactive picker so a missing fzf yields a clear message instead of a
# raw "command not found" and an empty selection.
_nt_need_fzf() {
  command -v fzf >/dev/null 2>&1 && return 0
  print -u2 "nt: fzf required for the interactive picker (pass a branch/target to skip it)"
  return 1
}

# Human description of where a brand-new branch is forked from, for help text —
# e.g. "latest origin/main", or "main (no remote)" when there's no remote. Pure
# read: resolves the remote's default branch without fetching.
_nt_base_desc() {
  emulate -L zsh
  local remote="origin" def=""
  command git remote get-url "$remote" >/dev/null 2>&1 || remote=""
  if [[ -n "$remote" ]]; then
    def="$(command git symbolic-ref --quiet "refs/remotes/$remote/HEAD" 2>/dev/null)"; def="${def##*/}"
  fi
  if [[ -z "$def" ]]; then
    if   [[ -n "$remote" ]] && command git show-ref --quiet --verify "refs/remotes/$remote/main";   then def="main"
    elif [[ -n "$remote" ]] && command git show-ref --quiet --verify "refs/remotes/$remote/master"; then def="master"
    else def="main"
    fi
  fi
  if [[ -n "$remote" ]]; then print -r -- "latest $remote/$def"
  else                        print -r -- "$def (no remote)"
  fi
}

# Pretty `nt ls`: one row per worktree with a dirty marker, ahead/behind vs
# upstream, and the path (the main checkout tagged). Columns are aligned.
#   $1 = output of `git worktree list --porcelain`   $2 = main checkout path
#
# Speed: ahead/behind for every branch comes from ONE `git for-each-ref`, and the
# per-worktree `git status` dirty checks (the slow part) are fanned out in
# parallel — so a tree of many worktrees lists in ~one `git status`, not N of them.
_nt_ls() {
  emulate -L zsh
  local wtlist="$1" main_dir="$2" tab=$'\t'
  # Up/down glyphs: arrows under a UTF-8 locale, ASCII fallback elsewhere so the
  # columns stay aligned (and render) on a C/POSIX locale or a sparse terminal.
  local up dn lc="${LC_ALL:-${LC_CTYPE:-${LANG:-}}}"
  if [[ "${lc:l}" == *utf*8* ]]; then up=$'↑'; dn=$'↓'; else up='^'; dn='v'; fi

  # path<TAB>branch per worktree, preserving porcelain order; a branch-less
  # (detached) worktree has an EMPTY branch field (we don't use an in-band
  # sentinel — "(detached)" is a legal branch name, so an empty field is the
  # unambiguous signal).
  local -a Pp Bb
  local line p b
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    Pp+=("${line%%$tab*}"); Bb+=("${line#*$tab}")
  done <<<"$(awk '
    /^worktree /{ if (p!="") print p"\t"b; p=substr($0,10); b="" }
    /^branch /  { b=$2; sub("refs/heads/","",b) }
    END         { if (p!="") print p"\t"b }
  ' <<<"$wtlist")"
  (( ${#Pp} )) || return 0

  # Ahead/behind for ALL local branches in a single call (no per-worktree
  # rev-list): %(upstream) tells us whether an upstream exists, %(upstream:track)
  # gives "[ahead N, behind M]" / "[gone]" / "" (in sync).
  local -A UPS TRK
  local bn us tk
  while IFS=$tab read -r bn us tk; do
    [[ -z "$bn" ]] && continue
    [[ -n "$us" ]] && UPS[$bn]=1
    TRK[$bn]="$tk"
  done < <(command git for-each-ref --format="%(refname:short)$tab%(upstream)$tab%(upstream:track)" refs/heads 2>/dev/null)

  # Dirty flags, computed in parallel — `git status` per worktree is the slow
  # part, so fan out across worktrees (bounded) and collect a path->dirty map.
  local -A DIRTY
  local dp
  while IFS=$tab read -r dp _; do
    [[ -n "$dp" ]] && DIRTY[$dp]=1
  done < <(print -rl -- "${Pp[@]}" | xargs -P 16 -I {} sh -c \
            '[ -n "$(git -C "$1" status --porcelain 2>/dev/null)" ] && printf "%s\t*\n" "$1"' _ {})

  # Build rows (compute widths), then print aligned.
  local -a Mm Bd Aa Tt
  local i disp marker ab tk2 ahd beh tag
  local maxb=1 maxa=1
  for (( i = 1; i <= ${#Pp}; i++ )); do
    p="${Pp[i]}"; b="${Bb[i]}"; disp="${b:-(detached)}"
    marker=" "; ab="-"; tag=""
    if [[ ! -d "$p" ]]; then
      marker="?"; ab="?"   # path missing on disk (run `nt prune`)
    else
      [[ -n "${DIRTY[$p]:-}" ]] && marker="*"
      if [[ -n "$b" ]]; then
        tk2="${TRK[$b]:-}"
        if   [[ "$tk2" == *gone* ]];   then ab="gone"          # upstream deleted on remote
        elif [[ -n "${UPS[$b]:-}" ]]; then
          if [[ -z "$tk2" ]]; then ab="="
          else
            ahd=0; beh=0
            [[ "$tk2" == *ahead\ * ]]  && { ahd="${tk2#*ahead }";  ahd="${ahd%%[^0-9]*}"; }
            [[ "$tk2" == *behind\ * ]] && { beh="${tk2#*behind }"; beh="${beh%%[^0-9]*}"; }
            if   (( ahd > 0 && beh > 0 )); then ab="${up}${ahd}${dn}${beh}"
            elif (( ahd > 0 ));           then ab="${up}${ahd}"
            elif (( beh > 0 ));           then ab="${dn}${beh}"
            else                               ab="="
            fi
          fi
        fi
      fi
    fi
    [[ "$p" == "$main_dir" ]] && tag="  (main)"
    Mm+=("$marker"); Bd+=("$disp"); Aa+=("$ab"); Tt+=("$tag")
    (( ${#disp} > maxb )) && maxb=${#disp}
    (( ${#ab}   > maxa )) && maxa=${#ab}
  done

  for (( i = 1; i <= ${#Pp}; i++ )); do
    printf '%s %-*s  %-*s  %s%s\n' "${Mm[i]}" "$maxb" "${Bd[i]}" "$maxa" "${Aa[i]}" "${Pp[i]}" "${Tt[i]}"
  done
  print -- "* = uncommitted changes"
}

# --- nt() --------------------------------------------------------------------
# nt <branch> [base]    create/switch to a worktree and cd into it
# nt cd   [branch]      cd to an existing worktree (fzf picker if branch omitted)
# nt rm   [-f] [target] remove worktree(s) (fzf multi-picker if target omitted)
# nt done [-f] [target] remove a worktree AND delete its local branch
# nt prune              prune stale worktrees + empty dirs; offer to delete gone branches
# nt home               cd back to the main checkout
# nt ls                 list this repo's worktrees, with dirty/ahead-behind   (nt -h for help)
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
      print "  nt <branch> [base]    create/switch to a worktree (new branch from $(_nt_base_desc))"
      print "  nt cd   [branch]      cd to a worktree (fzf picker if branch omitted)"
      print "  nt rm   [-f] [target] remove worktree(s) (fzf multi-picker if target omitted)"
      print "  nt done [-f] [target] remove a worktree AND delete its local branch"
      print "  nt prune              prune stale worktrees + empty dirs; offer to delete gone branches"
      print "  nt home               cd back to the main checkout"
      print "  nt ls                 list this repo's worktrees, with dirty/ahead-behind"
      return 0
      ;;
    "")
      print "nt <branch> | nt cd | nt rm | nt done | nt prune | nt home | nt ls   (nt -h for help)"
      _nt_ls "$wtlist" "$main_dir"
      return 0
      ;;
    home)
      cd "$main_dir" && print "nt: → $main_dir  (main checkout)"
      return
      ;;
    ls|list)
      _nt_ls "$wtlist" "$main_dir"
      return 0
      ;;
    cd)
      shift
      local target
      if [[ -n "${1:-}" ]]; then
        target="$(awk -v b="refs/heads/$1" '/^worktree /{p=substr($0,10)} $0=="branch "b{print p; exit}' <<<"$wtlist")"
        [[ -z "$target" ]] && { print -u2 "nt: no worktree for branch '$1'"; return 1; }
      else
        _nt_need_fzf || return 1
        # Feed fzf full paths (one per line) so a worktree path with spaces in it
        # survives — the picked line IS the path, no field-splitting needed.
        target="$(awk '/^worktree /{print substr($0,10)}' <<<"$wtlist" | fzf --height=40% --reverse --prompt='cd worktree> ')" || return
      fi
      [[ -n "$target" ]] && cd "$target" && print "nt: → $target"
      return
      ;;
    rm|remove)
      shift
      # -f/--force may sit anywhere among the targets, so `nt rm a b -f` reads as
      # naturally as `nt rm -f a b`; everything else is a worktree identifier.
      local force="" arg
      local -a rest
      for arg in "$@"; do
        case "$arg" in
          -f|--force) force="--force" ;;
          *)          rest+=("$arg") ;;
        esac
      done
      set -- "${rest[@]}"
      # Collect the worktree path(s) to remove: explicit target(s), or — when none
      # are given — an fzf multi-picker (space to mark several, like `nt prune`).
      local -a targets
      if [[ -n "${1:-}" ]]; then
        # Resolve every explicit target up front, and refuse the main checkout
        # here too — so an unknown/ambiguous identifier (or a stray `main`) bails
        # before anything is removed: a typo never half-finishes the batch.
        local t rcr
        for arg in "$@"; do
          t="$(_nt_resolve_wt "$arg" "$wtlist")"; rcr=$?
          (( rcr == 2 )) && return 1            # ambiguous; candidates already listed
          if (( rcr != 0 )) || [[ -z "$t" ]]; then
            print -u2 "nt: no worktree for branch or path '$arg'"; return 1
          fi
          if [[ "$t" == "$main_dir" ]]; then
            print -u2 "nt: refusing to remove the main checkout"; return 1
          fi
          targets+=("$t")
        done
      else
        _nt_need_fzf || return 1
        local picks choice t
        picks="$(_nt_wt_targets | fzf --multi --height=40% --reverse \
                   --bind 'space:toggle' \
                   --header='space=mark · enter=remove (no marks = highlighted row)' \
                   --prompt='remove worktree> ')" || return
        [[ -z "$picks" ]] && return
        for choice in ${(f)picks}; do
          [[ -z "$choice" ]] && continue
          t="$(_nt_resolve_wt "$choice" "$wtlist")" || continue
          [[ -n "$t" ]] && targets+=("$t")
        done
      fi
      targets=(${(u)targets})                  # de-dupe, preserve order
      (( ${#targets} )) || return 0            # nothing resolved (picker race) -> clean no-op
      # Remove each. The main checkout is refused above for explicit targets and
      # never offered by the picker, but keep the guard as a backstop. If we're
      # standing inside a tree being removed, step out to main first.
      local target rc=0
      for target in "${targets[@]}"; do
        if [[ "$target" == "$main_dir" ]]; then
          print -u2 "nt: refusing to remove the main checkout"
          rc=1; continue
        fi
        case "$PWD/" in "$target"/*) cd "$main_dir";; esac
        if git worktree remove $force "$target"; then
          print "nt: removed $target"
        else
          rc=1
        fi
      done
      return $rc
      ;;
    done|finish)
      shift
      local dforce=""
      if [[ "${1:-}" == "-f" || "${1:-}" == "--force" ]]; then dforce=1; shift; fi
      local target choice doneb rcr delflag
      if [[ -n "${1:-}" ]]; then
        target="$(_nt_resolve_wt "$1" "$wtlist")"; rcr=$?
        (( rcr == 2 )) && return 1            # ambiguous; candidates already listed
        if (( rcr != 0 )) || [[ -z "$target" ]]; then
          print -u2 "nt: no worktree for branch or path '$1'"; return 1
        fi
      else
        _nt_need_fzf || return 1
        choice="$(_nt_wt_targets | fzf --height=40% --reverse --prompt='done (remove + delete branch)> ')" || return
        [[ -z "$choice" ]] && return
        target="$(_nt_resolve_wt "$choice" "$wtlist")" || return
      fi
      [[ -z "$target" ]] && return
      if [[ "$target" == "$main_dir" ]]; then
        print -u2 "nt: refusing to remove the main checkout"
        return 1
      fi
      # Branch backing this worktree (empty for a detached/branch-less one).
      doneb="$(awk -v t="$target" '
        /^worktree /{ p=substr($0,10) }
        p==t && /^branch /{ b=$2; sub("refs/heads/","",b); print b; exit }
      ' <<<"$wtlist")"
      # If we're standing inside the tree, step out to main before removing it.
      case "$PWD/" in "$target"/*) cd "$main_dir";; esac
      git worktree remove ${dforce:+--force} "$target" || return 1
      print "nt: removed $target"
      if [[ -z "$doneb" ]]; then
        print "nt: (detached worktree — no branch to delete)"
        return 0
      fi
      delflag="-d"; [[ -n "$dforce" ]] && delflag="-D"
      local donesha; donesha="$(git rev-parse --short "$doneb" 2>/dev/null)"
      if git branch $delflag "$doneb" >/dev/null 2>&1; then
        print "nt: deleted branch $doneb  (was $donesha — recover via git reflog)"
      else
        print -u2 "nt: branch '$doneb' not fully merged — kept. Use 'nt done -f $doneb' or 'git branch -D $doneb'."
        return 1
      fi
      return 0
      ;;
    prune|clean)
      shift
      # 1) Drop stale worktree admin entries (dirs that vanished from disk).
      git worktree prune
      # 2) Remove the now-empty dirs under the worktrees root (git's own prune
      #    leaves empty 'team/'-style parents behind from nested branch names) —
      #    but NEVER descend into a live worktree's working tree, so empty dirs
      #    you keep inside a checked-out worktree are left untouched.
      local removed=0 d w
      if [[ -d "$root" ]]; then
        local -a live prune_expr cand
        live=(${(f)"$(git worktree list --porcelain | awk '/^worktree /{print substr($0,10)}')"})
        for w in "${live[@]}"; do prune_expr+=(-path "$w" -prune -o); done
        # Candidate dirs under root, skipping live worktrees and their contents.
        cand=(${(f)"$(find "$root" "${prune_expr[@]}" -type d ! -path "$root" -print 2>/dev/null)"})
        # rmdir empties, repeating so nested parents collapse once their children go.
        local changed=1
        while (( changed )); do
          changed=0
          for d in "${cand[@]}"; do
            [[ -n "$d" && -d "$d" ]] && rmdir "$d" 2>/dev/null && { (( ++removed )); changed=1 }
          done
        done
        rmdir "$root" 2>/dev/null && (( ++removed ))   # the root itself, if now empty
      fi
      print "nt: pruned stale worktree entries; removed $removed empty dir(s)"
      # 3) Offer to delete local branches whose upstream is gone.
      local -a gone todelete
      gone=(${(f)"$(_nt_gone_branches)"}); gone=(${gone:#})
      if (( ${#gone} == 0 )); then
        print "nt: no gone-upstream branches (run 'git fetch -p' first if you expected some)"
        return 0
      fi
      # No terminal to drive the picker/prompt (script, CI): list, don't delete.
      if [[ ! -t 0 ]]; then
        print "nt: local branches with a gone upstream (run 'nt prune' interactively to delete; 'git fetch -p' to refresh):"
        printf '  %s\n' "${gone[@]}"
        return 0
      fi
      if command -v fzf >/dev/null 2>&1; then
        local picks
        picks="$(
          { print -- "[ALL] delete every gone-upstream branch below"
            printf '%s\n' "${gone[@]}"
          } | fzf --multi --height=40% --reverse \
                  --bind 'space:toggle' \
                  --header='space=mark · enter=delete (no marks = highlighted row; pick [ALL] to delete all)' \
                  --prompt='prune gone branches> '
        )" || { print "nt: prune: kept all branches"; return 0; }
        [[ -z "$picks" ]] && { print "nt: prune: kept all branches"; return 0; }
        if [[ "$picks" == *'[ALL]'* ]]; then todelete=("${gone[@]}"); else todelete=(${(f)picks}); fi
      else
        print "nt: local branches with a gone upstream:"
        printf '  %s\n' "${gone[@]}"
        printf 'nt: delete all of them? [y/N] '
        local ans; read -r ans
        [[ "$ans" == [yY]* ]] || { print "nt: prune: kept all branches"; return 0; }
        todelete=("${gone[@]}")
      fi
      todelete=(${todelete:#\[ALL\]*})   # drop the sentinel row if it slipped in
      _nt_delete_branches "${todelete[@]}"
      return 0
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
