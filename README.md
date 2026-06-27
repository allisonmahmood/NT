# nt — navigate tree

Git worktrees, minus the ceremony. `nt` (short for **navigate tree**) is a tiny
zsh command for hopping around worktrees: it spins one up — or jumps to it if it
already exists — `cd`s you in, and gets out of your way.

```sh
nt fix-login   # worktree up, cd'd in, go
```

Honestly? You should just ask your agent to build this for you — it's a 20-minute
job. But if you're too scared to let it loose on your dotfiles, here you go.

## Where the trees live

Worktrees sit right next to the main checkout in **`<repo>.worktrees/<branch>`** —
no scattering them across `/tmp`, no losing track of where they went:

```
~/Developer/
  acme/                       <- the main checkout
  acme.worktrees/
    fix-login/                <- nt fix-login
    team/issue-123-thing/     <- nt team/issue-123-thing
```

## Install

```sh
# clone is already at ~/Developer/nt
echo 'source ~/Developer/nt/nt.plugin.zsh' >> ~/.zshrc
exec zsh        # or open a new tab
```

That one `source` line defines the `nt` command **and** wires up tab completion
(it adds `completions/` to `fpath` and registers `_nt`, even when compinit has
already run — as it has under oh-my-zsh). One line. Done.

## Usage

| Command | What it does |
|---|---|
| `nt <branch> [base]` | spin up / jump to a worktree and `cd` in |
| `nt cd [branch]` | `cd` to an existing worktree (fzf picker if no branch) |
| `nt rm [-f] [target...]` | nuke worktree(s) (fzf multi-picker if no target) |
| `nt done [-f] [target]` | nuke a worktree **and** delete its local branch |
| `nt prune` | tidy up: drop stale worktrees + empty dirs, offer to delete gone branches |
| `nt home` | `cd` back to the main checkout, wherever you are |
| `nt ls` | list this repo's worktrees, with dirty + ahead/behind |
| `nt` / `nt -h` | list + hint / full usage |

### Point it at a branch

The argument is *the branch you want to be on*, and `nt` figures out the rest —
always pulling the latest from origin so you're never stranded on some stale
local copy:

- **new name** → new branch off the latest `origin/main` (it fetches first)
- **exists on origin** (e.g. `nt team/issue-123-foo`) → worktree tracking
  `origin/team/issue-123-foo` at its latest
- **exists locally** → checks the branch out in a fresh worktree, fast-forwarded
  to origin when that's a clean FF. Diverged local commits? Kept, never clobbered —
  it just tells you and uses your copy.
- **already has a worktree** → skips the theatrics and `cd`s you there
- `nt <name> <base>` → fork the new branch off an explicit base instead

(The `-h` text says exactly where new branches fork from — `latest origin/main`,
or your actual remote/default, or `main (no remote)` — so it never lies to you.)

### Housekeeping

When you're done with a branch, or the worktree dir has filled up with stuff
you've long since merged:

- **`nt rm [target...]`** → remove worktree(s), **fast**. Deleting a worktree is
  mostly the cost of `rm`-ing its working tree (a fat `node_modules` can take
  seconds *each*), so `nt rm` renames each tree aside instantly and reclaims the
  disk in the background — it returns at once even when you nuke ten heavy trees.
  Same safety as `git worktree remove`: it refuses a dirty or locked worktree (and
  anything git can't vouch for) unless you pass `-f`. No target → fzf multi-picker.
- **`nt done [target]`** → remove the worktree **and** delete its local branch in
  one move. Safe by default (`git branch -d` refuses an unmerged branch and keeps
  it, telling you so); `nt done -f` forces both the removal and the delete. No
  target → fzf picker. Detached worktree? It just removes it (no branch to delete).
- **`nt prune`** → `git worktree prune` to clear stale entries, sweep up the empty
  `team/`-style parent dirs git leaves behind, then — if any local branches have a
  **gone upstream** (merged & deleted on origin) — pop an fzf picker to clean them:
  **enter** on the `[ALL]` row deletes them all, or **space** to mark just the ones
  you want. Run `git fetch -p` first so "gone" is accurate. In a non-interactive
  shell it only *lists* the gone branches — it never deletes without you there.

### A nicer `nt ls`

`nt ls` (and bare `nt`) show a dirty marker and ahead/behind vs upstream:

```
  main             =      ~/Developer/acme   (main)
* fix-login        ↑2     ~/Developer/acme.worktrees/fix-login
  team/issue-123   ↓1     ~/Developer/acme.worktrees/team/issue-123

* = uncommitted changes
```

(The main checkout always lists first, and the branch column shows the full
short name — `team/issue-123`, not just the leaf.)

`*` = uncommitted changes, `↑n`/`↓n` = ahead/behind the upstream, `=` = in sync,
`-` = no upstream, `?` = the dir is gone (a `nt prune` candidate). In a non-UTF-8
locale the arrows degrade to `^n`/`vn` so the columns still line up.

### Tab completion

- `nt <tab>` → subcommands (`cd` / `rm` / `done` / `prune` / `home` / `ls`) plus existing branch names
- `nt cd <tab>` → branches that currently have a worktree
- `nt rm <tab>` / `nt done <tab>` → every worktree except the main checkout, plus `-f`.
  Branch-backed worktrees show by branch name; branch-less (detached) ones — say,
  created by some other tool — show by full path so they're still reachable.
- `nt <branch> <tab>` → branches, to pick a base

## Notes & knobs

- New branches are created `--no-track`, so a stray `git push`/`git pull` won't
  accidentally nuke `main`. Push with `-u` when you're ready.
- `NT_NO_FETCH=1 nt foo` skips the network fetch (offline, or just impatient).
- A local branch is only ever fast-forwarded to origin when it's a clean FF —
  your diverged local commits are never touched.
- `nt rm [target...]` (and `nt done <target>`) take a branch name, a full worktree
  path, or a unique path tail (the last path component, usually). `nt rm` takes
  several at once (`nt rm fix-login spike`, and `-f` may go anywhere) — it resolves
  the whole list up front and removes **nothing** unless *every* target checks out:
  an ambiguous tail is refused with the matches listed, and naming the main checkout
  aborts the batch too. No target → an fzf **multi-picker**: **space** to mark each
  worktree you want gone, **enter** to remove them all (no marks = just the
  highlighted row), over every worktree but the main checkout. Either way it flat-out
  refuses to remove the main checkout, and if you `rm`/`done` the worktree you're
  standing in, it steps you back home first.
- `nt rm` frees the disk **asynchronously**: it renames each tree to a hidden
  `.nt-trash-*` dir next to it and `rm`s that in a disowned background job, so the
  command returns instantly. `nt prune` reaps any `.nt-trash-*` a killed delete left
  behind (handy after a reboot mid-delete). One caveat: that reaper only sweeps under
  `<repo>.worktrees/`, so trash left beside a worktree you keep *elsewhere* (e.g. one
  another tool created outside that dir) won't be auto-reaped — `find ~ -name '.nt-trash-*'`
  clears it. The fast path only handles worktrees `nt` can confirm are simple (clean,
  unlocked, no submodule); a dirty, **locked**, or submodule-containing worktree is
  handed to `git worktree remove` itself, so it behaves exactly as git does — refused
  without `-f`, and a lock still needs an explicit `git worktree unlock` (git won't drop
  a lock on a single `-f`).
- The interactive pickers (`nt cd`/`nt rm`/`nt done` with no argument, and the
  `nt prune` branch cleanup) need [`fzf`](https://github.com/junegunn/fzf). Without
  it you get a one-line "fzf required …" nudge instead of a cryptic error — just
  pass an explicit branch/target and everything else works fzf-free.
- Worktree location is one line in `nt.plugin.zsh` — change it if you hate it:
  `root="${main_dir:h}/${repo_name}.worktrees"`.

## Tests

```sh
zsh test/run.zsh
```

- `test/test_nt.zsh` — end-to-end behaviour against throwaway repos
- `test/test_completion.zsh` — completion registration + candidate generation

MIT.
