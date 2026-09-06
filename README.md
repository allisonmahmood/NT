# nt — navigate tree

Git worktrees, minus the ceremony. `nt` (short for **navigate tree**) is a tiny
cross-shell command for hopping around worktrees: it spins one up — or jumps to
it if it already exists — `cd`s you in, and gets out of your way.

```sh
nt fix-login   # worktree up, cd'd in, go
```

A single Go binary plus a one-line shell hook. Works in **zsh, bash, and fish**.

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

**[v0.1.1](https://github.com/allisonmahmood/NT/releases/tag/v0.1.1)** is available
for Linux x86-64 and ARM64. Install a binary or local Arch package, then add your
shell hook below.

### Go toolchain

With Go 1.25 or newer:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/allisonmahmood/nt@v0.1.1
go version -m "$HOME/.local/bin/nt" | grep 'mod.*github.com/allisonmahmood/nt.*v0.1.1'
```

To build the current development source instead:

```sh
mkdir -p "$HOME/.local/bin" && git clone https://github.com/allisonmahmood/NT nt && cd nt && go build -o "$HOME/.local/bin/nt" .
```

Go-toolchain builds report `nt version dev`; `go version -m` verifies the tagged
module version. GitHub build attestations cover release archives, not local
builds.

### Arch Linux

The checked-in [`nt`](packaging/arch/nt/PKGBUILD) package builds the tagged
source, while [`nt-bin`](packaging/arch/nt-bin/PKGBUILD) installs the prebuilt
Linux archive and verifies it against the release's `checksums.txt`.

```sh
git clone https://github.com/allisonmahmood/NT
cd NT/packaging/arch/nt       # or NT/packaging/arch/nt-bin for the prebuilt archive
makepkg -si
nt --version
```

AUR submission is pending; these repository-hosted packages can be built and
installed locally now.

### Release archive

These commands require an authenticated [`gh`](https://cli.github.com/). Paste
the complete block into bash or zsh. It stops on any failed check without changing
your shell's error-handling settings.

#### Linux

```sh
(
set -euo pipefail
version=v0.1.1
platform="linux_$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')"
archive="nt_${version#v}_${platform}.tar.gz"
mkdir -p "nt-$version-$platform"
cd "nt-$version-$platform"
gh release verify "$version" --repo allisonmahmood/NT
gh release download "$version" --repo allisonmahmood/NT \
  --pattern "$archive" --pattern checksums.txt
gh release verify-asset "$version" "$archive" --repo allisonmahmood/NT
awk -v archive="$archive" '$2 == archive' checksums.txt | sha256sum --check
gh attestation verify "$archive" --repo allisonmahmood/NT \
  --signer-workflow allisonmahmood/NT/.github/workflows/release.yml \
  --source-ref "refs/tags/$version"
gh attestation verify checksums.txt --repo allisonmahmood/NT \
  --signer-workflow allisonmahmood/NT/.github/workflows/release.yml \
  --source-ref "refs/tags/$version"
tar -xzf "$archive"
reported_version="$(./nt --version)"
test "$reported_version" = "nt version ${version#v}"
mkdir -p "$HOME/.local/bin"
install -m 0755 nt "$HOME/.local/bin/nt"
)
```

A v0.1.1 archive must report `nt version 0.1.1`. Release archives support Linux
on x86-64 (`amd64`) and ARM64 (`arm64`). macOS and Windows releases are not
currently supported.

`gh release verify` confirms GitHub's signed, immutable release;
`verify-asset` ties the download to its exact release asset; and the checksum
manifest verifies its bytes. `gh attestation verify` requires provenance from
this repository's release workflow and the selected version tag. Each archive
also has an SPDX SBOM named `$archive.sbom.json`. These checks prove origin and
integrity, not code safety.

macOS restoration is tracked in [#23](https://github.com/allisonmahmood/NT/issues/23).

### Shell integration

Add the matching hook, which keeps the install directory on `PATH` and defines
the `nt` command and tab completion:

```sh
# ~/.zshrc
export PATH="$HOME/.local/bin:$PATH"
autoload -Uz compinit && compinit
eval "$(nt init zsh)"

# ~/.bashrc
export PATH="$HOME/.local/bin:$PATH"
eval "$(nt init bash)"

# ~/.config/fish/config.fish
fish_add_path "$HOME/.local/bin"
nt init fish | source
```

Start a new terminal after saving the hook. If your zsh framework already runs
`compinit`, keep its existing setup and put the hook after it.

### Update or uninstall

For an archive installation, repeat the verified archive instructions with the
new release version. Use a fresh download directory. For a Go installation,
repeat `go install` with the new version. Start a new shell after updating.

For a local Arch package, update this checkout and rerun `makepkg -si` in the
same recipe directory. To uninstall it, run `sudo pacman -R nt` (or `nt-bin`,
whichever you installed), then remove the shell hook as described below.

To uninstall a binary installed from an archive or through Go:

```sh
rm -- "$HOME/.local/bin/nt"
```

Remove the `nt init` hook from your shell configuration, then start a new shell.
Keep the `~/.local/bin` PATH entry if other programs use it. Uninstalling `nt`
leaves your repositories, branches, and worktrees intact; they remain usable
with Git.

### Why a shell hook?

`nt`'s whole job is to **`cd` your shell** into a worktree — and a child process
can't change its parent shell's directory. So `nt` is a binary *plus* a thin shell
function (the [zoxide](https://github.com/ajeetdsouza/zoxide) model). The binary
writes its target directory to a temp file named in `$NT_CD_FILE`; the function
reads it and does the `cd`. Rich output (the `ls` table, status lines) stays on
stdout, and paths with spaces or quotes round-trip as raw bytes — no `eval`, no
quoting games.

## Usage

| Command | What it does |
|---|---|
| `nt <branch> [base]` | spin up / jump to a worktree and `cd` in |
| `nt cd [branch]` | `cd` to an existing worktree (picker if branch omitted) |
| `nt rm [-f] [target...]` | nuke worktree(s) (multi-picker if no target) |
| `nt done [-f] [target]` | nuke a worktree **and** delete its local branch |
| `nt prune` | tidy up: drop stale worktrees + empty dirs, offer to delete gone branches |
| `nt home` | safely refresh the main checkout and `cd` home, even when already there |
| `nt <new-branch> --take` | take uncommitted work into a new worktree at its current base |
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

### A current home, and a place for unfinished work

`nt home` fetches your configured remote and fast-forwards the original checkout
when it is clean and on that remote's default branch. It works from home too;
there is no separate sync command. `nt cd main` is still navigation only. When
`nt done` or `nt rm` removes your current worktree, returning home attempts the
same safe refresh. A blocked refresh never prevents the directory change.

After a successful home refresh, new worktree creation also keeps home current
using the same fetched default-branch commit. NT remembers the branch, remote,
and commit it established there. If you move home to another commit or branch
yourself, automatic maintenance pauses until you run `nt home` successfully.
This intentionally also pauses after a harmless manual pull. The first automatic
attempt leaves an unknown checkout alone and tells you to run `nt home` once.

Edits, staged changes, untracked files, local-only commits, interrupted Git
operations, submodules, and uncertain index states leave home untouched. Updates
are fast-forward only, protect ignored-file collisions, and do not run repository
hooks, autostash, rebase, reset, or switch branches. Fetch failures and
`NT_NO_FETCH=1` leave home as it is, without claiming it is current. New worktrees
can still use cached refs as before.

An explicit `nt home` also reports how many new commits arrived since your last
successful `nt home`, including updates NT already made while you were elsewhere.
Ordinary shell `cd` visits are not tracked. State is local to the repository's
shared Git directory, never committed or pushed.

To disable opportunistic home updates in a repository:

```sh
git config --local nt.autoRefreshHome false
```

Explicit returns (`nt home`, or removal of your current worktree) still attempt
refresh. A clean Git status cannot reveal unsaved editor buffers or a running
demo: use this setting when home needs to stay fixed while agents create work.
NT serializes its own home refreshes and transfers; this does not lock out
external Git commands or editors.

If an investigation at home has become a task, give it a worktree:

```sh
nt checkout-fix --take
```

This transfers staged, unstaged, and ordinary untracked files from your current
worktree, preserving binary files, symlinks, and the staging split. The new branch
starts at the exact commit you were editing, even if the remote has advanced.
Home can then catch up independently if eligible. Without `--take`, creation
continues to leave your unfinished work where it was.

`--take` requires a new branch and destination and cannot be combined with a base.
It refuses interrupted Git operations, submodules, nested repositories/worktrees,
tracked files replaced by directories or special files, intent-to-add, and hidden
index flags. Ignored environment/build files and unsaved
editor buffers stay where they are. Existing local commits stay on the source
branch too; NT never rewinds home to move committed work away.

Every transfer leaves a named `nt-take-…` recovery stash, including on success.
The command prints its name before capturing work and its commit after success.
This avoids deleting another worktree's stash during concurrent Git use. Inspect
`git stash list` and drop the named entry yourself once you no longer need it.
If interrupted, inspect both directories and that stash before retrying. In a
clean checkout at the original base, `git stash apply --index <printed-commit>`
restores the captured work; a failed destination apply leaves its partial work
and the recovery stash intact and does not refresh home.

### Housekeeping

- **`nt rm [target...]`** → remove worktree(s), **fast**. Deleting a worktree is
  mostly the cost of `rm`-ing its working tree (a fat `node_modules` can take
  seconds *each*), so `nt rm` renames each tree aside instantly and reclaims the
  disk in a background process — it returns at once even when you nuke ten heavy
  trees. Same safety as `git worktree remove`: it refuses a dirty or locked
  worktree (and anything git can't vouch for) unless you pass `-f`. No target → a
  multi-select picker.
- **`nt done [target]`** → remove the worktree **and** delete its local branch in
  one move. Safe by default (`git branch -d` refuses an unmerged branch and keeps
  it, telling you so); `nt done -f` forces both. Detached worktree? It just removes
  it (no branch to delete).
- **`nt prune`** → fetch and prune the configured remote, drop stale worktree
  entries, sweep up the empty `team/`-style parent dirs git leaves behind, reap
  any leftover background-delete trash, then — if any local branches have a
  **gone upstream** (merged & deleted on the remote) — offer to clean them. In a
  non-interactive shell it only *lists* the gone branches — it never deletes
  without you there.

### A nicer `nt ls`

`nt ls` (and bare `nt`) show a dirty marker and ahead/behind vs upstream:

```
  main             =      ~/Developer/acme   (main)
* fix-login        ↑2     ~/Developer/acme.worktrees/fix-login
  team/issue-123   ↓1     ~/Developer/acme.worktrees/team/issue-123

* = uncommitted changes
```

`*` = uncommitted changes, `↑n`/`↓n` = ahead/behind the upstream, `=` = in sync,
`-` = no upstream, `gone` = upstream deleted, `?` = the dir is gone (a `nt prune`
candidate). The main checkout always lists first, columns are aligned, and color
is used only on a real terminal (`NO_COLOR` is honored). In a non-UTF-8 locale the
arrows degrade to `^n`/`vn` so the columns still line up.

### Tab completion

Completion is generated by the binary and works across zsh/bash/fish:

- `nt <tab>` → subcommands plus existing branch names
- `nt cd <tab>` → branches that currently have a worktree
- `nt rm <tab>` / `nt done <tab>` → every worktree except the main checkout (by
  branch name, or full path for detached ones), plus `-f`
- `nt <branch> <tab>` → branches, to pick a base

## Notes & knobs

- New branches are created `--no-track`, so a stray `git push`/`git pull` won't
  accidentally nuke `main`. Push with `-u` when you're ready.
- `NT_NO_FETCH=1 nt …` skips the network fetch (offline, or just impatient).
- `NT_ROOT=/path nt …` overrides where worktrees live (default `<repo>.worktrees`).
- `NT_REMOTE=upstream nt …` fetches/tracks against a remote other than `origin`.
- `nt rm`/`nt done` take a branch name, a full worktree path, or a unique trailing
  path segment. `nt rm` takes several at once (`-f` may go anywhere) — it resolves
  the whole list up front and removes **nothing** unless *every* target checks out
  (an ambiguous tail is refused with the matches listed; naming the main checkout
  aborts the batch). It flat-out refuses to remove the main checkout, and if you
  `rm`/`done` the worktree you're standing in, it steps you back home first.
- The fast deferred delete only handles worktrees `nt` can confirm are simple
  (clean, unlocked, no submodule); a dirty, **locked**, or submodule-containing
  worktree is handed to `git worktree remove` itself, so it behaves exactly as git
  does. Nested worktrees must be named in the same `nt rm` command and are removed
  before their parent. If any child is omitted or its removal fails, the parent
  is preserved. `nt prune` reaps any `.nt-trash-*` a killed delete left behind
  (handy after a reboot mid-delete).
- The interactive pickers are built in — **no `fzf` dependency**. In a
  non-interactive shell, just pass an explicit branch/target.

## Tests

```sh
go test ./...            # unit + end-to-end (testscript) — no shell needed
zsh test/parity/run.zsh  # behavioral parity oracle: drives the binary through a real zsh shim
test/shell-integration.sh # real bash/zsh/fish hooks; optionally pass a packaged nt binary
```

The parity suite is the behavioral spec: ~100 assertions that exercise the binary
through the actual `nt init zsh` cd-shim, so "works the same way" is *proven*, not
claimed. CI runs the parity suite on Linux and exercises the bash, zsh, and fish hooks.

MIT.
