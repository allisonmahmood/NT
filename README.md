# nt — git worktree quick-switch

A tiny zsh command for spinning up git worktrees without ceremony. Run it in any
repo: it creates (or reuses) a worktree, `cd`s you into it, and you carry on —
e.g. `nt fix-login` then `cc`.

Worktrees live next to the main checkout in **`<repo>.worktrees/<branch>`**:

```
~/Developer/
  example-repo/                 <- main checkout
  example-repo.worktrees/
    fix-login/                  <- nt fix-login
    team/issue-123-.../         <- nt team/issue-123-...
```

## Install

```sh
# clone is already at ~/Developer/nt
echo 'source ~/Developer/nt/nt.plugin.zsh' >> ~/.zshrc
exec zsh        # or open a new tab
```

That single `source` line defines the `nt` command **and** wires up tab
completion (it adds `completions/` to `fpath` and registers `_nt`, even when
compinit has already run — as it has under oh-my-zsh).

## Usage

| Command | What it does |
|---|---|
| `nt <branch> [base]` | create/switch to a worktree and `cd` in |
| `nt cd [branch]` | `cd` to an existing worktree (fzf picker if no branch) |
| `nt rm [-f] [target]` | remove a worktree (fzf picker if no target) |
| `nt ls` | list this repo's worktrees |
| `nt` / `nt -h` | list + hint / full usage |

The argument to the create form is *the branch you want to be on*:

- **new name** → new branch from the latest `origin/main` (fetches first)
- **exists on origin** (e.g. `nt team/issue-123-foo`) → worktree tracking
  `origin/team/issue-123-foo` at its latest
- **already has a worktree** → just `cd`s you there
- `nt <name> <base>` → fork the new branch off an explicit base

### Tab completion

- `nt <tab>` → subcommands (`cd`/`rm`/`ls`) plus existing branch names
- `nt cd <tab>` → branches that currently have a worktree
- `nt rm <tab>` → every worktree except the main checkout, and `-f`. Branch-backed
  worktrees show by branch name; branch-less (detached) worktrees — e.g. ones
  created by other tools — show by full path so they're reachable too.
- `nt <branch> <tab>` → branches, to pick a base

## Notes & knobs

- New branches are created `--no-track`, so a bare `git push`/`git pull` won't
  accidentally target `main`. Push with `-u` when ready.
- `NT_NO_FETCH=1 nt foo` skips the network fetch (offline / speed).
- A local branch is only ever fast-forwarded to origin when it's a clean FF —
  diverged local commits are never clobbered.
- `nt rm <target>` takes a branch name, a full worktree path, or a unique path
  tail (e.g. the last path component); an ambiguous tail is refused, listing the
  matches. With no target it opens an fzf picker over every worktree but the main
  checkout. Either way it refuses the main checkout, and if you remove the
  worktree you're standing in, it steps you back to the main checkout first.
- Worktree location is one line in `nt.plugin.zsh`:
  `root="${main_dir:h}/${repo_name}.worktrees"`.

## Tests

```sh
zsh test/run.zsh
```

- `test/test_nt.zsh` — end-to-end behaviour against throwaway repos
- `test/test_completion.zsh` — completion registration + candidate generation
